package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"gpu-optimizer/master/models"
)

const maxPendingTasksPerWorker = 20

type Manager struct {
	workers         map[string]*models.WorkerInfo
	mu              sync.RWMutex
	currentTask     *models.Task
	stopCondition   models.StopCondition
	stopTimer       *time.Timer
	currentTaskID   string
	isRunning       bool
	startTime       *time.Time
	lastElapsed     float64 // хранит время при остановке, чтобы не обнулялось
	totalTasks      int
	completedTasks  int
	bestValue       float64
	bestX           []float64
	totalIterations int64
	// pendingTasks stores task IDs currently sent to each worker and not yet completed.
	pendingTasks map[string][]string
	// sequential counter for task IDs
	taskCounter int64
}

func NewManager() *Manager {
	return &Manager{
		workers:      make(map[string]*models.WorkerInfo),
		bestValue:    1e18,
		pendingTasks: make(map[string][]string),
	}
}

func (m *Manager) RegisterWorker(info *models.WorkerInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info.LastSeen = time.Now()
	m.workers[info.ID] = info
	fmt.Printf("📌 Воркер зарегистрирован: %s (%s)\n", info.ID, info.GPUName)
}

func (m *Manager) GetWorkers() []models.WorkerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]models.WorkerInfo, 0, len(m.workers))
	for _, w := range m.workers {
		result = append(result, *w)
	}
	return result
}

func (m *Manager) StartComputation(task *models.Task, workerCount int, stopCond models.StopCondition) {
	m.mu.Lock()
	m.isRunning = true
	m.currentTaskID = task.BatchID
	m.currentTask = task
	m.stopCondition = stopCond
	m.taskCounter = 0
	// очистим очередь pending, чтобы не мешало при повторных запусках
	m.pendingTasks = make(map[string][]string)
	if m.stopTimer != nil {
		m.stopTimer.Stop()
	}
	if stopCond.Type == "time" && stopCond.Duration > 0 {
		m.stopTimer = time.AfterFunc(time.Duration(stopCond.Duration)*time.Second, func() {
			m.StopComputation()
		})
	}
	now := time.Now()
	m.startTime = &now
	m.lastElapsed = 0
	m.totalTasks = 0
	m.completedTasks = 0
	m.bestValue = 1e18
	m.bestX = nil
	m.totalIterations = 0
	m.mu.Unlock()

	// Заполняем очередь на каждом воркере (до maxPendingTasksPerWorker)
	workers := m.GetWorkers()
	for _, w := range workers {
		go m.fillWorkerQueue(w.ID)
	}
}

func (m *Manager) StopComputation() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}
	m.isRunning = false
	if m.stopTimer != nil {
		m.stopTimer.Stop()
	}
	// Запомним, сколько прошло времени до остановки
	if m.startTime != nil {
		m.lastElapsed = time.Since(*m.startTime).Seconds()
	}
	m.startTime = nil
	// Очистим внутренние очереди задач, чтобы не мешали при следующем запуске
	m.pendingTasks = make(map[string][]string)

	// Скопируем список воркеров, чтобы не держать лок на время сетевых вызовов
	workers := make([]models.WorkerInfo, 0, len(m.workers))
	for _, w := range m.workers {
		workers = append(workers, *w)
	}
	m.mu.Unlock()

	for _, w := range workers {
		go m.SendStopToWorker(w)
	}
}

func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

func (m *Manager) GetProgress() models.ProgressUpdate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	elapsed := m.lastElapsed
	if m.startTime != nil {
		elapsed = time.Since(*m.startTime).Seconds()
	}

	return models.ProgressUpdate{
		Timestamp:       time.Now(),
		TotalTasks:      m.totalTasks,
		CompletedTasks:  m.completedTasks,
		BestValue:       m.bestValue,
		BestX:           m.bestX,
		ElapsedTime:     elapsed,
		TotalIterations: m.totalIterations,
		IsRunning:       m.isRunning,
	}
}

func (m *Manager) shouldStopLocked() bool {
	if !m.isRunning {
		return true
	}

	sc := m.stopCondition
	if sc.Type == "time" && sc.Duration > 0 && m.startTime != nil {
		if time.Since(*m.startTime).Seconds() >= float64(sc.Duration) {
			return true
		}
	}
	if sc.Type == "iterations" && sc.Iterations > 0 {
		if m.totalIterations >= sc.Iterations {
			return true
		}
	}
	return false
}

func (m *Manager) ProcessResult(result *models.TaskResult) {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}

	m.completedTasks++
	m.totalIterations += int64(result.Iterations)

	// Убираем задачу из очереди (по ID) — освободили слот для следующей
	pending, ok := m.pendingTasks[result.WorkerID]
	if ok {
		for i, id := range pending {
			if id == result.TaskID {
				m.pendingTasks[result.WorkerID] = append(pending[:i], pending[i+1:]...)
				break
			}
		}
	}

	if result.BestValue < m.bestValue {
		m.bestValue = result.BestValue
		m.bestX = result.BestX
	}

	stop := m.shouldStopLocked()
	if stop {
		m.isRunning = false
		if m.stopTimer != nil {
			m.stopTimer.Stop()
		}
	}
	m.mu.Unlock()

	if stop {
		return
	}

	// Заполняем очередь для этого воркера заново (асинхронно)
	go m.fillWorkerQueue(result.WorkerID)
}

func (m *Manager) fillWorkerQueue(workerID string) {
	for {
		m.mu.Lock()
		if !m.isRunning {
			m.mu.Unlock()
			return
		}

		workerInfo, ok := m.workers[workerID]
		if !ok {
			m.mu.Unlock()
			return
		}

		pending := m.pendingTasks[workerID]
		if len(pending) >= maxPendingTasksPerWorker {
			m.mu.Unlock()
			return
		}

		if m.currentTask == nil {
			m.mu.Unlock()
			return
		}

		// Формируем новую задачу (копия) с уникальным ID/PacketID и seed/threads, специфичными для воркера
		task := *m.currentTask
		task.ID = fmt.Sprintf("%d", m.taskCounter)
		task.PacketID = m.taskCounter
		// batch_id — общий для всего запуска, чтобы отличать разные запуски
		task.BatchID = m.currentTaskID
		// per-worker параметры
		// Используем 31-битный seed, чтобы не превышать ограничения C long на стороне Python
		task.Seed = int(rand.Int31())
		task.ThreadCount = workerInfo.ThreadCount
		m.taskCounter++
		m.pendingTasks[workerID] = append(pending, task.ID)
		m.totalTasks++
		m.mu.Unlock()

		if err := m.SendTaskToWorker(*workerInfo, &task); err != nil {
			fmt.Printf("❌ Не удалось отправить задачу воркеру %s (%s): %v\n", workerInfo.ID, workerInfo.Address, err)
			// если не удалось отправить, возвращаем слот
			m.mu.Lock()
			pending = m.pendingTasks[workerID]
			for i, id := range pending {
				if id == task.ID {
					m.pendingTasks[workerID] = append(pending[:i], pending[i+1:]...)
					break
				}
			}
			m.totalTasks--
			m.mu.Unlock()
			return
		}
	}
}

func (m *Manager) SendTaskToWorker(worker models.WorkerInfo, task *models.Task) error {
	addr := worker.Address
	if addr == "" {
		addr = worker.ID
	}
	if !strings.Contains(addr, "://") {
		// assume default port 5000 if not provided
		if !strings.Contains(addr, ":") {
			addr = fmt.Sprintf("%s:5000", addr)
		}
		addr = "http://" + addr
	}
	url := fmt.Sprintf("%s/task", strings.TrimRight(addr, "/"))
	payload, _ := json.Marshal(task)
	fmt.Printf("📤 Отправляю задачу на %s (id=%s)\n", url, task.ID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("worker returned status %d", resp.StatusCode)
	}
	return nil
}

func (m *Manager) SendStopToWorker(worker models.WorkerInfo) error {
	addr := worker.Address
	if addr == "" {
		addr = worker.ID
	}
	if !strings.Contains(addr, "://") {
		// assume default port 5000 if not provided
		if !strings.Contains(addr, ":") {
			addr = fmt.Sprintf("%s:5000", addr)
		}
		addr = "http://" + addr
	}
	url := fmt.Sprintf("%s/stop", strings.TrimRight(addr, "/"))
	payload := []byte("{}")
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("worker stop returned status %d", resp.StatusCode)
	}
	return nil
}

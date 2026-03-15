package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gpu-optimizer/master/models"
)

type Manager struct {
	workers         map[string]*models.WorkerInfo
	mu              sync.RWMutex
	currentTask     *models.Task
	stopCondition   models.StopCondition
	stopTimer       *time.Timer
	currentTaskID   string
	isRunning       bool
	startTime       *time.Time
	totalTasks      int
	completedTasks  int
	bestValue       float64
	bestX           []float64
	totalIterations int64
}

func NewManager() *Manager {
	return &Manager{
		workers:   make(map[string]*models.WorkerInfo),
		bestValue: 1e18,
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
	m.currentTaskID = task.ID
	m.currentTask = task
	m.stopCondition = stopCond
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
	m.totalTasks = 0
	m.completedTasks = 0
	m.bestValue = 1e18
	m.bestX = nil
	m.totalIterations = 0
	m.mu.Unlock()

	// Отправляем начальные задачи каждому воркеру
	workers := m.GetWorkers()
	for _, w := range workers {
		for i := 0; i < 20; i++ {
			if err := m.SendTaskToWorker(w, task); err != nil {
				fmt.Printf("❌ Не удалось отправить задачу воркеру %s (%s): %v\n", w.ID, w.Address, err)
				continue
			}
			m.mu.Lock()
			m.totalTasks++
			m.mu.Unlock()
		}
	}
}

func (m *Manager) StopComputation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isRunning = false
	if m.stopTimer != nil {
		m.stopTimer.Stop()
	}
	m.startTime = nil
}

func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

func (m *Manager) GetProgress() models.ProgressUpdate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	elapsed := 0.0
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

	// Отправляем новую задачу воркеру
	m.mu.RLock()
	workerInfo, ok := m.workers[result.WorkerID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	if err := m.SendTaskToWorker(*workerInfo, m.currentTask); err != nil {
		fmt.Printf("❌ Ошибка отправки следующей задачи воркеру %s: %v\n", workerInfo.ID, err)
		return
	}
	m.mu.Lock()
	m.totalTasks++
	m.mu.Unlock()
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
	fmt.Printf("📤 Отправляю задачу на %s\n", url)
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

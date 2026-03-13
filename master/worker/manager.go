package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gpu-optimizer/master/models"
)

type Manager struct {
	workers         map[string]*models.WorkerInfo
	mu              sync.RWMutex
	taskQueue       chan *models.Task
	resultsChan     chan *models.TaskResult
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
		workers:     make(map[string]*models.WorkerInfo),
		taskQueue:   make(chan *models.Task, 1000),
		resultsChan: make(chan *models.TaskResult, 1000),
		bestValue:   1e18,
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

func (m *Manager) StartComputation(task *models.Task, workerCount int) {
	m.mu.Lock()
	m.isRunning = true
	m.currentTaskID = task.ID
	now := time.Now()
	m.startTime = &now
	m.totalTasks = workerCount * 20 // 20 задач на воркер при старте
	m.completedTasks = 0
	m.bestValue = 1e18
	m.bestX = nil
	m.totalIterations = 0
	m.mu.Unlock()

	// Отправляем начальные задачи каждому воркеру
	m.mu.RLock()
	workers := make([]*models.WorkerInfo, 0, len(m.workers))
	for _, w := range m.workers {
		workers = append(workers, w)
	}
	m.mu.RUnlock()

	for range workers {
		for i := 0; i < 20; i++ {
			select {
			case m.taskQueue <- task:
			default:
			}
		}
	}
}

func (m *Manager) StopComputation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isRunning = false
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

func (m *Manager) ProcessResult(result *models.TaskResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.completedTasks++
	m.totalIterations += int64(result.Iterations)

	if result.BestValue < m.bestValue {
		m.bestValue = result.BestValue
		m.bestX = result.BestX
	}

	// Отправляем новую задачу воркеру (если ещё есть что отправлять)
	// Это упрощённая логика - в реальности нужно отслеживать по воркерам
}

func (m *Manager) SendTaskToWorker(workerAddr string, task *models.Task) error {
	payload, _ := json.Marshal(task)
	resp, err := http.Post(fmt.Sprintf("http://%s/task", workerAddr),
		"application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("worker returned status %d", resp.StatusCode)
	}
	return nil
}

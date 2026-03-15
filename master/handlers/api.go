package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gpu-optimizer/master/models"
	"gpu-optimizer/master/worker"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	manager    *worker.Manager
	wsHub      *WebSocketHub
	currentReq *models.TaskRequest
	mu         sync.RWMutex
}

func NewAPIHandler(manager *worker.Manager, wsHub *WebSocketHub) *APIHandler {
	return &APIHandler{
		manager: manager,
		wsHub:   wsHub,
	}
}

func (h *APIHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/worker/register", h.RegisterWorker)
		api.POST("/start", h.StartComputation)
		api.POST("/stop", h.StopComputation)
		api.GET("/status", h.GetStatus)
		api.GET("/workers", h.GetWorkers)
		api.POST("/task/result", h.ReceiveTaskResult)
	}
}

func (h *APIHandler) RegisterWorker(c *gin.Context) {
	var info models.WorkerInfo
	if err := c.ShouldBindJSON(&info); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if info.Address == "" {
		info.Address = c.ClientIP()
	}
	// Если адрес не содержит порт, добавляем стандартный порт 5000
	if !strings.Contains(info.Address, ":") {
		info.Address = info.Address + ":5000"
	}
	h.manager.RegisterWorker(&info)
	c.JSON(200, gin.H{"success": true, "worker_id": info.ID})
}

func (h *APIHandler) StartComputation(c *gin.Context) {
	var req models.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	h.currentReq = &req
	h.mu.Unlock()

	task := &models.Task{
		ID:            time.Now().Format("20060102150405"),
		Formula:       req.Formula,
		Mode:          req.Mode,
		Target:        req.Target,
		VariableCount: req.VariableCount,
		RangeMin:      req.RangeMin,
		RangeMax:      req.RangeMax,
		Iterations:    100000,
		Seed:          int(time.Now().Unix()),
		ThreadCount:   1024,
	}

	workerCount := len(h.manager.GetWorkers())
	if workerCount == 0 {
		c.JSON(400, gin.H{"error": "no workers available"})
		return
	}

	h.manager.StartComputation(task, workerCount, req.StopCondition)
	h.wsHub.StartBroadcasting()

	c.JSON(200, models.StartResponse{
		Success: true,
		Message: "Computation started",
		TaskID:  task.ID,
	})
}

func (h *APIHandler) StopComputation(c *gin.Context) {
	h.manager.StopComputation()
	h.wsHub.StopBroadcasting()
	c.JSON(200, models.StopResponse{
		Success: true,
		Message: "Computation stopped",
	})
}

func (h *APIHandler) GetStatus(c *gin.Context) {
	progress := h.manager.GetProgress()
	workers := h.manager.GetWorkers()

	var startTime *time.Time
	h.mu.RLock()
	if h.currentReq != nil && h.manager.IsRunning() {
		ts := h.manager.GetProgress().Timestamp
		startTime = &ts // ← берём адрес значения
	}
	h.mu.RUnlock()

	c.JSON(200, models.StatusResponse{
		IsRunning: h.manager.IsRunning(),
		Progress:  progress,
		Workers:   workers,
		StartTime: startTime,
	})
}

func (h *APIHandler) GetWorkers(c *gin.Context) {
	workers := h.manager.GetWorkers()
	c.JSON(200, workers)
}

func (h *APIHandler) ReceiveTaskResult(c *gin.Context) {
	var result models.TaskResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("📥 Получен результат от воркера %s: best_value=%.6f, iterations=%d\n", result.WorkerID, result.BestValue, result.Iterations)
	h.manager.ProcessResult(&result)
	c.JSON(200, gin.H{"success": true})
}

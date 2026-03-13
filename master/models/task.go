package models

import "time"

type TaskMode string

const (
	ModeMinimize   TaskMode = "minimize"
	ModeFindTarget TaskMode = "find_target"
)

type StopCondition struct {
	Type       string `json:"type"`       // "time" or "iterations"
	Duration   int    `json:"duration"`   // seconds (if type=time)
	Iterations int64  `json:"iterations"` // total iterations (if type=iterations)
}

type TaskRequest struct {
	Formula       string        `json:"formula"`
	Mode          TaskMode      `json:"mode"`
	Target        float64       `json:"target,omitempty"`
	VariableCount int           `json:"variable_count"`
	RangeMin      float64       `json:"range_min"`
	RangeMax      float64       `json:"range_max"`
	StopCondition StopCondition `json:"stop_condition"`
}

type Task struct {
	ID            string   `json:"id"`
	Formula       string   `json:"formula"`
	Mode          TaskMode `json:"mode"`
	Target        float64  `json:"target,omitempty"`
	VariableCount int      `json:"variable_count"`
	RangeMin      float64  `json:"range_min"`
	RangeMax      float64  `json:"range_max"`
	Iterations    int      `json:"iterations"` // per thread
	Seed          int      `json:"seed"`
	ThreadCount   int      `json:"thread_count"`
}

type TaskResult struct {
	TaskID     string    `json:"task_id"`
	BestValue  float64   `json:"best_value"`
	BestX      []float64 `json:"best_x"`
	Iterations int       `json:"iterations"`
	TimeSpent  float64   `json:"time_spent"` // seconds
	WorkerID   string    `json:"worker_id"`
}

type WorkerInfo struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	GPUName     string    `json:"gpu_name"`
	ThreadCount int       `json:"thread_count"`
	LastSeen    time.Time `json:"last_seen"`
	TasksDone   int       `json:"tasks_done"`
}

type ProgressUpdate struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalTasks      int       `json:"total_tasks"`
	CompletedTasks  int       `json:"completed_tasks"`
	BestValue       float64   `json:"best_value"`
	BestX           []float64 `json:"best_x"`
	ElapsedTime     float64   `json:"elapsed_time"`
	TotalIterations int64     `json:"total_iterations"`
	IsRunning       bool      `json:"is_running"`
}

type StartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TaskID  string `json:"task_id,omitempty"`
}

type StopResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type StatusResponse struct {
	IsRunning bool           `json:"is_running"`
	Progress  ProgressUpdate `json:"progress"`
	Workers   []WorkerInfo   `json:"workers"`
	StartTime *time.Time     `json:"start_time,omitempty"`
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Task represents a user-initiated task tracked by the system.
type Task struct {
	ID        string `json:"id"`
	Goal      string `json:"goal"`
	Status    string `json:"status"` // pending, running, done, failed
	Steps     int    `json:"steps"`
	StartedAt string `json:"started_at"`
	Duration  string `json:"duration"`
	CreatedAt int64  `json:"created_at"`
}

// TaskManager provides thread-safe in-memory task storage.
type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	order []string // insertion order
}

// NewTaskManager creates a new task manager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
	}
}

// List returns all tasks in insertion order.
func (tm *TaskManager) List() []Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	out := make([]Task, 0, len(tm.order))
	for _, id := range tm.order {
		if t, ok := tm.tasks[id]; ok {
			out = append(out, *t)
		}
	}
	return out
}

// Get returns a task by ID or nil.
func (tm *TaskManager) Get(id string) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.tasks[id]
	if !ok {
		return nil
	}
	cp := *t
	return &cp
}

// Create adds a new task and returns it.
func (tm *TaskManager) Create(goal string) Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	now := time.Now()
	t := &Task{
		ID:        id,
		Goal:      goal,
		Status:    "pending",
		Steps:     0,
		StartedAt: now.Format("15:04:05"),
		Duration:  "0s",
		CreatedAt: now.Unix(),
	}
	tm.tasks[id] = t
	tm.order = append(tm.order, id)
	return *t
}

// Update modifies an existing task. Returns the updated task or nil if not found.
func (tm *TaskManager) Update(id string, status string, steps int, duration string) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tasks[id]
	if !ok {
		return nil
	}
	if status != "" {
		t.Status = status
	}
	if steps >= 0 {
		t.Steps = steps
	}
	if duration != "" {
		t.Duration = duration
	}
	cp := *t
	return &cp
}

// Delete removes a task. Returns true if found.
func (tm *TaskManager) Delete(id string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, ok := tm.tasks[id]; !ok {
		return false
	}
	delete(tm.tasks, id)
	// Remove from order slice
	for i, oid := range tm.order {
		if oid == id {
			tm.order = append(tm.order[:i], tm.order[i+1:]...)
			break
		}
	}
	return true
}

// RegisterTaskRoutes adds task CRUD endpoints to the mux.
func (s *Server) RegisterTaskRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			// GET /api/tasks — list all tasks
			json.NewEncoder(w).Encode(s.taskMgr.List())

		case http.MethodPost:
			// POST /api/tasks — create a task
			var req struct {
				Goal string `json:"goal"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Goal == "" {
				http.Error(w, `{"error":"goal is required"}`, http.StatusBadRequest)
				return
			}
			task := s.taskMgr.Create(req.Goal)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(task)

		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// /api/tasks/ handles individual task operations (PUT, DELETE)
	mux.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Extract task ID from URL path: /api/tasks/{id}
		id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
		if id == "" {
			http.Error(w, `{"error":"task id required"}`, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPut:
			// PUT /api/tasks/{id} — update task
			var req struct {
				Status   string `json:"status"`
				Steps    int    `json:"steps"`
				Duration string `json:"duration"`
			}
			req.Steps = -1 // sentinel: don't update if not provided
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			task := s.taskMgr.Update(id, req.Status, req.Steps, req.Duration)
			if task == nil {
				http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(task)

		case http.MethodDelete:
			// DELETE /api/tasks/{id} — remove task
			if !s.taskMgr.Delete(id) {
				http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
}

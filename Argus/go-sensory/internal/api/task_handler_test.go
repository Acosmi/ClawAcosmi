package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskHandler_CRUD(t *testing.T) {
	mgr := NewTaskManager()
	srv := &Server{taskMgr: mgr}

	mux := http.NewServeMux()
	srv.RegisterTaskRoutes(mux)

	// 1. GET /api/tasks — initially empty
	t.Run("ListEmpty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var tasks []Task
		json.NewDecoder(w.Body).Decode(&tasks)
		if len(tasks) != 0 {
			t.Fatalf("expected 0 tasks, got %d", len(tasks))
		}
	})

	// 2. POST /api/tasks — create
	var createdID string
	t.Run("Create", func(t *testing.T) {
		body := bytes.NewBufferString(`{"goal":"test task"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w.Code)
		}
		var task Task
		json.NewDecoder(w.Body).Decode(&task)
		if task.Goal != "test task" {
			t.Fatalf("expected goal 'test task', got %q", task.Goal)
		}
		if task.Status != "pending" {
			t.Fatalf("expected status 'pending', got %q", task.Status)
		}
		if task.ID == "" {
			t.Fatal("expected non-empty ID")
		}
		createdID = task.ID
	})

	// 3. GET /api/tasks — should have 1 task
	t.Run("ListOne", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var tasks []Task
		json.NewDecoder(w.Body).Decode(&tasks)
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
	})

	// 4. PUT /api/tasks/{id} — update
	t.Run("Update", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"done","steps":3,"duration":"5s"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/tasks/"+createdID, body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var task Task
		json.NewDecoder(w.Body).Decode(&task)
		if task.Status != "done" {
			t.Fatalf("expected status 'done', got %q", task.Status)
		}
		if task.Steps != 3 {
			t.Fatalf("expected 3 steps, got %d", task.Steps)
		}
		if task.Duration != "5s" {
			t.Fatalf("expected duration '5s', got %q", task.Duration)
		}
	})

	// 5. PUT /api/tasks/nonexistent — 404
	t.Run("UpdateNotFound", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"done"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/tasks/not-exists", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	// 6. DELETE /api/tasks/{id} — remove
	t.Run("Delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/tasks/"+createdID, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	// 7. GET /api/tasks — back to empty
	t.Run("ListAfterDelete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var tasks []Task
		json.NewDecoder(w.Body).Decode(&tasks)
		if len(tasks) != 0 {
			t.Fatalf("expected 0 tasks after delete, got %d", len(tasks))
		}
	})

	// 8. POST /api/tasks — invalid (empty goal)
	t.Run("CreateInvalid", func(t *testing.T) {
		body := bytes.NewBufferString(`{"goal":""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

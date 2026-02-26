package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/input"
	"Argus-compound/go-sensory/internal/pipeline"
	"Argus-compound/go-sensory/internal/vlm"

	"golang.org/x/net/websocket"
)

// Server provides WebSocket and HTTP endpoints for the sensory system.
type Server struct {
	capturer          capture.Capturer
	inputCtrl         input.InputController
	port              int
	hub               *Hub
	vlmRouter         *vlm.Router
	scaler            *imaging.Scaler
	guardrails        *input.ActionGuardrails
	pipeline          *pipeline.Pipeline
	keyframeExtractor *pipeline.KeyframeExtractor
	monitor           *pipeline.Monitor
	taskMgr           *TaskManager
}

// NewServer creates a new API server.
func NewServer(capturer capture.Capturer, inputCtrl input.InputController, port int, vlmRouter *vlm.Router, scaler *imaging.Scaler) *Server {
	return &Server{
		capturer:  capturer,
		inputCtrl: inputCtrl,
		port:      port,
		hub:       NewHub(capturer),
		vlmRouter: vlmRouter,
		scaler:    scaler,
		taskMgr:   NewTaskManager(),
	}
}

// corsMiddleware wraps a handler with CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Start begins the HTTP/WebSocket server.
func (s *Server) Start() error {
	startTime := time.Now()

	mux := http.NewServeMux()

	// Embedded test dashboard at /
	RegisterDashboard(mux)

	// VLM routes (/v1/chat/completions, /api/vlm/health)
	if s.vlmRouter != nil {
		s.vlmRouter.RegisterRoutes(mux)
	}

	// WebSocket handler for frame streaming (JSON + base64)
	mux.Handle("/ws/frames", websocket.Handler(func(ws *websocket.Conn) {
		s.handleFrameStream(ws)
	}))

	// WebSocket handler for binary frame streaming (compact header + raw JPEG)
	mux.Handle("/ws/frames/binary", websocket.Handler(func(ws *websocket.Conn) {
		s.handleBinaryFrameStream(ws)
	}))

	// WebSocket handler for commands
	mux.Handle("/ws/control", websocket.Handler(func(ws *websocket.Conn) {
		s.handleControl(ws)
	}))

	// Health check
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		s.handleStatus(w, startTime)
	})

	// Display info
	mux.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		s.handleDisplayInfo(w)
	})

	// Window management (list, exclude)
	registerWindowRoutes(mux, s.capturer)

	// Single frame capture
	mux.HandleFunc("/api/capture/once", func(w http.ResponseWriter, r *http.Request) {
		s.handleCaptureOnce(w)
	})

	// Action REST endpoints (P3-A: bridge for Python ReAct loop)
	s.RegisterActionRoutes(mux)

	// Pipeline REST endpoints (P5-A)
	s.RegisterPipelineRoutes(mux)

	// Analysis + Health endpoints (P5-C)
	s.RegisterAnalysisRoutes(mux)
	s.RegisterHealthRoutes(mux)

	// VLM Monitor endpoint
	if s.monitor != nil {
		s.RegisterMonitorRoutes(mux)
	}

	// Task management endpoints
	s.RegisterTaskRoutes(mux)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("API server starting on %s", addr)
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

// handleStatus returns system health information.
func (s *Server) handleStatus(w http.ResponseWriter, startTime time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var frameNo uint64
	if f := s.capturer.LatestFrame(); f != nil {
		frameNo = f.FrameNo
	}

	// 通过类型检测实际后端
	backend := "cg"
	switch s.capturer.(type) {
	case *capture.SCKCapturer:
		backend = "sck"
	case *capture.RustCapturer:
		backend = "rust"
	}

	resp := StatusResponse{
		Status:    "ok",
		Backend:   backend,
		Uptime:    time.Since(startTime).String(),
		Capturing: s.capturer.IsRunning(),
		Display:   s.capturer.DisplayInfo(),
		FrameNo:   frameNo,
		Clients:   s.hub.ClientCount(),
	}

	json.NewEncoder(w).Encode(resp)
}

// handleDisplayInfo returns display metadata.
func (s *Server) handleDisplayInfo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.capturer.DisplayInfo())
}

// handleCaptureOnce captures a single frame and returns it as JPEG.
func (s *Server) handleCaptureOnce(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	frame := s.capturer.LatestFrame()
	if frame == nil {
		http.Error(w, "no frame available", http.StatusServiceUnavailable)
		return
	}

	jpegData, err := frameToJPEG(frame, 80)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(jpegData)
}

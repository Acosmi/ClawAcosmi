package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ProcManager manages child processes (sensory-server + Next.js).
type ProcManager struct {
	mu      sync.Mutex
	baseDir string
	sensory *exec.Cmd
	nextjs  *exec.Cmd
	running bool
}

// NewProcManager creates a new process manager.
func NewProcManager() (*ProcManager, error) {
	// Detect base directory (project root)
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	baseDir := findProjectRoot(exePath)
	if baseDir == "" {
		return nil, fmt.Errorf("cannot find project root from %s", exePath)
	}
	return &ProcManager{baseDir: baseDir}, nil
}

// findProjectRoot locates the project root containing go-sensory/.
func findProjectRoot(from string) string {
	dir := filepath.Dir(from)

	// If inside .app bundle (Contents/MacOS/exe), start from .app's parent
	if filepath.Base(dir) == "MacOS" {
		contents := filepath.Dir(dir)
		if filepath.Base(contents) == "Contents" {
			appDir := filepath.Dir(contents) // .app dir
			dir = filepath.Dir(appDir)       // parent of .app
		}
	}

	// Walk up looking for go-sensory/
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go-sensory")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// StartAll starts all child processes.
func (pm *ProcManager) StartAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return nil
	}

	goSensoryDir := filepath.Join(pm.baseDir, "go-sensory")
	binPath := filepath.Join(goSensoryDir, "argus-sensory")

	// Build sensory-server binary first
	log.Println("[ProcMgr] building sensory-server...")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	build.Dir = goSensoryDir
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build sensory-server: %w\n%s", err, out)
	}
	log.Println("[ProcMgr] sensory-server built successfully")

	// Start compiled binary
	pm.sensory = exec.Command(binPath, "--open-browser=false")
	pm.sensory.Dir = goSensoryDir
	pm.sensory.Stdout = os.Stdout
	pm.sensory.Stderr = os.Stderr
	if err := pm.sensory.Start(); err != nil {
		return fmt.Errorf("start sensory-server: %w", err)
	}
	log.Println("[ProcMgr] sensory-server started")

	// Start Next.js
	pm.nextjs = exec.Command("npm", "start")
	pm.nextjs.Dir = filepath.Join(pm.baseDir, "web-console")
	pm.nextjs.Stdout = os.Stdout
	pm.nextjs.Stderr = os.Stderr
	if err := pm.nextjs.Start(); err != nil {
		return fmt.Errorf("start next.js: %w", err)
	}
	log.Println("[ProcMgr] Next.js started")

	pm.running = true
	return nil
}

// WaitForPorts blocks until both ports are ready.
func (pm *ProcManager) WaitForPorts(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ports := []int{8090, 3090}

	for _, port := range ports {
		for time.Now().Before(deadline) {
			conn, err := net.DialTimeout("tcp",
				fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
			if err == nil {
				conn.Close()
				log.Printf("[ProcMgr] port %d ready", port)
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

// StopAll gracefully stops all child processes.
func (pm *ProcManager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return
	}

	stopProc := func(name string, cmd *exec.Cmd) {
		if cmd == nil || cmd.Process == nil {
			return
		}
		log.Printf("[ProcMgr] stopping %s (pid=%d)", name, cmd.Process.Pid)
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			log.Printf("[ProcMgr] force killing %s", name)
			_ = cmd.Process.Kill()
		}
	}

	stopProc("next.js", pm.nextjs)
	stopProc("sensory-server", pm.sensory)
	pm.running = false
	log.Println("[ProcMgr] all processes stopped")
}

// Status returns current status string.
func (pm *ProcManager) Status() string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return "stopped"
	}
	return "running"
}

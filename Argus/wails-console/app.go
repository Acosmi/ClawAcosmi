package main

import (
	"context"
	"fmt"
	"net"
	"time"
)

// App is the Wails application backend.
type App struct {
	ctx     context.Context
	procMgr *ProcManager
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called when the Wails app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the Wails app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.procMgr != nil {
		a.procMgr.StopAll()
	}
}

// ServiceStatus represents the status of backend services.
type ServiceStatus struct {
	SensoryRunning bool   `json:"sensoryRunning"`
	NextjsRunning  bool   `json:"nextjsRunning"`
	SensoryPort    int    `json:"sensoryPort"`
	NextjsPort     int    `json:"nextjsPort"`
	Message        string `json:"message"`
}

// CheckServices checks if required services are running.
func (a *App) CheckServices() ServiceStatus {
	sensory := checkPort(8090)
	nextjs := checkPort(3090)

	msg := ""
	if sensory && nextjs {
		msg = "所有服务已就绪"
	} else if !sensory && !nextjs {
		msg = "sensory-server 和 Next.js 均未启动"
	} else if !sensory {
		msg = "sensory-server 未启动"
	} else {
		msg = "Next.js 前端未启动"
	}

	return ServiceStatus{
		SensoryRunning: sensory,
		NextjsRunning:  nextjs,
		SensoryPort:    8090,
		NextjsPort:     3090,
		Message:        msg,
	}
}

// StartServices launches all required services.
func (a *App) StartServices() string {
	if a.procMgr != nil && a.procMgr.Status() == "running" {
		return "服务已在运行中"
	}

	mgr, err := NewProcManager()
	if err != nil {
		return fmt.Sprintf("初始化失败: %v", err)
	}
	a.procMgr = mgr

	if err := a.procMgr.StartAll(); err != nil {
		return fmt.Sprintf("启动失败: %v", err)
	}

	// Wait for ports
	if err := a.procMgr.WaitForPorts(30 * time.Second); err != nil {
		return fmt.Sprintf("等待端口超时: %v", err)
	}

	return "ok"
}

// GetStatus returns service status info.
func (a *App) GetStatus() string {
	if a.procMgr == nil {
		return "not initialized"
	}
	return a.procMgr.Status()
}

// GetVersion returns version info.
func (a *App) GetVersion() string {
	return "Argus Console v1.0.0"
}

func checkPort(port int) bool {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

package acp

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------- AcpSessionStore 接口 ----------

// AcpSessionStore ACP 会话存储接口（对应 TS AcpSessionStore）。
type AcpSessionStore interface {
	// CreateSession 创建新会话。
	CreateSession(opts CreateSessionOpts) *AcpSession
	// GetSession 按会话 ID 获取。
	GetSession(sessionID string) *AcpSession
	// GetSessionByKey 按 session key 获取。
	GetSessionByKey(sessionKey string) *AcpSession
	// GetSessionByRunId 按活跃运行 ID 获取。
	GetSessionByRunId(runID string) *AcpSession
	// SetActiveRun 为会话设置活跃运行。
	SetActiveRun(sessionID, runID string, cancel context.CancelFunc)
	// CancelActiveRun 取消会话的活跃运行。
	CancelActiveRun(sessionID string) bool
	// ClearAllSessionsForTest 清空所有数据（仅测试）。
	ClearAllSessionsForTest()
}

// CreateSessionOpts 创建会话选项。
type CreateSessionOpts struct {
	SessionKey string
	Cwd        string
}

// ---------- 内存实现 ----------

type inMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*AcpSession // sessionID → session
	keyIndex map[string]string      // sessionKey → sessionID
	runIndex map[string]string      // runID → sessionID
}

// NewInMemorySessionStore 创建内存会话存储。
// 对应 TS: createInMemorySessionStore()
func NewInMemorySessionStore() AcpSessionStore {
	return &inMemorySessionStore{
		sessions: make(map[string]*AcpSession),
		keyIndex: make(map[string]string),
		runIndex: make(map[string]string),
	}
}

func (s *inMemorySessionStore) CreateSession(opts CreateSessionOpts) *AcpSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := uuid.New().String()
	session := &AcpSession{
		SessionID:  sessionID,
		SessionKey: opts.SessionKey,
		Cwd:        opts.Cwd,
		CreatedAt:  time.Now().UnixMilli(),
	}
	s.sessions[sessionID] = session
	s.keyIndex[opts.SessionKey] = sessionID
	return session
}

func (s *inMemorySessionStore) GetSession(sessionID string) *AcpSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

func (s *inMemorySessionStore) GetSessionByKey(sessionKey string) *AcpSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid, ok := s.keyIndex[sessionKey]
	if !ok {
		return nil
	}
	return s.sessions[sid]
}

func (s *inMemorySessionStore) GetSessionByRunId(runID string) *AcpSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid, ok := s.runIndex[runID]
	if !ok {
		return nil
	}
	return s.sessions[sid]
}

func (s *inMemorySessionStore) SetActiveRun(sessionID, runID string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	// 清除旧运行
	if session.ActiveRunID != "" {
		delete(s.runIndex, session.ActiveRunID)
	}
	session.ActiveRunID = runID
	session.CancelFunc = cancel
	s.runIndex[runID] = sessionID
}

func (s *inMemorySessionStore) CancelActiveRun(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.ActiveRunID == "" {
		return false
	}
	if session.CancelFunc != nil {
		session.CancelFunc()
	}
	delete(s.runIndex, session.ActiveRunID)
	session.ActiveRunID = ""
	session.CancelFunc = nil
	return true
}

func (s *inMemorySessionStore) ClearAllSessionsForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[string]*AcpSession)
	s.keyIndex = make(map[string]string)
	s.runIndex = make(map[string]string)
}

// DefaultSessionStore 默认包级单例。
var DefaultSessionStore = NewInMemorySessionStore()

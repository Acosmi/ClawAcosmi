package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// CdpResponse is a single CDP protocol response.
type CdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *CdpError       `json:"error,omitempty"`
}

// CdpError is a CDP protocol error.
type CdpError struct {
	Message string `json:"message"`
}

// CdpSendFn sends a CDP command and returns the result.
type CdpSendFn func(method string, params map[string]any) (json.RawMessage, error)

// IsLoopbackHost returns true if the host refers to the local machine.
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	switch h {
	case "localhost", "127.0.0.1", "0.0.0.0", "[::1]", "::1", "[::]", "::":
		return true
	}
	return false
}

// AppendCdpPath appends a path suffix to a CDP URL.
func AppendCdpPath(cdpURL, path string) (string, error) {
	parsed, err := url.Parse(cdpURL)
	if err != nil {
		return "", err
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	suffix := path
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	parsed.Path = basePath + suffix
	return parsed.String(), nil
}

// FetchJSON fetches a URL and decodes the JSON response.
func FetchJSON(ctx context.Context, rawURL string, result any, timeoutMs int) error {
	if timeoutMs <= 0 {
		timeoutMs = DefaultFetchTimeoutMs
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// FetchOK fetches a URL and returns an error if non-2xx.
func FetchOK(ctx context.Context, rawURL string, timeoutMs int) error {
	if timeoutMs <= 0 {
		timeoutMs = DefaultFetchTimeoutMs
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// cdpSender manages pending CDP requests over a WebSocket.
type cdpSender struct {
	conn    *websocket.Conn
	nextID  atomic.Int64
	pending sync.Map // map[int]chan cdpPendingResult
	done    chan struct{}
}

type cdpPendingResult struct {
	result json.RawMessage
	err    error
}

func newCdpSender(conn *websocket.Conn) *cdpSender {
	s := &cdpSender{
		conn: conn,
		done: make(chan struct{}),
	}
	s.nextID.Store(1)
	go s.readLoop()
	return s
}

func (s *cdpSender) readLoop() {
	defer close(s.done)
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			s.closeWithError(fmt.Errorf("CDP socket closed: %w", err))
			return
		}
		var resp CdpResponse
		if err := json.Unmarshal(msg, &resp); err != nil || resp.ID == 0 {
			continue
		}
		if ch, ok := s.pending.LoadAndDelete(resp.ID); ok {
			result := cdpPendingResult{}
			if resp.Error != nil && resp.Error.Message != "" {
				result.err = fmt.Errorf("%s", resp.Error.Message)
			} else {
				result.result = resp.Result
			}
			ch.(chan cdpPendingResult) <- result
		}
	}
}

func (s *cdpSender) send(method string, params map[string]any) (json.RawMessage, error) {
	id := int(s.nextID.Add(1) - 1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	ch := make(chan cdpPendingResult, 1)
	s.pending.Store(id, ch)

	if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.pending.Delete(id)
		return nil, err
	}

	select {
	case result := <-ch:
		return result.result, result.err
	case <-s.done:
		return nil, fmt.Errorf("CDP socket closed")
	}
}

func (s *cdpSender) closeWithError(err error) {
	s.pending.Range(func(key, value any) bool {
		ch := value.(chan cdpPendingResult)
		ch <- cdpPendingResult{err: err}
		s.pending.Delete(key)
		return true
	})
}

// WithCdpSocket opens a CDP WebSocket, runs fn with the send function,
// then closes the connection.
func WithCdpSocket(ctx context.Context, wsURL string, fn func(send CdpSendFn) error) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Duration(DefaultHandshakeTimeoutMs) * time.Millisecond,
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("CDP connect: %w", err)
	}

	sender := newCdpSender(conn)
	defer func() {
		conn.Close()
		<-sender.done
	}()

	return fn(sender.send)
}

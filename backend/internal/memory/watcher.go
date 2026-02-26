package memory

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher monitors file system paths for changes and triggers a
// callback with debouncing. Replaces the TS chokidar-based watcher in
// manager.ts (L813-1048).
//
// Key mapping from chokidar:
//   - ignoreInitial → fsnotify doesn't fire on startup (same behavior)
//   - awaitWriteFinish.stabilityThreshold → debounce timer
//   - on("add"/"change"/"unlink") → fsnotify Create/Write/Remove ops
type FileWatcher struct {
	mu         sync.Mutex
	watcher    *fsnotify.Watcher
	debounceMs int
	onDirty    func()
	logger     *slog.Logger
	timer      *time.Timer
	stopCh     chan struct{}
	stopped    bool
}

// NewFileWatcher creates a new file watcher monitoring the given paths.
func NewFileWatcher(paths []string, debounceMs int, onDirty func(), logger *slog.Logger) (*FileWatcher, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if debounceMs <= 0 {
		debounceMs = 500
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, p := range paths {
		if err := w.Add(p); err != nil {
			logger.Debug("file watcher: skip path", "path", p, "err", err)
		}
	}

	return &FileWatcher{
		watcher:    w,
		debounceMs: debounceMs,
		onDirty:    onDirty,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}, nil
}

// Start begins watching for file events in a background goroutine.
// The provided context controls the watcher lifetime.
func (fw *FileWatcher) Start(ctx context.Context) {
	go fw.run(ctx)
}

func (fw *FileWatcher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fw.Stop()
			return
		case <-fw.stopCh:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			// Only react to create, write, remove (matching chokidar add/change/unlink)
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
				continue
			}
			fw.scheduleDirty()
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Warn("file watcher error", "err", err)
		}
	}
}

// scheduleDirty resets the debounce timer, calling onDirty after the debounce period.
func (fw *FileWatcher) scheduleDirty() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if fw.stopped {
		return
	}
	if fw.timer != nil {
		fw.timer.Stop()
	}
	fw.timer = time.AfterFunc(time.Duration(fw.debounceMs)*time.Millisecond, func() {
		fw.mu.Lock()
		fw.timer = nil
		fw.mu.Unlock()
		if fw.onDirty != nil {
			fw.onDirty()
		}
	})
}

// Stop closes the watcher and cancels any pending debounce timer.
func (fw *FileWatcher) Stop() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if fw.stopped {
		return
	}
	fw.stopped = true
	close(fw.stopCh)
	if fw.timer != nil {
		fw.timer.Stop()
		fw.timer = nil
	}
	fw.watcher.Close()
}

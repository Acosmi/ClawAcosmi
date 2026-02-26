package tts

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TS 对照: tts/tts.ts L1100-1200 (缓存 + 临时文件管理)

// ---------- 音频缓存 ----------

// audioCache 内存缓存。
var audioCache struct {
	mu      sync.Mutex
	entries map[string]cachedAudio
}

type cachedAudio struct {
	path      string
	createdAt time.Time
}

func init() {
	audioCache.entries = make(map[string]cachedAudio)
}

// CacheKey 生成缓存键。
func CacheKey(text string, provider TtsProvider, voice string) string {
	return string(provider) + ":" + voice + ":" + text
}

// GetCached 获取缓存的音频路径。
func GetCached(key string) (string, bool) {
	audioCache.mu.Lock()
	defer audioCache.mu.Unlock()
	entry, ok := audioCache.entries[key]
	if !ok {
		return "", false
	}
	// 检查文件是否仍然存在
	if _, err := os.Stat(entry.path); err != nil {
		delete(audioCache.entries, key)
		return "", false
	}
	return entry.path, true
}

// SetCached 缓存音频路径。
func SetCached(key, audioPath string) {
	audioCache.mu.Lock()
	defer audioCache.mu.Unlock()
	audioCache.entries[key] = cachedAudio{
		path:      audioPath,
		createdAt: time.Now(),
	}
}

// ---------- 临时文件管理 ----------

// CreateTempAudioFile 创建临时音频文件。
// TS 对照: tts.ts L1150-1170
func CreateTempAudioFile(extension string) (string, error) {
	tmpDir := os.TempDir()
	ttsDir := filepath.Join(tmpDir, "openacosmi-tts")
	if err := os.MkdirAll(ttsDir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(ttsDir, "tts-*"+extension)
	if err != nil {
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

// ScheduleCleanup 延迟清理临时音频文件。
// TS 对照: tts.ts L1172-1185
func ScheduleCleanup(audioPath string, delay time.Duration) {
	if delay <= 0 {
		delay = TempFileCleanupDelay
	}
	go func() {
		time.Sleep(delay)
		_ = os.Remove(audioPath)
		// 同时从缓存中移除
		audioCache.mu.Lock()
		defer audioCache.mu.Unlock()
		for key, entry := range audioCache.entries {
			if entry.path == audioPath {
				delete(audioCache.entries, key)
				break
			}
		}
	}()
}

// CleanupAllTemp 清理所有 TTS 临时文件。
func CleanupAllTemp() {
	tmpDir := filepath.Join(os.TempDir(), "openacosmi-tts")
	_ = os.RemoveAll(tmpDir)
	audioCache.mu.Lock()
	defer audioCache.mu.Unlock()
	audioCache.entries = make(map[string]cachedAudio)
}

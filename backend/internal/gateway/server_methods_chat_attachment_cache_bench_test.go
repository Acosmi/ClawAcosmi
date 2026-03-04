package gateway

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/media"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

func BenchmarkChatAttachmentProviderCache_ProcessAttachments(b *testing.B) {
	attachments := testAttachments()
	ctx := context.Background()

	b.Run("cache_hit", func(b *testing.B) {
		cache := newChatAttachmentProviderCache(30 * time.Second)
		var sttInitCount int64
		var docInitCount int64
		cache.newSTTProvider = func(cfg *types.STTConfig) (media.STTProvider, error) {
			atomic.AddInt64(&sttInitCount, 1)
			return &fakeSTTProvider{transcript: "bench-stt"}, nil
		}
		cache.newDocConverter = func(cfg *types.DocConvConfig) (media.DocConverter, error) {
			atomic.AddInt64(&docInitCount, 1)
			return &fakeDocConverter{markdown: "bench-doc"}, nil
		}
		loader := &staticCfgLoader{cfg: testChatAttachmentConfig("openai")}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = processAttachmentsForChatWithCache(ctx, "base", attachments, loader, cache)
		}
		b.StopTimer()

		b.ReportMetric(float64(atomic.LoadInt64(&sttInitCount)), "stt_init_total")
		b.ReportMetric(float64(atomic.LoadInt64(&docInitCount)), "doc_init_total")
	})

	b.Run("cache_miss_config_change", func(b *testing.B) {
		cache := newChatAttachmentProviderCache(30 * time.Second)
		var sttInitCount int64
		var docInitCount int64
		cache.newSTTProvider = func(cfg *types.STTConfig) (media.STTProvider, error) {
			atomic.AddInt64(&sttInitCount, 1)
			return &fakeSTTProvider{transcript: "bench-stt"}, nil
		}
		cache.newDocConverter = func(cfg *types.DocConvConfig) (media.DocConverter, error) {
			atomic.AddInt64(&docInitCount, 1)
			return &fakeDocConverter{markdown: "bench-doc"}, nil
		}
		loader := &staticCfgLoader{cfg: testChatAttachmentConfig("openai")}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// 每次变更配置签名，模拟无复用场景（旧行为基线）。
			if i%2 == 0 {
				loader.cfg = testChatAttachmentConfig("openai")
			} else {
				loader.cfg = testChatAttachmentConfig("qwen")
			}
			_ = processAttachmentsForChatWithCache(ctx, "base", attachments, loader, cache)
		}
		b.StopTimer()

		b.ReportMetric(float64(atomic.LoadInt64(&sttInitCount)), "stt_init_total")
		b.ReportMetric(float64(atomic.LoadInt64(&docInitCount)), "doc_init_total")
	})
}

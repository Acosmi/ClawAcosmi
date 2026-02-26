package pipeline

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// FrameInput represents a raw frame for pipeline processing.
type FrameInput struct {
	Pixels    []byte
	Width     int
	Height    int
	FrameNo   int64
	Timestamp float64
}

// Pipeline orchestrates: frame source → keyframe extraction → consumer channel.
type Pipeline struct {
	extractor *KeyframeExtractor
	piiFilter *PIIFilter

	keyCh   chan Keyframe // downstream keyframe channel
	inputCh chan FrameInput

	running atomic.Bool
	wg      sync.WaitGroup

	// Stats
	framesProcessed  atomic.Int64
	keyframesEmitted atomic.Int64
	framesDropped    atomic.Int64
}

// NewPipeline creates a pipeline with default config.
func NewPipeline(extractor *KeyframeExtractor, piiFilter *PIIFilter, queueSize int) *Pipeline {
	if queueSize <= 0 {
		queueSize = 32
	}
	return &Pipeline{
		extractor: extractor,
		piiFilter: piiFilter,
		keyCh:     make(chan Keyframe, queueSize),
		inputCh:   make(chan FrameInput, queueSize),
	}
}

// Start launches the pipeline goroutine.
func (p *Pipeline) Start() {
	if p.running.Swap(true) {
		return // already running
	}
	p.wg.Add(1)
	go p.runLoop()
	log.Println("[Pipeline] Started async frame processing")
}

// Stop gracefully shuts down the pipeline.
func (p *Pipeline) Stop() {
	if !p.running.Swap(false) {
		return
	}
	close(p.inputCh)
	p.wg.Wait()
	close(p.keyCh)
	log.Printf("[Pipeline] Stopped. Processed=%d, Keyframes=%d, Dropped=%d",
		p.framesProcessed.Load(), p.keyframesEmitted.Load(), p.framesDropped.Load())
}

// Submit sends a frame to the pipeline for processing.
// Non-blocking: drops the frame if the input channel is full.
func (p *Pipeline) Submit(frame FrameInput) bool {
	if !p.running.Load() {
		return false
	}
	select {
	case p.inputCh <- frame:
		return true
	default:
		p.framesDropped.Add(1)
		return false
	}
}

// Keyframes returns the read-only channel for downstream consumers.
func (p *Pipeline) Keyframes() <-chan Keyframe {
	return p.keyCh
}

// GetKeyframe waits for the next keyframe with a timeout.
// Returns nil if timeout expires.
func (p *Pipeline) GetKeyframe(timeout time.Duration) *Keyframe {
	select {
	case kf, ok := <-p.keyCh:
		if !ok {
			return nil
		}
		return &kf
	case <-time.After(timeout):
		return nil
	}
}

// Stats returns pipeline statistics.
func (p *Pipeline) Stats() map[string]any {
	return map[string]any{
		"running":           p.running.Load(),
		"frames_processed":  p.framesProcessed.Load(),
		"keyframes_emitted": p.keyframesEmitted.Load(),
		"frames_dropped":    p.framesDropped.Load(),
		"pending_keyframes": len(p.keyCh),
		"extractor_stats":   p.extractor.Stats(),
	}
}

// runLoop is the main goroutine loop.
func (p *Pipeline) runLoop() {
	defer p.wg.Done()

	for frame := range p.inputCh {
		p.framesProcessed.Add(1)

		kf := p.extractor.ProcessFrame(
			frame.Pixels, frame.Width, frame.Height,
			frame.FrameNo, frame.Timestamp,
		)
		if kf == nil {
			continue
		}

		p.keyframesEmitted.Add(1)

		// Try to push to downstream channel.
		select {
		case p.keyCh <- *kf:
			// delivered
		default:
			// Channel full — drop oldest then push.
			select {
			case <-p.keyCh:
			default:
			}
			select {
			case p.keyCh <- *kf:
			default:
				p.framesDropped.Add(1)
			}
		}
	}
}

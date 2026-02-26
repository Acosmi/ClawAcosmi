package telegram

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func intPtr(v int) *int { return &v }

func makeDMMessage(chatID int64, text string) *TelegramMessage {
	return &TelegramMessage{
		MessageID: 1,
		Chat:      TelegramChat{ID: chatID, Type: "private"},
		Text:      text,
		From:      &TelegramUser{ID: chatID, FirstName: "Alice"},
	}
}

func makeGroupMessage(chatID int64, text string) *TelegramMessage {
	return &TelegramMessage{
		MessageID: 1,
		Chat:      TelegramChat{ID: chatID, Type: "supergroup"},
		Text:      text,
		From:      &TelegramUser{ID: 999, FirstName: "Bob"},
	}
}

func makeForumMessage(chatID int64, threadID int, text string) *TelegramMessage {
	return &TelegramMessage{
		MessageID:       1,
		Chat:            TelegramChat{ID: chatID, Type: "supergroup", IsForum: true},
		Text:            text,
		From:            &TelegramUser{ID: 999, FirstName: "Bob"},
		MessageThreadID: intPtr(threadID),
	}
}

func makeUpdate(msg *TelegramMessage) *TelegramUpdate {
	return &TelegramUpdate{
		UpdateID: 1,
		Message:  msg,
	}
}

func makeReactionUpdate(chatID int64) *TelegramUpdate {
	return &TelegramUpdate{
		UpdateID: 2,
		MessageReaction: &MessageReaction{
			Chat:      TelegramChat{ID: chatID, Type: "supergroup"},
			MessageID: 42,
		},
	}
}

func makeCallbackQueryUpdate(chatID int64, text string) *TelegramUpdate {
	return &TelegramUpdate{
		UpdateID: 3,
		CallbackQuery: &CallbackQuery{
			ID: "cb1",
			Message: &TelegramMessage{
				MessageID: 5,
				Chat:      TelegramChat{ID: chatID, Type: "private"},
				Text:      text,
			},
			Data: "some_data",
		},
	}
}

// ---------------------------------------------------------------------------
// TestBotGetTelegramSequentialKey
// ---------------------------------------------------------------------------

func TestBotGetTelegramSequentialKey(t *testing.T) {
	tests := []struct {
		name        string
		update      *TelegramUpdate
		botUsername string
		want        string
	}{
		{
			name:        "nil update returns unknown",
			update:      nil,
			botUsername: "testbot",
			want:        "telegram:unknown",
		},
		{
			name:        "regular DM message returns telegram:{chatId}",
			update:      makeUpdate(makeDMMessage(12345, "hello")),
			botUsername: "testbot",
			want:        "telegram:12345",
		},
		{
			name:        "regular group message returns telegram:{chatId}",
			update:      makeUpdate(makeGroupMessage(-100999, "hello group")),
			botUsername: "testbot",
			want:        "telegram:-100999",
		},
		{
			name:        "forum topic message returns telegram:{chatId}:topic:{threadId}",
			update:      makeUpdate(makeForumMessage(-100888, 42, "hello topic")),
			botUsername: "testbot",
			want:        "telegram:-100888:topic:42",
		},
		{
			name: "forum message without explicit threadID defaults to general topic",
			update: makeUpdate(&TelegramMessage{
				MessageID: 1,
				Chat:      TelegramChat{ID: -100888, Type: "supergroup", IsForum: true},
				Text:      "hello",
				From:      &TelegramUser{ID: 999, FirstName: "Bob"},
			}),
			botUsername: "testbot",
			want:        "telegram:-100888:topic:1",
		},
		{
			name:        "control command /reset returns telegram:{chatId}:control",
			update:      makeUpdate(makeDMMessage(12345, "/reset")),
			botUsername: "testbot",
			want:        "telegram:12345:control",
		},
		{
			name:        "control command /reset@botname returns telegram:{chatId}:control",
			update:      makeUpdate(makeGroupMessage(-100999, "/reset@testbot")),
			botUsername: "testbot",
			want:        "telegram:-100999:control",
		},
		{
			name:        "control command /help returns telegram:{chatId}:control",
			update:      makeUpdate(makeDMMessage(12345, "/help")),
			botUsername: "testbot",
			want:        "telegram:12345:control",
		},
		{
			name:        "non-control slash text is not a control command",
			update:      makeUpdate(makeDMMessage(12345, "/notacommand")),
			botUsername: "testbot",
			want:        "telegram:12345",
		},
		{
			name:        "reaction update returns telegram:{chatId}:reaction",
			update:      makeReactionUpdate(-100777),
			botUsername: "testbot",
			want:        "telegram:-100777:reaction",
		},
		{
			name: "reaction with zero chat ID returns unknown",
			update: &TelegramUpdate{
				UpdateID:        4,
				MessageReaction: &MessageReaction{Chat: TelegramChat{ID: 0}},
			},
			botUsername: "testbot",
			want:        "telegram:unknown",
		},
		{
			name: "edited message uses editedMessage field",
			update: &TelegramUpdate{
				UpdateID:      5,
				EditedMessage: makeGroupMessage(-100666, "edited text"),
			},
			botUsername: "testbot",
			want:        "telegram:-100666",
		},
		{
			name:        "callback query uses inner message chat",
			update:      makeCallbackQueryUpdate(55555, "callback text"),
			botUsername: "testbot",
			want:        "telegram:55555",
		},
		{
			name: "control command in caption",
			update: makeUpdate(&TelegramMessage{
				MessageID: 1,
				Chat:      TelegramChat{ID: 12345, Type: "private"},
				Caption:   "/reset",
				From:      &TelegramUser{ID: 12345, FirstName: "Alice"},
			}),
			botUsername: "testbot",
			want:        "telegram:12345:control",
		},
		{
			name: "DM with message thread ID returns topic key",
			update: makeUpdate(&TelegramMessage{
				MessageID:       1,
				Chat:            TelegramChat{ID: 12345, Type: "private"},
				Text:            "hello",
				From:            &TelegramUser{ID: 12345, FirstName: "Alice"},
				MessageThreadID: intPtr(77),
			}),
			botUsername: "testbot",
			want:        "telegram:12345:topic:77",
		},
		{
			name: "update with no message and no reaction",
			update: &TelegramUpdate{
				UpdateID: 10,
			},
			botUsername: "testbot",
			want:        "telegram:unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTelegramSequentialKey(tt.update, tt.botUsername)
			if got != tt.want {
				t.Errorf("getTelegramSequentialKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBotSequentializer
// ---------------------------------------------------------------------------

func TestBotSequentializer(t *testing.T) {
	t.Run("same key updates are serialized", func(t *testing.T) {
		seq := newSequentializer()

		var order []int
		var mu sync.Mutex
		done := make(chan struct{})

		const count = 5
		var wg sync.WaitGroup
		wg.Add(count)

		for i := 0; i < count; i++ {
			idx := i
			seq.run("same-key", func() {
				// Each task sleeps briefly to ensure ordering matters
				time.Sleep(5 * time.Millisecond)
				mu.Lock()
				order = append(order, idx)
				mu.Unlock()
				wg.Done()
			})
		}

		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for sequentialized tasks")
		}

		// Tasks with the same key should execute in FIFO order
		mu.Lock()
		defer mu.Unlock()
		if len(order) != count {
			t.Fatalf("expected %d completions, got %d", count, len(order))
		}
		for i := 0; i < count; i++ {
			if order[i] != i {
				t.Errorf("order[%d] = %d, want %d (FIFO violated)", i, order[i], i)
			}
		}
	})

	t.Run("different keys run concurrently", func(t *testing.T) {
		seq := newSequentializer()

		var concurrent int64
		var maxConcurrent int64
		done := make(chan struct{})

		const keys = 5
		var wg sync.WaitGroup
		wg.Add(keys)

		for i := 0; i < keys; i++ {
			key := "key-" + string(rune('A'+i))
			seq.run(key, func() {
				cur := atomic.AddInt64(&concurrent, 1)
				// Track max concurrency
				for {
					old := atomic.LoadInt64(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
						break
					}
				}
				// Hold the goroutine to give others a chance to start
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt64(&concurrent, -1)
				wg.Done()
			})
		}

		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for concurrent tasks")
		}

		mc := atomic.LoadInt64(&maxConcurrent)
		if mc < 2 {
			t.Errorf("expected at least 2 concurrent goroutines for different keys, got %d", mc)
		}
	})
}

// ---------------------------------------------------------------------------
// TestBotRecoverMiddleware
// ---------------------------------------------------------------------------

func TestBotRecoverMiddleware(t *testing.T) {
	t.Run("panic is recovered and does not propagate", func(t *testing.T) {
		// This should not panic the test
		didRun := false
		recoverMiddleware("test-panic", func() {
			didRun = true
			panic("intentional test panic")
		})
		if !didRun {
			t.Error("function was not executed")
		}
	})

	t.Run("non-panicking function runs normally", func(t *testing.T) {
		result := 0
		recoverMiddleware("test-normal", func() {
			result = 42
		})
		if result != 42 {
			t.Errorf("expected result 42, got %d", result)
		}
	})

	t.Run("subsequent calls work after a panic", func(t *testing.T) {
		recoverMiddleware("first", func() {
			panic("boom")
		})

		secondRan := false
		recoverMiddleware("second", func() {
			secondRan = true
		})
		if !secondRan {
			t.Error("second call did not run after first panic")
		}
	})
}

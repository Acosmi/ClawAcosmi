package skills

// serialize.go — 键控串行化执行
// 对应 TS: agents/skills/serialize.ts (14L)
//
// 提供按 key 串行化的任务执行能力，
// 确保同一 key 下的操作不会并发执行。
// Go 等价于 TS 的 Promise-chain serialization。

import (
	"sync"
)

// serializer 全局串行化器（TS: SKILLS_SYNC_QUEUE Map<string, Promise>）。
var serializer = &keySerializer{
	locks: make(map[string]*sync.Mutex),
}

type keySerializer struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// getLock 获取或创建指定 key 的锁。
func (s *keySerializer) getLock(key string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[key] = lock
	}
	return lock
}

// cleanup 清理不再使用的 key 锁。
func (s *keySerializer) cleanup(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lock, ok := s.locks[key]; ok {
		// 尝试锁定以检查是否有其他 goroutine 在等待
		if lock.TryLock() {
			lock.Unlock()
			delete(s.locks, key)
		}
	}
}

// SerializeByKey 按 key 串行化执行任务。
// 对应 TS: serializeByKey<T>(key: string, task: () => Promise<T>)
// Go 语义：同一 key 下的 task 串行执行，不同 key 可并发。
func SerializeByKey[T any](key string, task func() (T, error)) (T, error) {
	lock := serializer.getLock(key)
	lock.Lock()
	defer func() {
		lock.Unlock()
		serializer.cleanup(key)
	}()
	return task()
}

// SerializeByKeyVoid 无返回值版本。
func SerializeByKeyVoid(key string, task func() error) error {
	lock := serializer.getLock(key)
	lock.Lock()
	defer func() {
		lock.Unlock()
		serializer.cleanup(key)
	}()
	return task()
}

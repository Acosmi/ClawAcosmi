package understanding

import "sync"

// TS 对照: media-understanding/concurrency.ts (34L)

// RunWithConcurrency 以给定并发限制执行任务列表。
// 返回结果数组（与输入顺序一致）。
// TS 对照: concurrency.ts L5-34
func RunWithConcurrency[T any](tasks []func() (T, error), limit int) ([]T, []error) {
	if limit <= 0 {
		limit = 1
	}
	n := len(tasks)
	if n == 0 {
		return nil, nil
	}

	results := make([]T, n)
	errors := make([]error, n)
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量
		go func(idx int, fn func() (T, error)) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量
			result, err := fn()
			results[idx] = result
			errors[idx] = err
		}(i, task)
	}

	wg.Wait()
	return results, errors
}

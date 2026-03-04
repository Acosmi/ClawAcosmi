package media

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestNormalizeTempExtension(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{input: "", want: ".bin"},
		{input: "txt", want: ".txt"},
		{input: ".md", want: ".md"},
		{input: "  .pdf  ", want: ".pdf"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeTempExtension(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeTempExtension(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWriteTempInputFileConcurrentUnique(t *testing.T) {
	const workers = 40

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		paths   = make(map[string]struct{}, workers)
		cleanup []string
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := []byte(fmt.Sprintf("payload-%d", idx))
			path, err := writeTempInputFile("docconv-input", ".txt", payload)
			if err != nil {
				t.Errorf("writeTempInputFile failed: %v", err)
				return
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read temp file failed: %v", err)
				return
			}
			if string(data) != string(payload) {
				t.Errorf("temp file content mismatch: got=%q want=%q", string(data), string(payload))
			}

			mu.Lock()
			if _, exists := paths[path]; exists {
				t.Errorf("duplicate temp path generated: %s", path)
			}
			paths[path] = struct{}{}
			cleanup = append(cleanup, path)
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	t.Cleanup(func() {
		for _, path := range cleanup {
			_ = os.Remove(path)
		}
	})

	if len(paths) != workers {
		t.Fatalf("temp file count mismatch: got %d, want %d", len(paths), workers)
	}
}

func TestCreateTempOutputPathConcurrentUnique(t *testing.T) {
	const workers = 30

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		paths   = make(map[string]struct{}, workers)
		cleanup []string
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := createTempOutputPath("docconv-output", ".md")
			if err != nil {
				t.Errorf("createTempOutputPath failed: %v", err)
				return
			}

			if _, err := os.Stat(path); err != nil {
				t.Errorf("temp output path does not exist: %v", err)
				return
			}

			mu.Lock()
			if _, exists := paths[path]; exists {
				t.Errorf("duplicate output path generated: %s", path)
			}
			paths[path] = struct{}{}
			cleanup = append(cleanup, path)
			mu.Unlock()
		}()
	}

	wg.Wait()
	t.Cleanup(func() {
		for _, path := range cleanup {
			_ = os.Remove(path)
		}
	})

	if len(paths) != workers {
		t.Fatalf("temp output path count mismatch: got %d, want %d", len(paths), workers)
	}
}

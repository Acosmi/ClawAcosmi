// registry.go — 沙箱容器注册表持久化。
//
// TS 对照: agents/sandbox/registry.ts (117L)
//
// 管理 JSON 注册表文件，记录活跃的沙箱容器和浏览器，
// 包括读写、更新和删除操作。使用 sync.Mutex 保证并发安全。
package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// 注册表文件锁（按路径索引）。
var registryMu sync.Map // map[string]*sync.Mutex

func getRegistryLock(registryPath string) *sync.Mutex {
	val, _ := registryMu.LoadOrStore(registryPath, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// ---------- 容器注册表 ----------

// ReadRegistry 读取容器注册表。
// TS 对照: registry.ts readRegistry()
func ReadRegistry(registryPath string) (*Registry, error) {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	return readRegistryUnlocked(registryPath)
}

func readRegistryUnlocked(registryPath string) (*Registry, error) {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, err
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		// 文件损坏时返回空注册表
		return &Registry{}, nil
	}
	return &reg, nil
}

// WriteRegistry 写入容器注册表（原子写入）。
// TS 对照: registry.ts writeRegistry()
func WriteRegistry(registryPath string, reg *Registry) error {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	return writeRegistryUnlocked(registryPath, reg)
}

func writeRegistryUnlocked(registryPath string, reg *Registry) error {
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	// 原子写入：先写临时文件再重命名
	tmpPath := registryPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, registryPath)
}

// UpdateRegistryEntry 更新或插入注册表条目。
// TS 对照: registry.ts updateRegistryEntry()
func UpdateRegistryEntry(registryPath string, entry RegistryEntry) error {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	reg, err := readRegistryUnlocked(registryPath)
	if err != nil {
		return err
	}

	found := false
	for i, e := range reg.Entries {
		if e.ContainerName == entry.ContainerName {
			reg.Entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		reg.Entries = append(reg.Entries, entry)
	}

	return writeRegistryUnlocked(registryPath, reg)
}

// RemoveRegistryEntry 从注册表中移除条目。
// TS 对照: registry.ts removeRegistryEntry()
func RemoveRegistryEntry(registryPath string, containerName string) error {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	reg, err := readRegistryUnlocked(registryPath)
	if err != nil {
		return err
	}

	filtered := make([]RegistryEntry, 0, len(reg.Entries))
	for _, e := range reg.Entries {
		if e.ContainerName != containerName {
			filtered = append(filtered, e)
		}
	}
	reg.Entries = filtered

	return writeRegistryUnlocked(registryPath, reg)
}

// ---------- 浏览器注册表 ----------

// ReadBrowserRegistry 读取浏览器注册表。
func ReadBrowserRegistry(registryPath string) (*BrowserRegistry, error) {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &BrowserRegistry{}, nil
		}
		return nil, err
	}

	var reg BrowserRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return &BrowserRegistry{}, nil
	}
	return &reg, nil
}

// UpdateBrowserRegistryEntry 更新或插入浏览器注册表条目。
func UpdateBrowserRegistryEntry(registryPath string, entry BrowserRegistryEntry) error {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(registryPath)
	var reg BrowserRegistry
	if err == nil {
		_ = json.Unmarshal(data, &reg)
	}

	found := false
	for i, e := range reg.Entries {
		if e.ContainerName == entry.ContainerName {
			reg.Entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		reg.Entries = append(reg.Entries, entry)
	}

	if err := os.MkdirAll(filepath.Dir(registryPath), 0o700); err != nil {
		return err
	}
	out, err := json.MarshalIndent(&reg, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := registryPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, registryPath)
}

// RemoveBrowserRegistryEntry 从浏览器注册表中移除条目。
func RemoveBrowserRegistryEntry(registryPath string, containerName string) error {
	mu := getRegistryLock(registryPath)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var reg BrowserRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil
	}

	filtered := make([]BrowserRegistryEntry, 0, len(reg.Entries))
	for _, e := range reg.Entries {
		if e.ContainerName != containerName {
			filtered = append(filtered, e)
		}
	}
	reg.Entries = filtered

	out, err := json.MarshalIndent(&reg, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := registryPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, registryPath)
}

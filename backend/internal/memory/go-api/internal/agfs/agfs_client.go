package agfs

import (
	"encoding/json"
	"fmt"

	sdk "github.com/c4pt0r/agfs/agfs-sdk/go"
)

// AGFSClient 封装 agfs-sdk/go 客户端，提供 UHMS 业务所需的高层 API。
type AGFSClient struct {
	client *sdk.Client
}

// NewAGFSClient 创建 AGFS 客户端。
// serverURL 示例: "http://localhost:8090" 或 "http://agfs-server:8080"
func NewAGFSClient(serverURL string) *AGFSClient {
	return &AGFSClient{
		client: sdk.NewClient(serverURL),
	}
}

// Health 检查 AGFS Server 健康状态。
func (c *AGFSClient) Health() error {
	return c.client.Health()
}

// ─── 队列操作 (queuefs) ──────────────────────────────────────

// QueueEnqueue 向指定队列写入数据。
// queueName 示例: "embedding_tasks"
func (c *AGFSClient) QueueEnqueue(queueName string, data []byte) error {
	path := fmt.Sprintf("/queuefs/%s/enqueue", queueName)
	_, err := c.client.Write(path, data)
	return err
}

// QueueDequeue 从指定队列读取一条数据。
// 队列为空时返回空切片和 nil error。
func (c *AGFSClient) QueueDequeue(queueName string) ([]byte, error) {
	path := fmt.Sprintf("/queuefs/%s/dequeue", queueName)
	data, err := c.client.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// QueueEnqueueJSON 序列化后入队。
func (c *AGFSClient) QueueEnqueueJSON(queueName string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal enqueue payload: %w", err)
	}
	return c.QueueEnqueue(queueName, data)
}

// ─── KV 操作 (kvfs) ──────────────────────────────────────────

// KVSet 写入一个 KV 对。
func (c *AGFSClient) KVSet(key string, value []byte) error {
	path := fmt.Sprintf("/kvfs/keys/%s", key)
	_, err := c.client.Write(path, value)
	return err
}

// KVGet 读取一个 KV 对。
func (c *AGFSClient) KVGet(key string) ([]byte, error) {
	path := fmt.Sprintf("/kvfs/keys/%s", key)
	return c.client.Read(path, 0, -1)
}

// KVSetJSON 序列化后写入 KV。
func (c *AGFSClient) KVSetJSON(key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal kv value: %w", err)
	}
	return c.KVSet(key, data)
}

// ─── 文件操作 (localfs) ──────────────────────────────────────

// WriteFile 通过 AGFS localfs 写入文件。
func (c *AGFSClient) WriteFile(path string, data []byte) error {
	fullPath := fmt.Sprintf("/localfs%s", path)
	_, err := c.client.Write(fullPath, data)
	return err
}

// ReadFile 通过 AGFS localfs 读取文件。
func (c *AGFSClient) ReadFile(path string) ([]byte, error) {
	fullPath := fmt.Sprintf("/localfs%s", path)
	return c.client.Read(fullPath, 0, -1)
}

// Mkdir 通过 AGFS localfs 创建目录。
func (c *AGFSClient) Mkdir(path string) error {
	fullPath := fmt.Sprintf("/localfs%s", path)
	return c.client.Mkdir(fullPath, 0755)
}

// ListDir 列出 AGFS localfs 目录内容。
func (c *AGFSClient) ListDir(path string) ([]sdk.FileInfo, error) {
	fullPath := fmt.Sprintf("/localfs%s", path)
	return c.client.ReadDir(fullPath)
}

// DeleteFile 通过 AGFS localfs 删除单个文件或空目录。
func (c *AGFSClient) DeleteFile(path string) error {
	fullPath := fmt.Sprintf("/localfs%s", path)
	return c.client.Remove(fullPath)
}

// RemoveAll 通过 AGFS localfs 递归删除目录及其所有内容。
func (c *AGFSClient) RemoveAll(path string) error {
	fullPath := fmt.Sprintf("/localfs%s", path)
	return c.client.RemoveAll(fullPath)
}

// Stat 返回 AGFS localfs 路径的文件信息。
func (c *AGFSClient) Stat(path string) (*sdk.FileInfo, error) {
	fullPath := fmt.Sprintf("/localfs%s", path)
	return c.client.Stat(fullPath)
}

// Rename 重命名/移动 AGFS localfs 中的文件或目录。
func (c *AGFSClient) Rename(oldPath, newPath string) error {
	fullOld := fmt.Sprintf("/localfs%s", oldPath)
	fullNew := fmt.Sprintf("/localfs%s", newPath)
	return c.client.Rename(fullOld, fullNew)
}

// FileExists 检查 AGFS localfs 路径是否存在。
func (c *AGFSClient) FileExists(path string) bool {
	_, err := c.Stat(path)
	return err == nil
}

// ─── 临时存储操作 (memfs) ────────────────────────────────────

// MemWrite 写入临时存储。
func (c *AGFSClient) MemWrite(path string, data []byte) error {
	fullPath := fmt.Sprintf("/memfs%s", path)
	_, err := c.client.Write(fullPath, data)
	return err
}

// MemRead 从临时存储读取。
func (c *AGFSClient) MemRead(path string) ([]byte, error) {
	fullPath := fmt.Sprintf("/memfs%s", path)
	return c.client.Read(fullPath, 0, -1)
}

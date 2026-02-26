package memory

// sync_sessions.go — Session 文件索引同步
// 对应 TS: memory/sync-session-files.ts (132L)
//
// 扫描 session 文件目录中的 .jsonl 文件（对话转录），
// 按 hash 增量索引到 SQLite 数据库中，使其可被 memory 搜索检索。
// 当 FileWatcher 检测到 session 文件变更时，由 Sync() 触发此流程。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// syncSessionFiles 扫描并索引 session 文件（.jsonl 对话转录）。
// 对应 TS: sync-session-files.ts → syncSessionFiles()
//
// 行为等价于 TS 版本：
//  1. 列举 sessionsDir 下所有 .jsonl 文件
//  2. 对比 DB 中的 hash，仅处理变更文件
//  3. 变更文件读取内容 → 分块 → 生成 embedding → 写入 DB
//  4. 清理 DB 中不再存在的 session 文件条目
func (m *Manager) syncSessionFiles(ctx context.Context, opts *SyncOptions) error {
	if m.sessionsDir == "" {
		return nil
	}

	// 1. 扫描 session 文件
	sessionFiles, err := listSessionFiles(m.sessionsDir)
	if err != nil {
		return fmt.Errorf("list session files: %w", err)
	}
	if len(sessionFiles) == 0 {
		return nil
	}

	m.logger.Debug("memory sync: indexing session files",
		"count", len(sessionFiles),
		"force", opts != nil && opts.Force,
	)

	// 构建活跃文件路径集合（用于清理陈旧条目）
	activePaths := make(map[string]struct{}, len(sessionFiles))

	providerModel := ""
	if m.provider != nil {
		providerModel = m.provider.ID + ":" + m.provider.Model
	}

	chunkCfg := ChunkingConfig{Tokens: 512, Overlap: 50}

	for _, absPath := range sessionFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 计算相对路径
		relPath, _ := filepath.Rel(m.sessionsDir, absPath)
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		activePaths[relPath] = struct{}{}

		// 读取并计算 hash
		content, err := os.ReadFile(absPath)
		if err != nil {
			m.logger.Debug("memory sync: skip session file", "path", absPath, "err", err)
			continue
		}
		hash := HashText(string(content))

		// 检查 DB 中是否已有相同 hash
		if opts == nil || !opts.Force {
			var storedHash string
			row := m.db.QueryRow("SELECT hash FROM files WHERE path = ? AND source = ?", relPath, "sessions")
			if err := row.Scan(&storedHash); err == nil && storedHash == hash {
				continue // 未变更，跳过
			}
		}

		// 分块
		chunks := ChunkMarkdown(string(content), chunkCfg)
		if len(chunks) == 0 {
			continue
		}

		// 生成 embeddings（可选）
		var embeddings [][]float64
		if m.provider != nil && m.provider.EmbedBatch != nil {
			texts := make([]string, len(chunks))
			for j, c := range chunks {
				texts[j] = c.Text
			}
			emb, err := m.provider.EmbedBatch(ctx, texts)
			if err != nil {
				m.logger.Warn("memory sync: embed session batch failed", "path", relPath, "err", err)
			} else {
				embeddings = emb
			}
		}

		// 事务写入
		info, _ := os.Stat(absPath)
		mtimeMs := int64(0)
		fileSize := int64(0)
		if info != nil {
			mtimeMs = info.ModTime().UnixMilli()
			fileSize = info.Size()
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			m.logger.Warn("memory sync: session tx begin failed", "err", err)
			continue
		}

		// 删除旧 chunks
		if _, err := tx.Exec("DELETE FROM chunks WHERE path = ? AND source = ?", relPath, "sessions"); err != nil {
			m.logger.Warn("memory sync: session delete chunks failed", "path", relPath, "err", err)
		}
		if m.ftsAvailable {
			if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE path = ? AND source = ?", m.ftsTable), relPath, "sessions"); err != nil {
				m.logger.Warn("memory sync: session fts delete failed", "path", relPath, "err", err)
			}
		}

		model := providerModel
		if model == "" {
			model = "none"
		}

		// 插入新 chunks
		for j, chunk := range chunks {
			chunkID := "sessions:" + relPath + ":" + fmt.Sprintf("%d-%d", chunk.StartLine, chunk.EndLine)
			embJSON := "[]"
			if j < len(embeddings) && len(embeddings[j]) > 0 {
				embJSON = embeddingToJSON(embeddings[j])
			}

			if _, err := tx.Exec(
				`INSERT OR REPLACE INTO chunks (id, path, source, start_line, end_line, hash, model, text, embedding, updated_at)
				 VALUES (?, ?, 'sessions', ?, ?, ?, ?, ?, ?, ?)`,
				chunkID, relPath, chunk.StartLine, chunk.EndLine,
				chunk.Hash, model, chunk.Text, embJSON, mtimeMs,
			); err != nil {
				m.logger.Warn("memory sync: session insert chunk failed", "id", chunkID, "err", err)
				continue
			}

			// FTS5 索引
			if m.ftsAvailable {
				if _, err := tx.Exec(
					fmt.Sprintf(
						"INSERT INTO %s (text, id, path, source, model, start_line, end_line) VALUES (?, ?, ?, 'sessions', ?, ?, ?)",
						m.ftsTable,
					),
					chunk.Text, chunkID, relPath, model, chunk.StartLine, chunk.EndLine,
				); err != nil {
					m.logger.Warn("memory sync: session fts insert failed", "id", chunkID, "err", err)
				}
			}
		}

		// Upsert files 表
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO files (path, source, hash, mtime, size) VALUES (?, 'sessions', ?, ?, ?)`,
			relPath, hash, mtimeMs, fileSize,
		); err != nil {
			m.logger.Warn("memory sync: session upsert files failed", "path", relPath, "err", err)
		}

		if err := tx.Commit(); err != nil {
			m.logger.Warn("memory sync: session commit failed", "path", relPath, "err", err)
			continue
		}
	}

	// 清理 DB 中不再存在的 session 文件条目
	// 对应 TS: sync-session-files.ts L109-131
	staleRows, err := m.db.QueryContext(ctx, "SELECT path FROM files WHERE source = ?", "sessions")
	if err == nil {
		defer staleRows.Close()
		for staleRows.Next() {
			var stalePath string
			if err := staleRows.Scan(&stalePath); err != nil {
				continue
			}
			if _, exists := activePaths[stalePath]; exists {
				continue
			}
			// 删除陈旧条目
			m.db.Exec("DELETE FROM files WHERE path = ? AND source = ?", stalePath, "sessions")
			m.db.Exec("DELETE FROM chunks WHERE path = ? AND source = ?", stalePath, "sessions")
			if m.ftsAvailable {
				m.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE path = ? AND source = ?", m.ftsTable), stalePath, "sessions")
			}
		}
	}

	m.logger.Info("memory sync: session files complete", "total", len(sessionFiles))
	return nil
}

// listSessionFiles 列举目录下所有 .jsonl 文件。
// 对应 TS: session-files.ts → listSessionFilesForAgent()
func listSessionFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jsonl") || strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

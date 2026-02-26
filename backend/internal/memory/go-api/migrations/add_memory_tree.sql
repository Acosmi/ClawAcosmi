-- MemTree: 记忆树节点表
-- 实现论文 arXiv:2410.14052 的动态树结构记忆
CREATE TABLE IF NOT EXISTS memory_tree_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(100) NOT NULL,
    parent_id UUID REFERENCES memory_tree_nodes(id) ON DELETE CASCADE,
    memory_id UUID REFERENCES memories(id) ON DELETE
    SET NULL,
        content TEXT NOT NULL DEFAULT '',
        category VARCHAR(50) NOT NULL DEFAULT 'fact',
        depth INT NOT NULL DEFAULT 0,
        is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
        node_type VARCHAR(20) NOT NULL DEFAULT 'leaf',
        children_count INT NOT NULL DEFAULT 0,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- 高频查询索引
CREATE INDEX IF NOT EXISTS idx_tree_user_category ON memory_tree_nodes(user_id, category);
CREATE INDEX IF NOT EXISTS idx_tree_parent ON memory_tree_nodes(parent_id);
CREATE INDEX IF NOT EXISTS idx_tree_user_leaf ON memory_tree_nodes(user_id, is_leaf)
WHERE is_leaf = TRUE;
CREATE INDEX IF NOT EXISTS idx_tree_memory ON memory_tree_nodes(memory_id)
WHERE memory_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tree_user_type ON memory_tree_nodes(user_id, node_type);
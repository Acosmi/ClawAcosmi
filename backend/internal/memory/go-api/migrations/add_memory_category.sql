-- Migration: Add category field to memories table
-- This enables semantic classification of memory content (preference, habit, profile, etc.)
ALTER TABLE memories
ADD COLUMN IF NOT EXISTS category VARCHAR(50) NOT NULL DEFAULT 'fact';
CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
CREATE INDEX IF NOT EXISTS idx_memories_user_category ON memories(user_id, category);
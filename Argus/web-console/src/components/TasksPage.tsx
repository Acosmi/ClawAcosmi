'use client';

import { useState, useEffect, useCallback } from 'react';
import type { TaskItem } from '@/types';
import { useT } from '@/i18n';

const API_BASE = '/api/sensory';

type FilterType = 'all' | 'running' | 'pending' | 'done' | 'failed';

export default function TasksPage() {
    const [tasks, setTasks] = useState<TaskItem[]>([]);
    const [filter, setFilter] = useState<FilterType>('all');
    const [expandedId, setExpandedId] = useState<string | null>(null);
    const t = useT();

    // Fetch tasks from backend
    const fetchTasks = useCallback(async () => {
        try {
            const resp = await fetch(`${API_BASE}/tasks`);
            if (resp.ok) {
                const data = await resp.json();
                setTasks(data || []);
            }
        } catch { /* backend unavailable */ }
    }, []);

    // Poll every 3 seconds
    useEffect(() => {
        fetchTasks();
        const interval = setInterval(fetchTasks, 3000);
        return () => clearInterval(interval);
    }, [fetchTasks]);

    // Cancel a running task
    const handleCancel = useCallback(async (id: string) => {
        try {
            await fetch(`${API_BASE}/tasks/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ status: 'failed', duration: 'cancelled' }),
            });
            fetchTasks();
        } catch { /* ignore */ }
    }, [fetchTasks]);

    // Retry a failed task
    const handleRetry = useCallback(async (id: string) => {
        try {
            await fetch(`${API_BASE}/tasks/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ status: 'running', steps: 0, duration: '0s' }),
            });
            fetchTasks();
        } catch { /* ignore */ }
    }, [fetchTasks]);

    // Delete a task
    const handleDelete = useCallback(async (id: string) => {
        try {
            await fetch(`${API_BASE}/tasks/${id}`, { method: 'DELETE' });
            fetchTasks();
        } catch { /* ignore */ }
    }, [fetchTasks]);

    const filtered = filter === 'all'
        ? tasks
        : tasks.filter(tk => tk.status === filter);

    const counts = {
        running: tasks.filter(tk => tk.status === 'running').length,
        pending: tasks.filter(tk => tk.status === 'pending').length,
        done: tasks.filter(tk => tk.status === 'done').length,
        failed: tasks.filter(tk => tk.status === 'failed').length,
    };

    const filters: { key: FilterType; label: string; count?: number }[] = [
        { key: 'all', label: t('tasks_filter_all') },
        { key: 'running', label: t('tasks_running'), count: counts.running },
        { key: 'pending', label: t('tasks_pending'), count: counts.pending },
        { key: 'done', label: t('tasks_done'), count: counts.done },
        { key: 'failed', label: t('task_status_failed'), count: counts.failed },
    ];

    return (
        <div className="subpage">
            <div className="subpage-header">
                <h2>{t('tasks_title')}</h2>
                <div className="task-stats">
                    <span className="stat-badge running">{counts.running} {t('tasks_running')}</span>
                    <span className="stat-badge pending">{counts.pending} {t('tasks_pending')}</span>
                    <span className="stat-badge done">{counts.done} {t('tasks_done')}</span>
                </div>
            </div>

            {/* Filter tabs */}
            <div className="filter-tabs">
                {filters.map(f => (
                    <button
                        key={f.key}
                        className={`filter-tab ${filter === f.key ? 'active' : ''}`}
                        onClick={() => setFilter(f.key)}
                    >
                        {f.label}{f.count !== undefined ? ` (${f.count})` : ''}
                    </button>
                ))}
            </div>

            {filtered.length === 0 ? (
                <div className="empty-state">
                    <span className="empty-icon">📋</span>
                    <p>{t('tasks_empty')}</p>
                    <p className="empty-hint">{t('tasks_empty_hint')}</p>
                </div>
            ) : (
                <div className="task-list">
                    {filtered.map(task => (
                        <div key={task.id} className={`task-card status-${task.status}`}>
                            <div className="task-status-indicator" />
                            <div
                                className="task-content"
                                onClick={() => setExpandedId(expandedId === task.id ? null : task.id)}
                                style={{ cursor: 'pointer' }}
                            >
                                <div className="task-goal">{task.goal}</div>
                                <div className="task-meta">
                                    <span>{task.steps} {t('tasks_steps')}</span>
                                    <span>⏱ {task.duration}</span>
                                    <span>{task.startedAt}</span>
                                </div>
                                {expandedId === task.id && (
                                    <div className="task-details">
                                        <div className="task-detail-row">
                                            <span>ID:</span>
                                            <code>{task.id}</code>
                                        </div>
                                    </div>
                                )}
                            </div>
                            <div className="task-actions">
                                {task.status === 'running' && (
                                    <button className="action-btn-sm danger" onClick={() => handleCancel(task.id)}>
                                        {t('tasks_cancel')}
                                    </button>
                                )}
                                {task.status === 'failed' && (
                                    <button className="action-btn-sm primary" onClick={() => handleRetry(task.id)}>
                                        {t('tasks_retry')}
                                    </button>
                                )}
                                {(task.status === 'done' || task.status === 'failed') && (
                                    <button className="action-btn-sm" onClick={() => handleDelete(task.id)}>✕</button>
                                )}
                            </div>
                            <div className={`task-badge ${task.status}`}>
                                {task.status === 'running' ? t('task_status_running') :
                                    task.status === 'done' ? t('task_status_done') :
                                        task.status === 'failed' ? t('task_status_failed') : t('task_status_pending')}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

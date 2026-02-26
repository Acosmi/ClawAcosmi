'use client';

import { useState, useEffect, useCallback } from 'react';
import type { Keyframe } from '@/types';
import { useT } from '@/i18n';

const API_BASE = '/api/sensory';

// Backend keyframe response shape
interface BackendKeyframe {
    frame_no: number;
    timestamp: number;
    change_ratio: number;
    trigger_reason: string;
    thumbnail_b64?: string;
    metadata?: Record<string, unknown>;
}

function mapKeyframe(bk: BackendKeyframe, index: number): Keyframe {
    return {
        id: bk.metadata?.keyframe_id as number ?? index,
        frameNo: bk.frame_no,
        timestamp: bk.timestamp,
        thumbnail: bk.thumbnail_b64 || '',
        reason: bk.trigger_reason,
    };
}

export default function TimelinePage() {
    const [keyframes, setKeyframes] = useState<Keyframe[]>([]);
    const [selectedId, setSelectedId] = useState<number | null>(null);
    const [filter, setFilter] = useState('');
    const [autoRefresh, setAutoRefresh] = useState(true);
    const t = useT();

    // Fetch keyframes from backend
    const fetchKeyframes = useCallback(async () => {
        try {
            const resp = await fetch(`${API_BASE}/pipeline/keyframes?n=50`);
            if (resp.ok) {
                const data = await resp.json();
                if (data.keyframes && Array.isArray(data.keyframes)) {
                    setKeyframes(data.keyframes.map((kf: BackendKeyframe, i: number) => mapKeyframe(kf, i)));
                }
            }
        } catch { /* backend unavailable */ }
    }, []);

    // Poll every 5 seconds when autoRefresh is on
    useEffect(() => {
        fetchKeyframes();
        if (!autoRefresh) return;
        const interval = setInterval(fetchKeyframes, 5000);
        return () => clearInterval(interval);
    }, [fetchKeyframes, autoRefresh]);

    const filtered = keyframes.filter(kf =>
        kf.reason.toLowerCase().includes(filter.toLowerCase())
    );

    return (
        <div className="subpage">
            <div className="subpage-header">
                <h2>{t('timeline_title')}</h2>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <input
                        className="search-input"
                        placeholder={t('timeline_search')}
                        value={filter}
                        onChange={e => setFilter(e.target.value)}
                    />
                    <button
                        className={`action-btn-sm ${autoRefresh ? 'primary' : ''}`}
                        onClick={() => setAutoRefresh(!autoRefresh)}
                        title={t('timeline_auto_refresh')}
                    >
                        {autoRefresh ? '⏸' : '▶'}
                    </button>
                </div>
            </div>

            {filtered.length === 0 ? (
                <div className="empty-state">
                    <span className="empty-icon">📷</span>
                    <p>{t('timeline_empty')}</p>
                    <p className="empty-hint">{t('timeline_empty_hint')}</p>
                </div>
            ) : (
                <div className="timeline-grid">
                    {filtered.map(kf => (
                        <div
                            key={kf.id}
                            className={`timeline-card ${selectedId === kf.id ? 'selected' : ''}`}
                            onClick={() => setSelectedId(kf.id)}
                        >
                            <div className="kf-thumbnail">
                                {kf.thumbnail ? (
                                    <img src={kf.thumbnail} alt={`Frame ${kf.frameNo}`} />
                                ) : (
                                    <div className="kf-placeholder">#{kf.frameNo}</div>
                                )}
                            </div>
                            <div className="kf-info">
                                <span className="kf-frame">#{kf.frameNo}</span>
                                <span className="kf-reason">{kf.reason}</span>
                                <span className="kf-time">
                                    {new Date(kf.timestamp * 1000).toLocaleTimeString('zh-CN')}
                                </span>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Interactive Timeline Bar */}
            <div className="timeline-bar-container">
                <div className="timeline-bar">
                    {filtered.map((kf, idx) => (
                        <div
                            key={kf.id}
                            className={`timeline-marker ${selectedId === kf.id ? 'active' : ''}`}
                            style={{ left: `${((idx + 1) / (filtered.length + 1)) * 100}%` }}
                            title={`#${kf.frameNo}: ${kf.reason}`}
                            onClick={() => setSelectedId(kf.id)}
                        />
                    ))}
                    <div className="timeline-progress" />
                </div>
            </div>
        </div>
    );
}

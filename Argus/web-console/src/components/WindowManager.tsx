'use client';

import { useEffect, useState, useCallback, useMemo } from 'react';
import { useT } from '@/i18n';

// --- Types (must match Go JSON snake_case tags) ---
interface WindowInfo {
    window_id: number;
    title: string;
    app_name: string;
    bundle_id: string;
    x: number;
    y: number;
    width: number;
    height: number;
    on_screen: boolean;
    layer: number;
}

// --- API helpers ---
const API = '/api/sensory/windows';

async function fetchWindows(): Promise<WindowInfo[]> {
    const resp = await fetch(API);
    if (!resp.ok) return [];
    const data = await resp.json();
    return data.windows ?? [];
}

async function fetchExcluded(): Promise<number[]> {
    const resp = await fetch(`${API}/exclude`);
    if (!resp.ok) return [];
    const data = await resp.json();
    return data.excluded_window_ids ?? [];
}

async function setExcluded(ids: number[]): Promise<boolean> {
    const resp = await fetch(`${API}/exclude`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ window_ids: ids }),
    });
    return resp.ok;
}

async function clearExcluded(): Promise<boolean> {
    const resp = await fetch(`${API}/exclude`, { method: 'DELETE' });
    return resp.ok;
}

async function excludeApp(bundleID: string): Promise<boolean> {
    const resp = await fetch(`${API}/exclude/app`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ bundle_id: bundleID }),
    });
    return resp.ok;
}

// --- Helpers ---
interface AppGroup {
    appName: string;
    bundleID: string;
    windows: WindowInfo[];
}

function groupByApp(windows: WindowInfo[]): AppGroup[] {
    const map = new Map<string, AppGroup>();
    for (const w of windows) {
        const key = w.bundle_id || w.app_name || 'unknown';
        if (!map.has(key)) {
            map.set(key, { appName: w.app_name || '(unnamed)', bundleID: w.bundle_id, windows: [] });
        }
        map.get(key)!.windows.push(w);
    }
    // Sort: groups with more windows first
    return Array.from(map.values()).sort((a, b) => b.windows.length - a.windows.length);
}

// --- Component ---
export default function WindowManager() {
    const t = useT();
    const [windows, setWindows] = useState<WindowInfo[]>([]);
    const [excludedIds, setExcludedIds] = useState<Set<number>>(new Set());
    const [loading, setLoading] = useState(true);
    const [actionLoading, setActionLoading] = useState('');

    const loadData = useCallback(async () => {
        setLoading(true);
        const [wins, excl] = await Promise.all([fetchWindows(), fetchExcluded()]);
        setWindows(wins);
        setExcludedIds(new Set(excl));
        setLoading(false);
    }, []);

    useEffect(() => { loadData(); }, [loadData]);

    const groups = useMemo(() => groupByApp(windows), [windows]);
    const excludedCount = excludedIds.size;

    // Toggle single window exclusion
    const handleToggle = async (windowID: number) => {
        setActionLoading(`toggle-${windowID}`);
        const next = new Set(excludedIds);
        if (next.has(windowID)) {
            next.delete(windowID);
        } else {
            next.add(windowID);
        }
        const ok = await setExcluded(Array.from(next));
        if (ok) setExcludedIds(next);
        setActionLoading('');
    };

    // Exclude entire app
    const handleExcludeApp = async (bundleID: string) => {
        if (!bundleID) return;
        setActionLoading(`app-${bundleID}`);
        const ok = await excludeApp(bundleID);
        if (ok) {
            // Refresh to get updated exclusion state
            await loadData();
        }
        setActionLoading('');
    };

    // Clear all exclusions
    const handleClearAll = async () => {
        setActionLoading('clear');
        const ok = await clearExcluded();
        if (ok) setExcludedIds(new Set());
        setActionLoading('');
    };

    return (
        <div className="settings-section window-manager-section">
            <h3>{t('settings_windows')}</h3>

            {/* Toolbar */}
            <div className="window-manager-toolbar">
                <button
                    className="action-btn-sm"
                    onClick={loadData}
                    disabled={loading}
                >
                    {loading ? '...' : `🔄 ${t('windows_refresh')}`}
                </button>
                {excludedCount > 0 && (
                    <button
                        className="action-btn-sm danger"
                        onClick={handleClearAll}
                        disabled={actionLoading === 'clear'}
                    >
                        {actionLoading === 'clear' ? '...' : `🗑 ${t('windows_clear_all')}`}
                    </button>
                )}
                {excludedCount > 0 && (
                    <span className="window-excluded-badge">
                        {t('windows_excluded_count').replace('{n}', String(excludedCount))}
                    </span>
                )}
            </div>

            {/* Content */}
            {loading ? (
                <div className="provider-loading">{t('windows_loading')}</div>
            ) : windows.length === 0 ? (
                <div className="provider-empty">
                    <span className="empty-icon">🖥</span>
                    <span>{t('windows_empty')}</span>
                </div>
            ) : (
                <div className="window-groups">
                    {groups.map(group => (
                        <div key={group.bundleID || group.appName} className="window-group">
                            <div className="window-group-header">
                                <span className="window-group-name">
                                    {group.appName}
                                    <span className="window-group-count">· {group.windows.length}</span>
                                </span>
                                {group.bundleID && (
                                    <button
                                        className="action-btn-sm danger"
                                        onClick={() => handleExcludeApp(group.bundleID)}
                                        disabled={actionLoading === `app-${group.bundleID}`}
                                        title={t('windows_exclude_app_btn')}
                                    >
                                        {actionLoading === `app-${group.bundleID}` ? '...' : t('windows_exclude_app')}
                                    </button>
                                )}
                            </div>
                            <div className="window-group-list">
                                {group.windows.map(w => {
                                    const isExcluded = excludedIds.has(w.window_id);
                                    return (
                                        <div
                                            key={w.window_id}
                                            className={`window-card ${isExcluded ? 'excluded' : ''}`}
                                        >
                                            <div className="window-card-info">
                                                <span className="window-card-title">
                                                    {w.title || '(untitled)'}
                                                </span>
                                                <span className="window-card-meta">
                                                    {w.width}×{w.height} · ID {w.window_id}
                                                </span>
                                            </div>
                                            <button
                                                className={`exclude-toggle ${isExcluded ? 'excluded' : ''}`}
                                                onClick={() => handleToggle(w.window_id)}
                                                disabled={actionLoading === `toggle-${w.window_id}`}
                                            >
                                                {actionLoading === `toggle-${w.window_id}`
                                                    ? '...'
                                                    : isExcluded
                                                        ? t('windows_restore')
                                                        : t('windows_exclude')
                                                }
                                            </button>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

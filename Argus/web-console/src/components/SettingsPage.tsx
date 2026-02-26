'use client';

import { useEffect, useState, useCallback } from 'react';
import type { SystemStatus } from '@/types';
import { useT } from '@/i18n';
import WindowManager from './WindowManager';

// --- Types ---
interface ProviderView {
    name: string;
    type: string;
    endpoint: string;
    model: string;
    active: boolean;
    has_key: boolean;
}

interface ProviderHealth {
    name: string;
    healthy: boolean;
    latency_ms: number;
    last_check: string;
    error?: string;
}

interface NewProvider {
    name: string;
    type: string;
    endpoint: string;
    api_key: string;
    model: string;
    active: boolean;
}

// --- API helpers ---
const API_BASE = '/api/config/providers';

async function fetchProviders(): Promise<ProviderView[]> {
    const resp = await fetch(API_BASE);
    if (!resp.ok) return [];
    const data = await resp.json();
    return data.providers ?? [];
}

async function createProvider(p: NewProvider): Promise<boolean> {
    const resp = await fetch(API_BASE, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(p),
    });
    return resp.ok;
}

async function deleteProvider(name: string): Promise<boolean> {
    const resp = await fetch(`${API_BASE}/${name}`, { method: 'DELETE' });
    return resp.ok;
}

async function setActiveProvider(name: string): Promise<boolean> {
    const resp = await fetch(`${API_BASE}/${name}`, { method: 'PATCH' });
    return resp.ok;
}

async function fetchHealth(): Promise<Map<string, ProviderHealth>> {
    try {
        const resp = await fetch('/api/sensory/vlm/health');
        if (!resp.ok) return new Map();
        const data = await resp.json();
        const healthMap = new Map<string, ProviderHealth>();
        if (Array.isArray(data.providers)) {
            for (const p of data.providers) {
                healthMap.set(p.name, p);
            }
        }
        return healthMap;
    } catch {
        return new Map();
    }
}

// --- Constants ---
const PROVIDER_TYPES = [
    { value: 'openai', label: 'OpenAI 兼容', icon: '🤖' },
    { value: 'gemini', label: 'Google Gemini', icon: '✨' },
];

const defaultEndpoints: Record<string, string> = {
    openai: 'https://api.openai.com/v1',
    gemini: 'https://generativelanguage.googleapis.com/v1beta',
};

// --- Component ---
interface SettingsPageProps {
    status: SystemStatus | null;
}

export default function SettingsPage({ status }: SettingsPageProps) {
    const t = useT();
    const [providers, setProviders] = useState<ProviderView[]>([]);
    const [healthMap, setHealthMap] = useState<Map<string, ProviderHealth>>(new Map());
    const [loading, setLoading] = useState(true);
    const [showAddForm, setShowAddForm] = useState(false);
    const [formData, setFormData] = useState<NewProvider>({
        name: '', type: 'openai', endpoint: defaultEndpoints.openai,
        api_key: '', model: '', active: false,
    });
    const [actionLoading, setActionLoading] = useState('');

    // Fetch providers
    const loadProviders = useCallback(async () => {
        setLoading(true);
        const data = await fetchProviders();
        setProviders(data);
        setLoading(false);
    }, []);

    useEffect(() => { loadProviders(); }, [loadProviders]);

    // Poll health status every 15s
    useEffect(() => {
        const poll = async () => {
            const hm = await fetchHealth();
            setHealthMap(hm);
        };
        poll();
        const interval = setInterval(poll, 15000);
        return () => clearInterval(interval);
    }, [providers]);

    // Handlers
    const handleAdd = async () => {
        if (!formData.name || !formData.endpoint) return;
        setActionLoading('add');
        const ok = await createProvider(formData);
        if (ok) {
            setShowAddForm(false);
            setFormData({ name: '', type: 'openai', endpoint: defaultEndpoints.openai, api_key: '', model: '', active: false });
            await loadProviders();
        }
        setActionLoading('');
    };

    const handleDelete = async (name: string) => {
        setActionLoading(`del-${name}`);
        const ok = await deleteProvider(name);
        if (ok) await loadProviders();
        setActionLoading('');
    };

    const handleActivate = async (name: string) => {
        setActionLoading(`act-${name}`);
        const ok = await setActiveProvider(name);
        if (ok) await loadProviders();
        setActionLoading('');
    };

    const handleTypeChange = (type: string) => {
        setFormData(prev => ({
            ...prev,
            type,
            endpoint: defaultEndpoints[type] ?? prev.endpoint,
        }));
    };

    return (
        <div className="subpage">
            <div className="subpage-header">
                <h2>{t('settings_title')}</h2>
            </div>

            <div className="settings-grid">
                {/* VLM Providers — Dynamic */}
                <div className="settings-section provider-section">
                    <h3>🤖 {t('settings_vlm_backend')}</h3>

                    {loading ? (
                        <div className="provider-loading">加载中...</div>
                    ) : providers.length === 0 ? (
                        <div className="provider-empty">
                            <span className="empty-icon">🔌</span>
                            <span>尚未配置任何 VLM Provider</span>
                        </div>
                    ) : (
                        <div className="provider-list">
                            {providers.map(p => {
                                const health = healthMap.get(p.name);
                                return (
                                    <div key={p.name} className={`provider-card ${p.active ? 'active' : ''}`}>
                                        <div className="provider-info">
                                            <div className="provider-name-row">
                                                <span className="provider-type-icon">
                                                    {p.type === 'gemini' ? '✨' : '🤖'}
                                                </span>
                                                <strong>{p.name}</strong>
                                                {p.active && <span className="active-badge">● Active</span>}
                                                {health && (
                                                    <span className={`health-badge ${health.healthy ? 'healthy' : 'unhealthy'}`}>
                                                        {health.healthy ? '🟢' : '🔴'}
                                                        {health.healthy ? `${health.latency_ms}ms` : '离线'}
                                                    </span>
                                                )}
                                            </div>
                                            <div className="provider-details">
                                                <span className="detail-chip type">{p.type}</span>
                                                <span className="detail-chip model">{p.model || '—'}</span>
                                                <span className={`detail-chip key ${p.has_key ? 'has' : 'no'}`}>
                                                    {p.has_key ? '🔑 已配置' : '⚠️ 无密钥'}
                                                </span>
                                            </div>
                                            <code className="provider-endpoint">{p.endpoint}</code>
                                            {health?.error && (
                                                <span className="health-error">⚠ {health.error}</span>
                                            )}
                                        </div>
                                        <div className="provider-actions">
                                            {!p.active && (
                                                <button
                                                    className="action-btn-sm primary"
                                                    onClick={() => handleActivate(p.name)}
                                                    disabled={actionLoading === `act-${p.name}`}
                                                >
                                                    {actionLoading === `act-${p.name}` ? '...' : '激活'}
                                                </button>
                                            )}
                                            <button
                                                className="action-btn-sm danger"
                                                onClick={() => handleDelete(p.name)}
                                                disabled={actionLoading === `del-${p.name}`}
                                            >
                                                {actionLoading === `del-${p.name}` ? '...' : '删除'}
                                            </button>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}

                    {/* Add Provider Form */}
                    {showAddForm ? (
                        <div className="add-provider-form">
                            <h4>添加 Provider</h4>
                            <div className="form-row">
                                <label>名称</label>
                                <input
                                    value={formData.name}
                                    onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
                                    placeholder="my-openai"
                                />
                            </div>
                            <div className="form-row">
                                <label>类型</label>
                                <div className="type-selector">
                                    {PROVIDER_TYPES.map(pt => (
                                        <button
                                            key={pt.value}
                                            className={`type-btn ${formData.type === pt.value ? 'selected' : ''}`}
                                            onClick={() => handleTypeChange(pt.value)}
                                        >
                                            {pt.icon} {pt.label}
                                        </button>
                                    ))}
                                </div>
                            </div>
                            <div className="form-row">
                                <label>端点</label>
                                <input
                                    value={formData.endpoint}
                                    onChange={e => setFormData(prev => ({ ...prev, endpoint: e.target.value }))}
                                    placeholder="https://api.openai.com/v1"
                                />
                            </div>
                            <div className="form-row">
                                <label>API Key</label>
                                <input
                                    type="password"
                                    value={formData.api_key}
                                    onChange={e => setFormData(prev => ({ ...prev, api_key: e.target.value }))}
                                    placeholder="sk-..."
                                />
                            </div>
                            <div className="form-row">
                                <label>模型</label>
                                <input
                                    value={formData.model}
                                    onChange={e => setFormData(prev => ({ ...prev, model: e.target.value }))}
                                    placeholder="gpt-4o"
                                />
                            </div>
                            <div className="form-actions">
                                <button className="action-btn primary" onClick={handleAdd} disabled={actionLoading === 'add'}>
                                    {actionLoading === 'add' ? '创建中...' : '✅ 创建'}
                                </button>
                                <button className="action-btn" onClick={() => setShowAddForm(false)}>取消</button>
                            </div>
                        </div>
                    ) : (
                        <button className="add-provider-btn" onClick={() => setShowAddForm(true)}>
                            ＋ 添加 Provider
                        </button>
                    )}
                </div>

                {/* Connection Info — Static */}
                <WindowManager />

                <div className="settings-section">
                    <h3>{t('settings_connection')}</h3>
                    <div className="setting-row">
                        <span>{t('settings_ws_endpoint')}</span>
                        <code>ws://localhost:8090/ws/frames/binary</code>
                    </div>
                    <div className="setting-row">
                        <span>{t('settings_chromadb')}</span>
                        <code>http://localhost:8000</code>
                    </div>
                </div>

                {/* Display Info — From status */}
                <div className="settings-section">
                    <h3>{t('settings_display')}</h3>
                    <div className="setting-row">
                        <span>{t('settings_resolution')}</span>
                        <span>{status?.display ? `${status.display.width}×${status.display.height}` : '—'}</span>
                    </div>
                    <div className="setting-row">
                        <span>{t('settings_scale')}</span>
                        <span>{status?.display?.scale_factor ?? '—'}x</span>
                    </div>
                    <div className="setting-row">
                        <span>{t('settings_display_id')}</span>
                        <span>{status?.display?.id ?? '—'}</span>
                    </div>
                </div>

                {/* Performance — Static */}
                <div className="settings-section">
                    <h3>{t('settings_performance')}</h3>
                    <div className="setting-row">
                        <span>{t('settings_frame_protocol')}</span>
                        <span className="accent-pill">Binary WS</span>
                    </div>
                    <div className="setting-row">
                        <span>{t('settings_shm')}</span>
                        <span className="accent-pill">{t('settings_enabled')}</span>
                    </div>
                    <div className="setting-row">
                        <span>{t('settings_pipeline')}</span>
                        <span className="accent-pill">{t('settings_enabled')}</span>
                    </div>
                </div>
            </div>
        </div>
    );
}

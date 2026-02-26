'use client';

import { useState, useEffect, useCallback } from 'react';
import type { Anomaly } from '@/types';
import { useT } from '@/i18n';

const API_BASE = '/api/sensory';

type SeverityFilter = 'all' | 'high' | 'medium' | 'low';

// MonitorObservation from the Go backend
interface MonitorObservation {
    timestamp: string;
    summary: string;
    image_size_kb: number;
    latency_ms: number;
    error?: string;
}

// Convert backend MonitorObservation → frontend Anomaly
function observationToAnomaly(obs: MonitorObservation, index: number): Anomaly {
    const hasError = !!obs.error;
    const isWarning = obs.summary?.toLowerCase().includes('error') ||
        obs.summary?.toLowerCase().includes('warning') ||
        obs.summary?.toLowerCase().includes('dialog');

    return {
        id: `obs-${index}-${new Date(obs.timestamp).getTime()}`,
        type: hasError ? 'VLM Error' : 'Monitor Observation',
        severity: hasError ? 'high' : isWarning ? 'medium' : 'low',
        description: hasError ? obs.error! : obs.summary,
        timestamp: new Date(obs.timestamp).toLocaleTimeString('zh-CN'),
        frameNo: 0, // observations don't track frame numbers
    };
}

export default function AnomalyPage() {
    const [anomalies, setAnomalies] = useState<Anomaly[]>([]);
    const [filter, setFilter] = useState<SeverityFilter>('all');
    const [acknowledged, setAcknowledged] = useState<Set<string>>(new Set());
    const t = useT();

    // Fetch from backend
    const fetchObservations = useCallback(async () => {
        try {
            const resp = await fetch(`${API_BASE}/monitor/observations?limit=50`);
            if (resp.ok) {
                const data: MonitorObservation[] = await resp.json();
                if (Array.isArray(data)) {
                    setAnomalies(data.map((obs, i) => observationToAnomaly(obs, i)));
                }
            }
        } catch { /* backend unavailable */ }
    }, []);

    // Poll every 10 seconds
    useEffect(() => {
        fetchObservations();
        const interval = setInterval(fetchObservations, 10000);
        return () => clearInterval(interval);
    }, [fetchObservations]);

    const handleAcknowledge = (id: string) => {
        setAcknowledged(prev => new Set(prev).add(id));
    };

    const visibleAnomalies = anomalies
        .filter(a => !acknowledged.has(a.id))
        .filter(a => filter === 'all' || a.severity === filter);

    const counts = {
        high: anomalies.filter(a => a.severity === 'high' && !acknowledged.has(a.id)).length,
        medium: anomalies.filter(a => a.severity === 'medium' && !acknowledged.has(a.id)).length,
        low: anomalies.filter(a => a.severity === 'low' && !acknowledged.has(a.id)).length,
    };

    return (
        <div className="subpage">
            <div className="subpage-header">
                <h2>{t('anomaly_title')}</h2>
                <div className="task-stats">
                    <span className="stat-badge high">{counts.high} {t('anomaly_high')}</span>
                    <span className="stat-badge medium">{counts.medium} {t('anomaly_medium')}</span>
                    <span className="stat-badge low">{counts.low} {t('anomaly_low')}</span>
                </div>
            </div>

            {/* Severity filter tabs */}
            <div className="filter-tabs">
                {(['all', 'high', 'medium', 'low'] as const).map(f => (
                    <button
                        key={f}
                        className={`filter-tab ${filter === f ? 'active' : ''}`}
                        onClick={() => setFilter(f)}
                    >
                        {f === 'all' ? t('anomaly_filter') :
                            f === 'high' ? t('anomaly_high') :
                                f === 'medium' ? t('anomaly_medium') : t('anomaly_low')}
                    </button>
                ))}
            </div>

            {visibleAnomalies.length === 0 ? (
                <div className="empty-state">
                    <span className="empty-icon">✅</span>
                    <p>{t('anomaly_normal')}</p>
                    <p className="empty-hint">{t('anomaly_normal_hint')}</p>
                </div>
            ) : (
                <div className="anomaly-list">
                    {visibleAnomalies.map(a => (
                        <div key={a.id} className={`anomaly-card severity-${a.severity}`}>
                            <div className="anomaly-icon">
                                {a.severity === 'high' ? '🔴' : a.severity === 'medium' ? '🟡' : '🟢'}
                            </div>
                            <div className="anomaly-content">
                                <div className="anomaly-type">{a.type}</div>
                                <div className="anomaly-desc">{a.description}</div>
                                <div className="anomaly-meta">
                                    {a.timestamp}
                                </div>
                            </div>
                            <button
                                className="action-btn-sm"
                                onClick={() => handleAcknowledge(a.id)}
                                title={t('anomaly_acknowledge')}
                            >
                                ✓
                            </button>
                            <div className={`severity-badge ${a.severity}`}>
                                {a.severity.toUpperCase()}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

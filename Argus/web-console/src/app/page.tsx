'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import type { SystemStatus, HITLMessage, Keyframe } from '@/types';
import { useBinaryFrameStream } from '@/hooks/useBinaryFrameStream';
import { useT } from '@/i18n';
import TimelinePage from '@/components/TimelinePage';
import TasksPage from '@/components/TasksPage';
import AnomalyPage from '@/components/AnomalyPage';
import SettingsPage from '@/components/SettingsPage';
import LangSwitch from '@/components/LangSwitch';

const API_BASE = '/api/sensory';

// --- Main Page (orchestrator only) ---
export default function DashboardPage() {
    const canvasRef = useRef<HTMLCanvasElement | null>(null);
    const [status, setStatus] = useState<SystemStatus | null>(null);
    const [activeNav, setActiveNav] = useState('dashboard');
    const t = useT();

    const [messages, setMessages] = useState<HITLMessage[]>([
        { id: 1, role: 'agent', text: t('hitl_system_ready'), time: '00:00' },
    ]);
    const [inputText, setInputText] = useState('');
    const [keyframes, setKeyframes] = useState<Keyframe[]>([]);
    const [anomalyCount, setAnomalyCount] = useState(0);

    const { connected, fps, frameNo } = useBinaryFrameStream(
        'ws://localhost:8090/ws/frames/binary',
        canvasRef
    );

    // Fetch status + dashboard data periodically
    useEffect(() => {
        const fetchAll = async () => {
            try {
                const [statusResp, kfResp, obsResp] = await Promise.all([
                    fetch(`${API_BASE}/status`).catch(() => null),
                    fetch(`${API_BASE}/pipeline/keyframes?n=20`).catch(() => null),
                    fetch(`${API_BASE}/monitor/observations?limit=50`).catch(() => null),
                ]);
                if (statusResp?.ok) setStatus(await statusResp.json());
                if (kfResp?.ok) {
                    const kfData = await kfResp.json();
                    if (kfData.keyframes) {
                        setKeyframes(kfData.keyframes.map((kf: Record<string, unknown>, i: number) => ({
                            id: (kf.metadata as Record<string, unknown>)?.keyframe_id ?? i,
                            frameNo: kf.frame_no,
                            timestamp: kf.timestamp,
                            thumbnail: kf.thumbnail_b64 || '',
                            reason: kf.trigger_reason,
                        })));
                    }
                }
                if (obsResp?.ok) {
                    const obsData = await obsResp.json();
                    if (Array.isArray(obsData)) setAnomalyCount(obsData.filter((o: Record<string, unknown>) => !!o.error).length);
                }
            } catch { }
        };
        fetchAll();
        const interval = setInterval(fetchAll, 5000);
        return () => clearInterval(interval);
    }, []);

    // Handle HITL message send — calls backend VLM and creates a task
    const handleSendMessage = useCallback(async () => {
        if (!inputText.trim()) return;
        const cmdText = inputText.trim();

        const newMsg: HITLMessage = {
            id: Date.now(),
            role: 'human',
            text: cmdText,
            time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
        };
        setMessages(prev => [...prev, newMsg]);
        setInputText('');

        // Create a task via backend API
        let taskId = '';
        try {
            const taskResp = await fetch(`${API_BASE}/tasks`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ goal: cmdText }),
            });
            if (taskResp.ok) {
                const task = await taskResp.json();
                taskId = task.id;
                // Immediately update to running
                await fetch(`${API_BASE}/tasks/${taskId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ status: 'running' }),
                });
            }
        } catch { }

        // Show "executing" in chat
        setMessages(prev => [...prev, {
            id: Date.now() + 1,
            role: 'agent',
            text: `${t('hitl_received')}："${cmdText}"。${t('hitl_executing')}`,
            time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
        }]);

        // Call VLM with the user's command
        const startTime = Date.now();
        try {
            const resp = await fetch('http://localhost:8090/v1/chat/completions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    messages: [{ role: 'user', content: cmdText }],
                }),
            });
            const data = await resp.json();
            const reply = data.choices?.[0]?.message?.content || JSON.stringify(data);
            const elapsed = Math.round((Date.now() - startTime) / 1000);

            // Update task as done via backend
            if (taskId) {
                fetch(`${API_BASE}/tasks/${taskId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ status: 'done', steps: 1, duration: `${elapsed}s` }),
                }).catch(() => { });
            }

            // Add VLM response to chat
            setMessages(prev => [...prev, {
                id: Date.now() + 2,
                role: 'agent',
                text: `✅ ${reply}`,
                time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
            }]);
        } catch (err) {
            const elapsed = Math.round((Date.now() - startTime) / 1000);
            if (taskId) {
                fetch(`${API_BASE}/tasks/${taskId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ status: 'failed', duration: `${elapsed}s` }),
                }).catch(() => { });
            }
            setMessages(prev => [...prev, {
                id: Date.now() + 2,
                role: 'agent',
                text: `❌ 执行失败: ${err instanceof Error ? err.message : String(err)}`,
                time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
            }]);
        }
    }, [inputText, t]);

    // Canvas click handler
    const handleCanvasClick = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const rect = canvas.getBoundingClientRect();
        const x = Math.round((e.clientX - rect.left) * (canvas.width / rect.width));
        const y = Math.round((e.clientY - rect.top) * (canvas.height / rect.height));
        setMessages(prev => [...prev, {
            id: Date.now(), role: 'human',
            text: `${t('hitl_click_coord')} (${x}, ${y})`,
            time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
        }]);
    }, [t]);

    const navItems = [
        { id: 'dashboard', icon: '📊', label: t('nav_dashboard') },
        { id: 'timeline', icon: '🕐', label: t('nav_timeline') },
        { id: 'tasks', icon: '📋', label: t('nav_tasks') },
        { id: 'anomaly', icon: '⚠️', label: t('nav_anomaly') },
        { id: 'settings', icon: '⚙️', label: t('nav_settings') },
    ];

    // --- Sub-page routing ---
    const renderContent = () => {
        switch (activeNav) {
            case 'timeline':
                return <TimelinePage />;
            case 'tasks':
                return <TasksPage />;
            case 'anomaly':
                return <AnomalyPage />;
            case 'settings':
                return <SettingsPage status={status} />;
            default:
                return renderDashboard();
        }
    };

    // --- Dashboard view ---
    const renderDashboard = () => (
        <>
            <div className="metrics-row">
                <div className="metric-card">
                    <div className="metric-label">{t('metric_frame_no')}</div>
                    <div className="metric-value cyan">{frameNo.toLocaleString()}</div>
                </div>
                <div className="metric-card">
                    <div className="metric-label">{t('metric_fps')}</div>
                    <div className="metric-value green">{fps}</div>
                </div>
                <div className="metric-card">
                    <div className="metric-label">{t('metric_uptime')}</div>
                    <div className="metric-value purple">{status?.uptime?.split('.')[0] || '—'}</div>
                </div>
                <div className="metric-card">
                    <div className="metric-label">{t('metric_resolution')}</div>
                    <div className="metric-value amber">
                        {status?.display ? `${status.display.width}×${status.display.height}` : '—'}
                    </div>
                </div>
                <div className="metric-card">
                    <div className="metric-label">{t('metric_keyframes')}</div>
                    <div className="metric-value cyan">{keyframes.length}</div>
                </div>
                <div className="metric-card">
                    <div className="metric-label">{t('metric_anomalies')}</div>
                    <div className="metric-value" style={{ color: anomalyCount > 0 ? 'var(--accent-red)' : 'var(--accent-green)' }}>
                        {anomalyCount}
                    </div>
                </div>
            </div>

            <div className="screen-canvas-container">
                <canvas ref={canvasRef} className="screen-canvas" onClick={handleCanvasClick} />
                <div className="canvas-overlay">
                    <span className="canvas-badge live">{t('canvas_live')}</span>
                    <span className="canvas-badge fps">{fps} FPS</span>
                    <span className="canvas-badge protocol">{t('canvas_protocol')}</span>
                </div>
            </div>

            <div className="control-panel">
                <div className="panel-card">
                    <div className="card-header"><span className="card-title">{t('quick_actions')}</span></div>
                    <div className="actions-grid">
                        <button className="action-btn primary"><span className="btn-icon">📸</span> {t('action_screenshot')}</button>
                        <button className="action-btn"><span className="btn-icon">🔍</span> {t('action_ocr')}</button>
                        <button className="action-btn"><span className="btn-icon">📝</span> {t('action_describe')}</button>
                        <button className="action-btn"><span className="btn-icon">⚠️</span> {t('action_scan_anomaly')}</button>
                        <button className="action-btn"><span className="btn-icon">🎯</span> {t('action_locate')}</button>
                        <button className="action-btn"><span className="btn-icon">⏸</span> {t('action_pause')}</button>
                    </div>
                </div>
                <div className="panel-card">
                    <div className="card-header"><span className="card-title">{t('hitl_title')}</span></div>
                    <div className="hitl-chat">
                        {messages.map(msg => (
                            <div key={msg.id} className={`hitl-message ${msg.role}`}>
                                <strong>{msg.role === 'agent' ? '🤖' : '👤'}</strong> {msg.text}
                            </div>
                        ))}
                    </div>
                    <div className="hitl-input-row">
                        <input
                            className="hitl-input" value={inputText}
                            onChange={e => setInputText(e.target.value)}
                            onKeyDown={e => e.key === 'Enter' && handleSendMessage()}
                            placeholder={t('hitl_placeholder')}
                        />
                        <button className="hitl-send" onClick={handleSendMessage}>{t('hitl_send')}</button>
                    </div>
                </div>
            </div>

            <div className="timeline-container">
                <div className="timeline-header">
                    <span className="card-title">{t('timeline_title')}</span>
                    <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                        {frameNo} {t('timeline_frames')} · {keyframes.length} {t('timeline_keyframes')}
                    </span>
                </div>
                <div className="timeline-track">
                    <div className="timeline-thumb">
                        {keyframes.slice(-20).map(kf => (
                            <div key={kf.id} className="timeline-kf-thumb" title={`#${kf.frameNo}: ${kf.reason}`}>
                                {kf.thumbnail ? <img src={kf.thumbnail} alt="" /> : <span className="kf-mini-label">#{kf.frameNo}</span>}
                            </div>
                        ))}
                    </div>
                    <div className="timeline-scrubber" />
                </div>
            </div>
        </>
    );

    return (
        <div className="app-shell">
            <header className="top-bar">
                <div className="logo">
                    <span className="eye-icon">👁</span>
                    {t('app_title')}
                </div>
                <div className="top-bar-right">
                    <LangSwitch />
                    <div className={`status-chip ${connected ? 'online' : 'offline'}`}>
                        <span className="status-dot" />
                        {connected ? t('online') : t('offline')}
                    </div>
                    <div className="status-chip info">🔗 {status?.connected_clients ?? 0} {t('clients')}</div>
                </div>
            </header>

            <nav className="sidebar">
                <div className="nav-section">{t('nav_monitor')}</div>
                {navItems.slice(0, 3).map(item => (
                    <div key={item.id} className={`nav-item ${activeNav === item.id ? 'active' : ''}`}
                        onClick={() => setActiveNav(item.id)}>
                        <span className="icon">{item.icon}</span>{item.label}
                    </div>
                ))}
                <div className="nav-section">{t('nav_manage')}</div>
                {navItems.slice(3).map(item => (
                    <div key={item.id} className={`nav-item ${activeNav === item.id ? 'active' : ''}`}
                        onClick={() => setActiveNav(item.id)}>
                        <span className="icon">{item.icon}</span>{item.label}
                    </div>
                ))}
            </nav>

            <main className="main-content">{renderContent()}</main>
        </div>
    );
}

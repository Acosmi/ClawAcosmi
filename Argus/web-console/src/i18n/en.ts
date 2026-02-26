/**
 * English language pack
 * All UI text in English
 */
import type { LangKeys } from './zh';

const en: Record<LangKeys, string> = {
    // === Global ===
    app_title: 'Argus — 24-Hour Eye',
    app_description: 'Intelligent Agent Visual Cortex Control Center',
    online: 'Online',
    offline: 'Offline',
    clients: 'Clients',

    // === Navigation ===
    nav_monitor: 'Monitor',
    nav_manage: 'Manage',
    nav_dashboard: 'Dashboard',
    nav_timeline: 'Timeline',
    nav_tasks: 'Tasks',
    nav_anomaly: 'Anomaly',
    nav_settings: 'Settings',

    // === Dashboard ===
    metric_frame_no: 'Frame #',
    metric_fps: 'Live FPS',
    metric_uptime: 'Uptime',
    metric_resolution: 'Resolution',
    metric_keyframes: 'Keyframes',
    metric_anomalies: 'Anomalies',

    // === Canvas ===
    canvas_live: '● LIVE',
    canvas_protocol: 'BIN',

    // === Quick Actions ===
    quick_actions: '⚡ Quick Actions',
    action_screenshot: 'Screenshot',
    action_ocr: 'OCR Extract',
    action_describe: 'Describe Scene',
    action_scan_anomaly: 'Scan Anomaly',
    action_locate: 'Locate Element',
    action_pause: 'Pause/Resume',

    // === HITL ===
    hitl_title: '🤖 HITL Interaction',
    hitl_placeholder: 'Enter command...',
    hitl_send: 'Send',
    hitl_system_ready: 'System ready. Awaiting commands...',
    hitl_received: 'Command received',
    hitl_executing: 'Executing...',
    hitl_click_coord: '🖱 Click at',

    // === Timeline ===
    timeline_title: '🕐 Keyframe Timeline',
    timeline_search: 'Search keyframes...',
    timeline_empty: 'No keyframe data',
    timeline_empty_hint: 'Keyframes will be captured automatically after system starts',
    timeline_frames: 'frames',
    timeline_keyframes: 'keyframes',
    timeline_loading: 'Loading...',
    timeline_auto_refresh: 'Auto refresh',

    // === Tasks ===
    tasks_title: '📋 Task Queue',
    tasks_running: 'Running',
    tasks_pending: 'Pending',
    tasks_done: 'Done',
    tasks_empty: 'No tasks',
    tasks_empty_hint: 'Send commands via HITL panel to create tasks',
    tasks_steps: 'steps',
    task_status_running: '⏳ Running',
    task_status_done: '✅ Done',
    task_status_failed: '❌ Failed',
    task_status_pending: '⏸ Pending',
    tasks_cancel: 'Cancel',
    tasks_retry: 'Retry',
    tasks_details: 'Details',
    tasks_filter_all: 'All',

    // === Anomaly ===
    anomaly_title: '⚠️ Anomaly Detection',
    anomaly_high: 'High',
    anomaly_medium: 'Medium',
    anomaly_low: 'Low',
    anomaly_normal: 'System running normally',
    anomaly_normal_hint: 'Anomaly detection active, all clear',
    anomaly_acknowledge: 'Acknowledge',
    anomaly_silence: 'Silence',
    anomaly_filter: 'All',
    anomaly_source: 'Source',
    anomaly_loading: 'Loading...',

    // === Settings ===
    settings_title: '⚙️ System Settings',
    settings_connection: '📡 Connection',
    settings_ws_endpoint: 'WebSocket Endpoint',
    settings_vlm_endpoint: 'VLM API Endpoint',
    settings_chromadb: 'ChromaDB',
    settings_display: '🖥 Display',
    settings_resolution: 'Resolution',
    settings_scale: 'Scale Factor',
    settings_display_id: 'Display ID',
    settings_vlm_backend: '🤖 VLM Backend',
    settings_current_backend: 'Current Backend',
    settings_available_backends: 'Available Backends',
    settings_performance: '📊 Performance',
    settings_frame_protocol: 'Frame Protocol',
    settings_shm: 'SHM Zero-Copy',
    settings_pipeline: 'Async Pipeline',
    settings_enabled: 'Enabled',

    // === Window Exclusion ===
    settings_windows: '🔲 Window Exclusion',
    windows_refresh: 'Refresh',
    windows_exclude_app: 'Exclude App',
    windows_clear_all: 'Clear All',
    windows_excluded_count: '{n} window(s) excluded',
    windows_loading: 'Loading windows...',
    windows_empty: 'No windows detected',
    windows_exclude: 'Exclude',
    windows_restore: 'Restore',
    windows_exclude_app_btn: 'Exclude all windows from this app',

    // === Language Switch ===
    lang_zh: '中文',
    lang_en: 'EN',
};

export default en;

/**
 * 中文语言包（默认语言）
 * 所有 UI 文本的中文定义
 */
const zh = {
    // === 全局 ===
    app_title: 'Argus — 24小时之眼',
    app_description: '智能体视觉皮层控制中心',
    online: '在线',
    offline: '离线',
    clients: '客户端',

    // === 导航 ===
    nav_monitor: '监控',
    nav_manage: '管理',
    nav_dashboard: '控制面板',
    nav_timeline: '时间线',
    nav_tasks: '任务队列',
    nav_anomaly: '异常检测',
    nav_settings: '系统设置',

    // === 控制面板 ===
    metric_frame_no: '帧编号',
    metric_fps: '实时FPS',
    metric_uptime: '运行时间',
    metric_resolution: '分辨率',
    metric_keyframes: '关键帧',
    metric_anomalies: '异常',

    // === 画布 ===
    canvas_live: '● LIVE',
    canvas_protocol: 'BIN',

    // === 快捷操作 ===
    quick_actions: '⚡ 快捷操作',
    action_screenshot: '截图',
    action_ocr: 'OCR提取',
    action_describe: '场景描述',
    action_scan_anomaly: '异常扫描',
    action_locate: '元素定位',
    action_pause: '暂停/继续',

    // === HITL 人机交互 ===
    hitl_title: '🤖 HITL 人机交互',
    hitl_placeholder: '输入指令...',
    hitl_send: '发送',
    hitl_system_ready: '系统就绪。等待指令...',
    hitl_received: '收到指令',
    hitl_executing: '正在执行...',
    hitl_click_coord: '🖱 点击坐标',

    // === 关键帧时间线 ===
    timeline_title: '🕐 关键帧时间线',
    timeline_search: '搜索关键帧...',
    timeline_empty: '暂无关键帧数据',
    timeline_empty_hint: '系统运行后将自动捕获关键帧',
    timeline_frames: '帧',
    timeline_keyframes: '关键帧',
    timeline_loading: '加载中...',
    timeline_auto_refresh: '自动刷新',

    // === 任务队列 ===
    tasks_title: '📋 任务队列',
    tasks_running: '运行中',
    tasks_pending: '等待中',
    tasks_done: '已完成',
    tasks_empty: '暂无任务',
    tasks_empty_hint: '通过 HITL 面板发送指令创建任务',
    tasks_steps: '步骤',
    task_status_running: '⏳ 运行中',
    task_status_done: '✅ 完成',
    task_status_failed: '❌ 失败',
    task_status_pending: '⏸ 等待',
    tasks_cancel: '取消',
    tasks_retry: '重试',
    tasks_details: '详情',
    tasks_filter_all: '全部',

    // === 异常检测 ===
    anomaly_title: '⚠️ 异常检测',
    anomaly_high: '高危',
    anomaly_medium: '中危',
    anomaly_low: '低危',
    anomaly_normal: '系统运行正常',
    anomaly_normal_hint: '异常检测运行中，一切正常',
    anomaly_acknowledge: '确认',
    anomaly_silence: '静默',
    anomaly_filter: '全部',
    anomaly_source: '来源',
    anomaly_loading: '加载中...',

    // === 系统设置 ===
    settings_title: '⚙️ 系统设置',
    settings_connection: '📡 连接',
    settings_ws_endpoint: 'WebSocket 端点',
    settings_vlm_endpoint: 'VLM API 端点',
    settings_chromadb: 'ChromaDB',
    settings_display: '🖥 显示器',
    settings_resolution: '分辨率',
    settings_scale: '缩放比',
    settings_display_id: '显示器 ID',
    settings_vlm_backend: '🤖 VLM 后端',
    settings_current_backend: '当前后端',
    settings_available_backends: '可用后端',
    settings_performance: '📊 性能',
    settings_frame_protocol: '帧传输协议',
    settings_shm: 'SHM 零拷贝',
    settings_pipeline: '异步 Pipeline',
    settings_enabled: '启用',

    // === 窗口排除 ===
    settings_windows: '🔲 窗口排除',
    windows_refresh: '刷新',
    windows_exclude_app: '排除应用',
    windows_clear_all: '清除全部',
    windows_excluded_count: '已排除 {n} 个窗口',
    windows_loading: '加载窗口列表...',
    windows_empty: '未检测到可用窗口',
    windows_exclude: '排除',
    windows_restore: '恢复',
    windows_exclude_app_btn: '排除此应用全部窗口',

    // === 语言切换 ===
    lang_zh: '中文',
    lang_en: 'EN',
} as const;

export type LangKeys = keyof typeof zh;
export default zh;

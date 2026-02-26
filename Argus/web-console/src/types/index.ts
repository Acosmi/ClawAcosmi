// --- Shared Types for Argus Web Console ---

export interface SystemStatus {
    status: string;
    uptime: string;
    capturing: boolean;
    display: { id: number; width: number; height: number; scale_factor: number };
    frame_no: number;
    connected_clients: number;
}

export interface HITLMessage {
    id: number;
    role: 'agent' | 'human';
    text: string;
    time: string;
}

export interface Keyframe {
    id: number;
    frameNo: number;
    timestamp: number;
    thumbnail: string; // data URL
    reason: string;
}

export interface TaskItem {
    id: string;
    goal: string;
    status: 'pending' | 'running' | 'done' | 'failed';
    steps: number;
    startedAt: string;
    duration: string;
}

export interface Anomaly {
    id: string;
    type: string;
    severity: 'low' | 'medium' | 'high';
    description: string;
    timestamp: string;
    frameNo: number;
}

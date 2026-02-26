'use client';

import { useEffect, useRef, useState, useCallback } from 'react';

/**
 * Binary WebSocket frame stream hook with auto-reconnection.
 *
 * Supports both binary protocol (20-byte header + JPEG) and
 * JSON fallback for backward compatibility.
 */
export function useBinaryFrameStream(
    url: string,
    canvasRef: React.RefObject<HTMLCanvasElement | null>
) {
    const [connected, setConnected] = useState(false);
    const [fps, setFps] = useState(0);
    const [frameNo, setFrameNo] = useState(0);
    const wsRef = useRef<WebSocket | null>(null);
    const frameCountRef = useRef(0);
    const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const mountedRef = useRef(true);

    const connect = useCallback(() => {
        if (!mountedRef.current) return;

        try {
            const ws = new WebSocket(url);
            ws.binaryType = 'arraybuffer';
            wsRef.current = ws;

            ws.onopen = () => {
                console.log('[FrameStream] Connected to', url);
                setConnected(true);
            };

            ws.onmessage = (event) => {
                try {
                    if (event.data instanceof ArrayBuffer) {
                        // Binary protocol: [4B width][4B height][8B frameNo][4B jpegSize][JPEG]
                        const view = new DataView(event.data);
                        const width = view.getUint32(0, true);
                        const height = view.getUint32(4, true);
                        const fno = Number(view.getBigUint64(8, true));
                        const jpegSize = view.getUint32(16, true);
                        const jpegData = new Uint8Array(event.data, 20, jpegSize);

                        frameCountRef.current++;
                        setFrameNo(fno);

                        const canvas = canvasRef.current;
                        if (!canvas) return;
                        const ctx = canvas.getContext('2d');
                        if (!ctx) return;

                        const blob = new Blob([jpegData], { type: 'image/jpeg' });
                        const bitmapUrl = URL.createObjectURL(blob);
                        const img = new Image();
                        img.onload = () => {
                            canvas.width = width;
                            canvas.height = height;
                            ctx.drawImage(img, 0, 0);
                            URL.revokeObjectURL(bitmapUrl);
                        };
                        img.src = bitmapUrl;
                    } else {
                        // Fallback: JSON protocol
                        const msg = JSON.parse(event.data);
                        if (msg.type === 'frame' && msg.image_b64) {
                            frameCountRef.current++;
                            setFrameNo(msg.frame_no);

                            const canvas = canvasRef.current;
                            if (!canvas) return;
                            const ctx = canvas.getContext('2d');
                            if (!ctx) return;

                            const img = new Image();
                            img.onload = () => {
                                canvas.width = img.width;
                                canvas.height = img.height;
                                ctx.drawImage(img, 0, 0);
                            };
                            img.src = `data:image/jpeg;base64,${msg.image_b64}`;
                        }
                    }
                } catch (err) {
                    console.error('[FrameStream] Message parse error:', err);
                }
            };

            ws.onclose = (ev) => {
                console.log('[FrameStream] Closed:', ev.code, ev.reason);
                setConnected(false);
                // Auto-reconnect after 2 seconds
                if (mountedRef.current) {
                    reconnectTimerRef.current = setTimeout(connect, 2000);
                }
            };

            ws.onerror = (err) => {
                console.error('[FrameStream] Error:', err);
                // onclose will fire after onerror, handling reconnection
            };
        } catch (err) {
            console.error('[FrameStream] Connect failed:', err);
            setConnected(false);
            if (mountedRef.current) {
                reconnectTimerRef.current = setTimeout(connect, 2000);
            }
        }
    }, [url, canvasRef]);

    useEffect(() => {
        mountedRef.current = true;

        // FPS counter
        const fpsInterval = setInterval(() => {
            setFps(frameCountRef.current);
            frameCountRef.current = 0;
        }, 1000);

        connect();

        return () => {
            mountedRef.current = false;
            clearInterval(fpsInterval);
            if (reconnectTimerRef.current) {
                clearTimeout(reconnectTimerRef.current);
            }
            wsRef.current?.close();
        };
    }, [connect]);

    return { connected, fps, frameNo };
}

/**
 * voice-recorder.ts — MediaRecorder 封装
 * 提供录音/停止/取消功能，返回 Blob + 元数据。
 */

/** 检测浏览器支持的录音 MIME 类型 */
export function getRecorderMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "";
  const preferred = [
    "audio/webm;codecs=opus", // Chrome/Firefox/Safari 18.4+
    "audio/webm",
    "audio/mp4", // 旧 Safari fallback
  ];
  for (const mime of preferred) {
    if (MediaRecorder.isTypeSupported(mime)) return mime;
  }
  return "";
}

/** 录音是否可用 */
export function isVoiceRecordingSupported(): boolean {
  return typeof navigator !== "undefined" && !!navigator.mediaDevices?.getUserMedia && getRecorderMimeType() !== "";
}

export type VoiceRecordingResult = {
  blob: Blob;
  mimeType: string;
  durationMs: number;
};

export class VoiceRecorder {
  private recorder: MediaRecorder | null = null;
  private chunks: Blob[] = [];
  private stream: MediaStream | null = null;
  private startTime = 0;
  private _durationTimer: ReturnType<typeof setInterval> | null = null;
  private _onDurationUpdate: ((seconds: number) => void) | null = null;

  get isRecording(): boolean {
    return this.recorder?.state === "recording";
  }

  /** 开始录音 */
  async start(onDurationUpdate?: (seconds: number) => void): Promise<void> {
    if (this.isRecording) return;

    const mimeType = getRecorderMimeType();
    if (!mimeType) throw new Error("No supported audio MIME type");

    this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    this.chunks = [];
    this.startTime = Date.now();
    this._onDurationUpdate = onDurationUpdate ?? null;

    this.recorder = new MediaRecorder(this.stream, { mimeType });
    this.recorder.ondataavailable = (e) => {
      if (e.data.size > 0) this.chunks.push(e.data);
    };
    this.recorder.start(250); // 每 250ms 收集一个 chunk

    // 更新计时器
    if (this._onDurationUpdate) {
      this._durationTimer = setInterval(() => {
        const elapsed = Math.floor((Date.now() - this.startTime) / 1000);
        this._onDurationUpdate?.(elapsed);
      }, 500);
    }
  }

  /** 停止录音并返回结果 */
  stop(): Promise<VoiceRecordingResult> {
    return new Promise((resolve, reject) => {
      if (!this.recorder || this.recorder.state === "inactive") {
        reject(new Error("Not recording"));
        return;
      }

      this.recorder.onstop = () => {
        const durationMs = Date.now() - this.startTime;
        const mimeType = this.recorder?.mimeType ?? "audio/webm";
        const blob = new Blob(this.chunks, { type: mimeType });
        this.cleanup();
        resolve({ blob, mimeType, durationMs });
      };

      this.recorder.stop();
    });
  }

  /** 取消录音（丢弃数据） */
  cancel(): void {
    if (this.recorder && this.recorder.state !== "inactive") {
      this.recorder.onstop = null;
      this.recorder.stop();
    }
    this.cleanup();
  }

  private cleanup(): void {
    if (this._durationTimer) {
      clearInterval(this._durationTimer);
      this._durationTimer = null;
    }
    this._onDurationUpdate = null;
    if (this.stream) {
      for (const track of this.stream.getTracks()) track.stop();
      this.stream = null;
    }
    this.recorder = null;
    this.chunks = [];
  }
}

/** 将 Blob 转为 base64 字符串（不含 data: 前缀） */
export function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => {
      const result = reader.result as string;
      const idx = result.indexOf(",");
      resolve(idx >= 0 ? result.substring(idx + 1) : result);
    };
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
}

/** 将 Blob 转为 data URL */
export function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => resolve(reader.result as string);
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
}

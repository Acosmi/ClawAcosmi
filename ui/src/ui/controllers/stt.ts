import type { GatewayBrowserClient } from "../gateway.ts";

/**
 * transcribeAudio 调用后端 stt.transcribe RPC 将音频转录为文本。
 * @param client   Gateway 客户端
 * @param audioBase64  base64 编码的音频数据（不含 data: 前缀）
 * @param mimeType 音频 MIME 类型（如 "audio/webm"）
 * @returns 转录文本
 */
export async function transcribeAudio(
  client: GatewayBrowserClient,
  audioBase64: string,
  mimeType: string,
): Promise<string> {
  const resp = await client.request<{ text?: string }>("stt.transcribe", {
    audio: audioBase64,
    mimeType,
  });
  return resp?.text ?? "";
}

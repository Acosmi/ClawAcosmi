package understanding

import "math"

// TS 对照: media-understanding/video.ts (11L)

// EstimateBase64Size 估算给定字节数的 Base64 编码大小。
// TS 对照: video.ts L3-5
func EstimateBase64Size(rawBytes int) int {
	return int(math.Ceil(float64(rawBytes)*4/3)) + 4
}

// ResolveVideoMaxBase64Bytes 解析视频最大 Base64 字节数。
// 如果 maxBytes <= 0 则使用默认值。
// TS 对照: video.ts L7-11
func ResolveVideoMaxBase64Bytes(maxBytes int) int {
	if maxBytes <= 0 {
		return DefaultVideoMaxBase64Bytes
	}
	return maxBytes
}

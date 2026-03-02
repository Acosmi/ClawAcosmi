package media

// ============================================================================
// media/image_exif.go — EXIF 方向读取
//
// 从 image_ops.go 拆分：EXIF 解析独立模块。
// TS 对照: image-ops.ts L30-125
// ============================================================================

import (
	"encoding/binary"
)

// ---------- EXIF 方向读取 ----------

// ReadJpegExifOrientation 从 JPEG buffer 读取 EXIF 方向值 (1-8)。
// 返回 0 表示未找到或非 JPEG。
// TS 对照: image-ops.ts L30-125
func ReadJpegExifOrientation(buffer []byte) int {
	if len(buffer) < 12 {
		return 0
	}
	// JPEG SOI 标记
	if buffer[0] != 0xFF || buffer[1] != 0xD8 {
		return 0
	}

	pos := 2
	for pos+4 < len(buffer) {
		if buffer[pos] != 0xFF {
			break
		}
		marker := buffer[pos+1]
		// APP1 (EXIF)
		if marker == 0xE1 {
			return readExifOrientation(buffer, pos)
		}
		// 跳过其他段
		if pos+4 > len(buffer) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(buffer[pos+2 : pos+4]))
		pos += 2 + segLen
	}
	return 0
}

// readExifOrientation 从 APP1 段中解析 EXIF 方向。
func readExifOrientation(buffer []byte, app1Start int) int {
	if app1Start+4 > len(buffer) {
		return 0
	}
	segLen := int(binary.BigEndian.Uint16(buffer[app1Start+2 : app1Start+4]))
	segEnd := app1Start + 2 + segLen
	if segEnd > len(buffer) {
		segEnd = len(buffer)
	}
	exifStart := app1Start + 4
	// 检查 "Exif\0\0"
	if exifStart+6 > segEnd {
		return 0
	}
	if string(buffer[exifStart:exifStart+4]) != "Exif" {
		return 0
	}

	tiffStart := exifStart + 6
	if tiffStart+8 > segEnd {
		return 0
	}

	// 字节序
	var bigEndian bool
	if buffer[tiffStart] == 'M' && buffer[tiffStart+1] == 'M' {
		bigEndian = true
	} else if buffer[tiffStart] == 'I' && buffer[tiffStart+1] == 'I' {
		bigEndian = false
	} else {
		return 0
	}

	readU16 := func(offset int) uint16 {
		if offset+2 > segEnd {
			return 0
		}
		if bigEndian {
			return binary.BigEndian.Uint16(buffer[offset : offset+2])
		}
		return binary.LittleEndian.Uint16(buffer[offset : offset+2])
	}

	// IFD0 偏移
	ifdOffsetRaw := tiffStart + 4
	if ifdOffsetRaw+4 > segEnd {
		return 0
	}
	var ifdOffset uint32
	if bigEndian {
		ifdOffset = binary.BigEndian.Uint32(buffer[ifdOffsetRaw : ifdOffsetRaw+4])
	} else {
		ifdOffset = binary.LittleEndian.Uint32(buffer[ifdOffsetRaw : ifdOffsetRaw+4])
	}

	ifdPos := tiffStart + int(ifdOffset)
	if ifdPos+2 > segEnd {
		return 0
	}
	numEntries := int(readU16(ifdPos))
	ifdPos += 2

	for i := 0; i < numEntries; i++ {
		entryPos := ifdPos + i*12
		if entryPos+12 > segEnd {
			break
		}
		tag := readU16(entryPos)
		if tag == 0x0112 { // Orientation
			value := readU16(entryPos + 8)
			if value >= 1 && value <= 8 {
				return int(value)
			}
		}
	}
	return 0
}

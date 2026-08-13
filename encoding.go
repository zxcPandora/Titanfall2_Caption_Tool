// encoding.go - UTF-8 / UTF-16LE 编码检测与转换
package main

import (
	"strings"
	"unicode/utf16"
)

// UTF16LEToString - UTF-16LE 字节 → Go string (UTF-8)
func UTF16LEToString(b []byte) string {
	// 每 2 字节一个 UTF-16 code unit
	u16 := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u16 = append(u16, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u16))
}

// StringToUTF16LE - Go string → UTF-16LE 字节
func StringToUTF16LE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(u16)*2)
	for _, u := range u16 {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// UTF16Len - 字符串的 UTF-16 单元数 (引擎按 UTF-16 单元计字符)
func UTF16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// DetectEncoding - 检测文本编码, 返回解码后的字符串
func DetectEncoding(data []byte) string {
	// BOM 检测
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return UTF16LEToString(data[2:])
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:])
	}
	// UTF-16LE 特征: 偶数长度 + 大量 null 字节
	if len(data) >= 4 && len(data)%2 == 0 {
		nulls := 0
		for i := 0; i < len(data); i++ {
			if data[i] == 0 {
				nulls++
			}
		}
		if float64(nulls)/float64(len(data)) > 0.3 {
			return UTF16LEToString(data)
		}
	}
	// 默认 UTF-8
	return string(data)
}

// ReadTextFile - 读取文本文件 (自动检测编码)
func ReadTextFile(path string) (string, error) {
	data, err := readFileBytes(path)
	if err != nil {
		return "", err
	}
	return DetectEncoding(data), nil
}

// WriteTextFileUTF8 - 写 UTF-8 文本文件 (CRLF)
func WriteTextFileUTF8(path, content string) error {
	// 统一 CRLF (和原版字幕文件一致)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")
	return writeFileBytes(path, []byte(content))
}

// WriteTextFileUTF16 - 写 UTF-16LE 文本文件 (带 BOM, CRLF)
func WriteTextFileUTF16(path, content string) error {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")
	out := []byte{0xFF, 0xFE}
	out = append(out, StringToUTF16LE(content)...)
	return writeFileBytes(path, out)
}

// HasHashComment - 检查行是否有 // hash=0x 注释
func HasHashComment(line string) (uint32, bool) {
	idx := strings.Index(line, "//")
	if idx < 0 {
		return 0, false
	}
	comment := line[idx:]
	hashIdx := strings.Index(comment, "hash=")
	if hashIdx < 0 {
		return 0, false
	}
	// 解析 0x 后面的十六进制
	rest := comment[hashIdx+5:]
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "0x") || strings.HasPrefix(rest, "0X") {
		rest = rest[2:]
	}
	var v uint32
	n := 0
	for _, c := range rest {
		var d uint32
		switch {
		case c >= '0' && c <= '9':
			d = uint32(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint32(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint32(c-'A') + 10
		default:
			return v, n > 0
		}
		v = v*16 + d
		n++
	}
	return v, n > 0
}

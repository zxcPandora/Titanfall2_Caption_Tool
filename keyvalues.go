// keyvalues.go - Valve KeyValues 格式解析
package main

import (
	"fmt"
	"os"
	"strings"
)

// KVPair - 解析结果
type KVPair struct {
	Token string
	Text  string
	Hash  uint32
	Set   bool // hash 来自注释
}

// StripComments - 移除 // 注释 (保留引号内内容, 识别 \" 转义)
func StripComments(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		var sb strings.Builder
		inStr := false
		escaped := false
		for i := 0; i < len(line); i++ {
			c := line[i]
			if escaped {
				escaped = false
				sb.WriteByte(c)
				continue
			}
			if c == '\\' && inStr {
				escaped = true
				sb.WriteByte(c)
				continue
			}
			if c == '"' {
				inStr = !inStr
				sb.WriteByte(c)
			} else if c == '/' && !inStr && i+1 < len(line) && line[i+1] == '/' {
				break
			} else {
				sb.WriteByte(c)
			}
		}
		out = append(out, sb.String())
	}
	return strings.Join(out, "\n")
}

// ParseKeyValues - 解析字幕 KeyValues 文本
// 返回所有条目 (token, text, hash注释, hash是否设置)
func ParseKeyValues(content string) ([]KVPair, error) {
	// 从完整文本提取 hash 注释 (含所有行)
	hashMap := buildHashMap(content)
	return parseKeyValues(content, hashMap)
}

// buildHashMap - 从原始文本提取所有 hash 注释
func buildHashMap(content string) map[string]uint32 {
	hashMap := make(map[string]uint32)
	for _, line := range strings.Split(content, "\n") {
		q1 := strings.Index(line, `"`)
		if q1 < 0 {
			continue
		}
		q2 := strings.Index(line[q1+1:], `"`)
		if q2 < 0 {
			continue
		}
		q2 += q1 + 1
		token := line[q1+1 : q2]
		if h, ok := HasHashComment(line); ok {
			hashMap[token] = h
		}
	}
	return hashMap
}

// parseKeyValues - 内部解析 (hashMap 从外层传入, 递归共享)
func parseKeyValues(content string, hashMap map[string]uint32) ([]KVPair, error) {
	// 剥离注释
	clean := StripComments(content)

	// 解析 Token "..." "..." 对
	pairs := make([]KVPair, 0)
	i := 0
	n := len(clean)
	for i < n {
		// 跳过空白
		for i < n && (clean[i] == ' ' || clean[i] == '\t' || clean[i] == '\r' || clean[i] == '\n') {
			i++
		}
		if i >= n || clean[i] != '"' {
			i++
			continue
		}
		// 读 token
		token, next, err := readQuoted(clean, i)
		if err != nil {
			return pairs, err
		}
		i = next
		// 跳过空白
		for i < n && (clean[i] == ' ' || clean[i] == '\t' || clean[i] == '\r' || clean[i] == '\n') {
			i++
		}
		if i >= n {
			break
		}
		if clean[i] == '{' {
			// 块开始 - 递归 (跳过引号内内容与 \" 转义, 防止文本中的 { } 干扰深度)
			depth := 1
			start := i + 1
			pos := start
			inQ := false
			for pos < n && depth > 0 {
				c := clean[pos]
				if inQ {
					if c == '\\' && pos+1 < n {
						pos++ // 跳过转义字符
					} else if c == '"' {
						inQ = false
					}
				} else if c == '"' {
					inQ = true
				} else if c == '{' {
					depth++
				} else if c == '}' {
					depth--
				}
				pos++
			}
			inner := clean[start : pos-1]
			sub, err := parseKeyValues(inner, hashMap)
			if err != nil {
				return pairs, err
			}
			pairs = append(pairs, sub...)
			i = pos
		} else if clean[i] == '"' {
			// 读文本值
			text, next2, err := readQuoted(clean, i)
			if err != nil {
				return pairs, err
			}
			i = next2
			// 跳过元数据
			if token == "lang" || token == "Language" || token == "Tokens" ||
				token == "english" || token == "schinese" || token == "tchinese" {
				continue
			}
			// L4D2 约定: [english] 前缀条目 = 英文回退, 编译目标语言时跳过
			if strings.HasPrefix(token, "[english]") {
				continue
			}
			p := KVPair{
				Token: token,
				Text:  text, // readQuoted 已完成转义还原, 不再二次反转义
			}
			if h, ok := hashMap[token]; ok {
				p.Hash = h
				p.Set = true
			}
			pairs = append(pairs, p)
		} else {
			i++
		}
	}
	return pairs, nil
}

// readQuoted - 从 pos 的 '"' 开始读带引号字符串
func readQuoted(s string, pos int) (string, int, error) {
	if pos >= len(s) || s[pos] != '"' {
		return "", pos, fmt.Errorf("expected quote at %d", pos)
	}
	var sb strings.Builder
	i := pos + 1
	for i < len(s) {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			sb.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c == '"' {
			return sb.String(), i + 1, nil
		}
		sb.WriteByte(c)
		i++
	}
	return sb.String(), i, fmt.Errorf("unterminated string")
}

// readFileBytes / writeFileBytes - 辅助文件操作
func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

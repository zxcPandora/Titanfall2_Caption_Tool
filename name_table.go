// name_table.go - hash→名称映射表 (JSON)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadNameTable - 加载 hash→名称映射表
// 默认读取程序同目录的 name_table.json
func LoadNameTable(path string) map[uint32]string {
	table := make(map[uint32]string)

	if path == "" {
		// 默认: 可执行文件同目录
		exe, err := os.Executable()
		if err == nil {
			path = filepath.Join(filepath.Dir(exe), "name_table.json")
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return table // 无表也正常工作
	}

	// JSON keys 是字符串
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ name_table.json 解析失败: %v\n", err)
		return table
	}

	// 键兼容: 0x 十六进制 (unpack 注释格式) → 十进制 → 裸十六进制
	for k, v := range raw {
		key := strings.TrimSpace(k)
		var h uint32
		if strings.HasPrefix(key, "0x") || strings.HasPrefix(key, "0X") {
			if _, err := fmt.Sscanf(key[2:], "%x", &h); err == nil {
				table[h] = v
			}
		} else if _, err := fmt.Sscanf(key, "%d", &h); err == nil {
			table[h] = v
		} else if _, err := fmt.Sscanf(key, "%x", &h); err == nil {
			// 纯数字键优先按十进制解释; 含字母的十六进制键 (如 "03115a36") 落这里
			table[h] = v
		}
	}

	if len(table) == 0 && len(raw) > 0 {
		fmt.Fprintf(os.Stderr, "⚠ name_table.json 的键没有匹配到任何条目 (键应为哈希: 0x 十六进制或十进制)\n")
	}

	return table
}

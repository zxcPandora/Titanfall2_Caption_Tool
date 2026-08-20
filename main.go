// main.go - Source 引擎字幕工具 CLI
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const version = "1.0.1"

// logOutput - -l 时所有输出镜像到 log.txt
var logOutput *os.File

// logf - 输出到控制台 + log.txt (-l 时)
func logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Print(msg)
	if logOutput != nil {
		logOutput.WriteString(msg)
	}
}

// openLog - 创建/清空 log.txt (同官方 -l 行为)
func openLog() {
	f, err := os.Create("log.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 无法创建 log.txt: %v\n", err)
		return
	}
	logOutput = f
}

// truncStr - 截断字符串用于显示 (按 rune 计)
func truncStr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

// padTrunc - 截断到 n 个 rune 并用空格补齐 (对齐输出用)
func padTrunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	if len(r) < n {
		return string(r) + strings.Repeat(" ", n-len(r))
	}
	return string(r)
}

func main() {
	// 启动横幅: 版本/作者/来源
	fmt.Printf("Titanfall 2 Caption Tool v%s\n", version)
	fmt.Printf("作者: zxcPandora | 来源: https://github.com/zxcPandora/titanfall2_caption_tool\n\n")

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	action := os.Args[1]
	args := os.Args[2:]

	// 拖拽模式: 处理完暂停, 方便看到结果
	dragged := false
	done := func() {
		if logOutput != nil {
			logOutput.Close()
			logOutput = nil
		}
		if dragged {
			fmt.Println("\n按任意键退出...")
			fmt.Scanln()
		}
	}

	switch action {
	case "pack":
		cmdPack(args)
	case "unpack":
		cmdUnpack(args)
	case "verify":
		cmdVerify(args)
	case "dump":
		cmdDump(args)
	case "help", "-h", "--help":
		usage()
	default:
		// 拖拽模式: 第一个参数是文件路径 → 按扩展名自动选命令
		if fi, err := os.Stat(action); err == nil && !fi.IsDir() {
			ext := strings.ToLower(filepath.Ext(action))
			args = append([]string{action}, args...)
			dragged = true
			switch ext {
			case ".dat":
				cmdUnpack(args)
			case ".txt":
				cmdPack(args)
			default:
				fmt.Fprintf(os.Stderr, "不支持的拖拽文件类型: %s\n", ext)
				fmt.Fprintf(os.Stderr, "支持: .dat (解包) 或 .txt (打包)\n")
				done()
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", action)
			usage()
			os.Exit(1)
		}
	}

	done()
}

func usage() {
	fmt.Printf(`Titanfall 2 Caption Tool v%s - Source 引擎字幕编译器

用法:
  caption_tool pack [选项] <输入.txt> [更多文件...]
  caption_tool unpack [选项] <输入.dat> [更多文件...]
  caption_tool verify <输入.dat>
  caption_tool dump <输入.dat>

选项:
  -o <文件>       输出文件 (单文件时)
  -v              详细输出 (逐条打印 + 编译统计)
  -l              所有输出同时写入 log.txt
  --utf16         unpack 输出 UTF-16LE (默认 UTF-8)
  --no-align      [实验性] pack 不对齐数据偏移
  --table <json>  使用 hash→名称映射表
  --force         hash 冲突时仍打包 (默认拒绝)
`, version)
}

// ── 命令行参数解析 ──

type commonFlags struct {
	output  string
	verbose bool
	logfile bool
	utf16   bool
	noAlign bool
	table   string
	force   bool
}

func parseFlags(args []string) (commonFlags, []string, error) {
	var f commonFlags
	inputs := make([]string, 0)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("%s 需要参数", a)
			}
			i++
			f.output = args[i]
		case strings.HasPrefix(a, "-o="):
			f.output = strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "--output="):
			f.output = strings.TrimPrefix(a, "--output=")
		case a == "-v" || a == "--verbose":
			f.verbose = true
		case a == "-l" || a == "--logfile":
			f.logfile = true
		case a == "--utf16":
			f.utf16 = true
		case a == "--no-align":
			f.noAlign = true
		case a == "--table" || a == "-table":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("%s 需要参数", a)
			}
			i++
			f.table = args[i]
		case a == "--force":
			f.force = true
		case strings.HasPrefix(a, "-"):
			return f, nil, fmt.Errorf("未知选项: %s", a)
		default:
			inputs = append(inputs, a)
		}
	}
	return f, inputs, nil
}

// ── pack ──

func cmdPack(args []string) {
	flags, inputs, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "需要输入 .txt 文件")
		os.Exit(1)
	}
	if flags.logfile {
		openLog()
	}
	if len(inputs) > 1 && flags.output != "" {
		fmt.Fprintln(os.Stderr, "错误: -o 只能用于单文件输入 (多文件请去掉 -o 或逐个处理)")
		os.Exit(1)
	}

	for _, input := range inputs {
		content, err := ReadTextFile(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取失败: %s: %v\n", input, err)
			continue
		}

		pairs, err := ParseKeyValues(content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "解析失败: %s: %v\n", input, err)
			continue
		}
		if len(pairs) == 0 {
			fmt.Fprintf(os.Stderr, "错误: 未找到字幕条目: %s\n", input)
			continue
		}

		// 构建条目
		entries := make([]CaptionEntry, 0, len(pairs))
		for _, p := range pairs {
			e := CaptionEntry{
				Token:   p.Token,
				Text:    p.Text,
				Hash:    p.Hash,
				HashSet: p.Set,
			}
			if !p.Set {
				e.Hash = CaptionHash(p.Token)
			}
			entries = append(entries, e)
		}

		// 检查格式外内容 (最后一个 } 后有内容)
		checkAfterBrace(content, input)

		// 检查超长词内含 BT (TTF2 引擎乱码\截断 bug)
		checkLongBtWords(entries, input)

		// -v: 逐条打印
		if flags.verbose {
			for _, e := range entries {
				logf("Processing: '%s' = '%s'\n", e.Token, truncStr(e.Text, 120))
			}
		}

		// 输出路径
		out := flags.output
		if out == "" {
			out = strings.TrimSuffix(input, filepath.Ext(input)) + ".dat"
		}

		if err := WriteCaptionFile(out, entries, !flags.noAlign, flags.force); err != nil {
			fmt.Fprintf(os.Stderr, "写入失败: %s: %v\n", out, err)
			continue
		}
		if flags.verbose {
			logf("打包 %d 条: %s\n", len(entries), out)
			// -v: 编译统计 (同官方)
			if cf, err := ReadCaptionFile(out); err == nil {
				totalText, longest := 0, 0
				for _, l := range cf.Lookups {
					totalText += int(l.Length)
					if int(l.Length) > longest {
						longest = int(l.Length)
					}
				}
				totalCap := len(cf.Blocks) * int(cf.Header.BlockSize)
				wasted := totalCap - totalText
				avg := 0.0
				if len(cf.Blocks) > 0 {
					avg = float64(wasted) / float64(len(cf.Blocks))
				}
				logf("Found %d strings in '%s'\n", len(entries), input)
				logf("Longest string = (%d) bytes\n", longest)
				logf("%d blocks (%d bytes each), %d bytes wasted (%.3f per block average), total bytes %d\n",
					len(cf.Blocks), cf.Header.BlockSize, wasted, avg, totalCap)
				logf("directory size %d entries, %d bytes, data size %d bytes\n",
					cf.Header.DirectorySize, cf.Header.DirectorySize*12, totalText)
			}
		} else {
			fmt.Printf("打包: %s\n", out)
		}
	}
}

// checkAfterBrace - 检查 } 闭合后是否有内容 (用去注释后的内容, 防误报)
func checkAfterBrace(content, path string) {
	clean := StripComments(content)
	lastBrace := strings.LastIndex(clean, "}")
	if lastBrace < 0 {
		return
	}
	after := strings.TrimSpace(clean[lastBrace+1:])
	if after != "" && strings.Contains(after, `"`) {
		fmt.Fprintf(os.Stderr, "⚠ %s: 文件末尾 } 闭合后有内容被忽略!\n", path)
	}
}

// checkLongBtWords - 检查 BT 名字后内容超 19 字符 (说话者标签除外)
// TTF2 引擎格式化问题 : BT 名字 (BT-7274 这类, BT+连字符数字) 之后,
// 跨空格扫描到第一个标点或词尾的内容 ≥20 个 UTF-16 字符 → 从第 20 个起截断 (显示前 19 个)。
// 40 字节样式化缓冲 = 20 字符 (19 显示 + null)。
// 标点 (，。！？、；：…,.!?;:) 是隔断; 空格/连字符/数字是内容延续 (空格不算隔断, 用户实测)。
// "BT:"/"BT：" (后跟冒号) 是说话者标签, 引擎正常处理, 不检查。
// 规避: 给名字后内容加标点隔断 (如 "BT-7274，") 或断句。
func checkLongBtWords(entries []CaptionEntry, path string) {
	tagRe := regexp.MustCompile(`<[^>]*>`)
	punctRe := regexp.MustCompile(`[，。！？、；：…─—,.!?;:]`)
	nameRe := regexp.MustCompile(`^-[\w]+`)
	for _, e := range entries {
		clean := tagRe.ReplaceAllString(e.Text, "")
		// 引擎按词拼接后跨词扫描 (空格是内容延续), 检查在整个条目文本上进行
		for btIdx := 0; btIdx < len(clean); {
			i := strings.Index(clean[btIdx:], "BT")
			if i < 0 {
				break
			}
			btIdx += i
			after := clean[btIdx+2:]
			// 说话者: BT 后跟冒号 → 跳过
			if strings.HasPrefix(after, ":") || strings.HasPrefix(after, "：") {
				btIdx += 2
				continue
			}
			// 名字 = BT + 连字符数字 (如 BT-7274)
			if m := nameRe.FindString(after); m != "" {
				after = after[len(m):]
			}
			// 到第一个标点或词尾 (空格/连字符/数字是内容延续)
			if end := punctRe.FindStringIndex(after); end != nil {
				after = after[:end[0]]
			}
			if UTF16Len(after) >= 20 {
				fmt.Fprintf(os.Stderr,
					"⚠ %s: 条目 \"%s\" 的 BT 后内容 \"%s\" (%d 字符 ≥20), TTF2 引擎从第 20 字符起截断, 建议加标点隔断\n",
					path, e.Token, truncStr(after, 40), UTF16Len(after))
			}
			btIdx += 2
		}
	}
}

// ── unpack ──

func cmdUnpack(args []string) {
	flags, inputs, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "需要输入 .dat 文件")
		os.Exit(1)
	}
	if flags.logfile {
		openLog()
	}
	if len(inputs) > 1 && flags.output != "" {
		fmt.Fprintln(os.Stderr, "错误: -o 只能用于单文件输入 (多文件请去掉 -o 或逐个处理)")
		os.Exit(1)
	}

	for _, input := range inputs {
		cf, err := ReadCaptionFile(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取失败: %s: %v\n", input, err)
			continue
		}

		// 加载名称映射表
		table := LoadNameTable(flags.table)

		// 构建输出
		var sb strings.Builder
		lang := DetectLanguage(input)

		sb.WriteString("\"lang\"\n{\n")
		fmt.Fprintf(&sb, "\t\"Language\" \"%s\"\n", lang)
		sb.WriteString("\t\"Tokens\"\n\t{\n")

		entries := make([]CaptionEntry, 0, len(cf.Lookups))
		for i, l := range cf.Lookups {
			text, err := cf.ExtractText(l)
			if err != nil {
				text = "<解码失败>"
			}
			token := fmt.Sprintf("Caption_%05d", i)
			// 用名称映射表恢复原名
			if name, ok := table[l.Hash]; ok && name != "" {
				token = name
			}
			// -v: 逐条打印 (同官方目录清单格式)
			if flags.verbose {
				logf("%3d: (%s) hash(%d), block(%d), offset(%d), len(%d) %s\n",
					i, padTrunc(token, 40), l.Hash, l.BlockNum, l.Offset, l.Length, truncStr(text, 100))
			}
			escaped := strings.ReplaceAll(text, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			fmt.Fprintf(&sb, "\t\t\"%s\"\t\"%s\"\t// hash=0x%08x\n", token, escaped, l.Hash)
			entries = append(entries, CaptionEntry{Token: token, Text: text, Hash: l.Hash})
		}
		sb.WriteString("\t}\n}\n")

		// 输出
		out := flags.output
		if out == "" {
			out = strings.TrimSuffix(input, filepath.Ext(input)) + ".txt"
		}
		if flags.utf16 {
			if err := WriteTextFileUTF16(out, sb.String()); err != nil {
				fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
				continue
			}
		} else {
			if err := WriteTextFileUTF8(out, sb.String()); err != nil {
				fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
				continue
			}
		}
		if flags.verbose {
			fmt.Printf("解包 %d 条: %s\n", len(entries), out)
		} else {
			fmt.Printf("解包: %s\n", out)
		}
	}
}

// ── verify ──

func cmdVerify(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "需要输入 .dat 文件")
		os.Exit(1)
	}
	for _, input := range args {
		cf, err := ReadCaptionFile(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取失败: %s: %v\n", input, err)
			continue
		}

		// 统计
		totalText := 0
		for _, l := range cf.Lookups {
			totalText += int(l.Length)
		}
		totalCap := len(cf.Blocks) * int(cf.Header.BlockSize)
		wasted := totalCap - totalText
		avgWaste := 0.0
		if len(cf.Blocks) > 0 {
			avgWaste = float64(wasted) / float64(len(cf.Blocks))
		}

		// 排序检查
		sorted := true
		for i := 0; i < len(cf.Lookups)-1; i++ {
			if cf.Lookups[i].Hash > cf.Lookups[i+1].Hash {
				sorted = false
				break
			}
		}

		fmt.Printf("编译字幕文件: %s\n", input)
		fmt.Printf("  大小: %d 字节\n", fileSize(input))
		fmt.Printf("  版本: %d, 条目: %d\n", cf.Header.Version, cf.Header.DirectorySize)
		fmt.Printf("  块: %d x %d 字节\n", cf.Header.NumBlocks, cf.Header.BlockSize)
		fmt.Printf("  数据偏移: %d (0x%x)\n", cf.Header.DataOffset, cf.Header.DataOffset)
		fmt.Printf("  排序: %v\n", sorted)
		fmt.Printf("  打包效率: %d/%d 字节, 浪费 %d (%.1f/块)\n",
			totalText, totalCap, wasted, avgWaste)

		// 前 5 条
		n := len(cf.Lookups)
		if n > 5 {
			n = 5
		}
		for i := 0; i < n; i++ {
			text, _ := cf.ExtractText(cf.Lookups[i])
			trunc := truncStr(text, 60)
			l := cf.Lookups[i]
			fmt.Printf("  [%d] hash=0x%08X block=%d off=%d len=%d \"%s\"\n",
				i, l.Hash, l.BlockNum, l.Offset, l.Length, trunc)
		}
		if len(cf.Lookups) > 5 {
			fmt.Printf("  ... 还有 %d 条\n", len(cf.Lookups)-5)
		}
	}
}

// ── dump ──

func cmdDump(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "需要输入 .dat 文件")
		os.Exit(1)
	}
	for _, input := range args {
		cf, err := ReadCaptionFile(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取失败: %s: %v\n", input, err)
			continue
		}
		for i, l := range cf.Lookups {
			text, _ := cf.ExtractText(l)
			fmt.Printf("%3d:  hash(%d), block(%d), offset(%d), len(%d) \"%s\"\n",
				i, l.Hash, l.BlockNum, l.Offset, l.Length, text)
		}
	}
}

// ── 辅助 ──

func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}

// langOrder - 语言检测顺序 (有序, 避免 map 随机遍历导致同名冲突时结果不确定)
var langOrder = []struct{ key, lang string }{
	{"_english", "English"},
	{"_schinese", "schinese"},
	{"_tchinese", "tchinese"},
	{"_french", "french"},
	{"_german", "german"},
	{"_italian", "italian"},
	{"_japanese", "japanese"},
	{"_korean", "korean"},
	{"_mspanish", "mspanish"},
	{"_polish", "polish"},
	{"_portuguese", "portuguese"},
	{"_russian", "russian"},
	{"_spanish", "spanish"},
}

// DetectLanguage - 从文件名推断语言
func DetectLanguage(path string) string {
	name := strings.ToLower(filepath.Base(path))
	for _, e := range langOrder {
		if strings.Contains(name, e.key) {
			return e.lang
		}
	}
	return "English"
}

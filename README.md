# Titanfall 2 Caption Tool — Source 引擎字幕工具

[English](README_EN.md) | **中文**

一个用 Go 编写的 Source 引擎编译字幕（VCCD `.dat`）打包/解包工具。支持 **Titanfall 2** 与 **Left 4 Dead 2** 等使用同一字幕格式的游戏。

本工具实现：打包、解包、校验、查看一体。

---

## 功能特性

- ✅ **打包**：`.txt` → `.dat`（VCCD 格式，CRC32 哈希，512 字节对齐）
- ✅ **解包**：`.dat` → `.txt`，保留 `// hash=0x...` 注释（回打包时可还原原哈希）
- ✅ **校验**：`verify` 输出文件结构统计（块、目录、浪费字节、排序）
- ✅ **查看**：`dump` 逐条列出 hash/块/偏移/长度/文本
- ✅ **名称映射表**：`--table` 用 JSON 表把 `Caption_NNNN` 恢复成真实 token 名
- ✅ **编码自动检测**：`.txt` 支持 UTF-8 / UTF-16LE（BOM/特征检测）；`.dat` 内文本为 UTF-16LE（引擎硬性要求）
- ✅ **拖拽使用**：把文件拖到 exe 上自动按扩展名选择打包/解包
- ✅ **关键字词检查**：打包时警告会触发 TTF2 引擎字幕乱码\截断 bug 的超长词（见"已知问题"）
- ✅ **`[english]` 兼容**：打包时自动跳过 `[english]` 前缀的英文回退条目
- ✅ **超长条目警告**：单条超过块大小（8192 字节）时直接报错
- ✅ **`-v` / `-l`**：详细输出与 log.txt 日志

## 使用方法

```
titanfall2_caption_tool pack [选项] <输入.txt> [更多文件...]
titanfall2_caption_tool unpack [选项] <输入.dat> [更多文件...]
titanfall2_caption_tool verify <输入.dat>
titanfall2_caption_tool dump <输入.dat>
```

### 选项

| 选项 | 说明 |
|---|---|
| `-o <文件>` | 输出文件（仅单文件时有效） |
| `-v` | 详细输出：逐条打印 + 编译统计 |
| `-l` | 所有输出同时写入 `log.txt` |
| `--utf16` | 解包输出 UTF-16LE（默认 UTF-8） |
| `--no-align` | ⚠️ 实验性：打包不对齐数据偏移（引擎无法读取！见已知问题） |
| `--table <json>` | 使用 hash→名称映射表恢复真实 token 名 |
| `--force` | hash 冲突时仍打包（默认拒绝，见下文） |

### 示例

```sh
# 打包（tchinese.txt → tchinese.dat）
titanfall2_caption_tool.exe pack subtitles_tchinese.txt

# 解包并输出 UTF-16LE 的 .txt（L4D2 风格）
titanfall2_caption_tool.exe unpack closecaption_english.dat --utf16 -o closecaption_english.txt

# 详细输出 + 日志
titanfall2_caption_tool.exe pack subtitles_tchinese.txt -v -l

# 校验/查看
titanfall2_caption_tool.exe verify subtitles_tchinese.dat
titanfall2_caption_tool.exe dump subtitles_english.dat | head
```

### 拖拽(Windows端独有功能)

把 `.dat` 拖到 `titanfall2_caption_tool.exe` → 自动解包为同名 `.txt`；把 `.txt` 拖上去 → 自动打包为同名 `.dat`。处理完会暂停等待按键，方便查看结果。

## 输入输出格式

### `.txt`（KeyValues）

```text
"lang"
{
    "Language" "tchinese"
    "Tokens"
    {
        "Caption_00000"    "字幕文本内容"    // hash=0x03115a36
    }
}
```

- 编码：UTF-8 或 UTF-16LE 自动检测；`--utf16` 可指定解包输出编码
- 文本支持引擎标签：`<clr:r,g,b>` `<sfx>` `<low>` `<delay:x>` `<len:x>` `<cr>` `<norepeat:x>` `<I>` `<B>` `<playerclr>` 等（原样透传）。标签语义见 [Valve 官方 wiki: Closed Captions](https://developer.valvesoftware.com/wiki/Closed_Captions)
- `// hash=0x...` 注释：解包时写入，打包时读取并作为该条目的最终哈希（保证跨语言对应同一 token）
- **转义规则**：打包解析时 `\\` 还原为 `\`、`\"` 还原为 `"`，其余 `\X` 一律还原为字面 `X`（不做 `\n` 等特殊解释，保证与解包输出往返一致）

### 关于 `Caption_NNNN` 占位名与哈希注释（重要）

`.dat` 文件里**只存 token 名的 CRC32 哈希，不存名字本身**——引擎靠哈希匹配字幕（音频事件 → 哈希 → 查找字幕）。CRC32 是单向的，所以解包时**无法从 .dat 还原原始 token 名**（如 L4D2 的 `barn.chatter`、TTF2 的 `diag_cooper_introvid_01`），只能生成 `Caption_00000`、`Caption_00001`… 这样的占位名。

- `// hash=0x...` 注释记录的是条目的**真实身份**，打包时工具把它作为最终哈希写回。
- **如果你不确定字幕的原始键名，请务必不要：**
  1. **删除 `// hash=0x...` 注释**——删除后工具会按新名字重新计算 CRC32，哈希一变，游戏里的音频事件就再也触发不了这条字幕；
  2. **把 `Caption_NNNN` 改成你猜的名字**——占位名对引擎没有意义（引擎只认哈希），改名只是为了人工阅读方便，猜的名字只会误导后来的人。若确需改名，必须保留哈希注释不变。
- **名称映射表（`--table`）**：如果你从游戏脚本或其他渠道拿到了"哈希 → 真实 token 名"的对应表（JSON：键为哈希，值为名字），可用 `--table name_table.json` 让解包输出真实名字，便于对照与维护。

### `.dat`（VCCD 编译字幕）

| 字段 | 说明 |
|---|---|
| Magic | `0x44434356`（"VCCD" 小端） |
| Version | 1 |
| 数据块 | 8192 字节 × N |
| 目录条目 | 12 字节 × 条目数（hash / block / offset / length） |
| 哈希 | token 名小写的 CRC32 |
| 文本编码 | UTF-16LE（引擎直接 reinterpret 为 wchar_t*，无转换） |
| 数据偏移 | 512 字节对齐（引擎硬性要求） |

## 支持平台

| 平台 | 架构 |
|---|---|
| Windows | amd64、arm64、386 |
| Linux | amd64、arm64、386 |
| macOS | amd64（Intel）、arm64（Apple Silicon） |
| Android（Termux） | arm64（amd64 设备用 Linux amd64 版） |

全部为纯静态链接（仅 Go 标准库，无外部依赖）。

## 如何编译

需要 [Go](https://go.dev/dl/) 1.21+（开发环境使用 1.26.5）。

```sh
cd caption_tool_go
go build -o titanfall2_caption_tool.exe .
```

交叉编译（示例）：

```sh
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o titanfall2_caption_tool_linux .
# macOS arm64
GOOS=darwin GOARCH=arm64 go build -o titanfall2_caption_tool_mac .
# Android arm64（Termux）
GOOS=android GOARCH=arm64 go build -o titanfall2_caption_tool_android .
# Windows 386
GOOS=windows GOARCH=386 go build -o titanfall2_caption_tool_windows386.exe .
```

## 已知问题

1. **TTF2 引擎 BT 词乱码\截断 bug**：引擎对"无空格长词（≥20 个 UTF-16 字符）内含 `BT`"的字幕会从第 20 字符起乱码（含 `BT` 字段的字幕会被重生进行特殊渲染的格式化，过长字符安会导致分词器缓冲溢出，属引擎问题，非工具问题）。工具打包检查到时会自动警告此类词。**规避：在 `BT\BT-7274\BT7274` 前后加空格**（Respawn 原版翻译风格）。
2. **`--no-align` 会破坏文件**：引擎按 512 字节对齐读取数据偏移，不对齐会导致全部字幕错位乱码。该选项仅供格式研究，不要用于正式输出。

## 参考资源

- [Valve 官方 wiki: Closed Captions](https://developer.valvesoftware.com/wiki/Closed_Captions)（字幕格式与标签官方文档）
- Source SDK 2013
- Left 4 Dead 2 SDK
- Alien Swarm SDK

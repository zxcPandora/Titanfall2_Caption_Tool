# Titanfall 2 Caption Tool — Source Engine Caption Tool

**English** | [中文](README.md)

A Go-based packer/decompiler for Source Engine compiled captions (VCCD `.dat` files). Supports **Titanfall 2**, **Left 4 Dead 2**, and other games sharing the same caption format.

This tool provides packing, unpacking, verification, and inspection in one binary.

---

## Features

- ✅ **Pack**: `.txt` → `.dat` (VCCD format, CRC32 hashes, 512-byte alignment)
- ✅ **Unpack**: `.dat` → `.txt`, preserving `// hash=0x...` comments (restores original hashes on re-pack)
- ✅ **Verify**: prints file structure stats (blocks, directory, wasted bytes, sort order)
- ✅ **Dump**: lists every entry — hash/block/offset/length/text
- ✅ **Name table**: `--table` restores real token names from a JSON hash→name map
- ✅ **Auto encoding detection**: `.txt` in UTF-8 or UTF-16LE (BOM/statistical detection); `.dat` text is UTF-16LE (engine hard requirement)
- ✅ **Drag & drop**: drop a file onto the exe to auto pack/unpack by extension
- ✅ **BT truncation check**: during pack, warns about content that would trigger the engine's subtitle truncation issue according to the detection rules (see Known Issues)
- ✅ **`[english]` support**: skips `[english]`-prefixed fallback entries on pack (L4D2 source convention)
- ✅ **Oversized entry guard**: entries larger than one block (8192 bytes) are rejected with an error
- ✅ **`-v` / `-l`**: verbose output and log.txt logging

## Usage

```
titanfall2_caption_tool pack [options] <input.txt> [more files...]
titanfall2_caption_tool unpack [options] <input.dat> [more files...]
titanfall2_caption_tool verify <input.dat>
titanfall2_caption_tool dump <input.dat>
```

### Options

| Option | Description |
|---|---|
| `-o <file>` | Output file (single input only) |
| `-v` | Verbose: per-entry listing + compile stats |
| `-l` | Mirror all output to `log.txt` |
| `--utf16` | Write unpacked `.txt` as UTF-16LE (default UTF-8) |
| `--no-align` | ⚠️ Experimental: skip data-offset alignment (engine cannot read it! see Known Issues) |
| `--table <json>` | Use a hash→name JSON table to restore real token names |
| `--force` | Allow packing despite hash collisions (rejected by default, see below) |

### Examples

```sh
# Pack (subtitles_tchinese.txt → subtitles_tchinese.dat)
titanfall2_caption_tool.exe pack subtitles_tchinese.txt

# Unpack to UTF-16LE .txt (L4D2 style)
titanfall2_caption_tool.exe unpack closecaption_english.dat --utf16 -o closecaption_english.txt

# Verbose + log file
titanfall2_caption_tool.exe pack subtitles_tchinese.txt -v -l

# Verify / dump
titanfall2_caption_tool.exe verify subtitles_tchinese.dat
titanfall2_caption_tool.exe dump subtitles_english.dat | head
```

### Drag & Drop (Windows-only)

Drop a `.dat` onto `titanfall2_caption_tool.exe` → auto-unpacks to a same-named `.txt`. Drop a `.txt` → auto-packs to a same-named `.dat`. It pauses for a keypress afterwards so you can read the result.

## Input / Output Formats

### `.txt` (KeyValues)

```text
"lang"
{
    "Language" "tchinese"
    "Tokens"
    {
        "Caption_00000"    "Caption text here"    // hash=0x03115a36
    }
}
```

- Encoding: UTF-8 or UTF-16LE auto-detected; `--utf16` selects unpack output encoding
- Engine tags pass through untouched: `<clr:r,g,b>` `<sfx>` `<low>` `<delay:x>` `<len:x>` `<cr>` `<norepeat:x>` `<I>` `<B>` `<playerclr>` etc. Tag semantics: [Valve Developer Wiki: Closed Captions](https://developer.valvesoftware.com/wiki/Closed_Captions)
- `// hash=0x...` comments: written on unpack, honored on pack (keeps the same token hash across languages)
- **Escape rules**: on pack, `\\` restores to `\`, `\"` restores to `"`, and any other `\X` restores to literal `X` (no special `\n`-style interpretation — this keeps the round-trip identical with unpack output)

### About `Caption_NNNN` placeholders and hash comments (important)

A `.dat` file stores **only the CRC32 hash of each token name — not the name itself**. The engine matches captions by hash (audio event → hash → caption lookup). CRC32 is one-way, so the original token names (e.g. L4D2's `barn.chatter`, TF2's `diag_cooper_introvid_01`) **cannot be recovered from the `.dat`** — the tool emits placeholders `Caption_00000`, `Caption_00001`, … instead.

- The `// hash=0x...` comment records the entry's **true identity**; the packer writes it back as the final hash.
- **If you don't know the original key name, do NOT:**
  1. **delete the `// hash=0x...` comment** — the packer would then recompute CRC32 from the new name; the hash changes and the in-game audio event can never trigger that caption again;
  2. **rename `Caption_NNNN` to a guessed name** — placeholders are meaningless to the engine (it only reads hashes); renaming is purely for human readability, and a guessed name will only mislead later editors. If you must rename, keep the hash comment untouched.
- **Name table (`--table`)**: if you have a hash→name map from game scripts or other sources (JSON: keys = hashes, values = names), use `--table name_table.json` so unpacking emits real names for easier reference and maintenance.

### `.dat` (VCCD compiled captions)

| Field | Description |
|---|---|
| Magic | `0x44434356` ("VCCD" little-endian) |
| Version | 1 |
| Data blocks | 8192 bytes × N |
| Directory entries | 12 bytes each (hash / block / offset / length) |
| Hash | CRC32 of the lowercased token name |
| Text encoding | UTF-16LE (engine reinterprets it directly as `wchar_t*`, no conversion) |
| Data offset | 512-byte aligned (engine hard requirement) |

## Supported Platforms

| Platform | Architectures |
|---|---|
| Windows | amd64, arm64, 386 |
| Linux | amd64, arm64, 386 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Android (Termux) | arm64 (amd64 devices use the Linux amd64 build) |

All builds are fully static (pure Go standard library — no external dependencies).

## Building

Requires [Go](https://go.dev/dl/) 1.21+ (developed with 1.26.5).

```sh
cd caption_tool_go
go build -o titanfall2_caption_tool.exe .
```

Cross-compile (examples):

```sh
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o titanfall2_caption_tool_linux .
# macOS arm64
GOOS=darwin GOARCH=arm64 go build -o titanfall2_caption_tool_mac .
# Android arm64 (Termux)
GOOS=android GOARCH=arm64 go build -o titanfall2_caption_tool_android .
# Windows 386
GOOS=windows GOARCH=386 go build -o titanfall2_caption_tool_windows386.exe .
```

## Known Issues

1. **Garbling/truncation in captions containing BT**: the engine truncates any caption where the content **after a BT name** (`BT-7274` and similar: BT + hyphen + digits), scanned **across spaces** up to the first punctuation mark or end of text, is **≥20 UTF-16 characters** — it cuts off from character 20 (a 40-byte styled-glyph buffer = 19 chars displayed + null; an engine formatting issue, not a tool bug). **Punctuation acts as a break** (`，。！？、；：…─—,.!?;:`); **spaces/hyphens/digits are content continuation** (e.g. `泰坦 BT-7274 已就緒歡迎...` with no punctuation triggers it); speaker tags `BT:`/`BT：` do not trigger. The tool checks by this rule and warns automatically during pack. **Workaround: put a punctuation break after the caption's BT name** (e.g. `BT-7274，`) or split the sentence.
2. **`--no-align` breaks files**: the engine reads the data offset with 512-byte alignment assumed; unaligned files misread every caption. This option is for format research only — never use it for real output.

## References

- [Valve Developer Wiki: Closed Captions](https://developer.valvesoftware.com/wiki/Closed_Captions) (official docs on caption format and tags)
- Source SDK 2013
- Left 4 Dead 2 SDK
- Alien Swarm SDK

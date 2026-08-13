// caption.go - VCCD 编译字幕文件格式读写
package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"sort"
	"strings"
)

// 格式常量
const (
	MagicVCCD      = 0x44434356 // "VCCD" 小端
	CaptionVersion = 1
	MaxBlockSize   = 1 << 13 // 8192
	HeaderSize     = 24
	LookupSize     = 12
)

// CompiledCaptionHeader - 文件头 (24 字节)
type CompiledCaptionHeader struct {
	Magic         uint32
	Version       uint32
	NumBlocks     uint32
	BlockSize     uint32
	DirectorySize uint32
	DataOffset    uint32
}

// CaptionLookup - 目录条目 (12 字节)
type CaptionLookup struct {
	Hash     uint32
	BlockNum uint32
	Offset   uint16
	Length   uint16
}

// CaptionEntry - 内存中的字幕条目
type CaptionEntry struct {
	Token   string // 名称 (Caption_NNN 或原名)
	Text    string // 字幕文本
	Hash    uint32 // CRC32 哈希
	HashSet bool   // hash 是否来自注释 (覆盖)
}

// CaptionFile - 完整的字幕文件
type CaptionFile struct {
	Header  CompiledCaptionHeader
	Lookups []CaptionLookup
	Blocks  [][]byte
}

// CaptionHash - 计算字幕 token 的 CRC32 (小写)
func CaptionHash(name string) uint32 {
	lower := strings.ToLower(name)
	return crc32.ChecksumIEEE([]byte(lower))
}

// ReadCaptionFile - 读取并解析 .dat 文件
func ReadCaptionFile(path string) (*CaptionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) < HeaderSize {
		return nil, fmt.Errorf("文件太小 (%d 字节)", len(data))
	}

	hdr := &CompiledCaptionHeader{
		Magic:         binary.LittleEndian.Uint32(data[0:]),
		Version:       binary.LittleEndian.Uint32(data[4:]),
		NumBlocks:     binary.LittleEndian.Uint32(data[8:]),
		BlockSize:     binary.LittleEndian.Uint32(data[12:]),
		DirectorySize: binary.LittleEndian.Uint32(data[16:]),
		DataOffset:    binary.LittleEndian.Uint32(data[20:]),
	}

	if hdr.Magic != MagicVCCD {
		return nil, fmt.Errorf("无效魔数: 0x%08X (期望 VCCD)", hdr.Magic)
	}
	if hdr.Version != CaptionVersion {
		return nil, fmt.Errorf("不支持版本 %d (期望 %d)", hdr.Version, CaptionVersion)
	}

	// 读取目录 (uint64 运算, 防止 32 位平台乘法溢出绕过检查)
	dirBytes := uint64(HeaderSize) + uint64(hdr.DirectorySize)*LookupSize
	if dirBytes > uint64(len(data)) {
		return nil, fmt.Errorf("目录数据越界")
	}

	lookups := make([]CaptionLookup, hdr.DirectorySize)
	for i := range lookups {
		off := HeaderSize + i*LookupSize
		lookups[i] = CaptionLookup{
			Hash:     binary.LittleEndian.Uint32(data[off:]),
			BlockNum: binary.LittleEndian.Uint32(data[off+4:]),
			Offset:   binary.LittleEndian.Uint16(data[off+8:]),
			Length:   binary.LittleEndian.Uint16(data[off+10:]),
		}
	}

	// 读取数据块 (uint64 运算 + 块大小校验, 防溢出与 OOM)
	if hdr.BlockSize == 0 {
		return nil, fmt.Errorf("块大小无效: 0")
	}
	totalBlocks := uint64(hdr.NumBlocks) * uint64(hdr.BlockSize)
	if uint64(hdr.DataOffset)+totalBlocks > uint64(len(data)) {
		return nil, fmt.Errorf("数据块越界")
	}
	blocks := make([][]byte, hdr.NumBlocks)
	for i := range blocks {
		start := int(hdr.DataOffset) + i*int(hdr.BlockSize)
		blocks[i] = data[start : start+int(hdr.BlockSize)]
	}

	return &CaptionFile{
		Header:  *hdr,
		Lookups: lookups,
		Blocks:  blocks,
	}, nil
}

// ExtractText - 从块中提取字幕文本 (UTF-16LE → string)
func (cf *CaptionFile) ExtractText(lookup CaptionLookup) (string, error) {
	if int(lookup.BlockNum) >= len(cf.Blocks) {
		return "", fmt.Errorf("块号越界: %d", lookup.BlockNum)
	}
	block := cf.Blocks[lookup.BlockNum]
	if int(lookup.Offset)+int(lookup.Length) > len(block) {
		return "", fmt.Errorf("文本越界: off=%d len=%d", lookup.Offset, lookup.Length)
	}
	utf16 := block[lookup.Offset : lookup.Offset+lookup.Length]
	// 去掉末尾 null
	for len(utf16) >= 2 && utf16[len(utf16)-2] == 0 && utf16[len(utf16)-1] == 0 {
		utf16 = utf16[:len(utf16)-2]
	}
	return UTF16LEToString(utf16), nil
}

// WriteCaptionFile - 写入 .dat 文件
// forceCollision: true 时哈希冲突仅警告放行 (默认拒绝)
func WriteCaptionFile(path string, entries []CaptionEntry, align bool, forceCollision bool) error {
	// 计算 hash (保留注释 hash 或重新计算)
	type packed struct {
		entry CaptionEntry
		utf16 []byte
	}
	items := make([]packed, 0, len(entries))
	for _, e := range entries {
		h := e.Hash
		if !e.HashSet {
			h = CaptionHash(e.Token)
		}
		utf16 := StringToUTF16LE(e.Text)
		utf16 = append(utf16, 0, 0) // null 终止
		items = append(items, packed{entry: CaptionEntry{Token: e.Token, Text: e.Text, Hash: h}, utf16: utf16})
	}

	// 检查哈希冲突 (引擎二分查找时重复哈希的条目只有一个可达, 默认拒绝打包)
	seen := make(map[uint32]string)
	for _, it := range items {
		if old, ok := seen[it.entry.Hash]; ok {
			if !forceCollision {
				return fmt.Errorf(
					"Hash collision: \"%s\" 和 \"%s\" 都算出 0x%08X — 引擎二分查找时其中一个条目永远不可达, 已拒绝打包 (确需打包请加 --force)",
					old, it.entry.Token, it.entry.Hash)
			}
			fmt.Fprintf(os.Stderr, "⚠ Hash collision: \"%s\" 和 \"%s\" 都算出 0x%08X (--force 放行)\n", old, it.entry.Token, it.entry.Hash)
		} else {
			seen[it.entry.Hash] = it.entry.Token
		}
	}

	// 按 hash 排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].entry.Hash < items[j].entry.Hash
	})

	// 打包到块
	blocks := make([][]byte, 0)
	cur := make([]byte, MaxBlockSize)
	curOff := 0
	blockIdx := 0
	lookups := make([]CaptionLookup, 0, len(items))

	for _, it := range items {
		dataLen := len(it.utf16)
		if dataLen > MaxBlockSize {
			// 单条超过块大小 → 引擎按 offset+len 读会越界, 直接报错
			return fmt.Errorf("条目 \"%s\" (%d 字节) 超过块大小 %d, 无法打包", it.entry.Token, dataLen, MaxBlockSize)
		}
		if curOff+dataLen > MaxBlockSize {
			blocks = append(blocks, cur)
			cur = make([]byte, MaxBlockSize)
			curOff = 0
			blockIdx++
		}
		copy(cur[curOff:], it.utf16)
		lookups = append(lookups, CaptionLookup{
			Hash:     it.entry.Hash,
			BlockNum: uint32(blockIdx),
			Offset:   uint16(curOff),
			Length:   uint16(dataLen),
		})
		curOff += dataLen
	}
	if curOff > 0 || len(blocks) == 0 {
		blocks = append(blocks, cur)
	}

	// 计算 dataoffset
	numBlocks := len(blocks)
	dirSize := len(lookups)
	dataOffset := HeaderSize + dirSize*LookupSize
	if align {
		aligned := (dataOffset + 511) &^ 511
		if aligned > dataOffset {
			dataOffset = aligned
		}
	}

	// 写入
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Header
	hdr := CompiledCaptionHeader{
		Magic:         MagicVCCD,
		Version:       CaptionVersion,
		NumBlocks:     uint32(numBlocks),
		BlockSize:     MaxBlockSize,
		DirectorySize: uint32(dirSize),
		DataOffset:    uint32(dataOffset),
	}
	if err := binary.Write(f, binary.LittleEndian, &hdr); err != nil {
		return err
	}

	// Directory
	for _, l := range lookups {
		if err := binary.Write(f, binary.LittleEndian, &l); err != nil {
			return err
		}
	}

	// 对齐填充
	if pad := dataOffset - HeaderSize - dirSize*LookupSize; pad > 0 {
		if _, err := f.Write(make([]byte, pad)); err != nil {
			return err
		}
	}

	// 数据块
	for _, b := range blocks {
		if _, err := f.Write(b); err != nil {
			return err
		}
	}

	return nil
}

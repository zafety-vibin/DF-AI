package topology

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

// CompressionConfig specifies how topology should be compressed
type CompressionConfig struct {
	Mode    string // full, active_z_levels, custom_bounds
	CenterZ uint16 // For active_z_levels mode
	ZRadius uint16 // ±radius levels from center
}

// CompressedTopology represents RLE-encoded topology for LLM context
type CompressedTopology struct {
	Data             []byte    // RLE-compressed byte stream
	OriginalWidth    uint16    // Map width
	OriginalHeight   uint16    // Map height
	OriginalDepth    uint16    // Map depth
	CompressedSize   uint32    // Size of Data in bytes
	UncompressedSize uint32    // Original bit array size
	CompressionRatio float64   // Compressed / Uncompressed (lower = better)
	Mode             string    // Compression mode used
	ZLevelsIncluded  []uint16  // List of Z-levels in compressed data
	CompressedAt     time.Time // When compression was performed
}

// Compress creates an RLE-compressed representation of the overlay
// Supports three modes: full, active_z_levels, custom_bounds
// Performance: <10 milliseconds for full map, <5ms for filtered
func (t *TopologyOverlay) Compress(config CompressionConfig) (*CompressedTopology, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Mode validation
	validModes := map[string]bool{"full": true, "active_z_levels": true, "custom_bounds": true}
	if !validModes[config.Mode] {
		return nil, fmt.Errorf("invalid compression mode: '%s' (valid: full, active_z_levels, custom_bounds)", config.Mode)
	}

	// Determine which Z-levels to include
	var zLevels []uint16
	var dataToCompress []byte

	switch config.Mode {
	case "full":
		// Include all Z-levels
		zLevels = make([]uint16, t.depth)
		for i := uint16(0); i < t.depth; i++ {
			zLevels[i] = i
		}
		dataToCompress = t.data

	case "active_z_levels":
		// Calculate Z range with clamping
		zMin := int32(config.CenterZ) - int32(config.ZRadius)
		zMax := int32(config.CenterZ) + int32(config.ZRadius)

		// Clamp to valid range
		if zMin < 0 {
			zMin = 0
		}
		if zMax >= int32(t.depth) {
			zMax = int32(t.depth) - 1
		}

		// Build Z-levels list
		zLevels = make([]uint16, 0, zMax-zMin+1)
		for z := zMin; z <= zMax; z++ {
			zLevels = append(zLevels, uint16(z))
		}

		// Extract bytes for included Z-levels
		dataToCompress = t.extractZLevels(zLevels)

	case "custom_bounds":
		// TODO: Implement custom bounding box filtering
		return nil, fmt.Errorf("custom_bounds mode not yet implemented")
	}

	// Build compressed output
	var buf bytes.Buffer

	// Write 12-byte header:
	// [2: width][2: height][2: depth][1: mode][1: zLevelCount][4: dataLength]
	binary.Write(&buf, binary.BigEndian, t.width)
	binary.Write(&buf, binary.BigEndian, t.height)
	binary.Write(&buf, binary.BigEndian, t.depth)

	// Mode byte
	var modeByte byte
	switch config.Mode {
	case "full":
		modeByte = 0
	case "active_z_levels":
		modeByte = 1
	case "custom_bounds":
		modeByte = 2
	}
	buf.WriteByte(modeByte)

	// Z-level count
	buf.WriteByte(byte(len(zLevels)))

	// dataLength placeholder - will fill at end
	dataLengthPos := buf.Len()
	binary.Write(&buf, binary.BigEndian, uint32(0))

	// Write Z-level list for filtered modes
	if config.Mode != "full" {
		for _, z := range zLevels {
			binary.Write(&buf, binary.BigEndian, z)
		}
	}

	// RLE compress the bit array
	startPos := buf.Len()
	if err := rleCompress(&buf, dataToCompress); err != nil {
		return nil, fmt.Errorf("RLE compression failed: %w", err)
	}

	// Fill dataLength in header
	dataLength := uint32(buf.Len() - startPos)
	bufBytes := buf.Bytes()
	binary.BigEndian.PutUint32(bufBytes[dataLengthPos:dataLengthPos+4], dataLength)

	compressed := &CompressedTopology{
		Data:             bufBytes,
		OriginalWidth:    t.width,
		OriginalHeight:   t.height,
		OriginalDepth:    t.depth,
		CompressedSize:   uint32(len(bufBytes)),
		UncompressedSize: uint32(len(dataToCompress)),
		CompressionRatio: float64(len(bufBytes)) / float64(len(dataToCompress)),
		Mode:             config.Mode,
		ZLevelsIncluded:  zLevels,
		CompressedAt:     time.Now(),
	}

	return compressed, nil
}

// extractZLevels extracts byte slices for specified Z-levels
// Returns concatenated byte slices for included levels
func (t *TopologyOverlay) extractZLevels(zLevels []uint16) []byte {
	// Calculate bytes per Z-level
	bytesPerLevel := (uint32(t.width) * uint32(t.height) + 7) / 8

	// Allocate output buffer
	output := make([]byte, 0, uint32(len(zLevels))*bytesPerLevel)

	for _, z := range zLevels {
		// Calculate byte offset for this Z-level
		startByte := uint32(z) * bytesPerLevel
		endByte := startByte + bytesPerLevel

		// Append this Z-level's bytes
		output = append(output, t.data[startByte:endByte]...)
	}

	return output
}

// rleCompress performs run-length encoding on byte array
// Encodes as: [varint: count][byte: value] pairs
func rleCompress(buf *bytes.Buffer, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	currentByte := data[0]
	runCount := uint32(1)

	for i := 1; i < len(data); i++ {
		if data[i] == currentByte && runCount < 16777215 { // Max 3-byte varint
			runCount++
		} else {
			// Emit run
			if err := writeVarint(buf, runCount); err != nil {
				return err
			}
			buf.WriteByte(currentByte)

			// Start new run
			currentByte = data[i]
			runCount = 1
		}
	}

	// Emit final run
	if err := writeVarint(buf, runCount); err != nil {
		return err
	}
	buf.WriteByte(currentByte)

	return nil
}

// writeVarint encodes count as 1, 2, or 3 bytes
// 1 byte: 0-127 (MSB=0)
// 2 bytes: 128-16383 (MSB=1 in first byte)
// 3 bytes: 16384+ (MSB=1 in first two bytes)
func writeVarint(buf *bytes.Buffer, count uint32) error {
	if count < 128 {
		// 1 byte
		buf.WriteByte(byte(count))
	} else if count < 16384 {
		// 2 bytes
		buf.WriteByte(byte((count>>7)|0x80)) // MSB=1
		buf.WriteByte(byte(count & 0x7F))
	} else {
		// 3 bytes
		buf.WriteByte(byte((count>>14)|0x80)) // MSB=1
		buf.WriteByte(byte((count>>7)|0x80))  // MSB=1
		buf.WriteByte(byte(count & 0x7F))
	}
	return nil
}

// Decompress reconstructs the original bit array from RLE data
// Used for validation and testing
// Performance: <20 milliseconds
func (c *CompressedTopology) Decompress() ([]byte, error) {
	// Parse header
	if len(c.Data) < 12 {
		return nil, fmt.Errorf("compressed data too short: %d bytes", len(c.Data))
	}

	// Skip header, read RLE data
	reader := bytes.NewReader(c.Data[12:])

	// Calculate expected output size
	bytesNeeded := (uint32(c.OriginalWidth) * uint32(c.OriginalHeight) * uint32(c.OriginalDepth) + 7) / 8
	output := make([]byte, bytesNeeded)
	outPos := 0

	for reader.Len() > 0 {
		// Read run count (varint)
		count, err := readVarint(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read run count: %w", err)
		}

		// Read byte value
		val, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read byte value: %w", err)
		}

		// Expand run
		for i := uint32(0); i < count; i++ {
			if outPos >= len(output) {
				return nil, fmt.Errorf("decompression overflow: outPos=%d, size=%d", outPos, len(output))
			}
			output[outPos] = val
			outPos++
		}
	}

	if outPos != len(output) {
		return nil, fmt.Errorf("decompressed size mismatch: got %d bytes, expected %d", outPos, len(output))
	}

	return output, nil
}

// readVarint decodes 1, 2, or 3 byte varint
func readVarint(reader *bytes.Reader) (uint32, error) {
	b1, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	// 1 byte (MSB=0)
	if b1&0x80 == 0 {
		return uint32(b1), nil
	}

	// 2 bytes
	b2, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	if b2&0x80 == 0 {
		// Only 2 bytes total
		return uint32(b1&0x7F)<<7 | uint32(b2), nil
	}

	// 3 bytes
	b3, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	return uint32(b1&0x7F)<<14 | uint32(b2&0x7F)<<7 | uint32(b3), nil
}

// Validate performs round-trip test: decompress and compare with original
// Returns error if decompressed data doesn't match original
func (c *CompressedTopology) Validate(original *TopologyOverlay) error {
	decompressed, err := c.Decompress()
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	// Compare with original
	original.mu.RLock()
	defer original.mu.RUnlock()

	if len(decompressed) != len(original.data) {
		return fmt.Errorf("size mismatch: decompressed=%d, original=%d", len(decompressed), len(original.data))
	}

	for i := range decompressed {
		if decompressed[i] != original.data[i] {
			return fmt.Errorf("byte mismatch at index %d: decompressed=0x%02X, original=0x%02X", i, decompressed[i], original.data[i])
		}
	}

	return nil
}

// GetSize returns compressed size in bytes
func (c *CompressedTopology) GetSize() uint32 {
	return c.CompressedSize
}

// GetRatio returns compression ratio (compressed / uncompressed)
func (c *CompressedTopology) GetRatio() float64 {
	return c.CompressionRatio
}

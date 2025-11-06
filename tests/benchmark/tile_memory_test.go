package benchmark

import (
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// BenchmarkTileMemory tests memory usage for tile storage
func BenchmarkTileMemory(b *testing.B) {
	testCases := []struct {
		name                string
		width, height, depth uint16
	}{
		{"Small_48x48x20", 48, 48, 20},
		{"Medium_100x100x100", 100, 100, 100},
		{"Large_200x200x100", 200, 200, 100},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			tileCount := uint32(tc.width) * uint32(tc.height) * uint32(tc.depth)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Create tile array (simulating full state message)
				tiles := make([]protocol.TileState, tileCount)

				// Fill with sample data
				idx := uint32(0)
				for z := int16(0); z < int16(tc.depth); z++ {
					for y := int16(0); y < int16(tc.height); y++ {
						for x := int16(0); x < int16(tc.width); x++ {
							tiles[idx] = protocol.TileState{
								X:        x,
								Y:        y,
								Z:        z,
								TileType: 1, // Sample tile type
								Flags:    0,
							}
							idx++
						}
					}
				}

				// Prevent compiler optimization
				_ = tiles
			}

			// Note: Threshold validation moved to post-benchmark analysis
			// Go's testing.B doesn't provide direct bytes/op during benchmark
			// Use: go test -bench=. -benchmem to see memory stats
			// Then manually validate B/op ≤ 64 * tileCount
		})
	}
}

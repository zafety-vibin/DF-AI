package benchmark

import (
	"math/rand"
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// BenchmarkTileAccess tests tile coordinate lookup speed
func BenchmarkTileAccess(b *testing.B) {
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
			// Setup: Create tile map
			tileCount := uint32(tc.width) * uint32(tc.height) * uint32(tc.depth)
			tiles := make(map[uint32]*protocol.TileState, tileCount)

			// Fill tile map with sample data
			idx := uint32(0)
			for z := int16(0); z < int16(tc.depth); z++ {
				for y := int16(0); y < int16(tc.height); y++ {
					for x := int16(0); x < int16(tc.width); x++ {
						tiles[idx] = &protocol.TileState{
							X:        x,
							Y:        y,
							Z:        z,
							TileType: 1,
							Flags:    0,
						}
						idx++
					}
				}
			}

			// Pre-generate random lookup indices
			lookupIndices := make([]uint32, 1000)
			rng := rand.New(rand.NewSource(12345)) // Deterministic seed
			for i := range lookupIndices {
				lookupIndices[i] = uint32(rng.Int31n(int32(tileCount)))
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Random coordinate lookup
				lookupIdx := lookupIndices[i%len(lookupIndices)]
				tile := tiles[lookupIdx]

				// Prevent compiler optimization
				_ = tile
			}

			// Note: Threshold validation can be done by examining benchmark output
			// Target: ns/op should be ≤500 nanoseconds
			// Use: go test -bench=. to see timing results
		})
	}
}

package persist

import "testing"

func TestBlockIndexMatchesClient(t *testing.T) {
	// Godot: lx + SIZE * (y + HEIGHT * lz)
	got := BlockIndex(3, 10, 5)
	want := 3 + ChunkSize*(10+ChunkHeight*5)
	if got != want {
		t.Fatalf("BlockIndex = %d, want %d", got, want)
	}
	if len(EmptyChunk(ChunkCoord{}).Blocks) != ChunkSize*ChunkSize*ChunkHeight {
		t.Fatal("chunk volume mismatch")
	}
}

func TestGenerateChunkInBounds(t *testing.T) {
	c := GenerateChunk(ChunkCoord{X: 0, Z: 0})
	if c.Blocks[BlockIndex(0, HeightAt(0, 0), 0)] != BlockOakPlanks {
		t.Fatalf("spawn plaza floor missing, got %d", c.Blocks[BlockIndex(0, HeightAt(0, 0), 0)])
	}
}

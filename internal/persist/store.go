package persist

import "context"

// ChunkSize is the horizontal extent of one chunk (Minecraft-like).
const ChunkSize = 16

// ChunkHeight is the vertical extent of one chunk column for v1.
const ChunkHeight = 128

// ChunkCoord identifies a chunk column in world space.
type ChunkCoord struct {
	X int32 `json:"x"`
	Z int32 `json:"z"`
}

// Chunk holds block IDs for one column. Blocks are stored as
// index = ((y * ChunkSize) + z) * ChunkSize + x with 0 = air.
type Chunk struct {
	Coord  ChunkCoord
	Blocks []uint16 // len == ChunkSize*ChunkSize*ChunkHeight when loaded
}

// EmptyChunk allocates an all-air chunk at coord.
func EmptyChunk(coord ChunkCoord) *Chunk {
	return &Chunk{
		Coord:  coord,
		Blocks: make([]uint16, ChunkSize*ChunkSize*ChunkHeight),
	}
}

// BlockIndex returns the flat index for local (x,y,z) inside a chunk.
func BlockIndex(x, y, z int) int {
	return ((y * ChunkSize) + z) * ChunkSize + x
}

// Store is the chunk persistence boundary. File/SQLite first; object storage later.
type Store interface {
	// GetChunk returns nil, nil when the chunk has never been written (caller may generate).
	GetChunk(ctx context.Context, worldID string, coord ChunkCoord) (*Chunk, error)
	PutChunk(ctx context.Context, worldID string, chunk *Chunk) error
	// ListWorlds returns known world IDs that have at least one persisted chunk.
	ListWorlds(ctx context.Context) ([]string, error)
}

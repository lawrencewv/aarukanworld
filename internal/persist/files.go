package persist

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileStore persists chunks as raw uint16 little-endian blobs under dataDir/worldID/cx_cz.bin.
type FileStore struct {
	root string
	mu   sync.Mutex
}

func NewFileStore(root string) (*FileStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("data dir is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{root: root}, nil
}

func (s *FileStore) GetChunk(_ context.Context, worldID string, coord ChunkCoord) (*Chunk, error) {
	worldID, err := sanitizeWorldID(worldID)
	if err != nil {
		return nil, err
	}
	path := s.chunkPath(worldID, coord)
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	want := ChunkSize * ChunkSize * ChunkHeight * 2
	if len(data) != want {
		return nil, fmt.Errorf("corrupt chunk %s: size %d", path, len(data))
	}
	blocks := make([]uint16, ChunkSize*ChunkSize*ChunkHeight)
	for i := range blocks {
		blocks[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	return &Chunk{Coord: coord, Blocks: blocks}, nil
}

func (s *FileStore) PutChunk(_ context.Context, worldID string, chunk *Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk is nil")
	}
	worldID, err := sanitizeWorldID(worldID)
	if err != nil {
		return err
	}
	want := ChunkSize * ChunkSize * ChunkHeight
	if len(chunk.Blocks) != want {
		return fmt.Errorf("chunk block count %d, want %d", len(chunk.Blocks), want)
	}
	dir := filepath.Join(s.root, worldID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	buf := make([]byte, want*2)
	for i, b := range chunk.Blocks {
		binary.LittleEndian.PutUint16(buf[i*2:], b)
	}
	path := s.chunkPath(worldID, chunk.Coord)
	tmp := path + ".tmp"
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *FileStore) ListWorlds(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func (s *FileStore) chunkPath(worldID string, coord ChunkCoord) string {
	name := fmt.Sprintf("%d_%d.bin", coord.X, coord.Z)
	return filepath.Join(s.root, worldID, name)
}

func sanitizeWorldID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("world_id is required")
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid world_id")
	}
	return id, nil
}

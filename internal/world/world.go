package world

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"aarukanworld/internal/persist"
)

// World is one authoritative creative map keyed by world_id.
type World struct {
	ID    string
	store persist.Store

	mu    sync.RWMutex
	peers map[string]*Peer // nick lower -> peer

	chunkMu sync.Mutex
	chunks  map[persist.ChunkCoord]*persist.Chunk
}

func newWorld(id string, store persist.Store) *World {
	return &World{
		ID:     id,
		store:  store,
		peers:  make(map[string]*Peer),
		chunks: make(map[persist.ChunkCoord]*persist.Chunk),
	}
}

func (w *World) addPeer(p *Peer) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := strings.ToLower(p.Nick)
	if existing, ok := w.peers[key]; ok && existing.ID != p.ID {
		return fmt.Errorf("nick already in world")
	}
	w.peers[key] = p
	return nil
}

func (w *World) removePeer(p *Peer) {
	w.mu.Lock()
	key := strings.ToLower(p.Nick)
	if cur, ok := w.peers[key]; ok && cur.ID == p.ID {
		delete(w.peers, key)
	}
	empty := len(w.peers) == 0
	w.mu.Unlock()
	if empty {
		w.flushAll()
	}
}

func (w *World) broadcast(msg Message, exceptPeerID string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, p := range w.peers {
		if exceptPeerID != "" && p.ID == exceptPeerID {
			continue
		}
		p.push(msg)
	}
}

// PeerNicks returns display nicks currently attached to this world.
func (w *World) PeerNicks() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, 0, len(w.peers))
	for _, p := range w.peers {
		out = append(out, p.Nick)
	}
	return out
}

func (w *World) HandleClient(p *Peer, msg Message) {
	switch msg.Type {
	case MsgPing:
		p.push(Message{Type: MsgPong})
	case MsgPose:
		p.SetPose(msg.X, msg.Y, msg.Z, msg.Yaw, msg.Pitch)
		w.broadcast(Message{
			Type:  MsgPeerPose,
			Nick:  p.Nick,
			X:     msg.X,
			Y:     msg.Y,
			Z:     msg.Z,
			Yaw:   msg.Yaw,
			Pitch: msg.Pitch,
		}, p.ID)
	case MsgBlockPlace:
		if err := w.applyBlock(int32(msg.X), int32(msg.Y), int32(msg.Z), msg.Block); err != nil {
			p.push(Message{Type: MsgError, Text: err.Error()})
			return
		}
		w.broadcast(Message{
			Type:  MsgBlockPlace,
			Nick:  p.Nick,
			X:     msg.X,
			Y:     msg.Y,
			Z:     msg.Z,
			Block: msg.Block,
		}, "")
	case MsgBlockBreak:
		if err := w.applyBlock(int32(msg.X), int32(msg.Y), int32(msg.Z), 0); err != nil {
			p.push(Message{Type: MsgError, Text: err.Error()})
			return
		}
		w.broadcast(Message{
			Type: MsgBlockBreak,
			Nick: p.Nick,
			X:    msg.X,
			Y:    msg.Y,
			Z:    msg.Z,
		}, "")
	case MsgChunkRequest:
		chunk, err := w.loadChunk(persist.ChunkCoord{X: msg.CX, Z: msg.CZ})
		if err != nil {
			p.push(Message{Type: MsgError, Text: err.Error()})
			return
		}
		p.push(Message{
			Type:   MsgChunkData,
			CX:     chunk.Coord.X,
			CZ:     chunk.Coord.Z,
			Blocks: chunk.Blocks,
		})
	default:
		p.push(Message{Type: MsgError, Text: "unknown message type"})
	}
}

func (w *World) applyBlock(bx, by, bz int32, block uint16) error {
	lx, ly, lz, ok := localBlock(bx, by, bz)
	if !ok {
		return fmt.Errorf("block out of range")
	}
	coord := chunkCoordFromBlock(bx, bz)
	chunk, err := w.loadChunk(coord)
	if err != nil {
		return err
	}
	chunk.Blocks[persist.BlockIndex(lx, ly, lz)] = block
	return w.store.PutChunk(context.Background(), w.ID, chunk)
}

func (w *World) loadChunk(coord persist.ChunkCoord) (*persist.Chunk, error) {
	w.chunkMu.Lock()
	defer w.chunkMu.Unlock()
	if c, ok := w.chunks[coord]; ok {
		return c, nil
	}
	c, err := w.store.GetChunk(context.Background(), w.ID, coord)
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = persist.GenerateChunk(coord)
	}
	w.chunks[coord] = c
	return c, nil
}

func (w *World) flushAll() {
	w.chunkMu.Lock()
	defer w.chunkMu.Unlock()
	for _, c := range w.chunks {
		if err := w.store.PutChunk(context.Background(), w.ID, c); err != nil {
			slog.Error("flush chunk failed", "world", w.ID, "err", err)
		}
	}
}

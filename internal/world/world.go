package world

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

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

	flagMu   sync.Mutex
	flags    map[string]*Flag
	flagOnce sync.Once
}

func newWorld(id string, store persist.Store) *World {
	w := &World{
		ID:     id,
		store:  store,
		peers:  make(map[string]*Peer),
		chunks: make(map[persist.ChunkCoord]*persist.Chunk),
		flags:  make(map[string]*Flag),
	}
	w.startFlagLoop()
	return w
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
		p.SetPose(msg.X, msg.Y, msg.Z, msg.Yaw, msg.Pitch, msg.Breaking, msg.Held, msg.Drawing, msg.Swinging)
		w.broadcast(Message{
			Type:     MsgPeerPose,
			Nick:     p.Nick,
			X:        msg.X,
			Y:        msg.Y,
			Z:        msg.Z,
			Yaw:      msg.Yaw,
			Pitch:    msg.Pitch,
			Breaking: msg.Breaking,
			Held:     msg.Held,
			Drawing:  msg.Drawing,
			Swinging: msg.Swinging,
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
	case MsgAttack:
		w.handleAttack(p, msg.Nick)
	case MsgArrowHit:
		w.handleArrowHit(p, msg.Nick)
	case MsgArrowShot:
		w.handleArrowShot(p, msg)
	case MsgRespawn:
		w.handleRespawn(p)
	default:
		p.push(Message{Type: MsgError, Text: "unknown message type"})
	}
}

func (w *World) handleAttack(attacker *Peer, targetNick string) {
	w.resolveHit(attacker, targetNick, SwordStrikeRange, SwordDamage, attacker.TryBeginAttack)
}

func (w *World) handleArrowHit(attacker *Peer, targetNick string) {
	w.resolveHit(attacker, targetNick, ArrowMaxRange, ArrowDamage, attacker.TryBeginArrowHit)
}

func (w *World) handleArrowShot(shooter *Peer, msg Message) {
	// Cosmetic relay: other clients spawn a visual arrow from the shooter's bow.
	if !shooter.Alive() {
		return
	}
	dx, dy, dz := msg.X, msg.Y, msg.Z
	len := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if len < 0.001 || len > 1.5 {
		return
	}
	w.broadcast(Message{
		Type: MsgPeerArrow,
		Nick: shooter.Nick,
		X:    dx / len,
		Y:    dy / len,
		Z:    dz / len,
	}, shooter.ID)
}

func (w *World) resolveHit(
	attacker *Peer,
	targetNick string,
	maxRange float64,
	damage int,
	beginCooldown func(time.Time) bool,
) {
	targetNick = strings.TrimSpace(targetNick)
	if targetNick == "" || strings.EqualFold(targetNick, attacker.Nick) {
		return
	}
	if !attacker.Alive() {
		return
	}
	w.mu.RLock()
	target := w.peers[strings.ToLower(targetNick)]
	w.mu.RUnlock()
	if target == nil || !target.Alive() {
		return
	}
	if !beginCooldown(time.Now().UTC()) {
		return
	}
	ap := attacker.Pose()
	tp := target.Pose()
	dx := ap.X - tp.X
	dy := ap.Y - tp.Y
	dz := ap.Z - tp.Z
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if dist > maxRange {
		return
	}
	health := target.ApplyDamage(damage)
	kx, kz := knockbackAway(ap, tp)
	w.broadcast(Message{
		Type:   MsgPeerHealth,
		Nick:   target.Nick,
		Health: health,
		KnockX: kx,
		KnockZ: kz,
		Dead:   health <= 0,
	}, "")
}

func (w *World) handleRespawn(p *Peer) {
	if p.Alive() {
		return
	}
	sx, sy, sz := spawnPoint()
	p.RespawnAt(sx, sy, sz)
	pose := p.Pose()
	w.broadcast(Message{
		Type:   MsgPeerHealth,
		Nick:   p.Nick,
		Health: MaxHealth,
		X:      sx,
		Y:      sy,
		Z:      sz,
		Dead:   false,
	}, "")
	w.broadcast(Message{
		Type:     MsgPeerPose,
		Nick:     p.Nick,
		X:        pose.X,
		Y:        pose.Y,
		Z:        pose.Z,
		Yaw:      pose.Yaw,
		Pitch:    pose.Pitch,
		Breaking: false,
	}, "")
}

func spawnPoint() (x, y, z float64) {
	// Centre of the world plaza — mirrors client VoxelWorld.spawn_position().
	return 0.5, float64(persist.HeightAt(0, 0) + 3), 0.5
}

func knockbackAway(attacker, target Pose) (kx, kz float64) {
	awayX := target.X - attacker.X
	awayZ := target.Z - attacker.Z
	len := math.Hypot(awayX, awayZ)
	if len < 0.001 {
		awayX = -math.Sin(attacker.Yaw)
		awayZ = -math.Cos(attacker.Yaw)
		len = math.Hypot(awayX, awayZ)
	}
	if len < 0.001 {
		return 0, 0
	}
	return (awayX / len) * SwordKnockbackSpeed, (awayZ / len) * SwordKnockbackSpeed
}

func (w *World) applyBlock(bx, by, bz int32, block uint16) error {
	lx, ly, lz, ok := localBlock(bx, by, bz)
	if !ok {
		return fmt.Errorf("block out of range")
	}
	if ly == 0 {
		return fmt.Errorf("cannot modify bedrock")
	}
	coord := chunkCoordFromBlock(bx, bz)
	chunk, err := w.loadChunk(coord)
	if err != nil {
		return err
	}
	idx := persist.BlockIndex(lx, ly, lz)
	if chunk.Blocks[idx] == persist.BlockBedrock && block != persist.BlockBedrock {
		return fmt.Errorf("cannot modify bedrock")
	}
	chunk.Blocks[idx] = block
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
	persist.SealBedrock(c)
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

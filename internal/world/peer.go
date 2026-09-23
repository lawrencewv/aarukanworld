package world

import (
	"io"
	"sync"
	"time"
)

// Pose is a player's authoritative transform snapshot.
type Pose struct {
	X, Y, Z    float64
	Yaw, Pitch float64
	Breaking   bool
	Held       int
	Drawing    bool
	UpdatedAt  time.Time
}

// Peer is one authenticated game client attached to a world.
type Peer struct {
	ID      string
	Nick    string
	WorldID string
	Session string // chat session id from play token, if present

	mu           sync.RWMutex
	pose         Pose
	health       int
	lastAttack   time.Time
	lastArrowHit time.Time
	connected    bool
	closed       bool

	subsMu sync.Mutex
	subs   map[chan Message]struct{}

	connMu sync.Mutex
	conn   io.Closer
}

func newPeer(id, nick, worldID, session string) *Peer {
	return &Peer{
		ID:        id,
		Nick:      nick,
		WorldID:   worldID,
		Session:   session,
		connected: true,
		health:    MaxHealth,
		subs:      make(map[chan Message]struct{}),
		pose: Pose{
			Y:         80,
			UpdatedAt: time.Now().UTC(),
		},
	}
}

func (p *Peer) SetConn(c io.Closer) {
	p.connMu.Lock()
	defer p.connMu.Unlock()
	p.conn = c
}

func (p *Peer) closeConn() {
	p.connMu.Lock()
	c := p.conn
	p.conn = nil
	p.connMu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

func (p *Peer) Subscribe(buffer int) chan Message {
	ch := make(chan Message, buffer)
	p.subsMu.Lock()
	p.subs[ch] = struct{}{}
	p.subsMu.Unlock()
	p.mu.Lock()
	p.connected = true
	p.mu.Unlock()
	return ch
}

func (p *Peer) Unsubscribe(ch chan Message) {
	p.subsMu.Lock()
	if _, ok := p.subs[ch]; ok {
		delete(p.subs, ch)
		close(ch)
	}
	open := len(p.subs) > 0
	p.subsMu.Unlock()
	if !open {
		p.mu.Lock()
		p.connected = false
		p.mu.Unlock()
	}
}

func (p *Peer) SetPose(x, y, z, yaw, pitch float64, breaking bool, held int, drawing bool) {
	p.mu.Lock()
	p.pose = Pose{
		X: x, Y: y, Z: z, Yaw: yaw, Pitch: pitch,
		Breaking: breaking, Held: held, Drawing: drawing,
		UpdatedAt: time.Now().UTC(),
	}
	p.mu.Unlock()
}

func (p *Peer) Pose() Pose {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pose
}

func (p *Peer) Health() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.health
}

// TryBeginAttack returns false if the peer is still on sword cooldown.
func (p *Peer) TryBeginAttack(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.lastAttack.IsZero() && now.Sub(p.lastAttack) < time.Duration(SwordAttackCooldown)*time.Millisecond {
		return false
	}
	p.lastAttack = now
	return true
}

// TryBeginArrowHit returns false if the peer is still on arrow-hit cooldown.
func (p *Peer) TryBeginArrowHit(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.lastArrowHit.IsZero() && now.Sub(p.lastArrowHit) < time.Duration(ArrowAttackCooldown)*time.Millisecond {
		return false
	}
	p.lastArrowHit = now
	return true
}

// ApplyDamage subtracts amount from health (floored at 0) and returns the new value.
// Already-dead peers are left at 0.
func (p *Peer) ApplyDamage(amount int) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.health <= 0 {
		return 0
	}
	p.health -= amount
	if p.health < 0 {
		p.health = 0
	}
	return p.health
}

// Alive reports whether the peer can act and be damaged.
func (p *Peer) Alive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.health > 0
}

// RespawnAt restores full health and snaps the pose to the given spawn point.
func (p *Peer) RespawnAt(x, y, z float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health = MaxHealth
	p.pose = Pose{
		X:         x,
		Y:         y,
		Z:         z,
		Yaw:       p.pose.Yaw,
		Pitch:     0,
		Breaking:  false,
		UpdatedAt: time.Now().UTC(),
	}
}

func (p *Peer) push(msg Message) {
	// Snapshot subscribers so we never block while holding subsMu (Unsubscribe closes chans).
	p.subsMu.Lock()
	subs := make([]chan Message, 0, len(p.subs))
	for ch := range p.subs {
		subs = append(subs, ch)
	}
	p.subsMu.Unlock()

	// Pose/join spam may drop under load; chunk + edit delivery must not.
	reliable := msg.Type == MsgChunkData ||
		msg.Type == MsgBlockPlace ||
		msg.Type == MsgBlockBreak ||
		msg.Type == MsgPeerHealth ||
		msg.Type == MsgError ||
		msg.Type == MsgWelcome

	for _, ch := range subs {
		if reliable {
			func() {
				defer func() { _ = recover() }() // channel may be closed mid-send
				ch <- msg
			}()
			continue
		}
		select {
		case ch <- msg:
		default:
			// Slow peer: drop rather than block the world fan-out.
		}
	}
}

func (p *Peer) markClosed() {
	p.mu.Lock()
	p.closed = true
	p.connected = false
	p.mu.Unlock()
	p.closeConn()
}

func (p *Peer) closeSubs() {
	p.subsMu.Lock()
	defer p.subsMu.Unlock()
	for ch := range p.subs {
		close(ch)
		delete(p.subs, ch)
	}
}

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
	UpdatedAt  time.Time
}

// Peer is one authenticated game client attached to a world.
type Peer struct {
	ID      string
	Nick    string
	WorldID string
	Session string // chat session id from play token, if present

	mu        sync.RWMutex
	pose      Pose
	connected bool
	closed    bool

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

func (p *Peer) SetPose(x, y, z, yaw, pitch float64) {
	p.mu.Lock()
	p.pose = Pose{X: x, Y: y, Z: z, Yaw: yaw, Pitch: pitch, UpdatedAt: time.Now().UTC()}
	p.mu.Unlock()
}

func (p *Peer) Pose() Pose {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pose
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

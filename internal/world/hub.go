package world

import (
	"fmt"
	"strings"
	"sync"

	"aarukanworld/internal/auth"
	"aarukanworld/internal/persist"

	"github.com/google/uuid"
)

// Hub owns live worlds and attaches authenticated peers.
type Hub struct {
	store persist.Store

	mu     sync.RWMutex
	worlds map[string]*World
	peers  map[string]*Peer // peer id -> peer
}

func NewHub(store persist.Store) *Hub {
	return &Hub{
		store:  store,
		worlds: make(map[string]*World),
		peers:  make(map[string]*Peer),
	}
}

// Attach validates play claims and registers a peer in the claimed world.
func (h *Hub) Attach(claims auth.PlayClaims) (*Peer, *World, error) {
	nick := strings.TrimSpace(claims.Nick)
	worldID := strings.TrimSpace(claims.WorldID)
	if nick == "" || worldID == "" {
		return nil, nil, fmt.Errorf("nick and world_id are required")
	}
	if strings.ContainsAny(nick, " \t\r\n") {
		return nil, nil, fmt.Errorf("nick must not contain whitespace")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	w := h.worlds[worldID]
	if w == nil {
		w = newWorld(worldID, h.store)
		h.worlds[worldID] = w
	}

	p := newPeer(uuid.NewString(), nick, worldID, claims.Session)
	if err := w.addPeer(p); err != nil {
		return nil, nil, err
	}
	h.peers[p.ID] = p

	w.broadcast(Message{Type: MsgPeerJoin, Nick: p.Nick}, p.ID)
	return p, w, nil
}

// Detach removes a peer and notifies the world.
func (h *Hub) Detach(peerID string) {
	h.mu.Lock()
	p, ok := h.peers[peerID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.peers, peerID)
	w := h.worlds[p.WorldID]
	h.mu.Unlock()

	if w != nil {
		w.removePeer(p)
		w.broadcast(Message{Type: MsgPeerLeave, Nick: p.Nick}, "")
	}
	p.markClosed()
	p.closeSubs()
}

func (h *Hub) GetPeer(id string) (*Peer, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	p, ok := h.peers[id]
	return p, ok
}

func (h *Hub) WorldInfo(worldID string) (peers int, ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	w, ok := h.worlds[worldID]
	if !ok {
		return 0, false
	}
	return len(w.PeerNicks()), true
}

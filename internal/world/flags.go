package world

import (
	"fmt"
	"math"
	"strings"
	"time"

	"aarukanworld/internal/persist"
)

// Flag is one capture point in a world.
type Flag struct {
	ID            string
	CX, CZ        int32
	X, Y, Z       float64
	Radius        float64
	OwnerNick     string
	CapturingNick string
	Progress      float64 // 0..1
	Contested     bool
}

func (w *World) startFlagLoop() {
	w.flagOnce.Do(func() {
		go w.flagLoop()
	})
}

func (w *World) flagLoop() {
	ticker := time.NewTicker(time.Duration(FlagTickInterval) * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		w.tickFlags(float64(FlagTickInterval) / 1000.0)
	}
}

func (w *World) tickFlags(dt float64) {
	w.ensureFlagsNearPeers()

	w.mu.RLock()
	peers := make([]*Peer, 0, len(w.peers))
	for _, p := range w.peers {
		peers = append(peers, p)
	}
	w.mu.RUnlock()

	w.flagMu.Lock()
	var updates []Message
	for _, f := range w.flags {
		if w.updateFlagCapture(f, peers, dt) {
			updates = append(updates, f.toMessage(MsgFlagState))
		}
	}
	w.flagMu.Unlock()

	for _, msg := range updates {
		w.broadcast(msg, "")
	}
}

func (w *World) ensureFlagsNearPeers() {
	w.mu.RLock()
	cells := make(map[[2]int32]struct{})
	for _, p := range w.peers {
		pose := p.Pose()
		cx := int32(math.Floor(pose.X / FlagCellSize))
		cz := int32(math.Floor(pose.Z / FlagCellSize))
		for dz := -FlagEnsureRadius; dz <= FlagEnsureRadius; dz++ {
			for dx := -FlagEnsureRadius; dx <= FlagEnsureRadius; dx++ {
				cells[[2]int32{cx + int32(dx), cz + int32(dz)}] = struct{}{}
			}
		}
	}
	w.mu.RUnlock()

	if len(cells) == 0 {
		return
	}

	w.flagMu.Lock()
	defer w.flagMu.Unlock()
	if w.flags == nil {
		w.flags = make(map[string]*Flag)
	}
	for cell := range cells {
		id := flagID(cell[0], cell[1])
		if _, ok := w.flags[id]; ok {
			continue
		}
		w.flags[id] = newFlagAtCell(cell[0], cell[1])
	}
}

func newFlagAtCell(cx, cz int32) *Flag {
	x, y, z := flagWorldPos(cx, cz)
	return &Flag{
		ID:     flagID(cx, cz),
		CX:     cx,
		CZ:     cz,
		X:      x,
		Y:      y,
		Z:      z,
		Radius: FlagCaptureRadius,
	}
}

func flagID(cx, cz int32) string {
	return fmt.Sprintf("%d:%d", cx, cz)
}

func flagWorldPos(cx, cz int32) (x, y, z float64) {
	// Keep a flag near the spawn plaza so new players always have an objective in sight.
	if cx == 0 && cz == 0 {
		wx := 8.5
		wz := 8.5
		wy := float64(persist.HeightAt(int(math.Floor(wx)), int(math.Floor(wz)))) + 1.0
		return wx, wy, wz
	}
	h := flagHash(cx, cz)
	// Keep the pole away from cell edges so neighbouring capture zones don't overlap.
	span := FlagCellSize - 24
	if span < 8 {
		span = 8
	}
	ox := float64(h%uint32(span)) + 12
	oz := float64((h>>16)%uint32(span)) + 12
	wx := float64(cx)*FlagCellSize + ox
	wz := float64(cz)*FlagCellSize + oz
	wy := float64(persist.HeightAt(int(math.Floor(wx)), int(math.Floor(wz)))) + 1.0
	return wx, wy, wz
}

func flagHash(cx, cz int32) uint32 {
	// Simple deterministic mix — not cryptographic.
	x := uint32(cx) * 374761393
	z := uint32(cz) * 668265263
	h := x ^ (z << 3) ^ (z >> 5)
	h *= 2246822519
	return h
}

func (w *World) updateFlagCapture(f *Flag, peers []*Peer, dt float64) bool {
	inside := make([]*Peer, 0, 2)
	for _, p := range peers {
		if !p.Alive() {
			continue
		}
		pose := p.Pose()
		dx := pose.X - f.X
		dz := pose.Z - f.Z
		if math.Hypot(dx, dz) <= f.Radius {
			inside = append(inside, p)
		}
	}

	prevOwner := f.OwnerNick
	prevCapturing := f.CapturingNick
	prevProgress := f.Progress
	prevContested := f.Contested

	switch len(inside) {
	case 1:
		sole := inside[0]
		f.Contested = false
		if strings.EqualFold(sole.Nick, f.OwnerNick) {
			// Already owned by the sole occupant — idle.
			f.CapturingNick = ""
			f.Progress = 0
		} else {
			if !strings.EqualFold(f.CapturingNick, sole.Nick) {
				f.CapturingNick = sole.Nick
				f.Progress = 0
			}
			f.Progress += dt / FlagCaptureSeconds
			if f.Progress >= 1.0-1e-9 {
				f.Progress = 0
				f.OwnerNick = sole.Nick
				f.CapturingNick = ""
			}
		}
	case 0:
		f.Contested = false
		f.CapturingNick = ""
		f.Progress = 0
	default:
		// Contested: more than one living player in the zone — reset.
		f.Contested = true
		f.CapturingNick = ""
		f.Progress = 0
	}

	return f.OwnerNick != prevOwner ||
		f.CapturingNick != prevCapturing ||
		f.Contested != prevContested ||
		progressBucket(f.Progress) != progressBucket(prevProgress) ||
		(f.Progress == 0) != (prevProgress == 0)
}

func progressBucket(p float64) int {
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return 20
	}
	return int(p * 20) // ~5% steps so capture ticks don't flood peers
}

func (f *Flag) toMessage(typ string) Message {
	return Message{
		Type:      typ,
		FlagID:    f.ID,
		X:         f.X,
		Y:         f.Y,
		Z:         f.Z,
		Radius:    f.Radius,
		Owner:     f.OwnerNick,
		Capturing: f.CapturingNick,
		Progress:  f.Progress,
		Contested: f.Contested,
	}
}

func (f *Flag) toWire() FlagWire {
	return FlagWire{
		ID:        f.ID,
		X:         f.X,
		Y:         f.Y,
		Z:         f.Z,
		Radius:    f.Radius,
		Owner:     f.OwnerNick,
		Capturing: f.CapturingNick,
		Progress:  f.Progress,
		Contested: f.Contested,
	}
}

// FlagsSnapshot returns every known flag for a joining peer.
func (w *World) FlagsSnapshot() Message {
	w.ensureFlagsNearPeers()
	// Always seed the origin cell so a lonely spawn still sees a nearby objective.
	w.flagMu.Lock()
	if w.flags == nil {
		w.flags = make(map[string]*Flag)
	}
	originID := flagID(0, 0)
	if _, ok := w.flags[originID]; !ok {
		w.flags[originID] = newFlagAtCell(0, 0)
	}
	out := make([]FlagWire, 0, len(w.flags))
	for _, f := range w.flags {
		out = append(out, f.toWire())
	}
	w.flagMu.Unlock()
	return Message{Type: MsgFlagsSnapshot, Flags: out}
}

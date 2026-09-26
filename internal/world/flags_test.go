package world

import (
	"math"
	"testing"
)

func TestFlagWorldPosDeterministic(t *testing.T) {
	x1, y1, z1 := flagWorldPos(0, 0)
	x2, y2, z2 := flagWorldPos(0, 0)
	if x1 != x2 || y1 != y2 || z1 != z2 {
		t.Fatalf("flag position not stable: (%v,%v,%v) vs (%v,%v,%v)", x1, y1, z1, x2, y2, z2)
	}
	if y1 < 1 {
		t.Fatalf("flag Y too low: %v", y1)
	}
}

func TestCaptureProgressAlone(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	if err := w.addPeer(a); err != nil {
		t.Fatal(err)
	}
	f := newFlagAtCell(0, 0)
	a.SetPose(f.X, f.Y, f.Z, 0, 0, false, 0, false, false)

	changed := w.updateFlagCapture(f, []*Peer{a}, 1.0)
	if !changed {
		t.Fatal("expected progress change")
	}
	if f.CapturingNick != "Alice" {
		t.Fatalf("capturing = %q", f.CapturingNick)
	}
	want := 1.0 / FlagCaptureSeconds
	if math.Abs(f.Progress-want) > 0.0001 {
		t.Fatalf("progress = %v, want %v", f.Progress, want)
	}
}

func TestCaptureCompletesInTenSeconds(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	if err := w.addPeer(a); err != nil {
		t.Fatal(err)
	}
	f := newFlagAtCell(0, 0)
	a.SetPose(f.X, f.Y, f.Z, 0, 0, false, 0, false, false)

	for i := 0; i < int(FlagCaptureSeconds); i++ {
		w.updateFlagCapture(f, []*Peer{a}, 1.0)
	}
	if f.OwnerNick != "Alice" {
		t.Fatalf("owner = %q, want Alice (progress=%v capturing=%q)", f.OwnerNick, f.Progress, f.CapturingNick)
	}
	if f.Progress != 0 || f.CapturingNick != "" {
		t.Fatalf("after capture progress=%v capturing=%q", f.Progress, f.CapturingNick)
	}
}

func TestCaptureResetsWhenContested(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	b := newPeer("b", "Bob", "w1", "")
	if err := w.addPeer(a); err != nil {
		t.Fatal(err)
	}
	if err := w.addPeer(b); err != nil {
		t.Fatal(err)
	}
	f := newFlagAtCell(0, 0)
	a.SetPose(f.X, f.Y, f.Z, 0, 0, false, 0, false, false)
	w.updateFlagCapture(f, []*Peer{a}, 5.0)
	if f.Progress < 0.4 {
		t.Fatalf("expected mid progress, got %v", f.Progress)
	}
	b.SetPose(f.X+1, f.Y, f.Z, 0, 0, false, 0, false, false)
	w.updateFlagCapture(f, []*Peer{a, b}, 0.1)
	if !f.Contested {
		t.Fatal("expected contested")
	}
	if f.Progress != 0 || f.CapturingNick != "" {
		t.Fatalf("contested should reset, progress=%v capturing=%q", f.Progress, f.CapturingNick)
	}
}

func TestCaptureResetsWhenEmpty(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	f := newFlagAtCell(0, 0)
	a.SetPose(f.X, f.Y, f.Z, 0, 0, false, 0, false, false)
	w.updateFlagCapture(f, []*Peer{a}, 4.0)
	w.updateFlagCapture(f, nil, 0.1)
	if f.Progress != 0 || f.CapturingNick != "" {
		t.Fatalf("empty zone should reset, progress=%v capturing=%q", f.Progress, f.CapturingNick)
	}
}

func TestOwnerDoesNotRecapture(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	f := newFlagAtCell(0, 0)
	f.OwnerNick = "Alice"
	a.SetPose(f.X, f.Y, f.Z, 0, 0, false, 0, false, false)
	w.updateFlagCapture(f, []*Peer{a}, 5.0)
	if f.Progress != 0 || f.CapturingNick != "" {
		t.Fatalf("owner should idle, progress=%v capturing=%q", f.Progress, f.CapturingNick)
	}
}

func TestDeadPeerDoesNotCapture(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	for a.Health() > 0 {
		a.ApplyDamage(SwordDamage)
	}
	f := newFlagAtCell(0, 0)
	a.SetPose(f.X, f.Y, f.Z, 0, 0, false, 0, false, false)
	w.updateFlagCapture(f, []*Peer{a}, 5.0)
	if f.Progress != 0 || f.CapturingNick != "" {
		t.Fatalf("dead peer should not capture, progress=%v capturing=%q", f.Progress, f.CapturingNick)
	}
}

func TestFlagsSnapshotIncludesOrigin(t *testing.T) {
	w := newWorld("w1", nil)
	snap := w.FlagsSnapshot()
	if snap.Type != MsgFlagsSnapshot {
		t.Fatalf("type = %q", snap.Type)
	}
	if len(snap.Flags) < 1 {
		t.Fatal("expected at least origin flag")
	}
	found := false
	for _, f := range snap.Flags {
		if f.ID == "0:0" {
			found = true
			if f.Radius != FlagCaptureRadius {
				t.Fatalf("radius = %v", f.Radius)
			}
		}
	}
	if !found {
		t.Fatal("origin flag 0:0 missing")
	}
}

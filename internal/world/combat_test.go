package world

import (
	"math"
	"testing"
	"time"
)

func TestPeerStartsAtMaxHealth(t *testing.T) {
	p := newPeer("id", "Alice", "w1", "")
	if got := p.Health(); got != MaxHealth {
		t.Fatalf("Health() = %d, want %d", got, MaxHealth)
	}
}

func TestApplyDamageStaysAtZero(t *testing.T) {
	p := newPeer("id", "Alice", "w1", "")
	if got := p.ApplyDamage(SwordDamage); got != MaxHealth-SwordDamage {
		t.Fatalf("after one hit Health = %d, want %d", got, MaxHealth-SwordDamage)
	}
	for i := 0; i < 9; i++ {
		p.ApplyDamage(SwordDamage)
	}
	if got := p.Health(); got != 0 {
		t.Fatalf("after lethal hit Health = %d, want 0", got)
	}
	if p.Alive() {
		t.Fatal("peer should be dead at 0 HP")
	}
	if got := p.ApplyDamage(SwordDamage); got != 0 {
		t.Fatalf("damaging a corpse changed health to %d", got)
	}
}

func TestRespawnRestoresHealth(t *testing.T) {
	p := newPeer("id", "Alice", "w1", "")
	for p.Health() > 0 {
		p.ApplyDamage(SwordDamage)
	}
	p.RespawnAt(0.5, 40, 0.5)
	if got := p.Health(); got != MaxHealth {
		t.Fatalf("after respawn Health = %d, want %d", got, MaxHealth)
	}
	pose := p.Pose()
	if pose.X != 0.5 || pose.Z != 0.5 {
		t.Fatalf("respawn pose = (%v,%v,%v)", pose.X, pose.Y, pose.Z)
	}
}

func TestAttackCooldown(t *testing.T) {
	p := newPeer("id", "Alice", "w1", "")
	now := time.Now().UTC()
	if !p.TryBeginAttack(now) {
		t.Fatal("first attack should succeed")
	}
	if p.TryBeginAttack(now.Add(100 * time.Millisecond)) {
		t.Fatal("attack within cooldown should fail")
	}
	if !p.TryBeginAttack(now.Add(time.Duration(SwordAttackCooldown) * time.Millisecond)) {
		t.Fatal("attack after cooldown should succeed")
	}
}

func TestHandleAttackInRange(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	b := newPeer("b", "Bob", "w1", "")
	if err := w.addPeer(a); err != nil {
		t.Fatal(err)
	}
	if err := w.addPeer(b); err != nil {
		t.Fatal(err)
	}
	a.SetPose(0, 80, 0, 0, 0, false)
	b.SetPose(2, 80, 0, 0, 0, false) // within SwordStrikeRange

	events := b.Subscribe(8)
	defer b.Unsubscribe(events)

	w.HandleClient(a, Message{Type: MsgAttack, Nick: "Bob"})

	select {
	case msg := <-events:
		if msg.Type != MsgPeerHealth || msg.Nick != "Bob" || msg.Health != MaxHealth-SwordDamage {
			t.Fatalf("unexpected health msg: %+v", msg)
		}
		if msg.KnockX == 0 && msg.KnockZ == 0 {
			t.Fatal("expected knockback on hit")
		}
		// Bob is east of Alice → knockback should push further +X.
		if msg.KnockX <= 0 {
			t.Fatalf("knock_x = %v, want positive (away from attacker)", msg.KnockX)
		}
	case <-time.After(time.Second):
		t.Fatal("expected peer_health broadcast")
	}
}

func TestHandleAttackOutOfRange(t *testing.T) {
	w := newWorld("w1", nil)
	a := newPeer("a", "Alice", "w1", "")
	b := newPeer("b", "Bob", "w1", "")
	_ = w.addPeer(a)
	_ = w.addPeer(b)
	a.SetPose(0, 80, 0, 0, 0, false)
	b.SetPose(SwordStrikeRange+1, 80, 0, 0, 0, false)

	w.HandleClient(a, Message{Type: MsgAttack, Nick: "Bob"})
	if b.Health() != MaxHealth {
		t.Fatalf("out-of-range attack changed health to %d", b.Health())
	}
}

func TestHandleRespawn(t *testing.T) {
	w := newWorld("w1", nil)
	b := newPeer("b", "Bob", "w1", "")
	_ = w.addPeer(b)
	for b.Health() > 0 {
		b.ApplyDamage(SwordDamage)
	}
	events := b.Subscribe(8)
	defer b.Unsubscribe(events)

	w.HandleClient(b, Message{Type: MsgRespawn})
	if b.Health() != MaxHealth {
		t.Fatalf("respawn Health = %d, want %d", b.Health(), MaxHealth)
	}
	select {
	case msg := <-events:
		if msg.Type != MsgPeerHealth || msg.Health != MaxHealth || msg.Dead {
			t.Fatalf("unexpected respawn health msg: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("expected peer_health on respawn")
	}
}

func TestPoseDistance(t *testing.T) {
	dx, dy, dz := 3.0, 0.0, 4.0
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if dist != 5.0 {
		t.Fatalf("dist = %v", dist)
	}
}

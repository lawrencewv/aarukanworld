package world

import "testing"

func TestWorldIDFromRoomStable(t *testing.T) {
	a := WorldIDFromRoom("aarukan")
	b := WorldIDFromRoom("#aarukan")
	c := WorldIDFromRoom("#Aarukan")
	if a == "" || a != b || a != c {
		t.Fatalf("expected stable id, got %q %q %q", a, b, c)
	}
	if len(a) != 32 {
		t.Fatalf("want 32 hex chars, got %d", len(a))
	}
	other := WorldIDFromRoom("#other")
	if other == a {
		t.Fatal("different rooms must not share world_id")
	}
}

func TestChunkCoordNegative(t *testing.T) {
	c := chunkCoordFromBlock(-1, -1)
	if c.X != -1 || c.Z != -1 {
		t.Fatalf("got %+v, want X=-1 Z=-1", c)
	}
	c = chunkCoordFromBlock(16, 15)
	if c.X != 1 || c.Z != 0 {
		t.Fatalf("got %+v, want X=1 Z=0", c)
	}
}

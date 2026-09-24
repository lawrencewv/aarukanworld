package world

import (
	"aarukanworld/internal/persist"
)

// Wire message types for the game WebSocket (JSON).
const (
	MsgHello        = "hello"
	MsgWelcome      = "welcome"
	MsgPing         = "ping"
	MsgPong         = "pong"
	MsgPose         = "pose"
	MsgBlockPlace   = "block_place"
	MsgBlockBreak   = "block_break"
	MsgChunkRequest = "chunk_request"
	MsgChunkData    = "chunk_data"
	MsgPeerJoin     = "peer_join"
	MsgPeerLeave    = "peer_leave"
	MsgPeerPose     = "peer_pose"
	MsgAttack       = "attack"
	MsgArrowHit     = "arrow_hit"
	MsgArrowShot    = "arrow_shot"
	MsgPeerArrow    = "peer_arrow"
	MsgPeerHealth   = "peer_health"
	MsgRespawn      = "respawn"
	MsgSystem       = "system"
	MsgError        = "error"
)

// Combat defaults for sword melee / bow.
const (
	MaxHealth           = 100
	SwordDamage         = 10
	SwordStrikeRange    = 3.5 // metres; slightly above client reach for lag
	SwordAttackCooldown  = 400 // milliseconds between validated thrusts
	SwordKnockbackSpeed = 6.5 // horizontal impulse applied to the victim
	ArrowDamage         = 10
	ArrowMaxRange       = 52.0 // metres; matches client arrow flight budget
	ArrowAttackCooldown  = 450 // milliseconds between validated arrow hits
)

// Message is a JSON-friendly game frame.
type Message struct {
	Type    string   `json:"type"`
	Nick    string   `json:"nick,omitempty"`
	WorldID string   `json:"world_id,omitempty"`
	Text    string   `json:"text,omitempty"`
	X       float64  `json:"x"`
	Y       float64  `json:"y"`
	Z       float64  `json:"z"`
	Yaw      float64 `json:"yaw"`
	Pitch    float64 `json:"pitch"`
	Breaking bool    `json:"breaking,omitempty"`
	Held     int     `json:"held"`
	Drawing  bool    `json:"drawing,omitempty"`
	Swinging bool    `json:"swinging,omitempty"`
	Block    uint16  `json:"block"`
	CX      int32    `json:"cx"`
	CZ      int32    `json:"cz"`
	Blocks  []uint16 `json:"blocks,omitempty"`
	Health  int      `json:"health"`
	KnockX  float64  `json:"knock_x,omitempty"`
	KnockZ  float64  `json:"knock_z,omitempty"`
	Dead    bool     `json:"dead,omitempty"`
}

func chunkCoordFromBlock(bx, bz int32) persist.ChunkCoord {
	return persist.ChunkCoord{
		X: divFloor(bx, persist.ChunkSize),
		Z: divFloor(bz, persist.ChunkSize),
	}
}

func divFloor(a int32, b int) int32 {
	bi := int32(b)
	if a >= 0 {
		return a / bi
	}
	return -((-a + bi - 1) / bi)
}

func localBlock(bx, by, bz int32) (x, y, z int, ok bool) {
	if by < 0 || by >= persist.ChunkHeight {
		return 0, 0, 0, false
	}
	lx := int(bx % int32(persist.ChunkSize))
	lz := int(bz % int32(persist.ChunkSize))
	if lx < 0 {
		lx += persist.ChunkSize
	}
	if lz < 0 {
		lz += persist.ChunkSize
	}
	return lx, int(by), lz, true
}

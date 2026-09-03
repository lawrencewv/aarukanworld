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
	MsgSystem       = "system"
	MsgError        = "error"
)

// Message is a JSON-friendly game frame.
type Message struct {
	Type    string  `json:"type"`
	Nick    string  `json:"nick,omitempty"`
	WorldID string  `json:"world_id,omitempty"`
	Text    string  `json:"text,omitempty"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	Z       float64 `json:"z,omitempty"`
	Yaw     float64 `json:"yaw,omitempty"`
	Pitch   float64 `json:"pitch,omitempty"`
	Block   uint16  `json:"block,omitempty"`
	CX      int32   `json:"cx,omitempty"`
	CZ      int32   `json:"cz,omitempty"`
	// Blocks is a dense row-major list for chunk_data (optional; large).
	Blocks []uint16 `json:"blocks,omitempty"`
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

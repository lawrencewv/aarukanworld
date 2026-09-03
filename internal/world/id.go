package world

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// WorldIDFromRoom derives the stable world_id for a chat room name.
// world_id = hex(sha256(canonicalize(room))) truncated to 32 hex chars.
func WorldIDFromRoom(room string) string {
	canon := CanonicalizeRoom(room)
	if canon == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.ToLower(canon)))
	return hex.EncodeToString(sum[:])[:32]
}

// CanonicalizeRoom matches aarukaninternalchat: trim, ensure #/& prefix.
func CanonicalizeRoom(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	if !strings.HasPrefix(name, "#") && !strings.HasPrefix(name, "&") {
		name = "#" + name
	}
	return name
}

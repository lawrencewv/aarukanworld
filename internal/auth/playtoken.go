package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PlayClaims is the payload signed by aarukaninternalchat and verified here.
// aarukanworld never sees room keys — only this opaque proof.
type PlayClaims struct {
	Nick    string `json:"nick"`
	WorldID string `json:"world_id"`
	Exp     int64  `json:"exp"` // unix seconds
	Session string `json:"session,omitempty"`
	Gen     int    `json:"gen,omitempty"`
}

var (
	ErrMissingToken  = errors.New("missing play token")
	ErrInvalidToken  = errors.New("invalid play token")
	ErrExpiredToken  = errors.New("play token expired")
	ErrMalformedToken = errors.New("malformed play token")
)

var playSecret []byte

// Configure sets the HMAC secret shared with aarukaninternalchat. Call once at startup.
func Configure(secret string) {
	playSecret = []byte(strings.TrimSpace(secret))
}

// ExtractPlayToken reads the token from Authorization: Bearer, X-Play-Token, or ?token=.
func ExtractPlayToken(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("Authorization")); h != "" {
		const prefix = "Bearer "
		if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
			return strings.TrimSpace(h[len(prefix):])
		}
	}
	if key := strings.TrimSpace(r.Header.Get("X-Play-Token")); key != "" {
		return key
	}
	if key := strings.TrimSpace(r.URL.Query().Get("token")); key != "" {
		return key
	}
	return ""
}

// Sign creates a compact play token for the given claims (used by tests and by chat later).
func Sign(claims PlayClaims) (string, error) {
	if len(playSecret) == 0 {
		return "", ErrInvalidToken
	}
	if strings.TrimSpace(claims.Nick) == "" || strings.TrimSpace(claims.WorldID) == "" {
		return "", fmt.Errorf("nick and world_id are required")
	}
	if claims.Exp == 0 {
		claims.Exp = time.Now().UTC().Add(2 * time.Minute).Unix()
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, playSecret)
	_, _ = mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

// Verify checks signature and expiry, returning the claims.
func Verify(token string) (PlayClaims, error) {
	var zero PlayClaims
	token = strings.TrimSpace(token)
	if token == "" {
		return zero, ErrMissingToken
	}
	if len(playSecret) == 0 {
		return zero, ErrInvalidToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return zero, ErrMalformedToken
	}

	mac := hmac.New(sha256.New, playSecret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, got) {
		return zero, ErrInvalidToken
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, ErrMalformedToken
	}
	var claims PlayClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return zero, ErrMalformedToken
	}
	claims.Nick = strings.TrimSpace(claims.Nick)
	claims.WorldID = strings.TrimSpace(claims.WorldID)
	if claims.Nick == "" || claims.WorldID == "" {
		return zero, ErrInvalidToken
	}
	if claims.Exp > 0 && time.Now().UTC().Unix() > claims.Exp {
		return zero, ErrExpiredToken
	}
	return claims, nil
}

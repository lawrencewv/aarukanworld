package auth

import (
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	Configure("test-secret")
	tok, err := Sign(PlayClaims{
		Nick:    "builder",
		WorldID: "abc123",
		Exp:     time.Now().UTC().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Nick != "builder" || claims.WorldID != "abc123" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	Configure("test-secret")
	tok, err := Sign(PlayClaims{Nick: "a", WorldID: "w", Exp: time.Now().Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	Configure("other-secret")
	if _, err := Verify(tok); err != ErrInvalidToken {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	Configure("test-secret")
	tok, err := Sign(PlayClaims{
		Nick:    "a",
		WorldID: "w",
		Exp:     time.Now().UTC().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(tok); err != ErrExpiredToken {
		t.Fatalf("got %v, want ErrExpiredToken", err)
	}
}

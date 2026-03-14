package token_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mayloo89/circl/backend/internal/token"
)

const testSecret = "supersecretfortesting-mustbe32chars!!"

func TestGenerate_and_Validate(t *testing.T) {
	tok, err := token.Generate("user-123", testSecret, time.Hour)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := token.Validate(tok, testSecret)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-123")
	}
}

func TestValidate_WrongSecret(t *testing.T) {
	tok, _ := token.Generate("user-123", testSecret, time.Hour)

	_, err := token.Validate(tok, "wrong-secret")
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	tok, _ := token.Generate("user-123", testSecret, -time.Second)

	_, err := token.Validate(tok, testSecret)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidate_MalformedToken(t *testing.T) {
	_, err := token.Validate("not.a.token", testSecret)
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

func TestValidate_UnexpectedSigningMethod(t *testing.T) {
	// Build a "none"-algorithm JWT to trigger the unexpected signing method guard.
	raw := jwt.New(jwt.SigningMethodNone)
	raw.Claims = jwt.RegisteredClaims{Subject: "test"}
	noneToken, err := raw.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("create none-token: %v", err)
	}

	_, err = token.Validate(noneToken, testSecret)
	if err == nil {
		t.Fatal("expected error for non-HMAC token, got nil")
	}
}

func TestGenerate_EmptySecret(t *testing.T) {
	// Empty secret still generates a token (JWT allows it),
	// but Validate with the correct empty secret should succeed.
	tok, err := token.Generate("user-123", "", time.Hour)
	if err != nil {
		t.Fatalf("Generate() with empty secret error: %v", err)
	}
	_, err = token.Validate(tok, "")
	if err != nil {
		t.Fatalf("Validate() with matching empty secret error: %v", err)
	}
}

package auth_test

import (
	"errors"
	"testing"
	"time"
	"Frank2006x/Pipe/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret string = "absadiafslihalfhaliifhaihflashflaihfashif"


func TestJwtMaker_GenerateAndValidateJWT(t *testing.T) {
	start := time.Now()
	defer func() {
		t.Logf("TestJwtMaker_GenerateAndValidateJWT completed in %v", time.Since(start))
	}()

	maker := auth.NewJwtMaker(testSecret)
	userID := int64(1337)

	genStart := time.Now()
	tokenString, err := maker.GenerateJWT(userID)
	if err != nil {
		t.Fatalf("unexpected error generating JWT: %v", err)
	}
	if tokenString == "" {
		t.Fatalf("expected non-empty token string")
	}
	t.Logf("  - GenerateJWT took %v", time.Since(genStart))

	valStart := time.Now()
	claims, err := maker.ValidateJWT(tokenString)
	if err != nil {
		t.Fatalf("unexpected error validating valid JWT: %v", err)
	}
	t.Logf("  - ValidateJWT took %v", time.Since(valStart))

	if claims.UserID != userID {
		t.Errorf("got UserID %d, want %d", claims.UserID, userID)
	}
}

func TestJwtMaker_ValidateJWT_Errors(t *testing.T) {
	start := time.Now()
	defer func() {
		t.Logf("TestJwtMaker_ValidateJWT_Errors completed in %v", time.Since(start))
	}()

	validMaker := auth.NewJwtMaker(testSecret)

	wrongMaker := auth.NewJwtMaker("different_secret_key_123456789012")
	wrongToken, err := wrongMaker.GenerateJWT(42)
	if err != nil {
		t.Fatalf("failed to generate wrong token: %v", err)
	}

	expiredClaims := &auth.Claims{
		UserID: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, err := expiredToken.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to create expired token string: %v", err)
	}

	invalidMethodToken := jwt.NewWithClaims(jwt.SigningMethodRS256, &auth.Claims{UserID: 42})
	invalidMethodString := invalidMethodToken.Raw

	tests := []struct {
		name        string
		tokenString string
		wantErr     error
	}{
		{
			name:        "malformed token string",
			tokenString: "invalid.jwt.token",
			wantErr:     jwt.ErrTokenMalformed,
		},
		{
			name:        "signed with wrong secret key",
			tokenString: wrongToken,
			wantErr:     jwt.ErrTokenSignatureInvalid,
		},
		{
			name:        "expired token",
			tokenString: expiredTokenString,
			wantErr:     jwt.ErrTokenExpired,
		},
		{
			name:        "unexpected signing method",
			tokenString: invalidMethodString,
			wantErr:     jwt.ErrTokenMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caseStart := time.Now()
			_, err := validMaker.ValidateJWT(tt.tokenString)
			t.Logf("  - Subtest %q took %v", tt.name, time.Since(caseStart))

			if err == nil {
				t.Fatalf("expected error for case %q, got nil", tt.name)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want error matching %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkGenerateJWT(b *testing.B) {
	maker := auth.NewJwtMaker(testSecret)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = maker.GenerateJWT(1337)
	}
}

func BenchmarkValidateJWT(b *testing.B) {
	maker := auth.NewJwtMaker(testSecret)
	tokenString, _ := maker.GenerateJWT(1337)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = maker.ValidateJWT(tokenString)
	}
}

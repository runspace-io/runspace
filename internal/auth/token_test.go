package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSigner(t *testing.T, now func() time.Time) *Signer {
	t.Helper()
	signer, err := NewSigner("a-shared-gateway-secret", now)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func fixedNow() time.Time { return time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC) }

func TestIssuedTokenVerifies(t *testing.T) {
	signer := testSigner(t, fixedNow)
	token, err := signer.Issue("alice", "runspace-web", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "alice" || claims.Issuer != "runspace-web" {
		t.Fatalf("claims=%+v", claims)
	}
}

// Refusing to start without a secret is the point: a default would silently
// restore the "trust whatever the caller claims" behaviour.
func TestSignerRequiresASecret(t *testing.T) {
	if _, err := NewSigner("   ", nil); !errors.Is(err, ErrNoSecret) {
		t.Fatalf("err=%v", err)
	}
}

func TestTokenFromAnotherSecretIsRejected(t *testing.T) {
	issuer := testSigner(t, fixedNow)
	token, err := issuer.Issue("alice", "runspace-web", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewSigner("a-different-secret", fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Verify(token); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("err=%v", err)
	}
}

// The classic JWT forgery: swap the algorithm to "none" and drop the signature.
func TestAlgNoneIsRejected(t *testing.T) {
	signer := testSigner(t, fixedNow)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"sub":"admin","exp":9999999999}`),
	)
	for _, forged := range []string{header + "." + payload + ".", header + "." + payload + ".x"} {
		if _, err := signer.Verify(forged); err == nil {
			t.Fatalf("accepted an unsigned token: %s", forged)
		}
	}
}

// Tampering with the subject must invalidate the signature, or anyone holding
// their own valid token could rewrite it into somebody else's.
func TestSubjectCannotBeTamperedWith(t *testing.T) {
	signer := testSigner(t, fixedNow)
	token, err := signer.Issue("alice", "runspace-web", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString(
		[]byte(`{"sub":"admin","exp":9999999999}`),
	)
	if _, err := signer.Verify(strings.Join(parts, ".")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("err=%v", err)
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	clock := fixedNow()
	signer := testSigner(t, func() time.Time { return clock })
	token, err := signer.Issue("alice", "runspace-web", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * time.Minute)
	if _, err := signer.Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("err=%v", err)
	}
}

func TestMalformedTokensAreRejected(t *testing.T) {
	signer := testSigner(t, fixedNow)
	for _, bad := range []string{"", "abc", "a.b", "a.b.c.d", "!!.??.$$"} {
		if _, err := signer.Verify(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestMiddlewareRejectsUnauthenticatedRequests(t *testing.T) {
	signer := testSigner(t, fixedNow)
	handler := Middleware(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(UserID(r)))
	}))
	request := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	// The header that used to be identity must now count for nothing.
	request.Header.Set("X-User-ID", "admin")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMiddlewareAcceptsBearerAndQueryTokens(t *testing.T) {
	signer := testSigner(t, fixedNow)
	token, err := signer.Issue("alice", "runspace-web", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	handler := Middleware(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(UserID(r)))
	}))
	header := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	header.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, header)
	if response.Code != http.StatusOK || response.Body.String() != "alice" {
		t.Fatalf("bearer status=%d body=%s", response.Code, response.Body.String())
	}
	// WebSocket clients cannot set headers, so the query form must work too.
	query := httptest.NewRequest(http.MethodGet, "/realtime?access_token="+token, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, query)
	if response.Code != http.StatusOK || response.Body.String() != "alice" {
		t.Fatalf("query status=%d body=%s", response.Code, response.Body.String())
	}
}

// A header-supplied identity must never leak through as a fallback.
func TestUserIDIgnoresTheLegacyHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	request.Header.Set("X-User-ID", "admin")
	if id := UserID(request); id != "" {
		t.Fatalf("UserID honoured a client header: %q", id)
	}
}

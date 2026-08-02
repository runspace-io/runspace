// Package auth establishes who is calling the gateway.
//
// Identity used to arrive as a plain X-User-ID header that any client could
// set, so every caller could act as anyone. Tokens are signed instead, and the
// verified subject is the only identity handlers may read.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNoSecret       = errors.New("gateway auth secret is not configured")
	ErrMalformed      = errors.New("token is malformed")
	ErrBadSignature   = errors.New("token signature does not verify")
	ErrExpired        = errors.New("token has expired")
	ErrMissingSubject = errors.New("token has no subject")
)

// Claims is the minimum a caller must prove: who they are, and until when.
type Claims struct {
	Subject   string `json:"sub"`
	Issuer    string `json:"iss,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

// Signer issues and verifies compact HS256 tokens. HMAC keeps the gateway,
// the web app, and a host agent on one shared secret without adding a
// dependency or an asymmetric key to distribute.
type Signer struct {
	secret []byte
	now    func() time.Time
}

func NewSigner(secret string, now func() time.Time) (*Signer, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrNoSecret
	}
	if now == nil {
		now = time.Now
	}
	return &Signer{secret: []byte(secret), now: now}, nil
}

func (s *Signer) Issue(subject, issuer string, ttl time.Duration) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", ErrMissingSubject
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	issued := s.now().UTC()
	claims, err := json.Marshal(Claims{
		Subject: subject, Issuer: issuer,
		IssuedAt: issued.Unix(), ExpiresAt: issued.Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	// The header is fixed, so a token can never negotiate its own algorithm.
	body := encodeSegment([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + encodeSegment(claims)
	return body + "." + encodeSegment(s.sign(body)), nil
}

func (s *Signer) Verify(token string) (Claims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return Claims{}, ErrMalformed
	}
	header, err := decodeSegment(parts[0])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var algorithm struct {
		Alg string `json:"alg"`
	}
	// Reject anything but HS256 before touching the signature: accepting "none",
	// or an algorithm the token chooses, is the classic JWT forgery.
	if json.Unmarshal(header, &algorithm) != nil || algorithm.Alg != "HS256" {
		return Claims{}, ErrMalformed
	}
	signature, err := decodeSegment(parts[2])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	if !hmac.Equal(signature, s.sign(parts[0]+"."+parts[1])) {
		return Claims{}, ErrBadSignature
	}
	payload, err := decodeSegment(parts[1])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil {
		return Claims{}, ErrMalformed
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return Claims{}, ErrMissingSubject
	}
	if claims.ExpiresAt <= 0 || s.now().UTC().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrExpired
	}
	return claims, nil
}

func (s *Signer) sign(body string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(body))
	return mac.Sum(nil)
}

func encodeSegment(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeSegment(segment string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(segment)
}

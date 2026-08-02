package main

import (
	"crypto/sha256"
	"os"
	"strings"
)

// gatewayAuthSecret is shared with the web app, which mints the tokens users
// present. Without it the gateway refuses to start rather than fall back to
// trusting whatever identity a caller claims.
func gatewayAuthSecret() string {
	if secret := strings.TrimSpace(os.Getenv("GATEWAY_AUTH_SECRET")); secret != "" {
		return secret
	}
	return strings.TrimSpace(os.Getenv("NEXTAUTH_SECRET"))
}

func channelSecretKey() []byte {
	value := os.Getenv("CHANNEL_SECRET_KEY")
	if value == "" {
		value = "runspace-local-channel-secret"
	}
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}

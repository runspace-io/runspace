package hostagent

import (
	"os"
	"strings"
	"time"

	"github.com/runspace/runspace/internal/auth"
)

// gatewayTokenTTL is short because the host agent can always mint another; the
// token only ever travels between this process and its own gateway.
const gatewayTokenTTL = 5 * time.Minute

func newGatewaySigner() *auth.Signer {
	secret := strings.TrimSpace(os.Getenv("GATEWAY_AUTH_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("NEXTAUTH_SECRET"))
	}
	signer, err := auth.NewSigner(secret, time.Now)
	if err != nil {
		return nil
	}
	return signer
}

// gatewayToken signs the owner's identity for a push to the gateway.
//
// The host agent runs on the owner's machine and is trusted to speak for the
// local user, but the gateway cannot verify that from a header — so it shares
// the same secret the web app uses and signs the claim instead.
func (s *Server) gatewayToken(userID string) (string, error) {
	if s.gatewaySigner == nil {
		return "", auth.ErrNoSecret
	}
	return s.gatewaySigner.Issue(userID, "runspace-host-agent", gatewayTokenTTL)
}

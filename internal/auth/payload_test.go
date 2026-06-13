package auth

import (
	"testing"
)

func TestOAuthCredentialsFromPayloadUsesSameSourcePair(t *testing.T) {
	payload := &authFilePayload{}
	payload.OAuth2ClientSettings.ClientID = "A"
	payload.Installed.ClientID = "B"
	payload.Installed.ClientSecret = "C"
	clientID, clientSecret := oauthCredentialsFromPayload(payload)
	if clientID != "B" || clientSecret != "C" {
		t.Fatalf("expected installed pair (B,C), got (%s,%s)", clientID, clientSecret)
	}
}

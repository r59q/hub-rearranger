package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func clear(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ORIGIN", "GITHUB_APP_ID", "GITHUB_APP_CLIENT_ID", "GITHUB_APP_CLIENT_SECRET", "IDENTITY_ENCRYPTION_KEY"} {
		t.Setenv(key, "")
	}
}

func TestUnconfiguredIdentityIsExplicitlyDisabled(t *testing.T) {
	clear(t)
	t.Setenv("GITHUB_TOKEN", "synthetic-discovery-token")

	value, err := Load()

	if err != nil || value.Enabled {
		t.Fatal("discovery token enabled user identity")
	}
}

func TestConfiguredIdentityRequiresCompleteSecretsAndSafeOrigin(t *testing.T) {
	for _, variant := range []string{"https", "local-http", "external-http", "partial", "invalid-key", "path", "userinfo"} {
		t.Run(variant, func(t *testing.T) {
			clear(t)
			t.Setenv("GITHUB_APP_ID", "9")
			t.Setenv("GITHUB_APP_CLIENT_ID", "Iv1.synthetic")
			t.Setenv("GITHUB_APP_CLIENT_SECRET", "synthetic-secret")
			t.Setenv("IDENTITY_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
			switch variant {
			case "https":
				t.Setenv("APP_ORIGIN", "https://hub.example.com")
			case "local-http":
				t.Setenv("APP_ORIGIN", "http://127.0.0.1:3000")
			case "external-http":
				t.Setenv("APP_ORIGIN", "http://hub.example.com")
			case "partial":
				t.Setenv("GITHUB_APP_CLIENT_SECRET", "")
			case "invalid-key":
				t.Setenv("IDENTITY_ENCRYPTION_KEY", "synthetic-invalid-key")
			case "path":
				t.Setenv("APP_ORIGIN", "https://hub.example.com/extra")
			case "userinfo":
				t.Setenv("APP_ORIGIN", "https://private-sentinel@hub.example.com")
			}

			value, err := Load()

			valid := variant == "https" || variant == "local-http"
			if valid && (err != nil || !value.Enabled || value.Secure != (variant == "https")) {
				t.Fatal("valid configuration failed")
			}
			if !valid && (err == nil || strings.Contains(err.Error(), "private-sentinel") || strings.Contains(err.Error(), "synthetic-secret")) {
				t.Fatal("invalid configuration accepted or exposed")
			}
		})
	}
}

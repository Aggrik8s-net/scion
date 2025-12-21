package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverAuth(t *testing.T) {
	// Setup a temporary home directory
	tmpHome, err := os.MkdirTemp("", "gswarm-home-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	// Mock HOME environment variable
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	geminiDir := filepath.Join(tmpHome, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Test OAuth discovery via host settings
	settingsPath := filepath.Join(geminiDir, "settings.json")
	settingsData := `{
		"security": {
			"auth": {
				"selectedType": "oauth-personal"
			}
		}
	}`
	if err := os.WriteFile(settingsPath, []byte(settingsData), 0644); err != nil {
		t.Fatal(err)
	}

	oauthCredsPath := filepath.Join(geminiDir, "oauth_creds.json")
	if err := os.WriteFile(oauthCredsPath, []byte(`{"dummy":"creds"}`), 0644); err != nil {
		t.Fatal(err)
	}

	auth := DiscoverAuth(nil)
	if auth.OAuthCreds != oauthCredsPath {
		t.Errorf("expected OAuthCreds to be %s, got %s", oauthCredsPath, auth.OAuthCreds)
	}

	// 2. Test OAuth discovery via agent settings (overriding host)
	os.WriteFile(settingsPath, []byte(`{"security":{"auth":{"selectedType":"gemini-api-key"}}}`), 0644)
	agentSettings := &GeminiSettings{}
	agentSettings.Security.Auth.SelectedType = "oauth-personal"

	auth = DiscoverAuth(agentSettings)
	if auth.OAuthCreds != oauthCredsPath {
		t.Errorf("expected OAuthCreds to be %s when requested by agent, got %s", oauthCredsPath, auth.OAuthCreds)
	}

	// 3. Test API Key fallback from host settings
	os.Remove(settingsPath)
	settingsData = `{
		"apiKey": "test-api-key"
	}`
	if err := os.WriteFile(settingsPath, []byte(settingsData), 0644); err != nil {
		t.Fatal(err)
	}

	// Clear env vars that might interfere
	origApiKey := os.Getenv("GEMINI_API_KEY")
	origGoogleApiKey := os.Getenv("GOOGLE_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")
	defer func() {
		os.Setenv("GEMINI_API_KEY", origApiKey)
		os.Setenv("GOOGLE_API_KEY", origGoogleApiKey)
	}()

	auth = DiscoverAuth(nil)
	if auth.GeminiAPIKey != "test-api-key" {
		t.Errorf("expected GeminiAPIKey to be 'test-api-key', got '%s'", auth.GeminiAPIKey)
	}
}


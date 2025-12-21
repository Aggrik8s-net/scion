package config

import (
	"os"
	"path/filepath"
)

type AuthConfig struct {
	GeminiAPIKey         string
	GoogleAPIKey         string
	VertexAPIKey         string
	GoogleAppCredentials string
	GoogleCloudProject   string
	OAuthCreds           string
}

func DiscoverAuth(agentSettings *GeminiSettings) AuthConfig {
	auth := AuthConfig{
		GeminiAPIKey:         os.Getenv("GEMINI_API_KEY"),
		GoogleAPIKey:         os.Getenv("GOOGLE_API_KEY"),
		VertexAPIKey:         os.Getenv("VERTEX_API_KEY"),
		GoogleAppCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		GoogleCloudProject:   os.Getenv("GOOGLE_CLOUD_PROJECT"),
	}

	if auth.GoogleCloudProject == "" {
		auth.GoogleCloudProject = os.Getenv("GCP_PROJECT")
	}

	home, _ := os.UserHomeDir()

	// 1. Check agent settings (from template) first to see if they specify a type
	selectedType := ""
	if agentSettings != nil {
		selectedType = agentSettings.Security.Auth.SelectedType
	}

	// 2. Load host settings if we don't have a type yet, or to find fallback API key
	hostSettings, _ := GetGeminiSettings()

	if selectedType == "" && hostSettings != nil {
		selectedType = hostSettings.Security.Auth.SelectedType
	}

	// 3. Fallback to settings.json for Gemini API Key if none found in env
	if auth.GeminiAPIKey == "" && auth.GoogleAPIKey == "" {
		// Prefer host settings for API key propagation unless agent settings has one
		if agentSettings != nil && agentSettings.ApiKey != "" {
			auth.GeminiAPIKey = agentSettings.ApiKey
		} else if hostSettings != nil && hostSettings.ApiKey != "" {
			auth.GeminiAPIKey = hostSettings.ApiKey
		}
	}

	// 4. Handle OAuth if selected
	if selectedType == "oauth-personal" {
		oauthPath := filepath.Join(home, ".gemini", "oauth_creds.json")
		if _, err := os.Stat(oauthPath); err == nil {
			auth.OAuthCreds = oauthPath
		}
	}

	return auth
}

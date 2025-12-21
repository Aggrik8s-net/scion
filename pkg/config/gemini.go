package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type GeminiSettings struct {
	ApiKey   string `json:"apiKey"`
	Security struct {
		Auth struct {
			SelectedType string `json:"selectedType"`
		} `json:"auth"`
	} `json:"security"`
	Tools struct {
		Sandbox interface{} `json:"sandbox"`
	} `json:"tools"`
}

func LoadGeminiSettings(path string) (*GeminiSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings GeminiSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func GetGeminiSettings() (*GeminiSettings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".gemini", "settings.json")
	return LoadGeminiSettings(path)
}

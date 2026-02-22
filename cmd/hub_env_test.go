// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envTestState captures and restores package-level vars for test isolation.
type envTestState struct {
	home           string
	grovePath      string
	envGroveScope  string
	envBrokerScope string
	envOutputJSON  bool
}

func saveEnvTestState() envTestState {
	return envTestState{
		home:           os.Getenv("HOME"),
		grovePath:      grovePath,
		envGroveScope:  envGroveScope,
		envBrokerScope: envBrokerScope,
		envOutputJSON:  envOutputJSON,
	}
}

func (s envTestState) restore() {
	os.Setenv("HOME", s.home)
	grovePath = s.grovePath
	envGroveScope = s.envGroveScope
	envBrokerScope = s.envBrokerScope
	envOutputJSON = s.envOutputJSON
}

// setupEnvGrove creates a grove directory with settings pointing to the given hub endpoint.
func setupEnvGrove(t *testing.T, home, endpoint string) string {
	t.Helper()
	groveDir := filepath.Join(home, "project", ".scion")
	require.NoError(t, os.MkdirAll(groveDir, 0755))

	settings := map[string]interface{}{
		"grove_id": "test-grove",
		"hub": map[string]interface{}{
			"enabled":  true,
			"endpoint": endpoint,
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(groveDir, "settings.json"), data, 0644))

	return groveDir
}

// newEnvListMockServer creates a mock Hub server that handles env list requests.
func newEnvListMockServer(t *testing.T, envVars []map[string]interface{}) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/healthz" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})

		case r.URL.Path == "/api/v1/env" && r.Method == http.MethodGet:
			scope := r.URL.Query().Get("scope")
			if scope == "" {
				scope = "user"
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"envVars": envVars,
				"scope":   scope,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return server
}

func TestHubEnvListCmd_Exists(t *testing.T) {
	// Verify the list subcommand is registered under hub env.
	found := false
	for _, sub := range hubEnvCmd.Commands() {
		if sub.Use == "list" {
			found = true
			break
		}
	}
	assert.True(t, found, "hubEnvCmd should have a 'list' subcommand")
}

func TestHubEnvListCmd_Flags(t *testing.T) {
	// Verify required flags are present on the list command.
	assert.NotNil(t, hubEnvListCmd.Flags().Lookup("grove"), "list command should have --grove flag")
	assert.NotNil(t, hubEnvListCmd.Flags().Lookup("broker"), "list command should have --broker flag")
	assert.NotNil(t, hubEnvListCmd.Flags().Lookup("json"), "list command should have --json flag")
}

func TestHubEnvListCmd_NoArgs(t *testing.T) {
	// Verify the command accepts no arguments.
	assert.Equal(t, "list", hubEnvListCmd.Use)
}

func TestRunEnvList_WithResults(t *testing.T) {
	orig := saveEnvTestState()
	defer orig.restore()

	envVars := []map[string]interface{}{
		{"key": "API_URL", "value": "https://api.example.com", "scope": "user", "injectionMode": "always"},
		{"key": "LOG_LEVEL", "value": "debug", "scope": "user", "injectionMode": "as_needed"},
	}

	server := newEnvListMockServer(t, envVars)
	defer server.Close()

	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Setenv("SCION_HUB_ENDPOINT", server.URL)

	groveDir := setupEnvGrove(t, tmpHome, server.URL)
	grovePath = groveDir

	envOutputJSON = false
	envGroveScope = ""
	envBrokerScope = ""

	err := runEnvList(hubEnvListCmd, nil)
	assert.NoError(t, err)
}

func TestRunEnvList_Empty(t *testing.T) {
	orig := saveEnvTestState()
	defer orig.restore()

	server := newEnvListMockServer(t, []map[string]interface{}{})
	defer server.Close()

	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Setenv("SCION_HUB_ENDPOINT", server.URL)

	groveDir := setupEnvGrove(t, tmpHome, server.URL)
	grovePath = groveDir

	envOutputJSON = false
	envGroveScope = ""
	envBrokerScope = ""

	err := runEnvList(hubEnvListCmd, nil)
	assert.NoError(t, err)
}

func TestRunEnvList_JSON(t *testing.T) {
	orig := saveEnvTestState()
	defer orig.restore()

	envVars := []map[string]interface{}{
		{"key": "MY_VAR", "value": "hello", "scope": "user"},
	}

	server := newEnvListMockServer(t, envVars)
	defer server.Close()

	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Setenv("SCION_HUB_ENDPOINT", server.URL)

	groveDir := setupEnvGrove(t, tmpHome, server.URL)
	grovePath = groveDir

	envOutputJSON = true
	envGroveScope = ""
	envBrokerScope = ""

	err := runEnvList(hubEnvListCmd, nil)
	assert.NoError(t, err)
}

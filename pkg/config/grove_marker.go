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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GroveMarker represents the content of a .scion marker file.
// When .scion is a file (not a directory), it points to an external
// grove-config directory under ~/.scion/grove-configs/.
type GroveMarker struct {
	GroveID   string `yaml:"grove-id"`
	GroveName string `yaml:"grove-name"`
	GroveSlug string `yaml:"grove-slug"`
}

// ShortUUID returns a short form of the grove ID for use in directory names.
func (m GroveMarker) ShortUUID() string {
	id := strings.ReplaceAll(m.GroveID, "-", "")
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// DirName returns the directory name used under ~/.scion/grove-configs/.
func (m GroveMarker) DirName() string {
	return fmt.Sprintf("%s__%s", m.GroveSlug, m.ShortUUID())
}

// ExternalGrovePath returns the absolute path to the external grove config
// directory: ~/.scion/grove-configs/<grove-slug>__<short-uuid>/.scion/
func (m GroveMarker) ExternalGrovePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, GlobalDir, "grove-configs", m.DirName(), DotScion), nil
}

// ReadGroveMarker reads and parses a .scion marker file.
func ReadGroveMarker(path string) (*GroveMarker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var marker GroveMarker
	if err := yaml.Unmarshal(data, &marker); err != nil {
		return nil, fmt.Errorf("invalid grove marker at %s: %w", path, err)
	}
	if marker.GroveID == "" || marker.GroveSlug == "" {
		return nil, fmt.Errorf("invalid grove marker at %s: missing grove-id or grove-slug", path)
	}
	return &marker, nil
}

// WriteGroveMarker writes a GroveMarker to the given path as a YAML file.
func WriteGroveMarker(path string, marker *GroveMarker) error {
	data, err := yaml.Marshal(marker)
	if err != nil {
		return fmt.Errorf("failed to marshal grove marker: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ResolveGroveMarker reads a .scion marker file and returns the resolved
// external grove path. Returns an error if the marker is invalid or the
// external path cannot be computed.
func ResolveGroveMarker(markerPath string) (string, error) {
	marker, err := ReadGroveMarker(markerPath)
	if err != nil {
		return "", err
	}
	return marker.ExternalGrovePath()
}

// IsGroveMarkerFile returns true if the given path is a regular file
// (not a directory) that could be a grove marker. Does not validate content.
func IsGroveMarkerFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsOldStyleNonGitGrove returns true if the path is a .scion directory
// in a non-git project (not the global ~/.scion/). This indicates an
// old-format grove that needs to be re-initialized.
func IsOldStyleNonGitGrove(scionPath string) bool {
	info, err := os.Stat(scionPath)
	if err != nil || !info.IsDir() {
		return false
	}

	// Don't flag the global grove
	home, err := os.UserHomeDir()
	if err == nil {
		globalDir := filepath.Join(home, GlobalDir)
		if abs, err := filepath.Abs(scionPath); err == nil {
			evalAbs, _ := filepath.EvalSymlinks(abs)
			evalGlobal, _ := filepath.EvalSymlinks(globalDir)
			if evalAbs == evalGlobal {
				return false
			}
		}
	}

	// Check if the parent directory is a git repo
	parent := filepath.Dir(scionPath)
	gitDir := filepath.Join(parent, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return false // Git grove — not old-style (handled by Phase 3)
	}

	return true
}

// ExtractSlugFromExternalDir extracts the grove slug from an external
// grove-config directory name in the format "slug__shortuuid".
func ExtractSlugFromExternalDir(dirName string) string {
	if parts := strings.SplitN(dirName, "__", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

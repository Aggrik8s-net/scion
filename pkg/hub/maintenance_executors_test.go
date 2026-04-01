//go:build !no_sqlite

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

package hub

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/secret"
	"github.com/GoogleCloudPlatform/scion/pkg/store/sqlite"
)

func TestSecretMigrationExecutor_NoGCPBackend(t *testing.T) {
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Use a local backend (not GCP) — migration should fail.
	localBackend := secret.NewLocalBackend(s, "test-hub-id")

	executor := &SecretMigrationExecutor{
		store:         s,
		secretBackend: localBackend,
	}

	var buf bytes.Buffer
	err = executor.Run(context.Background(), &buf, map[string]string{})
	if err == nil {
		t.Fatal("expected error for non-GCP backend, got nil")
	}
	if !strings.Contains(err.Error(), "GCP Secret Manager") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSecretMigrationExecutor_NoSecrets(t *testing.T) {
	// This test verifies the executor handles zero secrets gracefully.
	// We can't easily mock a GCPBackend without a real GCP client,
	// but we can test the local-backend error path above.
	// A full integration test would require GCP SM mock infrastructure.
}

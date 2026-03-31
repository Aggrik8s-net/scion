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

//go:build !no_sqlite

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/api"
	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaintenanceOperationsSeeded(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	ops, err := s.ListMaintenanceOperations(ctx)
	require.NoError(t, err)
	require.Len(t, ops, 4, "expected 4 seeded operations (1 migration + 3 operations)")

	// Verify categories
	var migrations, operations int
	for _, op := range ops {
		switch op.Category {
		case store.MaintenanceCategoryMigration:
			migrations++
		case store.MaintenanceCategoryOperation:
			operations++
		}
	}
	assert.Equal(t, 1, migrations, "expected 1 migration")
	assert.Equal(t, 3, operations, "expected 3 operations")
}

func TestMaintenanceGetOperationByKey(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	op, err := s.GetMaintenanceOperation(ctx, "secret-hub-id-migration")
	require.NoError(t, err)
	assert.Equal(t, "Secret Hub ID Namespace Migration", op.Title)
	assert.Equal(t, store.MaintenanceCategoryMigration, op.Category)
	assert.Equal(t, store.MaintenanceStatusPending, op.Status)

	// Not found
	_, err = s.GetMaintenanceOperation(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMaintenanceUpdateOperation(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	op, err := s.GetMaintenanceOperation(ctx, "secret-hub-id-migration")
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	op.Status = store.MaintenanceStatusCompleted
	op.StartedAt = &now
	op.CompletedAt = &now
	op.StartedBy = "admin-user"
	op.Result = `{"migrated": 5}`

	err = s.UpdateMaintenanceOperation(ctx, op)
	require.NoError(t, err)

	updated, err := s.GetMaintenanceOperation(ctx, "secret-hub-id-migration")
	require.NoError(t, err)
	assert.Equal(t, store.MaintenanceStatusCompleted, updated.Status)
	assert.Equal(t, "admin-user", updated.StartedBy)
	assert.Equal(t, `{"migrated": 5}`, updated.Result)
}

func TestMaintenanceRunCRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	runID := api.NewUUID()
	now := time.Now().UTC().Truncate(time.Second)

	run := &store.MaintenanceOperationRun{
		ID:           runID,
		OperationKey: "pull-images",
		Status:       store.MaintenanceStatusRunning,
		StartedAt:    now,
		StartedBy:    "admin-user",
		Log:          "Pulling images...",
	}

	err := s.CreateMaintenanceRun(ctx, run)
	require.NoError(t, err)

	// Get
	got, err := s.GetMaintenanceRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, "pull-images", got.OperationKey)
	assert.Equal(t, store.MaintenanceStatusRunning, got.Status)
	assert.Equal(t, "Pulling images...", got.Log)

	// Update
	completedAt := now.Add(30 * time.Second)
	got.Status = store.MaintenanceStatusCompleted
	got.CompletedAt = &completedAt
	got.Result = `{"pulled": 3}`
	got.Log = "Pulling images...\nDone."

	err = s.UpdateMaintenanceRun(ctx, got)
	require.NoError(t, err)

	updated, err := s.GetMaintenanceRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, store.MaintenanceStatusCompleted, updated.Status)
	assert.Equal(t, `{"pulled": 3}`, updated.Result)

	// List
	runs, err := s.ListMaintenanceRuns(ctx, "pull-images", 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, runID, runs[0].ID)

	// Not found
	_, err = s.GetMaintenanceRun(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

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
	"net/http"
	"strings"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

// handleAdminMaintenanceOps handles routes under /api/v1/admin/maintenance/operations.
// Phase 1: read-only list and get endpoints.
func (s *Server) handleAdminMaintenanceOps(w http.ResponseWriter, r *http.Request) {
	user := GetUserIdentityFromContext(r.Context())
	if user == nil || user.Role() != "admin" {
		Forbidden(w)
		return
	}

	if r.Method != http.MethodGet {
		MethodNotAllowed(w)
		return
	}

	// Extract sub-path: /api/v1/admin/maintenance/operations/{key}
	subPath := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/maintenance/operations")
	subPath = strings.TrimPrefix(subPath, "/")

	if subPath == "" {
		s.listMaintenanceOperations(w, r)
		return
	}

	// GET /api/v1/admin/maintenance/operations/{key}
	s.getMaintenanceOperation(w, r, subPath)
}

// listMaintenanceOperations returns all operations grouped by category.
func (s *Server) listMaintenanceOperations(w http.ResponseWriter, r *http.Request) {
	ops, err := s.store.ListMaintenanceOperations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to list maintenance operations", nil)
		return
	}

	var migrations []maintenanceOperationResponse
	var operations []maintenanceOperationWithLastRunResponse

	for _, op := range ops {
		if op.Category == store.MaintenanceCategoryMigration {
			migrations = append(migrations, toMaintenanceOperationResponse(op))
		} else {
			resp := maintenanceOperationWithLastRunResponse{
				maintenanceOperationResponse: toMaintenanceOperationResponse(op),
			}

			// Fetch the most recent run for this operation.
			runs, err := s.store.ListMaintenanceRuns(r.Context(), op.Key, 1)
			if err == nil && len(runs) > 0 {
				lastRun := toMaintenanceRunResponse(runs[0])
				resp.LastRun = &lastRun
			}

			operations = append(operations, resp)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"migrations": migrations,
		"operations": operations,
	})
}

// getMaintenanceOperation returns a single operation by key.
func (s *Server) getMaintenanceOperation(w http.ResponseWriter, r *http.Request, key string) {
	op, err := s.store.GetMaintenanceOperation(r.Context(), key)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "Operation not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get operation", nil)
		return
	}

	writeJSON(w, http.StatusOK, toMaintenanceOperationResponse(*op))
}

// Response types for maintenance operations API.

type maintenanceOperationResponse struct {
	ID          string      `json:"id"`
	Key         string      `json:"key"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Status      string      `json:"status"`
	CreatedAt   interface{} `json:"createdAt"`
	StartedAt   interface{} `json:"startedAt"`
	CompletedAt interface{} `json:"completedAt"`
	StartedBy   interface{} `json:"startedBy"`
	Result      interface{} `json:"result"`
}

type maintenanceOperationWithLastRunResponse struct {
	maintenanceOperationResponse
	LastRun *maintenanceRunResponse `json:"lastRun"`
}

type maintenanceRunResponse struct {
	ID          string      `json:"id"`
	Status      string      `json:"status"`
	StartedAt   interface{} `json:"startedAt"`
	CompletedAt interface{} `json:"completedAt"`
	StartedBy   interface{} `json:"startedBy"`
	Result      interface{} `json:"result"`
}

func toMaintenanceOperationResponse(op store.MaintenanceOperation) maintenanceOperationResponse {
	resp := maintenanceOperationResponse{
		ID:          op.ID,
		Key:         op.Key,
		Title:       op.Title,
		Description: op.Description,
		Category:    op.Category,
		Status:      op.Status,
		CreatedAt:   op.CreatedAt,
	}
	if op.StartedAt != nil {
		resp.StartedAt = op.StartedAt
	}
	if op.CompletedAt != nil {
		resp.CompletedAt = op.CompletedAt
	}
	if op.StartedBy != "" {
		resp.StartedBy = op.StartedBy
	}
	if op.Result != "" {
		resp.Result = op.Result
	}
	return resp
}

func toMaintenanceRunResponse(run store.MaintenanceOperationRun) maintenanceRunResponse {
	resp := maintenanceRunResponse{
		ID:        run.ID,
		Status:    run.Status,
		StartedAt: run.StartedAt,
	}
	if run.CompletedAt != nil {
		resp.CompletedAt = run.CompletedAt
	}
	if run.StartedBy != "" {
		resp.StartedBy = run.StartedBy
	}
	if run.Result != "" {
		resp.Result = run.Result
	}
	return resp
}

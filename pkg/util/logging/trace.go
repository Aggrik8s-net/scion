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

package logging

import (
	"net/http"
	"strings"
	"unicode"
)

const gcpTraceIDLabelKey = "appengine.googleapis.com/trace_id"

// ExtractTraceIDFromHeaders extracts and normalizes a trace ID from standard headers.
func ExtractTraceIDFromHeaders(r *http.Request) string {
	if r == nil {
		return ""
	}

	raw := strings.TrimSpace(r.Header.Get("X-Cloud-Trace-Context"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("traceparent"))
	}
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("X-Trace-ID"))
	}
	if raw == "" {
		return ""
	}

	normalized := NormalizeTraceID(raw)
	if normalized != "" {
		return normalized
	}
	return raw
}

// NormalizeTraceID returns a 32-char lowercase hex trace ID when possible.
func NormalizeTraceID(v string) string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return ""
	}

	if traceID := traceIDFromCloudTraceResource(raw); traceID != "" {
		return traceID
	}

	if parts := strings.Split(raw, "-"); len(parts) == 4 && strings.EqualFold(parts[0], "00") {
		if isValidTraceID(parts[1]) {
			return strings.ToLower(parts[1])
		}
	}

	if slash := strings.Index(raw, "/"); slash >= 0 {
		raw = raw[:slash]
	}
	if semi := strings.Index(raw, ";"); semi >= 0 {
		raw = raw[:semi]
	}
	raw = strings.TrimSpace(raw)

	if isValidTraceID(raw) {
		return strings.ToLower(raw)
	}
	return ""
}

func traceIDFromCloudTraceResource(v string) string {
	const marker = "/traces/"
	idx := strings.Index(v, marker)
	if idx < 0 {
		return ""
	}

	traceID := strings.TrimSpace(v[idx+len(marker):])
	if slash := strings.Index(traceID, "/"); slash >= 0 {
		traceID = traceID[:slash]
	}
	if isValidTraceID(traceID) {
		return strings.ToLower(traceID)
	}
	return ""
}

func isValidTraceID(v string) bool {
	if len(v) != 32 {
		return false
	}
	allZero := true
	for _, r := range v {
		if !isHexLowerUpper(r) {
			return false
		}
		if r != '0' {
			allZero = false
		}
	}
	return !allZero
}

func isHexLowerUpper(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// FormatCloudTraceResource formats a trace into Cloud Logging's trace resource path.
func FormatCloudTraceResource(projectID, traceID string) string {
	norm := NormalizeTraceID(traceID)
	if norm == "" {
		return ""
	}
	if strings.TrimSpace(projectID) == "" {
		return norm
	}
	return "projects/" + projectID + "/traces/" + norm
}

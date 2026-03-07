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
	"net/http/httptest"
	"testing"
)

func TestNormalizeTraceID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "x-cloud-trace-context",
			input: "4bf92f3577b34da6a3ce929d0e0e4736/123;o=1",
			want:  "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:  "traceparent",
			input: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			want:  "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:  "cloud trace resource",
			input: "projects/my-proj/traces/4bf92f3577b34da6a3ce929d0e0e4736",
			want:  "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:  "invalid",
			input: "not-a-trace",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTraceID(tt.input); got != tt.want {
				t.Fatalf("NormalizeTraceID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTraceIDFromHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	got := ExtractTraceIDFromHeaders(r)
	want := "4bf92f3577b34da6a3ce929d0e0e4736"
	if got != want {
		t.Fatalf("ExtractTraceIDFromHeaders() = %q, want %q", got, want)
	}
}

func TestFormatCloudTraceResource(t *testing.T) {
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	if got := FormatCloudTraceResource("proj-1", traceID); got != "projects/proj-1/traces/"+traceID {
		t.Fatalf("FormatCloudTraceResource() = %q", got)
	}
	if got := FormatCloudTraceResource("", traceID); got != traceID {
		t.Fatalf("FormatCloudTraceResource without project = %q", got)
	}
}

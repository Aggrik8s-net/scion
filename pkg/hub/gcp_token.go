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

import "context"

// GCPTokenGenerator generates GCP access and identity tokens via the IAM
// Credentials API using the Hub's own service account to impersonate target SAs.
type GCPTokenGenerator interface {
	GenerateAccessToken(ctx context.Context, serviceAccountEmail string, scopes []string) (*GCPAccessToken, error)
	GenerateIDToken(ctx context.Context, serviceAccountEmail string, audience string) (*GCPIDToken, error)
	VerifyImpersonation(ctx context.Context, serviceAccountEmail string) error
	// ServiceAccountEmail returns the email of the Hub's own GCP service account
	// used for impersonation. This is displayed in the UI when verification fails
	// to help users grant the correct IAM role.
	ServiceAccountEmail() string
}

// GCPAccessToken matches the GCE metadata server token response format.
type GCPAccessToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// GCPIDToken holds an OIDC identity token.
type GCPIDToken struct {
	Token string `json:"token"`
}

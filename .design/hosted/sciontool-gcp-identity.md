# GCP Identity for Agents via Metadata Server Emulation

## Status: Draft (Research & Design)

## Problem Statement

Scion agents running in containers frequently need to interact with Google Cloud APIs (GCS, BigQuery, Vertex AI, Cloud Build, etc.). Today, credentials are injected via environment variables or mounted credential files. This works for the LLM harness itself (e.g., `GOOGLE_APPLICATION_CREDENTIALS` for Vertex AI auth), but does not provide a general-purpose GCP identity to the agent's code execution environment. An agent writing a Cloud Function, querying BigQuery, or deploying infrastructure has no transparent way to authenticate.

We want agents to have seamless, transparent GCP identity so that any code using standard GCP client libraries (Go, Python, Node.js, Java) works without explicit credential management.

## Design Overview

Emulate the [GCE compute metadata server](https://cloud.google.com/compute/docs/metadata/overview) inside each agent container via a sciontool sidecar service. GCP client libraries follow a well-defined Application Default Credentials (ADC) discovery chain, and one of the standard steps is querying the metadata server at `169.254.169.254`. By intercepting this and serving tokens brokered through the Hub, we give agents a GCP identity that is:

- **Transparent**: No agent code changes required; standard client libraries just work.
- **Centrally managed**: Service account assignments are managed at the grove level via the Hub.
- **Auditable**: All token requests flow through the Hub, enabling logging and policy enforcement.
- **Secure**: No service account key files are distributed to agents or brokers.

### High-Level Flow

```
Agent Container                    Broker (sciontool)              Scion Hub                  GCP IAM
┌─────────────────┐               ┌──────────────────┐           ┌──────────────┐           ┌─────────┐
│ GCP client lib  │               │ metadata-svc     │           │              │           │         │
│ GET /token      │──────────────>│ (sidecar service) │           │              │           │         │
│                 │  HTTP to      │                  │           │              │           │         │
│                 │  GCE_METADATA │  validate request │           │              │           │         │
│                 │  _HOST        │  + agent identity │           │              │           │         │
│                 │               │                  │──────────>│ POST         │           │         │
│                 │               │                  │  SCION_   │ /agent/      │           │         │
│                 │               │                  │  AUTH_    │  gcp-token   │──────────>│ generate│
│                 │               │                  │  TOKEN    │              │  SA       │ access  │
│                 │               │                  │           │              │  imperson │ token   │
│                 │<──────────────│  return token     │<──────────│  return      │<──────────│         │
│ {access_token,  │               │                  │           │  {token,exp} │           │         │
│  expires_in}    │               │                  │           │              │           │         │
└─────────────────┘               └──────────────────┘           └──────────────┘           └─────────┘
```

## Prior Art & Reference

### GCE Metadata Server

The [GCE compute metadata server](https://cloud.google.com/compute/docs/metadata/overview) provides instance and project metadata to VMs running on Google Compute Engine. Key characteristics:

- **Endpoint**: `http://169.254.169.254` or `http://metadata.google.internal`
- **Required header**: `Metadata-Flavor: Google` (prevents SSRF)
- **Token endpoint**: `GET /computeMetadata/v1/instance/service-accounts/{account}/token`
  - Returns: `{"access_token": "...", "expires_in": 3599, "token_type": "Bearer"}`
  - `{account}` is typically `default` or a service account email
- **Other key endpoints**:
  - `/computeMetadata/v1/project/project-id`
  - `/computeMetadata/v1/instance/service-accounts/` (list)
  - `/computeMetadata/v1/instance/service-accounts/{account}/email`
  - `/computeMetadata/v1/instance/service-accounts/{account}/scopes`
  - `/computeMetadata/v1/instance/service-accounts/{account}/identity` (OIDC)

### Client Library Discovery

GCP client libraries implement ADC with this precedence:

1. `GOOGLE_APPLICATION_CREDENTIALS` env var (explicit key file)
2. User credentials from `gcloud auth application-default login`
3. Attached service account via metadata server
4. (Workload Identity Federation, etc.)

The metadata server check can be redirected via the `GCE_METADATA_HOST` environment variable (supported by Go, Python, Java, Node.js client libraries). Setting `GCE_METADATA_HOST=localhost:8080` causes all metadata requests to go to `http://localhost:8080` instead of `169.254.169.254`.

### salrashid123/gce_metadata_server

[This project](https://github.com/salrashid123/gce_metadata_server) provides a Go implementation of a metadata server emulator that can:
- Serve access tokens from a service account key file
- Support service account impersonation
- Serve project/instance metadata from a config file
- Bind to a local port or Unix domain socket

This is a useful reference, though our implementation differs significantly because we broker tokens through the Hub rather than holding key files locally.

## Detailed Design

### 1. Service Account Registration (Hub Resource)

Service accounts are a new grove-level resource type. They represent a GCP service account that can be assigned to agents in that grove.

**Why a new resource type (not a secret)?**

A service account assignment is not a secret — it is a *mapping* of a GCP service account email to a grove, with associated metadata. No key material is stored. The Hub's own GCP identity is used to impersonate the service account at token-generation time.

#### Data Model

```go
// GCPServiceAccount represents a GCP service account registered to a grove.
type GCPServiceAccount struct {
    ID               string    `json:"id"`                // UUID
    GroveID          string    `json:"grove_id"`          // FK to Grove
    Email            string    `json:"email"`             // e.g. "agent-worker@project.iam.gserviceaccount.com"
    ProjectID        string    `json:"project_id"`        // GCP project containing the SA
    DisplayName      string    `json:"display_name"`      // Human-friendly label
    Scopes           []string  `json:"scopes,omitempty"`  // OAuth scopes (default: cloud-platform)
    Verified         bool      `json:"verified"`          // Hub confirmed it can impersonate this SA
    VerifiedAt       time.Time `json:"verified_at,omitempty"`
    CreatedBy        string    `json:"created_by"`        // User who registered it
    CreatedAt        time.Time `json:"created_at"`
}
```

#### Hub API Endpoints

```
POST   /api/v1/groves/{groveId}/gcp-service-accounts       # Register SA
GET    /api/v1/groves/{groveId}/gcp-service-accounts       # List SAs
GET    /api/v1/groves/{groveId}/gcp-service-accounts/{id}  # Get SA details
DELETE /api/v1/groves/{groveId}/gcp-service-accounts/{id}  # Remove SA
POST   /api/v1/groves/{groveId}/gcp-service-accounts/{id}/verify  # Verify impersonation
```

#### Verification Flow

On registration (or explicit verify), the Hub attempts to generate a test token for the service account using its own credentials:

```go
// Hub calls IAM credentials API to verify it can act as the SA
iamService.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
    Name:      "projects/-/serviceAccounts/" + sa.Email,
    Scope:     []string{"https://www.googleapis.com/auth/cloud-platform"},
    Lifetime:  durationpb.New(300 * time.Second),
})
```

If successful, the SA is marked `Verified=true`. This confirms the Hub's own service account has `roles/iam.serviceAccountTokenCreator` on the target SA.

#### Authorization Requirements

- **Registering a service account**: Requires `manage` action on the grove (grove owner/admin).
- **Assigning a service account to an agent**: Requires `create` action on agents in the grove (standard agent creation permission).
- **Using tokens at runtime**: The agent's JWT scopes must include a new scope `grove:gcp:token` (granted automatically when GCP identity is assigned).

### 2. Agent GCP Identity Assignment

When creating an agent, the user can optionally assign a GCP identity:

```json
{
  "name": "my-agent",
  "template": "default",
  "gcp_identity": {
    "service_account_id": "uuid-of-registered-sa",
    "metadata_mode": "assign"
  }
}
```

#### Metadata Modes

The `metadata_mode` field on agent creation controls how the metadata server behaves:

| Mode | Behavior | Use Case |
|------|----------|----------|
| `block` | Metadata sidecar returns 403 for all token requests. Other metadata (project-id, zone) still served. | Prevent agents from inheriting broker/host GCP identity. Security-hardened agents. |
| `passthrough` | No metadata sidecar started. Metadata requests reach the real metadata server (if running on GCE) or fail naturally. | Local development, agents on GCE that should use the VM's identity. |
| `assign` | Metadata sidecar serves tokens for the assigned service account via Hub brokering. | Production agents needing GCP API access. |

**Default**: `block` — prevents accidental credential leakage from the broker's compute environment.

#### Agent Model Extension

```go
// In the Agent or AgentConfig model:
type GCPIdentityConfig struct {
    MetadataMode       string `json:"metadata_mode"`                  // "block", "passthrough", "assign"
    ServiceAccountID   string `json:"service_account_id,omitempty"`   // FK to GCPServiceAccount (required for "assign")
    ServiceAccountEmail string `json:"service_account_email,omitempty"` // Denormalized for runtime use
    ProjectID          string `json:"project_id,omitempty"`            // Denormalized
}
```

### 3. Metadata Server Sidecar (sciontool)

The metadata emulator runs as a sidecar service managed by sciontool's existing `ServiceManager`. It is not a separate binary — it is a built-in HTTP server within sciontool itself, started via a new subcommand.

#### Sciontool Command

```bash
sciontool metadata-server \
  --mode=assign \
  --port=18380 \
  --service-account-email=agent-worker@project.iam.gserviceaccount.com \
  --project-id=my-project
```

#### Service Spec (injected during provisioning)

When `metadata_mode` is `block` or `assign`, the provisioning pipeline injects a service spec into `scion-services.yaml`:

```yaml
- name: gcp-metadata
  command: ["sciontool", "metadata-server", "--mode=assign", "--port=18380",
            "--service-account-email=agent-worker@project.iam.gserviceaccount.com",
            "--project-id=my-project"]
  restart: always
  ready_check:
    type: http
    target: "http://localhost:18380/computeMetadata/v1/"
    timeout: "5s"
```

#### Environment Variable Injection

The provisioning pipeline also sets:

```
GCE_METADATA_HOST=localhost:18380
```

This redirects all GCP client library metadata lookups to the sidecar.

#### Endpoints Implemented

**Minimum viable set:**

| Endpoint | Response |
|----------|----------|
| `GET /` | `OK` (health) |
| `GET /computeMetadata/v1/` | Metadata root (recursive listing) |
| `GET /computeMetadata/v1/project/project-id` | Project ID string |
| `GET /computeMetadata/v1/project/numeric-project-id` | Numeric project ID (or empty) |
| `GET /computeMetadata/v1/instance/service-accounts/` | List: `default/\n{email}/\n` |
| `GET /computeMetadata/v1/instance/service-accounts/default/email` | SA email |
| `GET /computeMetadata/v1/instance/service-accounts/default/token` | Access token JSON |
| `GET /computeMetadata/v1/instance/service-accounts/default/scopes` | Scope list |
| `GET /computeMetadata/v1/instance/service-accounts/{email}/token` | Access token JSON |
| `GET /computeMetadata/v1/instance/service-accounts/{email}/email` | SA email |

**Token response format** (matches GCE exactly):

```json
{
  "access_token": "ya29.c.ElpSB...",
  "expires_in": 3599,
  "token_type": "Bearer"
}
```

**Header validation:**
- All requests must include `Metadata-Flavor: Google` header.
- Requests without this header receive `403 Forbidden` with body: `Missing Metadata-Flavor:Google header.`

#### Token Acquisition Flow (assign mode)

```
1. Client library requests GET /computeMetadata/v1/instance/service-accounts/default/token
2. Metadata sidecar validates Metadata-Flavor header
3. Sidecar checks token cache:
   a. If cached token exists and has >60s remaining → return cached token
   b. Otherwise → request fresh token from Hub
4. Sidecar calls Hub: POST /api/v1/agent/gcp-token
   - Authorization: Bearer <SCION_AUTH_TOKEN>
   - Body: {"service_account_email": "...", "scopes": ["https://www.googleapis.com/auth/cloud-platform"]}
5. Hub validates agent JWT, checks scope grove:gcp:token
6. Hub verifies the requested SA is assigned to this agent's grove
7. Hub calls GCP IAM Credentials API (generateAccessToken) using its own identity
8. Hub returns token to sidecar
9. Sidecar caches token and returns to client
```

#### Token Caching

The sidecar caches tokens locally to minimize Hub round-trips:

- Cache key: service account email + scopes
- Eviction: when `expires_in` drops below 60 seconds (refresh window)
- On cache miss or expiry: synchronous request to Hub
- Concurrent requests for the same token coalesce (singleflight pattern)

#### Block Mode Behavior

In `block` mode, the sidecar:
- Returns `403` for all `/token` and `/identity` endpoints
- Still serves project metadata (project-id) if configured
- Logs blocked requests for observability

### 4. Hub Token Brokering Endpoint

#### New Hub Endpoint

```
POST /api/v1/agent/gcp-token
```

**Authentication**: Agent JWT (Bearer token with `grove:gcp:token` scope)

**Request:**
```json
{
  "scopes": ["https://www.googleapis.com/auth/cloud-platform"]
}
```

Note: The service account email is **not** in the request body. The Hub resolves it from the agent's GCP identity assignment. This prevents an agent from requesting tokens for arbitrary service accounts.

**Response (200):**
```json
{
  "access_token": "ya29.c.ElpSB...",
  "expires_in": 3599,
  "token_type": "Bearer"
}
```

**Error Responses:**
- `401`: Invalid or expired agent token
- `403`: Agent does not have `grove:gcp:token` scope, or no GCP identity assigned
- `502`: Hub failed to generate token from GCP (impersonation failed)
- `503`: GCP IAM service unavailable

#### Hub Implementation

```go
func (s *Server) handleAgentGCPToken(w http.ResponseWriter, r *http.Request) {
    // 1. Extract agent identity from context (set by auth middleware)
    agent := GetAgentIdentityFromContext(r.Context())
    if agent == nil || !agent.HasScope(AgentTokenScopeGCPToken) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }

    // 2. Look up agent's GCP identity assignment
    agentRecord, _ := s.store.GetAgent(r.Context(), agent.ID())
    if agentRecord.GCPIdentity == nil || agentRecord.GCPIdentity.MetadataMode != "assign" {
        http.Error(w, "no GCP identity assigned", http.StatusForbidden)
        return
    }

    // 3. Parse requested scopes (or default)
    var req gcpTokenRequest
    json.NewDecoder(r.Body).Decode(&req)
    scopes := req.Scopes
    if len(scopes) == 0 {
        scopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
    }

    // 4. Generate access token via IAM Credentials API
    token, err := s.gcpTokenGenerator.GenerateAccessToken(r.Context(),
        agentRecord.GCPIdentity.ServiceAccountEmail, scopes)
    if err != nil {
        http.Error(w, "token generation failed", http.StatusBadGateway)
        return
    }

    // 5. Return token
    json.NewEncoder(w).Encode(token)
}
```

#### GCP Token Generator

```go
type GCPTokenGenerator interface {
    GenerateAccessToken(ctx context.Context, serviceAccountEmail string, scopes []string) (*GCPAccessToken, error)
    VerifyImpersonation(ctx context.Context, serviceAccountEmail string) error
}

type GCPAccessToken struct {
    AccessToken string `json:"access_token"`
    ExpiresIn   int    `json:"expires_in"`
    TokenType   string `json:"token_type"`
}
```

The default implementation uses the [IAM Credentials API](https://cloud.google.com/iam/docs/reference/credentials/rest/v1/projects.serviceAccounts/generateAccessToken):

```go
type iamTokenGenerator struct {
    client *credentials.IamCredentialsClient  // google.golang.org/genproto/googleapis/iam/credentials/v1
}

func (g *iamTokenGenerator) GenerateAccessToken(ctx context.Context, email string, scopes []string) (*GCPAccessToken, error) {
    resp, err := g.client.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
        Name:     fmt.Sprintf("projects/-/serviceAccounts/%s", email),
        Scope:    scopes,
        Lifetime: durationpb.New(3600 * time.Second),
    })
    if err != nil {
        return nil, fmt.Errorf("IAM generateAccessToken failed: %w", err)
    }
    return &GCPAccessToken{
        AccessToken: resp.AccessToken,
        ExpiresIn:   int(time.Until(resp.ExpireTime.AsTime()).Seconds()),
        TokenType:   "Bearer",
    }, nil
}
```

### 5. New Agent Token Scope

Add a new scope to `AgentTokenScope`:

```go
const (
    // existing scopes...
    AgentTokenScopeGCPToken AgentTokenScope = "grove:gcp:token"
)
```

This scope is automatically added to the agent's JWT when provisioned with `metadata_mode: assign`.

### 6. Provisioning Pipeline Changes

During agent provisioning (in `pkg/agent/provision.go` and `pkg/runtimebroker/start_context.go`):

1. **Resolve GCP identity**: If the agent has a GCP identity assignment, fetch the service account details.
2. **Inject metadata service spec**: Add the `gcp-metadata` service to `scion-services.yaml`.
3. **Set environment**: Add `GCE_METADATA_HOST=localhost:18380` to the agent's environment.
4. **Suppress conflicting auth**: When metadata mode is `assign` or `block`, ensure `GOOGLE_APPLICATION_CREDENTIALS` is **not** set (it takes higher precedence in ADC than the metadata server). If a user has explicitly set GAC via secrets, that should still win — the metadata server acts as a fallback.
5. **Add JWT scope**: Include `grove:gcp:token` in the agent's JWT scopes.

## Alternatives Considered

### A. Mount Service Account Key Files Directly

**Approach**: Store SA key files as grove secrets, mount into containers as files.

**Pros**: Simple; no metadata server needed; works offline.

**Cons**:
- Key files are long-lived credentials that can be exfiltrated by malicious agents.
- No centralized revocation — revoking requires rotating the key in GCP and updating all secrets.
- Violates Google's recommendation to avoid downloaded key files.
- No audit trail of token usage through the Hub.

**Verdict**: Rejected for production use. May be acceptable as a degenerate "bring your own key" fallback for local/solo mode.

### B. Direct Impersonation (No Metadata Server)

**Approach**: Use `GOOGLE_APPLICATION_CREDENTIALS` with a workload identity federation config that points to the Hub as a token source.

**Pros**: No metadata server sidecar; uses GCP's native WIF mechanism.

**Cons**:
- Requires generating and mounting a WIF credential config file per agent.
- WIF config format is complex and brittle.
- Not all client libraries support all WIF source types uniformly.
- The metadata server approach is more universally compatible.

**Verdict**: Interesting for future exploration, but metadata server emulation is more robust and universally supported.

### C. Network-Level Interception (iptables redirect)

**Approach**: Use iptables/nftables to redirect traffic destined for `169.254.169.254` to the local sidecar, instead of setting `GCE_METADATA_HOST`.

**Pros**: Transparent even to tools that don't respect `GCE_METADATA_HOST`; matches real GCE behavior exactly.

**Cons**:
- Requires `NET_ADMIN` capability in the container (security concern).
- More complex to set up and debug.
- May conflict with container networking (especially on Kubernetes where a real metadata server exists).
- `GCE_METADATA_HOST` is supported by all major GCP client libraries, making iptables unnecessary for the primary use case.

**Verdict**: Not needed initially. Can be added as an enhancement for edge cases (e.g., third-party tools that hardcode `169.254.169.254`).

### D. Hub-Side Token Caching

**Approach**: Cache tokens at the Hub level, returning cached tokens for repeat requests.

**Pros**: Reduces IAM API calls across all agents sharing the same SA.

**Cons**:
- Tokens are bearer tokens — caching at the Hub means a Hub compromise exposes all cached tokens.
- GCP access tokens are already valid for ~1 hour; sidecar-level caching is sufficient.
- Adds complexity to the Hub.

**Verdict**: Defer. Sidecar-level caching is adequate. Hub-level caching can be added later if IAM API rate limits become a concern.

## Open Questions

### Q1: Should the default metadata mode be `block` or `passthrough`?

**`block` (recommended)**: Prevents agents from accidentally using the broker's GCP identity. This is the safer default and follows the principle of least privilege. Agents that need GCP access must be explicitly granted it.

**`passthrough`**: Simpler for local development where the user is running on a GCE VM and wants agents to just work. Could be confusing in hosted mode where the broker's identity is not the user's.

**Proposal**: Default to `block` in hosted mode, `passthrough` in local/solo mode.

### Q2: What grove role should be required to register a service account?

Options:
- **Grove owner only**: Strictest. Only the person who created the grove can add SAs.
- **Grove admin** (recommended): Admins can manage SAs, which aligns with their existing ability to manage grove configuration.
- **Any grove member**: Too permissive — SA assignment affects all agents in the grove.

### Q3: Should agents be able to request tokens for specific scopes?

The current design has the Hub generate tokens with broad `cloud-platform` scope. Should agents be able to request narrower scopes?

**Pros of scope restriction**: Follows least privilege; limits blast radius if token is exfiltrated.

**Cons**: Complicates the API; most real usage needs `cloud-platform`; GCP's own metadata server doesn't restrict scopes per-request.

**Proposal**: Start with `cloud-platform` as the only supported scope. The scope is configured at SA registration time and not overridable per-request.

### Q4: How should OIDC identity tokens be handled?

The metadata server also serves OIDC identity tokens via `/instance/service-accounts/{account}/identity?audience=...`. These are used for Cloud Run service-to-service auth and other OIDC flows.

The IAM Credentials API also supports `generateIdToken`. We could support this via the same Hub brokering pattern.

**Proposal**: Defer to a follow-up. Access tokens cover the majority of use cases.

### Q5: Should the metadata server support multiple service accounts?

GCE VMs can have multiple service accounts attached. Should we support assigning multiple SAs to an agent?

**Proposal**: Start with one SA per agent (served as both `default` and by email). Multiple SAs can be added later if needed.

### Q6: Token lifetime and refresh strategy

GCP access tokens typically last 3600 seconds. The metadata sidecar caches and refreshes them. Questions:
- Should the Hub allow configuring shorter token lifetimes? (IAM API supports 300-3600s)
- Should the sidecar proactively refresh before expiry, or only refresh on demand?

**Proposal**: 3600s lifetime (GCP default), demand-based refresh with a 60-second buffer. Proactive refresh adds complexity for minimal benefit since most agent sessions are short-lived.

### Q7: How should this interact with existing harness auth?

If an agent has both a Gemini API key (for the LLM) and a GCP identity (for API access), they should coexist. The metadata server provides ambient GCP identity; the API key provides LLM access. No conflict.

However, if the harness auth is set to `vertex-ai` and relies on ADC, the metadata server identity would be used for the LLM too. This is actually desirable — it means the assigned SA needs Vertex AI permissions.

**Proposal**: Document the interaction clearly. No code changes needed — ADC precedence handles it naturally.

### Q8: Audit logging

Should the Hub log every token request? Every token generation? Options:
- Log every request (high volume, noisy)
- Log first request per agent per session (useful for audit, low volume)
- Log on errors only

**Proposal**: Log token generation events (not cache hits at the sidecar) with agent ID, grove ID, SA email, and timestamp. This provides an audit trail without excessive volume.

## Implementation Sketch

### Phase 1: Foundation (MVP)

**Goal**: End-to-end token flow for a single assigned SA.

1. **Store layer**: Add `GCPServiceAccount` model and store interface methods.
2. **Hub endpoints**: CRUD for grove service accounts + verify endpoint.
3. **Hub token endpoint**: `POST /api/v1/agent/gcp-token` with IAM Credentials integration.
4. **Agent token scope**: Add `grove:gcp:token` scope.
5. **sciontool metadata server**: New `metadata-server` subcommand with HTTP server implementing token and project-id endpoints.
6. **Provisioning changes**: Inject metadata service spec and `GCE_METADATA_HOST` env var.
7. **Agent model extension**: Add `GCPIdentityConfig` to agent creation/config.

**Files to create/modify**:

| File | Change |
|------|--------|
| `pkg/store/models.go` | Add `GCPServiceAccount` model |
| `pkg/store/store.go` | Add `GCPServiceAccountStore` interface |
| `pkg/store/sqlite.go` | Implement store (if using SQLite) |
| `pkg/hub/handlers_gcp_identity.go` | New: SA CRUD + verify handlers |
| `pkg/hub/gcp_token.go` | New: Token generation + Hub endpoint handler |
| `pkg/hub/server.go` | Register new routes |
| `pkg/hub/agenttoken.go` | Add `grove:gcp:token` scope |
| `pkg/sciontool/metadata/server.go` | New: Metadata HTTP server |
| `cmd/sciontool/commands/metadata.go` | New: `metadata-server` subcommand |
| `pkg/agent/provision.go` | Inject metadata service + env var |
| `pkg/runtimebroker/start_context.go` | Pass GCP identity config to provisioning |
| `pkg/api/types.go` | Add `GCPIdentityConfig` to relevant types |

### Phase 2: Hardening

- Block mode implementation (403 for token requests)
- Audit logging for token generation events
- CLI commands for SA management (`scion grove service-accounts add/list/remove/verify`)
- Web UI for SA management and agent identity assignment
- Rate limiting on the Hub token endpoint (per-agent)
- Metrics: token requests, cache hit rate, IAM API latency

### Phase 3: Extensions

- OIDC identity token support (`/identity` endpoint)
- Multiple service accounts per agent
- Scope restrictions per SA registration
- Hub-level token caching (if IAM API rate limits are hit)
- Support for Workload Identity Federation as an alternative backend
- iptables-based interception option for non-standard tools

## Security Considerations

### Threat Model

1. **Malicious agent requests token for wrong SA**: Prevented — Hub resolves SA from agent's assignment, not from request body.
2. **Agent exfiltrates access token**: Mitigated — tokens are short-lived (1 hour). No long-lived key material in the container.
3. **Agent bypasses metadata sidecar**: In `block` mode, if `GCE_METADATA_HOST` is unset/overridden, requests go to `169.254.169.254` which is the real metadata server. On non-GCE hosts this fails naturally. On GCE, the agent would get the broker's identity — this is the existing behavior and `block` mode's iptables variant (Phase 3) addresses it.
4. **Hub compromise exposes IAM credentials**: The Hub uses its own managed identity (GCE SA or Workload Identity) — no key files stored. Compromise of the Hub process allows token generation, but revoking the Hub's `serviceAccountTokenCreator` role immediately cuts off all agent tokens.
5. **Broker compromise**: Broker never holds SA credentials. It only holds the agent JWT, which is scoped and short-lived.

### Principle of Least Privilege

- Agents only get tokens for their assigned SA.
- The Hub's own SA only needs `roles/iam.serviceAccountTokenCreator` on target SAs — not broad IAM permissions.
- Agent JWTs require explicit `grove:gcp:token` scope.
- Default metadata mode is `block`, requiring explicit opt-in.

## Dependencies

- **GCP IAM Credentials API**: `google.golang.org/api/iamcredentials/v1` or the gRPC client `cloud.google.com/go/iam/credentials/apiv1`
- **Hub must run with a GCP identity** that has `roles/iam.serviceAccountTokenCreator` on target SAs.
- **Existing scion infrastructure**: ServiceManager (sidecar services), agent token scopes, Hub auth middleware, store layer.

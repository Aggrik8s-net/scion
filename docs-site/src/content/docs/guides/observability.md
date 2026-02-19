---
title: Observability
description: Monitoring agents with logs and metrics.
---

Scion provides comprehensive observability for agent containers and system components through the `sciontool` telemetry pipeline and OpenTelemetry log bridging. This guide covers how to monitor agent activity, collect logs, and integrate with cloud-native observability platforms like Google Cloud Logging and Trace.

## Architecture Overview

Scion's observability architecture follows a "forwarder" pattern where `sciontool` acts as a local collector inside each agent container, and system components (Hub and Broker) bridge their logs directly to a central backend.

```
┌─────────────────────────────────────────┐
│           Agent Container               │
│                                         │
│  ┌─────────────┐                       │
│  │   Agent     │ OTLP (localhost:4317) │
│  │  (Claude/   │───────┐               │
│  │   Gemini)   │       │               │
│  └─────────────┘       │               │
│                        ▼               │
│              ┌─────────────────┐       │
│              │   sciontool     │       │
│              │   forwarder     │       │
│              └────────┬────────┘       │
│                       │                │
│                       │ OTLP (Cloud)   │
└───────────────────────┼────────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │  Cloud Backend  │
              │ (Logging/Trace) │
              └─────────────────┘
                        ▲
                        │ OTLP (Cloud)
              ┌─────────┴─────────┐
              │    System Logs    │
              │  (Hub & Broker)   │
              └───────────────────┘
```

## Administrator Setup: Cloud Logging

To centralize logs and traces from all Scion components in Google Cloud, you must configure the OTLP endpoints and project identifiers.

### Connecting Hub and Broker Logs

The Scion Hub and Runtime Broker use structured logging (`slog`) with an OpenTelemetry bridge. To enable log forwarding to Google Cloud:

1.  **Configure Environment Variables**: Set the following on your Hub and Broker server processes:

    ```bash
    # Enable OTel log forwarding
    export SCION_OTEL_LOG_ENABLED=true

    # Set the GCP OTLP endpoint (standard for Cloud Trace/Logging)
    export SCION_OTEL_ENDPOINT="monitoring.googleapis.com:443"

    # Specify your GCP Project ID
    export SCION_GCP_PROJECT_ID="your-project-id"
    ```

2.  **Authentication**: Ensure the service account running the Hub/Broker has the following IAM roles:
    - `roles/logging.logWriter`
    - `roles/cloudtrace.agent`
    - `roles/monitoring.metricWriter`

### Direct Cloud Logging (Alternative)

As an alternative to the OTel pipeline, the Hub and Broker can send logs directly to Google Cloud Logging using the `cloud.google.com/go/logging` client library. This is simpler to set up when you only need log forwarding without traces or metrics:

```bash
# Enable direct Cloud Logging
export SCION_CLOUD_LOGGING=true
export SCION_GCP_PROJECT_ID="your-project-id"

# Optional: customize the log name (default: "scion")
export SCION_CLOUD_LOGGING_LOG_ID="scion-hub"

scion server start --enable-hub
```

Both approaches can be used simultaneously — OTel for the full telemetry pipeline and Cloud Logging for direct log delivery.

### Configuring Agent Telemetry

Agents use `sciontool` as their init process, which includes an embedded OTLP forwarder. This forwarder must be configured to point to your cloud backend.

#### Via Settings File (Recommended)

The preferred approach is to configure telemetry in `settings.yaml`. Settings at the global level apply to all agents; grove-level settings apply to a specific project. Templates and individual agents can further override these via their `scion-agent.yaml`.

```yaml
# In ~/.scion/settings.yaml (global) or .scion/settings.yaml (grove)
telemetry:
  enabled: true
  cloud:
    enabled: true
    endpoint: "monitoring.googleapis.com:443"
    protocol: grpc
  filter:
    events:
      exclude:
        - "agent.user.prompt"
```

See the [Orchestrator Settings Reference](/reference/orchestrator-settings/#telemetry-configuration-telemetry) for the full field reference, and [Metrics & OpenTelemetry](/guides/metrics/#configuration-hierarchy) for how settings merge across scopes.

#### Via Hub Environment Variables

For hosted deployments, environment variables can be set at the Grove or Broker level on the Hub. These are automatically injected into every agent container.

```bash
SCION_OTEL_ENDPOINT="monitoring.googleapis.com:443"
SCION_GCP_PROJECT_ID="your-project-id"
SCION_TELEMETRY_ENABLED="true"
```

#### Harness-Specific Configuration

If you are using agents that natively support OpenTelemetry (like `opencode`), you may need to explicitly tell the agent where to find the `sciontool` forwarder (which is `localhost` from the agent's perspective):

- **gRPC (Default)**: `OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"`
- **HTTP**: `OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"`

These harness-specific env vars are injected at agent start time via the harness config's `env` map and are separate from the Scion telemetry settings.

## Agent Logs

Agent logs are written to `/home/scion/agent.log` inside the container. The sciontool logging system writes to both stderr and this file.

### Log Ownership and Permissions

The `sciontool` utility ensures that `agent.log` is owned by the `scion` user during initialization, even if `sciontool` is initially run as root. The log file is created with permissive `0666` permissions to ensure multiple processes can contribute to the log stream.

### Log Levels

- **INFO**: Normal operational events
- **ERROR**: Critical failures
- **DEBUG**: Detailed information (enabled with `SCION_DEBUG=true` or `SCION_LOG_LEVEL=debug`)

## Telemetry Collection

The telemetry pipeline in sciontool collects and forwards OpenTelemetry (OTLP) data from agents. See the [Metrics & OpenTelemetry guide](/guides/metrics) for deep configuration details.

### What's Collected

| Data Type | Source | Description |
|-----------|--------|-------------|
| Traces | Agent OTLP | Span data for tool calls, API requests |
| Metrics | sciontool | Counters and histograms for tokens, tools, and latency |
| Correlated Logs | sciontool | Log records linked to traces for every hook event |
| Hook Events | Harness hooks | Tool calls, prompts, model invocations converted to spans |
| Session Metrics | Gemini session files | Token counts, turn counts, tool statistics |

### Privacy Controls

By default, user prompts (`agent.user.prompt`) are excluded from telemetry to protect privacy. Additionally, sensitive attributes are automatically redacted or hashed.

- **Redacted**: `prompt`, `user.email`, `tool_output`, `tool_input`
- **Hashed**: `session_id`

## Troubleshooting for Admins

### Logs Not Appearing in GCP

1.  **Verify Endpoints**: Ensure `SCION_OTEL_ENDPOINT` is set to `monitoring.googleapis.com:443`.
2.  **Check Permissions**: Verify the Workload Identity or Service Account has `roles/logging.logWriter`.
3.  **Inspect Agent Init**: View the agent container logs (stderr) to see if `sciontool` reported a telemetry startup failure:
    ```
    [sciontool] ERROR: Failed to start telemetry: connection refused
    ```
4.  **Network Policy**: If running in Kubernetes, ensure Egress is allowed to GCP APIs.

### Missing Trace Correlation

If you see logs but they aren't linked to traces in the Cloud Trace waterfall:
1.  Ensure the agent is using the `sciontool` gRPC port (4317).
2.  Verify `SCION_OTEL_LOG_ENABLED=true` is set on the system components.

## Related Guides

- [Metrics & OpenTelemetry](/guides/metrics) - Detailed telemetry configuration
- [Hub Server](/guides/hub-server) - Hub integration for hosted mode
- [Runtime Broker](/guides/runtime-broker) - Broker setup and configuration

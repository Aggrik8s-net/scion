---
title: Agent Configuration (scion-agent.yaml)
description: Reference for Scion agent templates and configuration.
---

The `scion-agent.yaml` file acts as the blueprint for an agent. It defines the environment, resources, and harness configuration required to run the agent.

## File Locations

- **Templates**: `.scion/templates/<template-name>/scion-agent.yaml`
- **Active Agents**: `.scion/agents/<agent-name>/scion-agent.yaml`

:::note[Format Change]
Previous versions of Scion used `scion-agent.json`. The new versioned settings system uses `scion-agent.yaml`, though JSON is still supported as valid YAML.
:::

## Configuration Fields

### Core Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `schema_version` | string | Should be `"1"`. |
| `default_harness_config` | string | The name of the default harness config to use (e.g., `gemini`, `claude`). |
| `image` | string | Override the container image defined in the harness config. |
| `env` | map | Environment variables to inject into the container. |
| `volumes` | list | Additional volume mounts. |
| `detached` | bool | Run in background (default `true`). |
| `command_args` | list | Additional arguments passed to the harness entrypoint. |

:::caution[Harness Field Deprecated]
The `harness` field is no longer supported in `scion-agent.yaml`. Templates must be harness-agnostic. Use `default_harness_config` to specify a preferred harness, which can be overridden by users at runtime.
:::

### Limits & Resources

| Field | Type | Description |
| :--- | :--- | :--- |
| `max_turns` | int | Maximum number of LLM turns before the agent stops. |
| `max_duration` | string | Maximum runtime duration (e.g., `"2h"`, `"30m"`). |
| `resources` | object | Container resource requests/limits (see below). |

### Resource Specification

```yaml
resources:
  requests:
    cpu: "500m"
    memory: "512Mi"
  limits:
    cpu: "2"
    memory: "2Gi"
  disk: "10Gi"
```

### Sidecar Services (`services`)

Define auxiliary containers to run alongside the agent (e.g., a headless browser).

```yaml
services:
  - name: browser
    command: ["chromium", "--headless"]
    ready_check:
      type: tcp
      target: "localhost:9222"
```

### Telemetry (`telemetry`)

Override telemetry settings for this template or agent. These merge on top of any telemetry configuration defined in `settings.yaml` (global or grove scope), using last-write-wins semantics.

```yaml
telemetry:
  enabled: true
  cloud:
    endpoint: "monitoring.googleapis.com:443"
  filter:
    events:
      exclude:
        - "agent.user.prompt"
  resource:
    service.name: "my-specialized-agent"
```

See the [Orchestrator Settings Reference](/reference/orchestrator-settings/#telemetry-configuration-telemetry) for the full field reference and the [Metrics guide](/guides/metrics/#configuration-hierarchy) for how telemetry settings merge across scopes.

### Kubernetes Specifics (`kubernetes`)

Overrides for Kubernetes runtimes.

```yaml
kubernetes:
  namespace: "custom-ns"
  service_account_name: "workload-identity-sa"
  runtime_class_name: "gvisor"
```

## Resolution Logic

When an agent starts:

1.  **Template Load**: Scion loads `scion-agent.yaml` from the selected template.
2.  **Harness Resolution**: It resolves the `harness_config` against the active profile's `harness_configs` map in `settings.yaml`.
3.  **Overrides**: CLI flags (e.g., `--image`, `--env`) override values in `scion-agent.yaml`.
4.  **Final Config**: The resolved configuration is written to the agent's runtime directory.

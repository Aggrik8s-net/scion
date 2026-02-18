---
title: Team Workflow
description: Connecting to a Scion Hub for team collaboration.
---

Scion's "Hosted" mode allows teams to share state, infrastructure, and agent configurations by connecting to a central Scion Hub.

## Connecting to a Hub

To connect your local CLI to a team Hub, you configure the `hub` section in your `settings.yaml`.

### Configuration

Edit `~/.scion/settings.yaml` (or use `scion config set`):

```yaml
hub:
  enabled: true
  endpoint: "https://scion.yourcompany.com"
  local_only: false
```

### Authentication

Once the endpoint is configured, authenticate your CLI:

```bash
scion hub auth login
```

This will open your browser to complete the OAuth flow.

## Project Linking (Groves)

In a team environment, a **Grove** represents a shared project. You link your local directory to a Grove on the Hub to share context with your team.

```bash
# Link the current directory to the Hub
scion hub link
```

If the project is already registered (matched by Git remote), Scion will link it automatically. If not, it will prompt you to register a new Grove.

### Grove Configuration

When linked, your `.scion/settings.yaml` will include the Grove ID:

```yaml
hub:
  grove_id: "uuid-of-the-grove"
```

## Using Remote Infrastructure

With the Hub connected, you can dispatch agents to **Runtime Brokers** managed by your team, rather than running them on your local laptop.

### Selecting a Broker
The Hub automatically routes tasks to available brokers. You can tag agents to request specific capabilities (e.g., `gpu-capable`).

### Local Fallback
If you want to temporarily run agents locally even while connected to the Hub, you can use the `--local` flag or set `hub.local_only: true` in your settings.

## Shared Secrets

Teams should manage secrets centrally on the Hub instead of sharing `.env` files.

```bash
# Set a secret for the project
scion hub secret set OPENAI_API_KEY=sk-...

# Secrets are injected into agents at runtime by the Broker
```

## Collaboration

- **Web Dashboard**: Use the Hub's web interface to view running agents, logs, and status.
- **Remote Attach**: You can attach to a remote agent's terminal session using `scion attach`, tunneling through the Hub.

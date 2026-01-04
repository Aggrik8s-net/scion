# scion Implementation Milestones

This document tracks the evolution of `scion`. It has been updated to reflect the transition to "grove" terminology and current project status.

## Milestone 1: Core Scaffolding & Configuration
**Goal**: Establish the basic CLI structure and filesystem management.

- [x] Implement `scion grove init` (**Completed**)
    - [x] Create `.scion/` directory structure.
    - [x] Seed `.scion/templates/default` with basic agent structure.
    - [x] Create global `~/.scion/` structure for cross-project use.
- [x] Implement Template Loading (**Completed**)
    - [x] Logic to find and load templates (Project-local vs. Global).
    - [x] Simple inheritance (custom template merged with `default`).
- [x] Grove Resolution logic (**Completed**)
    - [x] Precedence: Explicit flag > Project-local > Global.

## Milestone 2: Runtime Abstraction & Containerization
**Goal**: Unified interface for managing isolated agent environments.

- [x] Implement `Runtime` interface (**Completed**)
    - [x] Methods: `Run`, `Stop`, `Delete`, `List`, `GetLogs`, `Attach`.
- [x] Implement macOS `container` backend (**Completed**)
    - [x] Apple virtualization integration.
    - [x] Tmux integration for interactive TTY.
- [x] Implement `docker` backend (**Completed**)
- [x] Implement `mock` backend for testing (**Completed**)

## Milestone 3: Agent Provisioning & Git Integration
**Goal**: Launch isolated agents with workspace awareness.

- [x] Implement `scion start` (**Completed**)
    - [x] Template selection and home directory provisioning.
    - [x] Environment & Credential Propagation (API keys, OAuth, ADC).
    - [x] Labeling (scion.agent, scion.name, scion.grove).
- [x] Git Worktree Integration (**Completed**)
    - [x] Automated worktree creation in `.scion/agents/<name>/workspace`.
    - [x] Worktree cleanup on agent removal.
- [x] Multi-agent isolation (**Completed**)
    - [x] Distinct identities and file states per agent.

## Milestone 4: Lifecycle & Observability
**Goal**: Visibility and control over running agents.

- [x] Implement `scion list` (**Completed**)
    - [x] Multi-grove filtering and `--all` support.
- [x] Implement `scion stop` & `scion delete` (**Completed**)
    - [x] Separate stop (pause) and delete (cleanup) operations.
    - [x] `stop --rm` convenience flag.
- [x] Implement `scion attach` (**Completed**)
    - [x] Interactive session connection (tmux support).
- [x] Implement `scion logs` (**Completed**)

## Milestone 5: Refinement & Advanced UX (Next)
**Goal**: Improve management efficiency and user experience.

add a template lifecycle sub-command - working with the default or explicit grove.

- [x] Template Management Enhancements (**Completed**)
    - [x] `scion templates new <name>`: Create a new template, cloning the default as a starting point
    - [x] `scion templates list`: List available local and global templates.
    - [x] `scion templates show <name>`: Inspect template configuration.
    - [x] `scion templates delete <name>`: Inspect template configuration.


## Milestone 6: Inter-Agent Coordination (Future)
**Goal**: Enable agents to work together or under a supervisor.

- [ ] Supervisor Role
    - [ ] Specialized template for a "manager" agent that can spawn others.
- [ ] Grove-wide context
    - [ ] Shared memory or context file accessible to all agents in a grove.

## Current Issues & Debugging Tasks

- [ ] **Issue**: [Auth Dialog Appears Despite Valid Credentials](./issues/auth-dialog.md)
- [ ] **Issue**: [Apple Native Container Does Not Support Attach](./issues/apple-container-attach.md) (Mitigated by tmux, but direct attach still pending investigation)
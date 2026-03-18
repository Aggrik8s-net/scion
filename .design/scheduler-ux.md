# Scheduler UX: CLI and Web Interfaces for Timers & Recurring Schedules

## Status
**Design** | March 2026

## Problem

The Hub scheduler infrastructure (see [scheduler.md](hosted/scheduler.md)) is fully implemented with one-shot timers, recurring handler registration, and a `scheduled_events` persistence layer. However, the UX surfaces for managing these capabilities are minimal:

- **CLI**: Only `scion message --in/--at` exposes one-shot scheduling. There is no dedicated command for managing scheduled events (list, inspect, cancel) or for creating non-message timers. There is no CLI surface for recurring user-defined schedules.
- **Web**: The admin scheduler page (`/admin/scheduler`) is read-only and admin-scoped. There is no grove-level view for managing a grove's scheduled events, and no UI for creating or cancelling events. There is no support for user-defined recurring schedules.

Users need to:
1. **Create** one-shot timed events beyond just messages (e.g., agent lifecycle actions).
2. **List, inspect, and cancel** pending scheduled events from both CLI and web.
3. **Define recurring schedules** that automatically dispatch agents or send messages on a cron-like cadence.
4. **View schedule history** to audit what fired, when, and whether it succeeded.

### Goals

1. **`scion schedule` command group** for full CRUD management of one-shot and recurring scheduled resources.
2. **Grove-scoped scheduled events UI** in the web frontend for creating, listing, and cancelling one-shot events.
3. **Recurring schedule support** — a new `schedules` resource backed by a database table, with cron expression evaluation in the scheduler.
4. **Consistent patterns** — follow existing CLI (Cobra + Hub/local modes), API (RESTful grove-scoped), and web (Lit + Shoelace) conventions.

### Non-Goals (This Iteration)

- **Complex cron expressions**: Only standard 5-field cron (`minute hour day-of-month month day-of-week`) is targeted. Extended syntax (seconds, year, `@every`) is deferred.
- **Schedule-triggered agent dispatch**: Recurring schedules that automatically create/start agents. The infrastructure is designed for it, but the first iteration focuses on scheduled messages and the CRUD surfaces. Agent dispatch is Phase 3.
- **Local/solo mode scheduling**: Scheduling requires the Hub. Solo mode has no persistent scheduler.
- **Notification preferences for schedule events**: Covered by the existing notification system; no new notification types are introduced here.

---

## Current State

### What Exists

| Layer | Resource | Capability |
|---|---|---|
| **Store** | `scheduled_events` table | Full CRUD, pagination, filtering by grove/type/status |
| **Scheduler** | `Scheduler.ScheduleEvent()` / `CancelEvent()` | In-memory + DB one-shot timer management |
| **Scheduler** | `RegisterRecurring()` | Code-only recurring handler registration |
| **Hub API** | `POST/GET/DELETE /api/v1/groves/{id}/scheduled-events` | One-shot event CRUD |
| **Hub API** | `GET /api/v1/admin/scheduler` | Admin-only scheduler status |
| **Hub Client** | `ScheduledEventService` | `Create`, `Get`, `List`, `Cancel` |
| **CLI** | `scion message --in/--at` | Schedule a future message delivery |
| **Web** | `admin-scheduler` page | Read-only admin view of scheduler state |

### What's Missing

| Layer | Gap |
|---|---|
| **Store** | No `schedules` table for user-defined recurring schedules |
| **Scheduler** | No cron expression evaluation; recurring handlers are code-only |
| **Hub API** | No API for recurring schedules; no non-admin event listing per grove |
| **Hub Client** | No `ScheduleService` for recurring schedules |
| **CLI** | No `scion schedule` command group |
| **Web** | No grove-level scheduled events view; no create/cancel actions; no recurring schedule management |

---

## Design

### 1. CLI: `scion schedule` Command Group

A new top-level command group following the pattern of `scion grove`, `scion template`, etc.

#### 1.1 Command Structure

```
scion schedule
  scion schedule list              # List scheduled events and recurring schedules
  scion schedule get <id>          # Get details of a specific event/schedule
  scion schedule cancel <id>       # Cancel a pending one-shot event
  scion schedule create            # Create a one-shot scheduled event (interactive)
  scion schedule create-recurring  # Create a recurring schedule
  scion schedule delete <id>       # Delete a recurring schedule
  scion schedule pause <id>        # Pause a recurring schedule
  scion schedule resume <id>       # Resume a paused recurring schedule
  scion schedule history [id]      # View execution history
```

#### 1.2 `scion schedule list`

Lists both one-shot events and recurring schedules for the current grove.

```
$ scion schedule list
SCHEDULED EVENTS (one-shot)
ID              TYPE      STATUS    FIRE AT                    AGENT
a1b2c3d4        message   pending   2026-03-18T15:00:00Z       worker-1
e5f6g7h8        message   fired     2026-03-18T14:30:00Z       worker-2

RECURRING SCHEDULES
ID              NAME              CRON              NEXT RUN                   STATUS
r9s0t1u2        daily-standup     0 9 * * 1-5       2026-03-19T09:00:00Z       active
v3w4x5y6        hourly-check      0 * * * *         2026-03-18T16:00:00Z       paused
```

**Flags:**
- `--type events|recurring|all` — Filter by resource type (default: `all`)
- `--status pending|fired|cancelled|expired|active|paused` — Filter by status
- `--format json` — JSON output
- `--grove <path>` — Override grove

**Implementation notes:**
- Requires Hub mode. Returns an error with guidance if no Hub is configured.
- Calls `ScheduledEvents(groveID).List()` for one-shot events.
- Calls a new `Schedules(groveID).List()` for recurring schedules (Phase 2+).
- Uses `tabwriter` for table output, consistent with `scion list`.

#### 1.3 `scion schedule get <id>`

Shows detailed information about a scheduled event or recurring schedule.

```
$ scion schedule get a1b2c3d4
Scheduled Event: a1b2c3d4
  Type:       message
  Status:     pending
  Fire At:    2026-03-18T15:00:00Z (in 45 minutes)
  Grove:      my-project
  Agent:      worker-1
  Message:    "Time to wrap up your current task"
  Created:    2026-03-18T14:15:00Z by user@example.com
```

```
$ scion schedule get r9s0t1u2
Recurring Schedule: r9s0t1u2
  Name:       daily-standup
  Status:     active
  Cron:       0 9 * * 1-5 (weekdays at 9:00 AM)
  Next Run:   2026-03-19T09:00:00Z
  Last Run:   2026-03-18T09:00:00Z (success)
  Action:     message → "Good morning! Please share your status."
  Target:     all agents
  Created:    2026-03-01T10:00:00Z by admin@example.com
  Run Count:  12 total, 11 success, 1 error
```

#### 1.4 `scion schedule cancel <id>`

Cancels a pending one-shot event. Does not apply to recurring schedules (use `pause` or `delete`).

```
$ scion schedule cancel a1b2c3d4
Scheduled event a1b2c3d4 cancelled.
```

#### 1.5 `scion schedule create`

Creates a one-shot scheduled event. Extends the existing `scion message --in/--at` pattern to support additional event types.

```bash
# Schedule a message (equivalent to scion message --in)
scion schedule create --type message --agent worker-1 --message "Wrap up" --in 30m

# Schedule a message at a specific time
scion schedule create --type message --agent worker-1 --message "Standup" --at "2026-03-19T09:00:00Z"
```

**Flags:**
- `--type <event-type>` — Event type (required). Initially: `message`.
- `--in <duration>` — Duration from now (mutually exclusive with `--at`).
- `--at <timestamp>` — Absolute ISO 8601 time (mutually exclusive with `--in`).
- `--agent <name>` — Target agent name (for message events).
- `--message <text>` — Message body (for message events).
- `--interrupt` — Interrupt the agent (for message events).
- `--format json` — JSON output.

**Note:** `scion message --in/--at` remains as a convenience shorthand. The `schedule create` command provides the general-purpose entry point.

#### 1.6 Recurring Schedule Commands (Phase 2)

```bash
# Create a recurring schedule
scion schedule create-recurring \
  --name "daily-standup" \
  --cron "0 9 * * 1-5" \
  --type message \
  --agent all \
  --message "Good morning! Status update please."

# Pause/resume
scion schedule pause r9s0t1u2
scion schedule resume r9s0t1u2

# Delete
scion schedule delete r9s0t1u2

# View execution history
scion schedule history r9s0t1u2
```

---

### 2. Hub API: Recurring Schedules

#### 2.1 Data Model

A new `schedules` table for user-defined recurring schedules:

```sql
CREATE TABLE schedules (
    id TEXT PRIMARY KEY,
    grove_id TEXT NOT NULL,
    name TEXT NOT NULL,
    cron_expr TEXT NOT NULL,             -- Standard 5-field cron expression
    event_type TEXT NOT NULL,            -- "message" (future: "dispatch_agent")
    payload TEXT NOT NULL DEFAULT '{}',  -- JSON: handler-specific configuration
    status TEXT NOT NULL DEFAULT 'active',  -- active, paused, deleted
    timezone TEXT NOT NULL DEFAULT 'UTC',
    next_run_at TIMESTAMP,              -- Precomputed next fire time
    last_run_at TIMESTAMP,
    last_run_status TEXT,               -- success, error
    last_run_error TEXT,
    run_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (grove_id) REFERENCES groves(id) ON DELETE CASCADE,
    UNIQUE(grove_id, name)
);

CREATE INDEX idx_schedules_grove ON schedules(grove_id);
CREATE INDEX idx_schedules_next_run ON schedules(next_run_at) WHERE status = 'active';
```

#### 2.2 Store Interface

```go
type ScheduleStore interface {
    CreateSchedule(ctx context.Context, schedule *Schedule) error
    GetSchedule(ctx context.Context, id string) (*Schedule, error)
    ListSchedules(ctx context.Context, filter ScheduleFilter, opts ListOptions) (*ListResult[Schedule], error)
    UpdateSchedule(ctx context.Context, schedule *Schedule) error
    UpdateScheduleStatus(ctx context.Context, id string, status string) error
    UpdateScheduleAfterRun(ctx context.Context, id string, ranAt time.Time, nextRunAt time.Time, errMsg string) error
    DeleteSchedule(ctx context.Context, id string) error
}
```

#### 2.3 API Endpoints

```
POST   /api/v1/groves/{groveId}/schedules          # Create recurring schedule
GET    /api/v1/groves/{groveId}/schedules           # List schedules
GET    /api/v1/groves/{groveId}/schedules/{id}      # Get schedule
PATCH  /api/v1/groves/{groveId}/schedules/{id}      # Update schedule (name, cron, payload, status)
DELETE /api/v1/groves/{groveId}/schedules/{id}      # Delete schedule
POST   /api/v1/groves/{groveId}/schedules/{id}/pause   # Pause
POST   /api/v1/groves/{groveId}/schedules/{id}/resume  # Resume
GET    /api/v1/groves/{groveId}/schedules/{id}/history  # Execution history
```

**Create Request:**
```json
{
  "name": "daily-standup",
  "cronExpr": "0 9 * * 1-5",
  "eventType": "message",
  "timezone": "America/Los_Angeles",
  "payload": {
    "agentName": "all",
    "message": "Good morning! Status update please.",
    "interrupt": false
  }
}
```

#### 2.4 Scheduler Integration

The scheduler gains a new recurring handler (`schedule-evaluator`) that runs every 1 minute. On each tick, it queries `schedules` where `status = 'active' AND next_run_at <= NOW()`, executes the action, and updates `next_run_at` using cron evaluation.

```go
scheduler.RegisterRecurring("schedule-evaluator", 1, srv.evaluateSchedulesHandler())
```

The handler:
1. Queries active schedules whose `next_run_at` has passed.
2. For each, executes the appropriate action (e.g., send message).
3. Computes the next run time from the cron expression.
4. Updates the schedule record with `last_run_at`, `next_run_at`, and success/error status.

**Cron library**: Use `github.com/robfig/cron/v3` for expression parsing and next-time computation. This is a well-maintained, widely-used Go cron library that supports standard 5-field expressions.

---

### 3. Web Frontend

#### 3.1 Grove-Level Scheduled Events Section

Add a "Scheduled Events" section to the grove detail page (`grove-detail.ts`), below the Agents section. This follows the pattern of the Workspace Files section.

**Section layout:**
```
┌─────────────────────────────────────────────────┐
│ Scheduled Events                    [+ New Event]│
│ One-shot timed events for this grove.            │
│                                                  │
│ ┌──────────────────────────────────────────────┐ │
│ │ Type    Status   Fire At        Agent   Act. │ │
│ │ message pending  in 45 min      worker-1  ✕  │ │
│ │ message fired    30 min ago     worker-2  -  │ │
│ │ message expired  2 hours ago    worker-3  -  │ │
│ └──────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

**Interactions:**
- **"+ New Event" button** opens a create dialog (`<sl-dialog>`)
- **Cancel action (✕)** on pending events with confirmation
- **Row click** navigates to event detail (or expands inline)
- **Auto-refresh** via SSE or polling (follow existing agent status pattern)

**Create Dialog fields:**
- Event type (select: `message`)
- Target agent (select from grove's agents, or "all")
- Message body (textarea)
- Timing: radio toggle between "In..." (duration input) and "At..." (datetime-local input)
- Interrupt toggle (checkbox)

#### 3.2 Recurring Schedules Page (Phase 2)

A new page at `/groves/{id}/schedules` (or a tab within grove detail) for managing recurring schedules.

**Route:** `/groves/:id/schedules`
**Component:** `scion-page-grove-schedules`

**Layout:**
```
┌─────────────────────────────────────────────────────────┐
│ ← Back to Grove                                         │
│ Recurring Schedules                    [+ New Schedule]  │
│ Automated recurring tasks for this grove.                │
│                                                          │
│ ┌──────────────────────────────────────────────────────┐ │
│ │ Name           Cron          Next Run      Status    │ │
│ │ daily-standup   0 9 * * 1-5  Tomorrow 9am  active    │ │
│ │ hourly-check    0 * * * *    in 32 min     paused    │ │
│ └──────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

**Schedule Detail Dialog / Page:**
- Name, cron expression, timezone
- Human-readable cron description (e.g., "At 09:00 AM, Monday through Friday")
- Preview of next 5 run times
- Action configuration (event type, target, message)
- Execution history table
- Pause/Resume/Delete actions

**Create Schedule Dialog:**
- Name (text input)
- Cron expression (text input with presets dropdown: hourly, daily, weekdays, weekly)
- Timezone (select, default UTC)
- Event type (select)
- Target agent (select)
- Message body (textarea)

#### 3.3 Component Architecture

```
web/src/components/
├── pages/
│   ├── grove-detail.ts           # Modified: add scheduled events section
│   ├── grove-schedules.ts        # New: recurring schedules page
│   └── schedule-detail.ts        # New: schedule detail page (Phase 3)
├── shared/
│   ├── scheduled-event-list.ts   # New: reusable event list with CRUD
│   └── schedule-list.ts          # New: reusable recurring schedule list
```

**`scheduled-event-list` component:**
- Follows the `secret-list.ts` / `env-var-list.ts` pattern
- Props: `groveId: string`, `compact: boolean`
- Internal state: `events`, `loading`, `error`, `dialogOpen`
- Create dialog, cancel action, status badges
- Calls `/api/v1/groves/{groveId}/scheduled-events`

#### 3.4 Admin Scheduler Enhancement

Extend the existing `admin-scheduler.ts` page with:
- **Cancel action** on pending events (admin can cancel any event)
- **Recurring schedules section** showing all schedules across groves (Phase 2)
- **Auto-refresh** button or polling interval

---

## Open Questions

### 1. Should `scion schedule` be a top-level command or nested under `scion grove`?

**Option A (Proposed):** `scion schedule list` — top-level, similar to `scion message`.
**Option B:** `scion grove schedule list` — nested under grove, emphasizing grove-scoping.

The proposed approach (Option A) is more ergonomic for frequent use and consistent with `scion message` which is also grove-scoped but top-level.

### 2. Should recurring schedules support targeting a specific agent vs. agent name pattern?

Agents are ephemeral — a recurring schedule targeting "worker-1" may outlive the agent. Options:
- **By name**: Simple, but the schedule errors if the agent no longer exists.
- **By label/selector**: More resilient but adds complexity.
- **"All agents" broadcast**: Always works but may be noisy.
- **Agent name with graceful skip**: If the named agent doesn't exist, log a warning and skip. This is probably the right initial behavior.

### 3. Should the `scion message --in/--at` shorthand remain, or be deprecated in favor of `scion schedule create --type message`?

**Recommendation:** Keep both. `scion message --in/--at` is convenient for the common case. `scion schedule create` is the general-purpose interface. Document the relationship.

### 4. What timezone handling strategy for recurring schedules?

The `schedules` table includes a `timezone` field. Options:
- **UTC only (initially)**: Simplest. Cron expressions are always evaluated in UTC.
- **Per-schedule timezone**: More user-friendly for "every day at 9am local time" use cases, but adds DST complexity.

**Recommendation:** Start with UTC-only. Add timezone support in a later iteration using Go's `time.LoadLocation`.

### 5. How should schedule execution history be stored?

Options:
- **Reuse `scheduled_events` table**: Each recurring schedule fire creates a `scheduled_event` record with a `schedule_id` FK. Pros: unified event history. Cons: mixes user-created one-shot events with system-generated recurring fires.
- **Separate `schedule_runs` table**: Dedicated table for recurring schedule execution history. Pros: clean separation. Cons: another table.

**Recommendation:** Reuse `scheduled_events` with an optional `schedule_id` column. This keeps all event history in one place and allows the existing event list UI to show recurring schedule fires.

### 6. Should the web UI show scheduled events in the agent detail page?

Showing "pending events targeting this agent" on the agent detail page would be useful but adds API complexity (query by agent). This could be a future enhancement using the existing `scheduled_events` filter by payload content.

### 7. Cron expression validation — how strict?

Should the API reject invalid cron expressions at creation time, or allow any string and fail at evaluation time?

**Recommendation:** Validate at creation time using the cron library's parser. Return a 422 with a clear error message for invalid expressions. Also return a computed `next_run_at` in the creation response as confirmation.

---

## Implementation Phases

### Phase 1: One-Shot Event CLI & Web CRUD

**Goal:** Full lifecycle management of existing one-shot scheduled events from both CLI and web.

**Backend changes:**
- None required — the API already supports all needed operations.

**CLI (`cmd/schedule.go`):**
1. `scion schedule list` — list one-shot events (calls `ScheduledEvents.List`)
2. `scion schedule get <id>` — get event detail (calls `ScheduledEvents.Get`)
3. `scion schedule cancel <id>` — cancel pending event (calls `ScheduledEvents.Cancel`)
4. `scion schedule create` — create one-shot event with `--type`, `--in/--at`, `--agent`, `--message`, `--interrupt` flags (calls `ScheduledEvents.Create`)

**Web (`web/src/components/`):**
5. Create `shared/scheduled-event-list.ts` — reusable component with list, create dialog, cancel action
6. Add scheduled events section to `pages/grove-detail.ts`
7. Add cancel action to `pages/admin-scheduler.ts`

**Tests:**
8. CLI command unit tests (mock hub client)
9. Web component tests (if test infrastructure exists)

**Files affected:**
| File | Change |
|---|---|
| `cmd/schedule.go` | **New** — `scion schedule` command group |
| `cmd/root.go` | Add `scheduleCmd` to root |
| `web/src/components/shared/scheduled-event-list.ts` | **New** — reusable event list |
| `web/src/components/pages/grove-detail.ts` | Add scheduled events section |
| `web/src/components/pages/admin-scheduler.ts` | Add cancel action on pending events |

### Phase 2: Recurring Schedule Infrastructure

**Goal:** Database-backed recurring schedules with cron evaluation in the scheduler.

**Backend:**
1. Add `Schedule` model to `pkg/store/models.go`
2. Add `ScheduleStore` interface to `pkg/store/store.go`
3. Add `schedules` table migration to `pkg/store/sqlite/sqlite.go`
4. Implement `ScheduleStore` in SQLite
5. Add optional `schedule_id` column to `scheduled_events` table
6. Add `schedule-evaluator` recurring handler to scheduler
7. Integrate `robfig/cron/v3` for cron parsing
8. Add Hub API endpoints for schedule CRUD (`pkg/hub/handlers_schedules.go`)
9. Add `ScheduleService` to hub client (`pkg/hubclient/schedules.go`)

**CLI:**
10. `scion schedule create-recurring` command
11. `scion schedule pause/resume <id>` commands
12. `scion schedule delete <id>` command
13. `scion schedule history <id>` command
14. Update `scion schedule list` to include recurring schedules

**Tests:**
15. Store implementation tests (ScheduleStore CRUD)
16. Scheduler evaluator handler tests
17. API handler tests
18. CLI command tests

**Files affected:**
| File | Change |
|---|---|
| `pkg/store/models.go` | Add `Schedule` model, `ScheduleFilter` |
| `pkg/store/store.go` | Add `ScheduleStore` interface |
| `pkg/store/sqlite/sqlite.go` | New migration, `ScheduleStore` implementation |
| `pkg/hub/scheduler.go` | Add `evaluateSchedulesHandler` |
| `pkg/hub/handlers_schedules.go` | **New** — schedule API handlers |
| `pkg/hub/server.go` | Register new routes and scheduler handler |
| `pkg/hubclient/schedules.go` | **New** — `ScheduleService` client |
| `pkg/hubclient/client.go` | Add `Schedules(groveID)` method |
| `cmd/schedule.go` | Add recurring schedule commands |
| `go.mod` | Add `robfig/cron/v3` dependency |

### Phase 3: Recurring Schedules Web UI & Agent Dispatch

**Goal:** Web UI for recurring schedule management and scheduled agent dispatch.

**Web:**
1. Create `pages/grove-schedules.ts` — recurring schedules page
2. Create `shared/schedule-list.ts` — reusable schedule list component
3. Add route for `/groves/:id/schedules` in `client/main.ts`
4. Add navigation link in grove detail page
5. Add recurring schedules section to `admin-scheduler.ts`

**Backend (agent dispatch):**
6. Add `dispatch_agent` event type handler
7. Define payload schema for agent dispatch (template, prompt, branch name)
8. Integrate with `AgentDispatcher` for automatic agent creation

**CLI:**
9. `scion schedule create` with `--type dispatch_agent` support

**Files affected:**
| File | Change |
|---|---|
| `web/src/components/pages/grove-schedules.ts` | **New** — schedules page |
| `web/src/components/shared/schedule-list.ts` | **New** — schedule list component |
| `web/src/client/main.ts` | Add route |
| `web/src/components/pages/grove-detail.ts` | Add link to schedules page |
| `web/src/components/pages/admin-scheduler.ts` | Add recurring schedules section |
| `pkg/hub/scheduler.go` | Add `dispatch_agent` event handler |

---

## Related Documents

- [Scheduler Design](hosted/scheduler.md) — Core scheduler infrastructure (implemented)
- [Hub API](hosted/hub-api.md) — API conventions and patterns
- [Web Frontend Design](hosted/web-frontend-design.md) — Frontend architecture
- [Frontend Milestones](hosted/frontend-milestones.md) — Web development roadmap
- [Notifications](hosted/notifications.md) — Event notification system

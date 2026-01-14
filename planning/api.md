# Machinator v3: Control API

## Overview

The orchestrator exposes an HTTP API for control operations. State changes are persisted immediately.

---

## Orchestrator Control

### Pause/Resume Assignment

```
POST /api/pause-assignment
POST /api/resume-assignment
```

Stop/resume the assigner from claiming new tasks. Running agents continue.

### Pause/Resume Launches

```
POST /api/pause-launches
POST /api/resume-launches
```

Stop/resume starting new gemini processes. Assigned agents wait until resumed.

---

## Agent Management

### List Agents

```
GET /api/agents
Response: [
  { "id": 1, "state": "assigned", "task_id": "machinator-8rn", ... },
  { "id": 2, "state": "ready", ... }
]
```

### Add Agent

```
POST /api/agents
Response: { "id": 3, "state": "pending" }
```

Creates a new agent slot. SetupWatcher handles initialization.

### Remove Agent

```
DELETE /api/agents/:id?finish=true
DELETE /api/agents/:id?finish=false
```

- `finish=true`: Mark for removal, let current task complete
- `finish=false`: Kill immediately, then remove slot

### Kill Agent

```
POST /api/agents/:id/kill
```

Kill the running gemini process. Agent returns to ready state and can be reassigned.

### Kill and Bar Task

```
POST /api/agents/:id/kill-and-bar
```

Kill the agent AND add the current task to the barred list (never retry).

---

## Account Management

### List Accounts

```
GET /api/accounts
Response: [
  { "name": "primary", "quota": { "gemini-3-flash-preview": 0.85, ... } },
  { "name": "secondary", ... }
]
```

### Add Account

```
POST /api/accounts/:name
```

Creates account directory at `$MACHINATOR_DIR/accounts/:name/`.
User must then authenticate via gemini CLI.

### Remove Account

```
DELETE /api/accounts/:name
Response: 200 OK or 409 Conflict (if in use)
```

---

## Configuration

### Get Config

```
GET /api/config
Response: {
  "agents_count": 3,
  "idle_timeout": "10m",
  "max_runtime": "30m",
  "assignment_paused": false,
  "launches_paused": false
}
```

### Update Config

```
PATCH /api/config
Body: { "idle_timeout": "15m" }
```

Only updates provided fields. Some changes require all agents idle.

---

## Project Config

### Get Project

```
GET /api/project
Response: {
  "repo": "git@github.com:user/repo",
  "branch": "main",
  "simple_model_name": "gemini-3-flash-preview",
  "complex_model_name": "gemini-3-pro-preview"
}
```

### Update Project (requires all agents idle)

```
PUT /api/project
Body: { "branch": "develop" }
Response: 200 OK or 409 Conflict (agents still running)
```

---

## Quota

### Get Quota

```
GET /api/quota
Response: {
  "updated_at": "2026-01-13T23:00:00Z",
  "accounts": [
    { "name": "primary", "models": { "gemini-3-flash-preview": 0.85, ... } },
    ...
  ]
}
```

### Refresh Quota

```
POST /api/quota/refresh
```

Force immediate quota refresh (normally every 60s).

---

## Tasks

### List Tasks

```
GET /api/tasks
Response: [
  { "id": "machinator-8rn", "status": "open", "is_complex": true, ... },
  ...
]
```

### Bar Task

```
POST /api/tasks/:id/bar
```

Add task to barred list (never assign).

### Unbar Task

```
DELETE /api/tasks/:id/bar
```

Remove task from barred list.

---

## Events (WebSocket)

### Connect

```
WS /api/events
```

### Subscribe

```
Client → Server: { "subscribe": ["agents", "quota", "logs"] }
```

### Event Types

```
Server → Client: { "type": "agent_state", "agent": 1, "state": "assigned", ... }
Server → Client: { "type": "quota_update", "accounts": [...] }
Server → Client: { "type": "log", "agent": 1, "line": "..." }
```

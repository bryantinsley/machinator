# Local E2E Testing

This directory contains E2E tests for the Machinator orchestrator.

## Running Locally

To run these tests locally without Bazel (using `go test`), you must:

1. Build `machinator` and `dummy-gemini`.
2. Set `E2E_USE_LOCAL_BINS=1`.

### Prerequisites

```bash
# Build machinator (if not already built)
# From project root
go build -o machinator_bin/machinator ./orchestrator/cmd/machinator

# Build dummy-gemini
go build -o dummy-gemini ./orchestrator/tools/dummy-gemini
```

### Running the Test

```bash
export E2E_USE_LOCAL_BINS=1
go test ./orchestrator/e2e/...
```

The test `TestE2E_Happy` verifies that the orchestrator can:
1. Start up pointing to a fixture repo.
2. Detect a task (`task-1`).
3. Execute the task using `dummy-gemini` (in auto-close mode).
4. Verify the task becomes `closed`.

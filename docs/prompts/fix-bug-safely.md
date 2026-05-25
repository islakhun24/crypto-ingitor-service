# Prompt: Fix a Bug Safely

Use this prompt template when asking an AI to fix a bug without unnecessary refactoring.

---

## Prompt Template

```
Fix the following bug: {BUG_DESCRIPTION}

### Reproduction Steps
{REPRO_STEPS}

### Expected Behavior
{EXPECTED_BEHAVIOR}

### Actual Behavior
{ACTUAL_BEHAVIOR}

### Constraints
1. Reproduce the issue first. Run the relevant tests or provide a minimal reproduction.
2. Identify the minimal file set. Only inspect files directly related to the bug.
3. Modify the smallest scope possible. Change one function or one file if possible.
4. Add a regression test that fails before the fix and passes after.
5. Do NOT refactor unrelated code.
6. Do NOT rename variables, move functions, or change interfaces unless required to fix the bug.
7. Do NOT modify existing tests except to add the new regression test.
8. Do NOT change behavior of other exchanges, endpoints, or data types.

### Suggested Files to Inspect
{FILES_TO_INSPECT}

### Acceptance Criteria
- [ ] Regression test added and passes
- [ ] `go test ./...` passes
- [ ] No unrelated files modified
- [ ] Change is the minimal fix for the described bug
```

## Common Bug Presets

### Normalizer Bug
```
BUG_DESCRIPTION: Exchange {EXCHANGE} returns wrong/missing values for {DATA_TYPE}.
REPRO_STEPS: Run TestNormalize{DataType}Sample with the attached JSON payload.
FILES_TO_INSPECT:
- internal/exchanges/{EXCHANGE}/adapter.go
- internal/exchanges/{EXCHANGE}/adapter_test.go
- internal/hardening/validation.go (if data is being filtered)
```

### Endpoint Not Found
```
BUG_DESCRIPTION: Collector fails with "endpoint unavailable" for {EXCHANGE}/{DATA_TYPE}.
REPRO_STEPS: Run collector after fresh migration.
FILES_TO_INSPECT:
- migrations/000007_seed_exchange_api_endpoints.sql
- internal/endpoints/repository.go
- internal/exchanges/all/registry.go
```

### Data Quality Spike
```
BUG_DESCRIPTION: All/most rows from {EXCHANGE} are dropped as quality issues.
REPRO_STEPS: Check data_quality_issues table for recent rows.
FILES_TO_INSPECT:
- internal/hardening/validation.go
- internal/normalizers/validation.go
- internal/exchanges/{EXCHANGE}/adapter.go
```

### Job Stuck in Running
```
BUG_DESCRIPTION: Jobs remain in 'running' status after collector crash.
REPRO_STEPS: Kill collector mid-run, restart, observe jobs still running.
FILES_TO_INSPECT:
- internal/scheduler/recovery.go
- cmd/collector-service/main.go
```

### Retention Not Running
```
BUG_DESCRIPTION: Retention service reports 0 rows deleted.
REPRO_STEPS: Run retention-service, check data_cleanup_runs.
FILES_TO_INSPECT:
- data_retention_policies table (enabled, dry_run)
- internal/retention/specs.go (MinRetentionDays)
- internal/retention/engine.go
```

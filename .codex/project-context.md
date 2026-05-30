# Project Context

UTILS is a local developer utility hub that runs as a terminal app under standard user permissions.

## Non-Negotiables

- Keep Bubble Tea views MVU-compliant: no state mutation in `View()`, no side effects outside `tea.Cmd`.
- Use one `ViewState` enum per complex view.
- Register top-level tools through `AppFeature` and inject them into `ui.NewRouter()`.
- Keep core business logic independent from terminal rendering.
- Route core OS interactions through injected interfaces when practical.
- Destructive actions default to dry-run and require explicit confirmation.
- Never require admin/root elevation.
- Move hardcoded OS strings, magic numbers, layout dimensions, colors, file modes, command names/args, target labels, and shared copy into package-local `constants.go` files.
- Infrastructure workflows prefer safe stop/start operations and must not use routine destructive cleanup such as `terraform destroy`.
- Deployment workflows are container-first: build Docker images and push to Docker Hub, not legacy JAR execution flows.

## Runtime

- Language: Go
- Module: `utils`
- Go version: `1.26`
- TUI stack: Bubble Tea, Bubbles, Lip Gloss
- Concurrency helper: `golang.org/x/sync/errgroup`

## Main Flow

`cmd/tui/main.go` starts Bubble Tea with:

```go
ui.NewRouter(ui.DefaultFeatures()...)
```

The router lists registered features and activates the selected model. Current features:

- System & Credential Cleaner
- Infrastructure & Cloud Manager placeholder
- Application Deployment Helper placeholder

## Cleaner Core

`internal/core/cleaner` owns the cleanup engine.

- `Options` controls execute/dry-run and safety gates.
- `Report` captures activity and counters.
- `Cleaner` accepts `FileSystem` and `CommandRunner`.
- `Run(ctx, opts)` is the production convenience wrapper.
- `targets.go` contains filesystem targets.
- `cleaner.go` contains execution, safety checks, logging, process inspection, and Credential Manager logic.

Invariants:

- Logs are written inside the user profile.
- Cleanup refuses paths outside the user profile.
- Symlinks are resolved and checked before delete.
- Dry-run reports what would be deleted.
- Execute mode deletes with `RemoveAll` only after safety checks.
- Target categories run in stable high-level order; paths inside a category run concurrently.

## Cleaner UI

`internal/ui/views/cleaner.go` owns the Cleaner Bubble Tea model.

- `ViewState` drives screens.
- `CleanerModel` stores state, options, report, errors, notice text, and cancellation.
- `startRun` creates a cancellable timeout context.
- `runCleaner` returns a Bubble Tea command that calls the core cleaner.
- While running, `esc` and `ctrl+c` cancel the context.

UI options:

- Include SSH keys
- Include browser profiles
- Clean Windows Credential Manager allowlist

## Shared UI

`internal/ui/common` contains global key bindings, checkbox list wrapper, layout/wrapping helpers, Lip Gloss styles, shared dimensions, and colors. Prefer these helpers before adding view-local layout code.

## Tests

Black-box tests:

- `tests/core/cleaner/cleaner_test.go`
- `tests/ui/router/router_test.go`
- `tests/ui/views/cleaner_test.go`

Internal tests:

- `internal/ui/views/cleaner_internal_test.go`

Principles:

- Do not use real credentials or real user profile state.
- Use `t.TempDir()` and `t.Setenv()`.
- Assert state changes by sending Bubble Tea messages.
- Keep tests fast and deterministic.

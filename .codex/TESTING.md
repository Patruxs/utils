# Testing Guide

Tests live under top-level `tests/`, grouped by feature area. Add new folders that mirror feature boundaries as the app grows.

## Layout

```text
tests/
  core/
    cleaner/
      cleaner_test.go         # business logic and filesystem safety
      testdata/               # static cleaner fixtures
  ui/
    router/
      router_test.go          # route selection, navigation, quit/back keys
    views/
      cleaner_test.go         # CleanerModel state and rendering behavior
      testdata/               # view snapshots/fixtures
```

Examples for future features:

```text
tests/core/deployment/deployment_test.go
tests/core/infra/infra_test.go
tests/ui/views/deployment_test.go
```

If a feature grows large, split by behavior: `options_test.go`, `run_test.go`, `render_test.go`.

## Test Types

- Black-box tests in `tests/` import app packages and verify exported behavior.
- Unit tests cover pure logic, parsers, validators, option handling, and safety checks.
- Tiny unexported helpers should usually be tested through exported behavior first.
- Model tests drive Bubble Tea models with `tea.Msg` values instead of terminal automation.
- Integration tests use `t.TempDir()` and explicit fake inputs.
- End-to-end/manual tests belong in a separate document or script only when real terminal or external command behavior is necessary.

Never touch real user profile paths, credentials, shell history, browser profiles, system credential stores, cloud credentials, admin permissions, or browser state in normal tests.

## Naming

Use behavior-focused names:

```go
func TestCleanerDryRunKeepsExistingFile(t *testing.T)
func TestRouterOpensCleanerAndReturnsToMenu(t *testing.T)
func TestDeploymentBuildRejectsMissingImageName(t *testing.T)
```

Use subtests for many cases:

```go
func TestFeatureValidation(t *testing.T) {
	tests := []struct {
		name string
		// inputs and expected outputs
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// assertions
		})
	}
}
```

## Commands

Run all tests:

```powershell
go test ./...
```

If Windows blocks the default Go build cache:

```powershell
$env:GOCACHE='C:\Data\UTILS\.gocache'; go test ./...
```

Remove the temporary cache after the run if it was created.

## Future Features

- Add tests in the same change as the feature.
- Keep destructive actions behind temp dirs, mocks, or dry-run assertions.
- Test business logic before TUI model wiring.
- Use versioned `testdata/` fixtures.
- Prefer small focused tests over broad scenario tests with unclear failures.

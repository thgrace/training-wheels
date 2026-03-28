# Contributing

Thanks for contributing to Training Wheels.

## Before You Start

- Use GitHub issues for bug reports, feature requests, and design discussion.
- For security issues, follow [SECURITY.md](SECURITY.md) instead of opening a
  public issue.
- Keep pull requests focused. Small, reviewable changes move faster.

## Prerequisites

- Go 1.26 or newer
- Docker for smoke and policy end-to-end tests
- `golangci-lint` if you want to run lint locally

## Development Setup

Clone the repo and use the Makefile targets for day-to-day work:

```sh
make build
make test
make vet
```

If you have `golangci-lint` installed locally:

```sh
make lint
```

If you want the closest local approximation of CI:

```sh
make ci
```

## Tests

- Add or update tests for behavior changes.
- Prefer targeted unit tests near the affected package.
- Run fast local checks with `make test` and `make vet`.
- Use `make lint` when touching CLI surface, config loading, or pack/rule code.
- Use `make bench` for performance-sensitive changes. You can scope benchmarks
  with `BENCH=<pattern>` and increase iterations with `COUNT=<n>`.
- `make smoke` runs the Docker-backed smoke suite. Do not run the smoke scripts
  directly on the host.
- `make policy-test` runs Docker-backed policy end-to-end coverage.
- `make test-all` runs both Docker suites.
- `make ci` runs the main contributor check set: unit tests, vet, lint, and the
  Docker-backed suites.

## Rule And Parser Changes

- Training Wheels now matches commands structurally using the parsed shell AST.
  If you change parsing, normalization, command enrichment, or structural
  matching, add regression coverage for the exact command shape that changed.
- If you change rule behavior, cover both the CLI workflow (`tw rule ...`) and
  the runtime evaluation path.
- If you touch overrides, keep in mind that `tw override` is session/time scoped
  and persistent policy belongs in `tw rule`.
- Changes that affect shell-specific handling should include coverage for the
  relevant shell mode.

## Pull Requests

- Explain the problem and the approach in the PR description.
- Reference the related issue when one exists.
- Update documentation when user-facing behavior changes.
- Include the verification steps you ran.
- Avoid unrelated cleanup in the same PR.

## Coding Expectations

- Keep behavior explicit and predictable.
- Preserve the fail-open safety model unless the change is intentionally
  revisiting that design.
- Prefer structural command matching over ad hoc string heuristics when adding
  new rule behavior.
- Prefer clear tests over clever implementations.

## Review Process

Maintainers review pull requests for correctness, safety impact, test coverage,
and documentation impact. Feedback may include requests for smaller diffs or
additional regression tests.

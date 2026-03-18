# Contributing

Thanks for contributing to Training Wheels.

## Before You Start

- Use GitHub issues for bug reports, feature requests, and design discussion.
- For security issues, follow [SECURITY.md](SECURITY.md) instead of opening a
  public issue.
- Keep pull requests focused. Small, reviewable changes move faster.

## Development Setup

```sh
go test ./...
go vet ./...
```

If you have `golangci-lint` installed locally, run:

```sh
golangci-lint run ./...
```

Build the CLI with:

```sh
make build
```

## Tests

- Add or update tests for behavior changes.
- Prefer targeted unit tests near the affected package.
- Keep fuzz targets and regression coverage in mind for parser or sanitizer
  changes.
- `make smoke` runs containerized smoke tests and should only be used in a
  Docker-capable environment.

## Pull Requests

- Explain the problem and the approach in the PR description.
- Reference the related issue when one exists.
- Update documentation when user-facing behavior changes.
- Avoid unrelated cleanup in the same PR.

## Coding Expectations

- Keep behavior explicit and predictable.
- Preserve the fail-open safety model unless the change is intentionally
  revisiting that design.
- Prefer clear tests over clever implementations.

## Review Process

Maintainers review pull requests for correctness, safety impact, test coverage,
and documentation impact. Feedback may include requests for smaller diffs or
additional regression tests.

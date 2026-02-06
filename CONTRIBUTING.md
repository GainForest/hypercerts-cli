# Contributing to Hyper

Thank you for your interest in contributing to the Hypercerts CLI! This document provides guidelines and instructions for contributing.

## Getting Started

### Prerequisites

- Go 1.25 or later
- Git

### Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/hypercerts-cli
   cd hypercerts-cli
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build and test:
   ```bash
   make build
   make test
   ```

## Development Workflow

### Branch Naming

- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring

### Making Changes

1. Create a new branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes following our code style (see below)

3. Run tests and linting:
   ```bash
   make test
   make lint
   make fmt
   ```

4. Commit your changes with a clear message:
   ```bash
   git commit -m "Add feature: description of changes"
   ```

5. Push to your fork and open a Pull Request

## Code Style

### Imports

Three groups separated by blank lines: stdlib, third-party, local.

```go
import (
    "context"
    "fmt"

    "github.com/urfave/cli/v3"

    "github.com/GainForest/hypercerts-cli/internal/atproto"
)
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | `lowercase_underscores.go` | `activity.go`, `util_test.go` |
| Exported types/funcs | `PascalCase` | `CreateRecord`, `AuthSession` |
| Unexported funcs | `camelCase` | `runActivityCreate`, `requireAuth` |
| CLI actions | `run` + CommandName | `runAccountLogin`, `runActivityEdit` |
| Constants | `PascalCase` | `CollectionActivity` |
| Errors | `Err` prefix | `ErrNoAuthSession` |

### CLI Output

Always write to `cmd.Root().Writer` for testability:

```go
w := cmd.Root().Writer
fmt.Fprintf(w, "Created record: %s\n", uri)
```

### Error Handling

```go
// Wrap errors with context
return fmt.Errorf("failed to create record: %w", err)

// Use sentinel errors at package level
var ErrNoAuthSession = errors.New("no auth session found")
```

## Adding a New Record Type

1. **Add NSID constant** to `internal/atproto/collections.go`:
   ```go
   CollectionNewType = "org.hypercerts.claim.newType"
   ```

2. **Create command file** `cmd/newtype.go` with:
   - `type newTypeOption struct { ... }` for menu display
   - `fetchNewTypes()` to list records
   - `selectNewType()` for interactive selection (if needed)
   - `runNewTypeCreate/Edit/Delete/List()` CLI actions

3. **Add command definition** to `cmd/root.go`:
   - Add `cmdNewType` variable
   - Wire into `BuildApp()` Commands list

4. **Add tests** to `cmd/newtype_test.go`

5. **Update documentation**:
   - Add section to README.md
   - Update AGENTS.md command tree

## Testing

### Running Tests

```bash
# All tests
make test

# With race detector
make test-race

# Single test
go test -v -run TestFunctionName ./cmd/...

# Tests in a package
go test -v ./internal/menu/...
```

### Writing Tests

- Use table-driven tests for validation functions
- Test both interactive and non-interactive code paths
- Use `bytes.Buffer` to capture CLI output
- Use `t.TempDir()` for file system tests

Example:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"valid_input", "test", "expected"},
        {"empty_input", "", ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := myFunction(tt.input)
            if got != tt.want {
                t.Errorf("myFunction(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

## Pull Request Guidelines

### Before Submitting

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation is updated if needed
- [ ] Commit messages are clear and descriptive

### PR Description

Include:
- What the change does
- Why the change is needed
- Any breaking changes
- Related issues (if any)

### Review Process

1. PRs require at least one approval
2. All CI checks must pass
3. Address review feedback promptly
4. Squash commits before merging if needed

## Reporting Issues

When reporting bugs, please include:

- Go version (`go version`)
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Error messages (if any)

## Questions?

Feel free to open an issue for questions or discussions about the project.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

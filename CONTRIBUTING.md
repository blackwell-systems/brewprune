# Contributing to brewprune

Thank you for your interest in contributing to brewprune! This document provides guidelines and information for contributors.

## Development Setup

### Prerequisites

- **Go 1.21 or later** - brewprune is written in Go
- **Homebrew** - Required for testing (macOS only)
- **golangci-lint** - For code linting (optional but recommended)

Install golangci-lint:
```bash
brew install golangci-lint
```

### Building from Source

1. Clone the repository:
```bash
git clone https://github.com/blackwell-systems/brewprune.git
cd brewprune
```

2. Build the binary:
```bash
go build ./cmd/brewprune
```

3. Run the binary:
```bash
./brewprune --help
```

### Project Structure

```
brewprune/
├── cmd/brewprune/          # Main entry point
├── internal/
│   ├── analyzer/           # Scoring and recommendation logic
│   ├── app/                # CLI command implementations
│   ├── brew/               # Homebrew integration
│   ├── daemon/             # Watch daemon implementation
│   ├── monitor/            # FSEvents monitoring
│   ├── output/             # Terminal output and formatting
│   ├── scanner/            # Package scanning and indexing
│   ├── snapshots/          # Snapshot creation and restoration
│   └── store/              # SQLite database layer
└── testdata/               # Test fixtures
```

## Running Tests

### Run all tests
```bash
go test ./...
```

### Run tests with race detector
```bash
go test -race ./...
```

### Run tests with coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View coverage report
```

### Run specific package tests
```bash
go test ./internal/analyzer -v
```

### Run specific test
```bash
go test ./internal/analyzer -run TestComputeScore_NeverUsedLeafPackage -v
```

## Testing New User Experience

When making UX changes, test the first-time user experience:

```bash
# Simulate new user
rm -rf ~/.brewprune
brewprune quickstart

# Or use doctor for diagnostics
brewprune doctor
```

## Code Style

### Go Conventions

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting (automatically enforced)
- Use `golangci-lint` for linting before committing
- Write clear, self-documenting code with appropriate comments

### Running Linters

```bash
golangci-lint run
```

### Code Organization

- Keep packages focused and single-purpose
- Avoid circular dependencies between packages
- Use interfaces to decouple components
- Place test files alongside implementation files

### Naming

- Use descriptive variable names (avoid single letters except in short scopes)
- Use camelCase for local variables
- Use PascalCase for exported functions/types
- Use snake_case for SQL table/column names

## Writing Tests

### Test Coverage

- Write tests for new features and bug fixes
- Aim for high coverage but focus on meaningful tests
- Use table-driven tests for multiple scenarios
- Mock external dependencies (filesystem, Homebrew commands)

### Test Patterns

Example table-driven test:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "foo",
            expected: "FOO",
            wantErr:  false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

## Submitting Pull Requests

### Before You Start

- Check existing issues to avoid duplicate work
- For major changes, open an issue first to discuss the approach
- Fork the repository and create a feature branch

### PR Process

1. **Fork and branch:**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes:**
   - Write clean, tested code
   - Add tests for new functionality
   - Update documentation if needed

3. **Test thoroughly:**
   ```bash
   go test ./...
   go test -race ./...
   golangci-lint run
   ```

4. **Commit with clear messages:**
   ```bash
   git commit -m "feat: add package size filtering"
   ```

   Use conventional commit prefixes:
   - `feat:` - New feature
   - `fix:` - Bug fix
   - `docs:` - Documentation changes
   - `test:` - Test additions or fixes
   - `refactor:` - Code refactoring
   - `chore:` - Maintenance tasks

5. **Push and create PR:**
   ```bash
   git push origin feature/my-feature
   ```

   Then open a PR on GitHub with:
   - Clear title and description
   - Reference to related issues
   - Description of changes and motivation
   - Any breaking changes noted

### PR Guidelines

- Keep PRs focused on a single feature or fix
- Write clear commit messages
- Ensure all tests pass
- Update documentation for user-facing changes
- Add changelog entry for notable changes
- Respond to review feedback promptly

## Development Workflow

### Local Testing with Real Homebrew Packages

To test with real packages:

1. Build the binary:
   ```bash
   go build -o brewprune ./cmd/brewprune
   ```

2. Use a test database:
   ```bash
   export BREWPRUNE_DB_PATH=/tmp/brewprune-test.db
   ./brewprune scan
   ./brewprune unused
   ```

3. Clean up:
   ```bash
   rm /tmp/brewprune-test.db
   ```

### Database Schema Changes

If you modify the database schema:

1. Update `internal/store/schema.go`
2. Consider adding migration logic if needed
3. Test with both fresh databases and existing ones
4. Document schema changes in commit message

### Adding New Commands

To add a new CLI command:

1. Create command file in `internal/app/`
2. Define cobra command structure
3. Add command to root command in `internal/app/root.go`
4. Write tests in `internal/app/`
5. Update README.md with new command documentation

## Documentation

### Code Documentation

- Add package-level documentation at the top of each package
- Document all exported functions and types
- Use godoc conventions for formatting
- Include examples for complex functionality

### User Documentation

Update these files when making user-facing changes:

- **README.md** - Main documentation, examples, features
- **CHANGELOG.md** - Notable changes for each release
- **Command help text** - Update command descriptions in code

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Ask questions in issue comments or discussions
- Be respectful and constructive in all interactions

## Code of Conduct

- Be respectful and professional
- Welcome newcomers and help them get started
- Focus on constructive feedback
- Keep discussions on-topic and productive

## License

By contributing to brewprune, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to brewprune!

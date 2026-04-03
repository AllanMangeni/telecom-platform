# Contributing to TaaS Platform

Thank you for your interest in contributing to the TaaS Platform! This document provides guidelines for contributing to the project.

## Getting Started

1. **Fork the repository**
2. **Clone your fork:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/telecom-platform.git
   cd telecom-platform
   ```
3. **Set up development environment:**
   ```bash
   ./scripts/dev-setup.sh
   make all
   ```

## Development Workflow

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes:**
   - Write clear, documented code
   - Follow language-specific style guides
   - Add tests for new functionality

3. **Run tests:**
   ```bash
   make test
   ```

4. **Commit your changes:**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

5. **Push to your fork:**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**

## Code Style

### Go
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing

### Rust
- Follow [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)
- Use `cargo fmt` for formatting
- Run `cargo clippy` before committing

### TypeScript
- Follow [Airbnb Style Guide](https://github.com/airbnb/javascript)
- Use Prettier for formatting
- Use ESLint for linting

## Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(api): add eSIM deletion endpoint
fix(charging): correct credit deduction logic
docs: update deployment guide
```

## Testing

- Write unit tests for all new code
- Ensure existing tests pass
- Add integration tests for API endpoints
- Test edge cases and error conditions

## Pull Request Process

1. Update documentation if needed
2. Add tests for your changes
3. Ensure all tests pass
4. Update CHANGELOG.md
5. Request review from maintainers
6. Address review feedback
7. Wait for approval and merge

## Code Review

We review all contributions. Expect:
- Feedback on code quality
- Suggestions for improvements
- Questions about implementation
- Requests for tests or documentation

## Reporting Issues

- Use GitHub Issues
- Provide clear description
- Include reproduction steps
- Add relevant logs or screenshots
- Specify environment details

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

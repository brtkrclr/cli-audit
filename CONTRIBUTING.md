# Contributing to cli-audit

First off, thank you for considering contributing to `cli-audit`! It's people like you that make the open-source community such an amazing place.

## How Can I Contribute?

### Reporting Bugs
- Check the [Issues](https://github.com/yourusername/cli-audit/issues) to see if the bug has already been reported.
- If not, open a new issue. Clearly describe the problem, including steps to reproduce it and your environment (OS, package manager versions).

### Suggesting Enhancements
- Open a new issue and describe the feature you'd like to see, why it's useful, and how it should work.

### Pull Requests
1. Fork the repository.
2. Create a new branch (`git checkout -b feature/amazing-feature`).
3. Make your changes.
4. Ensure your code follows Go standards (`go fmt ./...`).
5. Commit your changes (`git commit -m 'Add some amazing feature'`).
6. Push to the branch (`git push origin feature/amazing-feature`).
7. Open a Pull Request.

## Development Setup

1. **Clone the repo**:
   ```bash
   git clone https://github.com/yourusername/cli-audit.git
   cd cli-audit
   ```

2. **Run locally**:
   ```bash
   go run main.go
   ```

3. **Build**:
   ```bash
   go build -o cli-audit
   ```

## Design Principles

- **Performance First**: Scans should be instantaneous. Avoid calling external CLI commands (like `brew list`) if you can read the filesystem directly.
- **Privacy**: No data leaves the user's machine. History parsing is done locally.
- **Cross-Platform**: Support for macOS (Homebrew) and Linux (APT/NPM) is a priority.

## Coding Style

- Use `go fmt` for formatting.
- Write descriptive variable and function names.
- Keep the TUI logic separated from the auditing logic where possible.

Thank you for your help!

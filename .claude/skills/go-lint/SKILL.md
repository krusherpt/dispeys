---
name: go-lint
description: Run golangci-lint with automatic fixes on Go code. Use this skill whenever the user asks to lint Go code, fix lint issues, run linters, or check code quality in Go projects. Also use when the user mentions static analysis, code cleanup, or wants to ensure Go code follows best practices. This skill should be triggered even if the user just says "lint this" or "fix linter errors" in a Go context.
---

## Go Lint Skill

When the user asks to lint Go code or check code quality:

1. **Check for golangci-lint config**
   - Look for `.golangci.yml` or `.golangci.yaml` in the project root
   - If none exists, create one with sensible defaults for typical Go projects:
     - Enable: `errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`
     - Enable auto-fix capable linters: `govet`, `staticcheck`, `dupword`, `copyloopvar`, `errname`, `errorlint`, `canonicalheader`
     - Set `run.timeout: 5m`
     - Enable `goimports` formatter with local prefixes for the project module
     - Add exclusions for common false positives (test files, generated code)

2. **Run the lint**
   ```bash
   golangci-lint run --fix ./...
   ```
   - This runs all enabled linters and automatically fixes what it can
   - Use `--fix` to apply automatic corrections (formatting, simple fixes)
   - Use `./...` to cover all packages in the project

3. **Show remaining issues**
   - Run `golangci-lint run ./...` (without `--fix`) to show issues that can't be auto-fixed
   - Summarize the remaining findings grouped by linter

4. **If no golangci-lint is installed**
   - Check `command -v golangci-lint`
   - If missing, suggest: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

## When NOT to use

- The user explicitly wants to review lint output without applying fixes (use `golangci-lint run` without `--fix`)
- The project uses a different linter (e.g., golint, revive)
- The Go files are generated or vendored third-party code

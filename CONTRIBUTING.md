# Contributing to EICL

Thanks for your interest in contributing to EICL — a system profiler-benchmarker focused on transparency, speed, and useful metrics.

This project is still in early development, so expect a lot of churn. That said, we’d love your contributions — especially if you follow the rules.

---

## Rules of Engagement

### Branching

- All work goes through **feature branches** via PRs into `staging-main`
- Never merge `staging-main` into your feature branch — use `git rebase`
- Keep commits atomic and clean. Squash if necessary.

### Commits

- All commits **must be signed** (`git commit -S`)
- No emojis in commit messages or PR descriptions
- Use clear, descriptive commit messages following this style:
  profiler: add basic CPU usage collection
  gpu: integrate HIP runtime kernel benchmarking
  tests: add integration tests for I/O stream profiler

### Testing

- Unit tests live in the same directory (`foo_test.go`)
- Integration & end-to-end tests go in the `/tests` folder
- Use logging at test entry/exit points to aid debugging
- Don’t spam logs in polling/looped functions

### Formatting & Style

- Run `gofmt` before pushing Go code
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Document all exported symbols with godoc-style comments
- Use meaningful names and consistent interfaces

### Folder Guidelines

- No random scripts or utils in root
- Keep implementation logic in internal packages
- Do not hardcode platform-specific behavior — use `build` tags where necessary

---

## Before You Submit a PR

1. Does your code follow the rules above?
2. Did you add tests where relevant?
3. Did you run `gofmt`?
4. Did you sign your commits?
5. Did you include meaningful comments and docs?

If yes, you’re good to go.

---

## Generating Issues

Please create GitHub issues for every planned feature, bugfix, or refactor. Use them to track work, link PRs, and document discussions. Good issue hygiene helps us all.

---

## Contact

DM or mention us in a PR or issue. We’re active and watching.

Welcome aboard.

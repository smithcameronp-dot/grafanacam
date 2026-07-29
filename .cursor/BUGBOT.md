# Bugbot rules (demo)

These rules help exercise Cursor Bugbot on intentional demo findings.
They apply to disposable demo code and are not production guidance.

## Dangerous dynamic execution

- **Title:** Dangerous dynamic execution
- **Severity:** blocking (security)
- **When to flag:** Any use of `eval(`, `exec(`, or `os/exec` that builds or runs a command from dynamic/caller-controlled input (for example `exec.Command("sh", "-c", userInput)`).
- **Why:** Dynamic shell/command execution from untrusted input is a command-injection risk and must be treated as a blocking security finding in review.

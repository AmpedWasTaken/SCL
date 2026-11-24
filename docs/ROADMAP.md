# SCL roadmap

## Core in this change
- HCL literal parsing, named blocks, strict optional schemas and defaults.
- Named presets with cycle checking, immutable copies for readers.
- Source-preserving attribute edits, formatting, guarded file replacement.
- Explicit reload with validation and JSON conversion; basic CLI.
- Command-driven multi-file manager with atomic in-memory publication and restart-only field checks.

## Next
- Commit the dependency lockfile after resolving the module graph.
- Full schema diagnostics with ranges and generated documentation/default templates.
- Typed collection element constraints, wildcard schemas, cross-field validation.
- File watching, host-specific runtime effects and transactional multi-file disk changes.
- Cross-process locking, durable backups, rollback and version history.
- Legacy INI migration with application-specific mapping, diff and redaction.
- VS Code grammar, completion and schema integration; Config Studio.
- Fuzzing, benchmarks, dependency and security review.

## Boundaries
Configuration is declarative. Do not evaluate untrusted code, fetch remote files or
read secrets implicitly. Source files may contain secrets and must be protected by
the application. Save performs a preflight comparison but is not a cross-process
lock; applications must serialize file writers. File replacement durability and
atomicity depend on the platform and filesystem. No release or host integration
is implied by this initial library.

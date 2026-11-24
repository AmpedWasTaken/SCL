# SCL roadmap

## Core in this change
- HCL literal parsing, named blocks, strict optional schemas and defaults.
- Named presets with cycle checking, immutable copies for readers.
- Source-preserving attribute edits, formatting, guarded file replacement.
- Explicit reload with validation and JSON conversion; basic CLI.
- Command-driven multi-file manager with atomic in-memory publication and restart-only field checks.

- Extended scalar kinds, homogeneous arrays, string/collection length and pattern constraints.
- Value-free reload change reports and coalesced generation events.

- Root-scoped local imports, disjoint merges, named profiles and reload integration.

- Opt-in single-file version backups, effective diffs and validated rollback.

- Explicit allowlisted environment secret references, schema starter generation and metadata export, typed decode and custom validation.
- VS Code syntax highlighting and snippets, parser fuzz target and benchmark.

## Next
- Commit the dependency lockfile after resolving the module graph.
- Full schema diagnostics with ranges and richer generated documentation.
- Wildcard schemas and transactional cross-field validation in Manager.
- File watching, host-specific runtime effects and transactional multi-file disk changes.
- Cross-process locking, multi-file history, retention, signed audit events and durable crash recovery.
- Legacy INI migration with application-specific mapping, diff and redaction.
- VS Code language server, schema-aware completion and Config Studio.
- Long-duration fuzzing, performance baselines, dependency and security review.

## Boundaries
Configuration is declarative. Do not evaluate untrusted code, fetch remote files or
read secrets implicitly. Source files may contain secrets and must be protected by
the application. Save performs a preflight comparison but is not a cross-process
lock; applications must serialize file writers. File replacement durability and
atomicity depend on the platform and filesystem. No release or host integration
is implied by this initial library.

# Configuration history

History is opt-in and currently covers individual Document files. Keep the history directory outside public assets, import trees and source control. Backups contain the exact original source, including comments and secrets; they are not encrypted. Unix directories and revision files require private permission bits; on Windows the operator must configure private ACLs. Use trusted directories and serialize writers across processes.

```go
rev, err := cfg.SaveWithHistory("private/history")
// On success, rev.ID identifies the previous source.
```

Backup archives the loaded on-disk source, not unsaved edits. Identical contents are deduplicated by SHA-256. History lists metadata sorted by file modification time, not a chronological change audit. Restore verifies the digest, validates against the current schema, backs up the existing source, and uses the same guarded save path. If save fails, the Document's effective values are restored in memory. Reload any running application separately after restoring its file. The generic CLI uses syntax validation rather than host-specific schemas.

```sh
scl backup server.scl private/history
scl set server.scl server.name '"Example"' --history private/history
scl history server.scl private/history
scl diff server.scl candidate.scl
scl diff-revision server.scl private/history REVISION_ID
scl restore server.scl private/history REVISION_ID
```

Diffs list changed paths and change kinds, never values. Comment-only changes appear in source backups but not effective diffs. History is scoped to the absolute configuration path; moving a file does not automatically move its history. Snapshot hashes detect accidental corruption but do not authenticate a malicious writer. Failed or interrupted backup writes are detected on read; preserve damaged copies for manual investigation. This is not a cross-process transaction, encrypted vault, or disaster recovery service. Automatic pruning, multi-file history, durable filesystem transaction recovery and signed audit events remain future work.

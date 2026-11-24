# Command-driven reload

SCL's Manager API is intended for a host application's existing reload command.
Register the exact local files at startup, including files ending in .conf or
.scl. SCL does not discover or interpret arbitrary directories automatically.

```go
manager, err := scl.OpenManager([]scl.FileSpec{
    {
        Name: "appearance",
        Path: "data/config/appearance/theme.scl",
        Schema: scl.Schema{
            "theme.fps": {Kind: scl.Integer, Default: 30, HotReload: true},
        },
    },
})
if err != nil {
    return err
}

// In the host's authorized reload-check handler:
report, err := manager.CheckReload()

// In the host's authorized reload handler:
// report, err := manager.Reload()
_ = report
return err
```

Each command reads every registered file, resolves presets, applies defaults
and validates schemas. All effective values are published under one lock
only after validation succeeds. A missing or invalid file rejects the entire
reload; the previous generation stays active. CheckReload never publishes.
Changes to fields with HotReload=false are rejected until process restart.
A nil schema has no restart-only metadata. No-op reloads retain the generation.

The host must use the manager's Snapshot/Get API instead of retaining stale
copies. Use one Snapshot per logical operation for consistency. The report
contains names and counts, not configuration values. Reload does not mutate
files or restart services. It does not automatically reconfigure listeners,
workers, caches or other external resources. The host must implement those
effects separately and use its own existing authorization rules. A check is
not a reservation: Reload revalidates from disk.

Concurrent reloads are serialized within the manager. If another process edits
several files while a reload is reading, the candidate may contain individually
valid files from different edit generations; coordinate writes externally.
This API does not offer cross-process locking or cross-file disk transactions.

## Sentinel integration

The existing host reload command should load SCL through the same typed host
configuration validation path as other configuration formats. It should report
how many SCL files were checked and reject invalid settings before publication.
The current legacy configuration reader still needs migration/integration;
adding this library alone does not change Sentinel's running command.

## Change reports and events

`ReloadReport.Changes` includes file names, segmented paths and added/removed/modified kinds. It never includes old/new values. Arrays are compared as a whole. `manager.Subscribe()` returns a generation channel and cancel function. Notifications are coalesced for slow readers; read a fresh Snapshot after notification. Preflight, failed reloads and no-op reloads never emit. Runtime changes are the application's responsibility; notification is not a transaction or rollback mechanism.

For modular configuration set `FileSpec.Project` to `ProjectOptions{Root: ..., Profile: ...}`. Reload re-reads and validates the entire import graph before publication. See [projects](PROJECTS.md).

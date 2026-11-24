# SCL — Sentinel Configuration Language

A declarative HCL-based configuration library for Go. Independent from any host application.

## Syntax

```hcl
preset animated {
  enabled = true
  fps     = 30
}

theme midnight {
  colors = [
    "#44DBD6", # Cyan
    "#B769FF", # Violet
  ]

  animation {
    use = "animated"
  }
}
```

Supports literal strings, numbers, booleans, null, lists, objects, named and nested blocks, HCL comments, and literal heredocs. No functions, scripting, environment lookup, interpolation or filesystem access during evaluation. Presets are root-level named blocks; explicit fields override inherited settings, nested objects merge, arrays replace. Cycles and duplicate blocks are rejected.

## Go API

```go
import "github.com/AmpedWasTaken/SCL"

schema := scl.Schema{
    "theme.midnight.animation.fps": {Kind: scl.Integer, Default: 30},
    "theme.midnight.animation.enabled": {Kind: scl.Boolean},
    "theme.midnight.colors": {Kind: scl.Array},
}
cfg, err := scl.Load("server.conf", schema)
if err != nil {
    return err
}
fps := cfg.Int("theme.midnight.animation.fps")
_ = fps
if err := cfg.Set("theme.midnight.animation.fps", 60); err != nil {
    return err
}
return cfg.Save()
```

Use a nil schema to accept arbitrary literal settings. Get returns a value and presence flag; convenience accessors return zero values on missing or incompatible values. Snapshot returns detached data. Set validates the full candidate and edits existing block bodies without regenerating the entire source. Reload keeps the last valid snapshot and refuses to discard unsaved edits.

## Command-driven reload

Use `OpenManager` with explicitly registered files, then call `CheckReload()` or `Reload()` from the host's existing reload command. All registered files are validated before a new snapshot is published; restart-only fields are rejected. See [reload integration](docs/RELOAD.md).

## CLI

```sh
go install github.com/AmpedWasTaken/SCL/cmd/scl@latest
scl validate examples/appearance.conf
scl fmt examples/appearance.conf
scl fmt examples/appearance.conf --write
scl set examples/appearance.conf theme.midnight.animation.fps 60
scl migrate-json old.json > new.conf
```

The generic CLI checks syntax and presets, not application-specific schema rules. JSON migration is non-destructive unless the operator redirects output over an existing file; write to a new path and validate before use.

## Development

```sh
go mod tidy
go test -race ./...
go vet ./...
```

Initial development version. The CI workflow exercises Linux and Windows. See docs/ROADMAP.md for remaining work and limitations.

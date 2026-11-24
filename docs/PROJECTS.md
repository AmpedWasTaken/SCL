# Composed SCL projects

```hcl
# server.scl
imports = [
  "appearance/theme.conf",
  "services/chat.conf",
]

server {
  name = "Example"
}

profile development {
  logging {
    level = "debug"
  }
}

profile production {
  logging {
    level = "warning"
  }
}
```

Call `scl.LoadProject("data/config/server.scl", schema, scl.ProjectOptions{Root: "data/config", Profile: "production"})`. Import paths are relative to the explicit project root, not the importing file. Only .conf and .scl are allowed. Imports are bounded by file count, total size and depth. Duplicate leaves, repeated imports, cycles, symlinks and paths outside the root are rejected. Protect the directory from concurrent untrusted mutation; this is not a filesystem sandbox.

Base documents merge disjoint object branches. The selected profile overrides base leaves, arrays replace, and presets resolve afterwards. Only the selected profile is schema-validated; validate each supported profile separately during CI. Unknown profiles are errors. A blank profile selects base settings. Profiles cannot declare their own imports or presets.

Register a FileSpec with Project set to the same options to re-read imported files on each Manager reload. Invalid imports or restart-only changes keep the last working generation. The selected profile remains fixed for the lifetime of that manager.

CLI: `scl validate-project data/config/server.scl data/config production`.

Project loading is read-only. The existing Document editor handles a single source file, not automatic edits to composed projects. The CLI validates structure and composition; host-specific schema validation happens via the Go API.

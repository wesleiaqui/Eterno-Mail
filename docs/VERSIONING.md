# Versioning

`VERSION` is the canonical Eterno Mail application version. It uses release
SemVer only: `X.Y.Z`.

Before a future release, update `VERSION`, materialize the same value in the
application metadata, then run:

```bash
./scripts/version.sh check
./scripts/version.sh check vX.Y.Z
```

The check compares `VERSION` with the Go application constant, frontend
package and lock metadata, Wails product version, current AppStream release,
and the main Flatpak manifest tag. It does not inspect or change dependency,
runtime, database-schema, migration, or historical release-note versions.

Review the resulting diff before creating a tag. The helper intentionally has
no `sync` command: version fields are updated explicitly so release notes and
their date can be reviewed together.

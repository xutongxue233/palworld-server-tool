# sav_cli

This directory contains the Palworld Server Tool adapter used to extract the
player and guild model expected by the Go API. It also exposes explicit
`export`, `rebuild`, `validate`, and `roundtrip` modes for offline save editing.
It also provides a Palworld 1.0.0 `sync-world-option` mode used by the Go
service to generate or synchronize `WorldOption.sav` without overwriting the
source file.

The save parser and writer are built from `palsav-flex` in
`deafdudecomputers/PalworldSaveTools` commit
`2cb6fd963120b002f0732dad153786e624f64b38`. The Windows Oodle runtime is
extracted from that project's verified `v2.0.0` release asset.

`palsav-flex` and `palooz` are licensed under GPL-3.0-or-later. The build script
copies their license next to the generated executable as
`sav_cli-GPL-3.0.txt`. The main Go application invokes `sav_cli` as a separate
process.

The original input save is never overwritten by the editing modes. Rebuilt
saves are parsed again before the command succeeds.

The WorldOption base template and setting type metadata are derived from
Bluefissure/pal-conf's Palworld 1.0.0 configuration at the pinned commit listed
in [`THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md). The template checksum,
metadata checksum, game version, and entry count are verified during builds.

## Offline editing workflow

Stop the Palworld server and back up the complete world save directory before
editing. Do not replace a live `Level.sav` while the server is running.

```powershell
# Confirm that the current save can be parsed and written without changes.
.\sav_cli.exe --mode validate --file .\Level.sav

# Export the complete property tree to editable JSON.
.\sav_cli.exe --mode export --file .\Level.sav --output .\Level.editable.json

# Edit the JSON, then rebuild and validate a separate SAV file.
.\sav_cli.exe --mode rebuild --file .\Level.editable.json --output .\Level.edited.sav

# Verify decompression and recompression without changing the GVAS payload.
.\sav_cli.exe --mode roundtrip --file .\Level.sav --output .\Level.roundtrip.sav

# Generate a separate WorldOption.sav from a validated server INI.
.\sav_cli.exe --mode sync-world-option `
  --file .\missing-WorldOption.sav `
  --settings-file .\PalWorldSettings.ini `
  --output .\WorldOption.generated.sav
```

The default `structure` mode is reserved for PST's scheduled synchronization
and extracts the player and guild records consumed by the Go service.

# Changelog

All notable changes to proxclt are documented here. Format: CalVer (`YYYY.MM.DD.TS`).

## v2026.04.11.1 — 2026-04-22

### Added

- Initial scaffold from Phase 1 of the proxclt + linuxctl plan
  (`itunified-io/infrastructure` docs/plans/024-proxclt-design.md).
- Go module `github.com/itunified-io/proxclt` on Go 1.23.
- Cobra command tree covering 9 subcommand groups:
  `config`, `env`, `vm`, `snapshot`, `kickstart`, `boot`, `workflow`, `license`, `version`.
  Every leaf subcommand returns `not implemented yet (scaffold)` — real logic lands in Phase 2+.
- `pkg/license` — `ToolTier` constants + `ToolCatalog` map + stub `Check()`.
- `pkg/state` — SQLite handle stub for Phase 2 `modernc.org/sqlite` integration.
- `pkg/config` — `Env`, `Hypervisor`, `Networks`, `StorageClasses`, `Cluster` structs;
  `ParsePlaceholders` regex for `${vault,env,file,gen,ref:…}` syntax.
- `pkg/version` — ldflags-injected `Version` / `Commit` / `Date`.
- AGPL-3.0 LICENSE, README, Makefile, .goreleaser.yaml, Dockerfile, CI workflow.
- Unit tests: root command + version + license gate + config validate + placeholder parser.

Ref: itunified-io/infrastructure#389

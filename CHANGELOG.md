# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1](https://platform.zone01.gr/git/lpapanthy/net-cat/compare/v0.2.0...v0.2.1) (2026-02-21)

### Added

- *(client)* implement input continuity with partial typing preservation (Task 22) ([#14](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/a20944749b6a81b4c81bc0f14f204a79d3509f2c))
- *(server)* implement midnight log rotation with history reset (Task 23) ([#13](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/4c2959a586aff16e8e3b232fdcc0dbad9cf9f172))

## [0.2.0](https://platform.zone01.gr/git/lpapanthy/net-cat/compare/v0.1.0...v0.2.0) (2026-02-21)

### Added

- *(server)* implement graceful shutdown with 5s timeout and force-close (Task 12) ([#7](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/6d3d1f86421d354a72ec886c6c13a3d83021fb2d))
- *(server)* implement admin system with operator terminal, admins.json persistence, and auto-restore (Task 18) ([#9](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/7ca26ba6b336e86dd39c5c33664a3b49aa5bec65))
- *(server)* implement IP-based moderation enforcement and comprehensive tests (Tasks 19-20) ([#10](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/debecd41db6bae972df36c8f840c6e7b08162872))
- *(server)* implement connection health heartbeat with ghost client detection (Task 21) ([#11](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/919477030bb71074613308bdab50d1c182a9fe7c))

## 0.1.0 (2026-02-20)

### Added

- implement core TCP chat server (Tasks 1-8) ([#2](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/0165798964f7431ca069958f3feb764bafa68e98))
- *(logger)* implement file-based activity logging (Task 9) ([#3](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/d0ef2a7d12b7795880ed2c5a060a98f9155afc72))
- *(server)* implement crash recovery from daily log files (Task 10) ([#4](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/ecf0c8c3755233a8b582d8c0cc939eb2883809ea))
- *(server)* implement connection capacity with 10-client limit and FIFO queue (Task 11) ([#5](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/3a65edc1c8f9f53755fc0ec08ba88e1683fc32b5))

### Documentation

- create .gitignore ([#1](https://platform.zone01.gr/git/lpapanthy/net-cat/commit/6a3fdd94580a8580fb7ef9321edd3fe2f6897764))

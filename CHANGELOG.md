# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.1.2][] - 2026-02-25

### Changed

* Fixed RAP decode for payloads with broken `offsetToEnums` header field:
  decoder now applies safe enum-footer fallback instead of failing hard with
  `enums offset out of range`

[0.1.2]: https://github.com/WoozyMasta/rap/compare/v0.1.1...v0.1.2

## [0.1.1][] - 2026-02-20

### Added

* Decode fallback for legacy name-first append streams
* Scalar `subtype=6` (`int64`) support in RAP decode/encode paths
* Regression tests for array append `subtype=5` wire layout `flags -> name`
* Binary fixture decode coverage in `codec_test.go`

### Changed

* Implemented fast-path in `prepareEncodeInput` for configs without enums;
  this gave about x2.6 for `EncodeConfigAST`
  and about x3.1 for `EncodeConfigBinDecodedAST` versus baseline.

[0.1.1]: https://github.com/WoozyMasta/rap/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2026-02-08

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/rap/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html

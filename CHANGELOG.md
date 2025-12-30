# Changelog

## [1.0.1](https://github.com/Aleph-Alpha/std/compare/v1.0.0...v1.0.1) (2025-12-30)


### Bug Fixes

* **embedding:** pass token as a parameter to Create method ([#49](https://github.com/Aleph-Alpha/std/issues/49)) ([3669b03](https://github.com/Aleph-Alpha/std/commit/3669b037726b64604442018a182ea67b5cb646ee))

## [1.0.0](https://github.com/Aleph-Alpha/std/compare/v0.15.0...v1.0.0) (2025-12-30)


### ⚠ BREAKING CHANGES

* Removed v1/database package and unified interface. This reverts the design introduced in v0.15.0, returning to v0.14.0's separate-interfaces-per-package approach.

### Features

* revert to separate interfaces per database package ([#47](https://github.com/Aleph-Alpha/std/issues/47)) ([8a3e111](https://github.com/Aleph-Alpha/std/commit/8a3e111e7820b73ac98c3b61ac4f9957cca8f6ce))

## [0.15.0](https://github.com/Aleph-Alpha/std/compare/v0.14.0...v0.15.0) (2025-12-30)


### Features

* Add database-agnostic interface and FX module for unified SQL client ([#45](https://github.com/Aleph-Alpha/std/issues/45)) ([c009db7](https://github.com/Aleph-Alpha/std/commit/c009db7379c4ffe75884c45c1578d9f3a54cf3a1))

## [0.13.0](https://github.com/Aleph-Alpha/std/compare/v0.12.0...v0.13.0) (2025-12-27)


### Features

* add observability hook ([#41](https://github.com/Aleph-Alpha/std/issues/41)) ([6dc5cf3](https://github.com/Aleph-Alpha/std/commit/6dc5cf3a2842f8f4057c0f01adad68c700fba0bc))

## [0.12.0](https://github.com/Aleph-Alpha/std/compare/v0.11.1...v0.12.0) (2025-12-25)


### Features

* Provide Interfaces in FX Modules ([#38](https://github.com/Aleph-Alpha/std/issues/38)) ([2b25bae](https://github.com/Aleph-Alpha/std/commit/2b25baeea4a3c95b00054e53f3d9b26bdc95e4b0))
* Refactor Metrics Package: Dual Endpoints + Interface-Based API ([#40](https://github.com/Aleph-Alpha/std/issues/40)) ([3bd349f](https://github.com/Aleph-Alpha/std/commit/3bd349fa1b885ea59918d5592e4ff54e29e0d3b6))

## [0.11.1](https://github.com/Aleph-Alpha/std/compare/v0.11.0...v0.11.1) (2025-12-24)


### Bug Fixes

* Refactor to Interface-First Design for Postgres and MariaDB ([#36](https://github.com/Aleph-Alpha/std/issues/36)) ([e3be51a](https://github.com/Aleph-Alpha/std/commit/e3be51aa1b019a73606b34ec43f4b47c3a4ef434))

## [0.11.0](https://github.com/Aleph-Alpha/std/compare/v0.10.0...v0.11.0) (2025-12-24)


### Features

* add mariadb package ([#35](https://github.com/Aleph-Alpha/std/issues/35)) ([13c1ec9](https://github.com/Aleph-Alpha/std/commit/13c1ec915cb26e2e1a1d37d6762d5e0633bac218))
* **vectordb:** Add database-agnostic vector search abstraction ([#27](https://github.com/Aleph-Alpha/std/issues/27)) ([94bca33](https://github.com/Aleph-Alpha/std/commit/94bca33a4cd39123cc488d4e1c6f66c54d5eb5f0))

## [0.10.0](https://github.com/Aleph-Alpha/std/compare/v0.9.1...v0.10.0) (2025-12-21)


### Features

* add kafka, redis, and schema registry packages ([#33](https://github.com/Aleph-Alpha/std/issues/33)) ([e6658ab](https://github.com/Aleph-Alpha/std/commit/e6658ab57896d892cea216311c223fa17e862b46))

## [0.7.0](https://github.com/Aleph-Alpha/std/compare/v0.6.0...v0.7.0) (2025-12-10)


### Features

* Update qdrant documentation to account for filtering ([#25](https://github.com/Aleph-Alpha/std/issues/25)) ([ed42b36](https://github.com/Aleph-Alpha/std/commit/ed42b36f856fef686e3584c7cda3412fb817a4a8))

## [0.6.0](https://github.com/Aleph-Alpha/std/compare/v0.5.0...v0.6.0) (2025-12-10)


### Features

* **qdrant:** Add filtering system for Qdrant vector searches ([#22](https://github.com/Aleph-Alpha/std/issues/22)) ([1ef9423](https://github.com/Aleph-Alpha/std/commit/1ef94237e77f06272e747f45438e56016a0ffd84))

## [0.5.0](https://github.com/Aleph-Alpha/std/compare/v0.4.0...v0.5.0) (2025-12-05)


### Features

* add sparse embedding service go client ([#18](https://github.com/Aleph-Alpha/std/issues/18)) ([e4ab9b8](https://github.com/Aleph-Alpha/std/commit/e4ab9b84be2085a832d38731d649b9cfd2faf7d7))

## [0.4.0](https://github.com/Aleph-Alpha/std/compare/v0.3.0...v0.4.0) (2025-12-05)


### Features

* trigger release ([39ab22f](https://github.com/Aleph-Alpha/std/commit/39ab22f160392eb53eb5bcd05110039dd5fb641a))

## [0.3.0](https://github.com/Aleph-Alpha/std/compare/v0.2.1...v0.3.0) (2025-12-05)


### Features

* refactor Search API with concurrent batch support and multi-collection capability ([#16](https://github.com/Aleph-Alpha/std/issues/16)) ([ad7a1b6](https://github.com/Aleph-Alpha/std/commit/ad7a1b6336d65d5e24fd9d7a5b43350c9fbb286a))

## [0.2.1](https://github.com/Aleph-Alpha/std/compare/v0.2.0...v0.2.1) (2025-12-03)


### Bug Fixes

* **embedding:** update endpoint URL and error message in inference provider ([df29810](https://github.com/Aleph-Alpha/std/commit/df29810b63fdc77291f11e8a3ae6bc666b021ccf))

## [0.2.0](https://github.com/Aleph-Alpha/std/compare/v0.1.0...v0.2.0) (2025-12-02)


### Features

* trigger release ([9806dae](https://github.com/Aleph-Alpha/std/commit/9806daee2501329f997336226447710b77dea7c3))

## [0.1.0](https://github.com/Aleph-Alpha/std/compare/v0.0.5...v0.1.0) (2025-11-24)


### Features

* add Octo STS trust policies for pharia-holmes and pharia-data-api ([#7](https://github.com/Aleph-Alpha/std/issues/7)) ([1c49f5b](https://github.com/Aleph-Alpha/std/commit/1c49f5bd24a08747baeaa35287910b4ce817544b))
* add Qdrant pkg ([#4](https://github.com/Aleph-Alpha/std/issues/4)) ([f75c8fc](https://github.com/Aleph-Alpha/std/commit/f75c8fcad16c4ecee1d09b125466d4b3e23aac03))
* **embedding:** Add embedding package ([#5](https://github.com/Aleph-Alpha/std/issues/5)) ([016b4be](https://github.com/Aleph-Alpha/std/commit/016b4be6b60de23bfc5a7d37b31647742691882e))


### Bug Fixes

* correct subject_pattern in Octo STS trust policies ([#9](https://github.com/Aleph-Alpha/std/issues/9)) ([62a21e1](https://github.com/Aleph-Alpha/std/commit/62a21e1313dd26a55a644373073d620d09ce9776))
* update trust policy format to match working examples ([#8](https://github.com/Aleph-Alpha/std/issues/8)) ([b1873ca](https://github.com/Aleph-Alpha/std/commit/b1873ca400904ee712a859839765817247a37d8a))

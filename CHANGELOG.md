# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.3.0] - 2025-11-25

### Added

* [9f40a54](https://github.com/runvoy/runvoy/commit/9f40a54d3f42417baec9bf2f3909d25c253fb76e) test: add comprehensive unit tests for authorization package
* [d7a3a8f6](https://github.com/runvoy/runvoy/commit/d7a3a8f6c5242996ecc146eed1caa13b1395691a) feat: add CloudWatch Logs Insights endpoint for admin request log queries
* [2a52a20](https://github.com/runvoy/runvoy/commit/2a52a20d418f0cf6b3d181d197f054ab86c73c95) feat: add Phase 1 webapp features (claim, settings, kill execution)
* [b22f58c0](https://github.com/runvoy/runvoy/commit/b22f58c0db55c05afbf034ed08be099f81eb1e41) feat: add SvelteKit routing for shareable URLs in webapp (#313)
* [27985710](https://github.com/runvoy/runvoy/commit/27985710a402ddfc66fec0d11bfd6a66b0321e04) feat: add backend health check to webapp settings view (#308)
* [f8ac9351](https://github.com/runvoy/runvoy/commit/f8ac935177c89003838cea7f9af4fa1e8475618d) feat: add execution list view to webapp
* [bf55a930](https://github.com/runvoy/runvoy/commit/bf55a930bca0d615ba80cae520ebaa3ab7ca180a) feat: add request ID tracking fields
* [2570cded](https://github.com/runvoy/runvoy/commit/2570cded8b854a473725ff139bb77313011c808d) feat: add trace command
* [7243aac5](https://github.com/runvoy/runvoy/commit/7243aac53ef0c276f5812487ec43e3c1fda81abf) feat: enrich GET /trace endpoint with related resources
* [7dc417b7](https://github.com/runvoy/runvoy/commit/7dc417b7c596a2d9d08dbf022dbc96da1556c17e) feat: standardize view layouts with consistent card styling (#305)
* [d2aedbbe](https://github.com/runvoy/runvoy/commit/d2aedbbe0e5e378a3db726c4bb79be7f4712e1ef) fix(webapp): make API key optional in ConnectionManager for claim flow (#332)
* [9468bba4](https://github.com/runvoy/runvoy/commit/9468bba48463aa3fd17abaf9a6441d9fe037708c) fix: add missing perm to event processor
* [4c59c8ec](https://github.com/runvoy/runvoy/commit/4c59c8eca68d2474fac526dbb6d7fd187286bac9) fix: address wrong deployment flow
* [915854fb](https://github.com/runvoy/runvoy/commit/915854fbc7e5a961f29251b72ce3e97c9ff300a4) fix: clean up CloudWatch Logs Insights query formatting (#325)
* [ae35bd38](https://github.com/runvoy/runvoy/commit/ae35bd385d9cae28c342b2de407f7ed7aec8927b) fix: inconsistent layout widths across views (#304)
* [d05cb006](https://github.com/runvoy/runvoy/commit/d05cb006cfa0386de5825e303e32f8e898a1f776) fix: lint webapp
* [be964901](https://github.com/runvoy/runvoy/commit/be964901298fae821a72bfee1a95ff8e7b1c3823) fix: parse cors env var correctly
* [6285ef14](https://github.com/runvoy/runvoy/commit/6285ef14e3c485e3db9b7019546bad7b32da1a92) fix: use ExtractRequestIDFromContext for modified_by_request_id in executions (#311)
* [e55b2b73](https://github.com/runvoy/runvoy/commit/e55b2b73573b3527a9a920d20731ff023a700b6f) fix: webapp: better handle version string
* [f2106a77](https://github.com/runvoy/runvoy/commit/f2106a77c37f44db952e137dec647a9afa886e30) fix: webapp: use join to avoid path mismatch
* [1c0ece3e](https://github.com/runvoy/runvoy/commit/1c0ece3e31ccecb7d1567f6dc5e4997ae3e7b77a) perf(dynamodb): replace Scan with Query for GetExecutionsByRequestID
* [cd4f713f](https://github.com/runvoy/runvoy/commit/cd4f713f474b22ab393be87cc2f69510f727a6a6) utils: add branch cleanup

## [v0.2.0] - 2025-11-19

### Added

- Add [CHANGELOG](./CHANGELOG.md)

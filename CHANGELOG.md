# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.5.0] - 2025-12-06

### Added

* [5979c48e](https://github.com/runvoy/runvoy/commit/5979c48e54d6acc8eebd9850f6e8b2649d38fed8) feat: add eventID to all logs (#379)

### Changed

* [183ff1c8](https://github.com/runvoy/runvoy/commit/183ff1c86387257aeceb7391914322c2674b92d2) refactor: drop logs insights API, use filterLogs instead (#394)
* [2b3a5ebd](https://github.com/runvoy/runvoy/commit/2b3a5ebda2148269f854ce31dc293292d3926c9d) test(webapp): expand log coverage (#395)
* [36e110c4](https://github.com/runvoy/runvoy/commit/36e110c4d9258d16cc48a86db9f8be995d8bc826) ci: remove bogus netlify build command
* [39652f60](https://github.com/runvoy/runvoy/commit/39652f60586cee46f194e1e1346d8e89ae144740) refactor(webapp): use props instead of stores for log events (#392)
* [3eed0542](https://github.com/runvoy/runvoy/commit/3eed0542e1f149180dfa7854d5ce41ed0af3ff18) refactor: api client initialization in load functions (#387)
* [4e128f03](https://github.com/runvoy/runvoy/commit/4e128f03f37ba2017f4c99dc2251ca7d63db3617) docs: add badge to readme
* [56f140b4](https://github.com/runvoy/runvoy/commit/56f140b4993750faecdd04c8256bed438d8c0e34) docs: add goreport badge
* [68d27d78](https://github.com/runvoy/runvoy/commit/68d27d780694f8d2e3eacc653cc3f7862a5b69fc) test(orchestrator): improve backend coverage (#396)
* [6a0ace07](https://github.com/runvoy/runvoy/commit/6a0ace07ac47111bda891cfd41819568cfd1e14e) refactor: simplify log lines index handling
* [6b28c5c2](https://github.com/runvoy/runvoy/commit/6b28c5c2be448b01781f6bdc01872b423d6364c0) refactor(webapp): reduce abuse of stores usage in logs view
* [79069974](https://github.com/runvoy/runvoy/commit/790699743bea2baa679a494d7641ae78d7013e9f) refactor(webapp): improve log viewer performance with virtualization and batching (#401)
* [7af9c155](https://github.com/runvoy/runvoy/commit/7af9c155ba8122abb312bbb1b2362da2cf9989a0) test(webapp): add more coverage
* [86023301](https://github.com/runvoy/runvoy/commit/86023301b605cf3a4be588786b96c641ccc2909f) ci(webapp): use netlify adapter to add proper url support
* [95856903](https://github.com/runvoy/runvoy/commit/95856903b2c400bcc4fa1c07a537b55131bcad4c) test: enable nilerr linter
* [9eaa4f29](https://github.com/runvoy/runvoy/commit/9eaa4f29a743c5427a193ad33be759e213106118) refactor(webapp): new exec view with modern SvelteKit patterns (#382)
* [a7c5efb9](https://github.com/runvoy/runvoy/commit/a7c5efb91d51cb135d002d6df942d1c744c80d10) refactor: remove linter suppressions
* [b04a31c4](https://github.com/runvoy/runvoy/commit/b04a31c44a813af1c0feab9ae3cf3634ee81ad46) refactor: use ExecutionStatusValue type for status fields (#386)
* [b36caa09](https://github.com/runvoy/runvoy/commit/b36caa09223b1b8b948ed721af43df2adb714c6a) refactor: enable nestif linter
* [c0af7453](https://github.com/runvoy/runvoy/commit/c0af7453b0fd30f5f3fd753572747bf2345e1eeb) refactor(web): split god logsview into modules (#403)
* [c77d8c63](https://github.com/runvoy/runvoy/commit/c77d8c633e7a88a3cc9a44b78efccafb82a70dc5) style: uniform container meta logs format
* [d8548d1c](https://github.com/runvoy/runvoy/commit/d8548d1c86012965601cb2bb52fed1dc2a563f1c) tool: move release cloudformation stacks into own folder
* [e429a42b](https://github.com/runvoy/runvoy/commit/e429a42b9be2e4e6bef979faa377d592d40e40ed) refactor(webapp): streamline API client setup (#389)
* [e682d34d](https://github.com/runvoy/runvoy/commit/e682d34d9e32047ead68df3cb23e613fcbd8662c) refactor(webapp): split view into component
* [ec7c1aff](https://github.com/runvoy/runvoy/commit/ec7c1aff147f752f93af4d704d55581b08f29d7d) test: enable wrapcheck and other linters
* [f5f51ad1](https://github.com/runvoy/runvoy/commit/f5f51ad1cf9df7c1a0a8c4250b822756a535c11e) test: add more linters

### Fixed

* [1c14baa7](https://github.com/runvoy/runvoy/commit/1c14baa7653d23217163352f39b204a30751ae3d) fix: add missing IAM permissions
* [24332e37](https://github.com/runvoy/runvoy/commit/24332e37e58b127ee762086d4f669f40e4f7bace) fix(webapp): add _redirects to make Netlify play along SPA
* [32c5c518](https://github.com/runvoy/runvoy/commit/32c5c518a05b4196683d7cabaa31346bb58b92a8) fix: remove unused orchestrator method
* [3990c5ca](https://github.com/runvoy/runvoy/commit/3990c5ca867a972d03d295e44f8bdbeaa6b11fcd) fix(webapp): add websocket event validation
* [42491d54](https://github.com/runvoy/runvoy/commit/42491d542fa557c3f1ae71ecb2eb614319e6d8c2) fix(webapp): update status correctly
* [51b80a58](https://github.com/runvoy/runvoy/commit/51b80a585d70685fae9d0f47afaef3369fc0795b) fix(webapp): remove double call to status after disconnect
* [56b9cd9e](https://github.com/runvoy/runvoy/commit/56b9cd9e412808033a179dfacbe3da37de80a92f) fix(webapp): make autoscroll work for logs view
* [71127d10](https://github.com/runvoy/runvoy/commit/71127d10d004aec42cbeabf0481a1f1259e28130) fix(webapp): correct redirect
* [7660c51d](https://github.com/runvoy/runvoy/commit/7660c51d0d7777954c54c02fadda7f95fc7ee98e) fix(processor): ensure execution logs are always marked for deletion
* [8ab8b948](https://github.com/runvoy/runvoy/commit/8ab8b94820b8965f494078ce42ac92b42da72449) fix(logs): disabimguate get logs response
* [96cc2098](https://github.com/runvoy/runvoy/commit/96cc20981171c82e5b3f802277d9820f8544e45c) fix(webapp): cleanup logs view from duplicated data
* [a75e2e16](https://github.com/runvoy/runvoy/commit/a75e2e16055a24a17dfba2841d39b1e18ef6bec7) fix(logs): show web link in the right place
* [b4e417f3](https://github.com/runvoy/runvoy/commit/b4e417f3dbfc0f79e24672bb7d3c7d6f91b4ba52) fix(webapp): remove duplicate execution id store (#393)
* [b9739ec0](https://github.com/runvoy/runvoy/commit/b9739ec0ecc1ad40d329134f885903732614cc96) fix: harden trace fetching and handlers (#398)
* [f5068167](https://github.com/runvoy/runvoy/commit/f506816758f3ba26595c33b2d30c3cffb0b575c0) fix: add missing web dev URL to orchestrator CORS
* [fe8aa1f4](https://github.com/runvoy/runvoy/commit/fe8aa1f409321c98cd9e741f75656f69d0b9d10a) fix(webapp): use correct id for log view
* [ffba3e7d](https://github.com/runvoy/runvoy/commit/ffba3e7db6ee1b4c9caff6f9ca0a233e318e3ecb) fix(webapp): display missing data to completed executions

## [v0.4.1] - 2025-11-29

### Added

* [46761c2](https://github.com/runvoy/runvoy/commit/46761c2e1e3908daf12138e269cc8d1cbd5f55e0) feat: Fetch and merge runner and sidecar logs

### Changed

* [59d6e4a](https://github.com/runvoy/runvoy/commit/59d6e4a8b6c61fab326113f218570e1438ace912) refactor: rename module path to play along with go proxy
* [5e7fbc4](https://github.com/runvoy/runvoy/commit/5e7fbc4ecfbe216a67ba46c96ac6fb0677c2e5bc) test: add coverage for aws.processor
* [85dab59](https://github.com/runvoy/runvoy/commit/85dab59fc6880e12238944b5d17e157c27843338) refactor: centralize constants to lib/constants.ts
* [9da208c](https://github.com/runvoy/runvoy/commit/9da208ce035e0c04b1720946c4a10378ffe2cd16) test: add coverage to aws.health
* [afadb01](https://github.com/runvoy/runvoy/commit/afadb012bd673fb7753ae9f6c93742d06ffca860) docs: update README.md
* [bd23f15](https://github.com/runvoy/runvoy/commit/bd23f154a28c7e85b963367e02d3dcd4c5ca8036) docs: update README, no more need for log streaming improvement
* [c1590dc](https://github.com/runvoy/runvoy/commit/c1590dc48184f0ec25cbe17cbb08a54f200fdb99) refactor(webapp): expose webapp API clients via page load data (#374)
* [da7266e](https://github.com/runvoy/runvoy/commit/da7266ef3822c3395125184373cf127cd1092573) test: add more coverage to logs

## [v0.4.0] - 2025-11-28

### Added

* [75ed4f0](https://github.com/runvoy/runvoy/commit/75ed4f05121d4e34d5b08f0eade72ee9429b8b62) feat(logs): add dynamodb buffer for improved log streaming
* [926de6f](https://github.com/runvoy/runvoy/commit/926de6ffbb8ca41dcb67815d0786f80a6a0db80a) feat(webapp): optimize log retrieval after websocket close #358
* [a8d16c0](https://github.com/runvoy/runvoy/commit/a8d16c0972c6ddfbda394367724c1d32c8a91c1f) feat(client): improve log streaming handling
* [b29d845](https://github.com/runvoy/runvoy/commit/b29d845cea2374aa41f95508c846c89c2ca43972) feat(logs): return websocket url from /run (#352)
* [d731686](https://github.com/runvoy/runvoy/commit/d73168644fa9ff7ab9b12c8d6b9152aa6aa97b98) feat(webapp): migrate webapp to Svelte 5 toolchain (#369)
* [ea61f40](https://github.com/runvoy/runvoy/commit/ea61f4089c0bbf600397a4ca3c51a94efb07f0c1) feat: add region and provider info to GET /health
* [f9866ba](https://github.com/runvoy/runvoy/commit/f9866ba17daff966f0ade52bea4523cc76273aba) feat(webapp): show backend provider and region
* [bbe93ce](https://github.com/runvoy/runvoy/commit/bbe93ce88df7c93780210b702784d757a037099a) tool: add update-changelog script

### Changed

* [0e59329](https://github.com/runvoy/runvoy/commit/0e593295f9d6e4960448013eba4d40db48f91c71) docs: update webapp and docs URLs
* [1338ad7](https://github.com/runvoy/runvoy/commit/1338ad73be6037f551a68c59024f157da7789871) refactor(aws): split orchestrator bootstrap wiring
* [1fb46ad](https://github.com/runvoy/runvoy/commit/1fb46adbbbf78037de6860fa1010a82c2b10231e) test: add more coverage to health package
* [3ae6dbb](https://github.com/runvoy/runvoy/commit/3ae6dbb93ebafc21639167c7c3eb4e5d46edf196) docs: add release badge
* [5e3fdbd](https://github.com/runvoy/runvoy/commit/5e3fdbda217a7256147284d0ca7e0e2aea240208) tool(just): trigger docs build in release flow
* [7bdbcaf](https://github.com/runvoy/runvoy/commit/7bdbcaf36b39705743b9badc0f0dac028d06e281) tool(just): add print-build-version
* [8b616ae](https://github.com/runvoy/runvoy/commit/8b616aefa8fb87f7c3969c5d69b997bcf47675d1) ci: moved dev web into its own Netlify project
* [915764c](https://github.com/runvoy/runvoy/commit/915764ca4c2b1c895c948ca33f8d9d3de7da86a2) refactor: modernize Svelte runes usage (#371)
* [9378298](https://github.com/runvoy/runvoy/commit/9378298eab527e59117b8d7ac1e918a68476bdb0) docs(web): update header link
* [b5937a9](https://github.com/runvoy/runvoy/commit/b5937a9523d350487d90af5b28bbf20a84881881) docs: update changelog
* [bdd0a52](https://github.com/runvoy/runvoy/commit/bdd0a52ea6ceccd8cc23a564fd9ac1c5c3de65ce) test(health): cover casbin reconciliation (#349)
* [c1404b0](https://github.com/runvoy/runvoy/commit/c1404b04f2d4763231eb9fe706eacd18d245ed99) refactor(webapp): remove legacy configure api views
* [d26ea23](https://github.com/runvoy/runvoy/commit/d26ea23924b5b6e9f1a13e6b6d9bc8ec6266085a) refactor(webapp): normalize log event types (#365)
* [d57a335](https://github.com/runvoy/runvoy/commit/d57a335a4c801c9eb922cbc95a72d038bae7a3c1) refactor(webapp): update config storage
* [edc1a9d](https://github.com/runvoy/runvoy/commit/edc1a9df42d50ce4b4327d03056619792a39fb83) refactor(cli): simplify logs retrieval (#360)
* [f8d44cf](https://github.com/runvoy/runvoy/commit/f8d44cf7bc13d156224870ce5bbba264145efd55) test(aws/health): cover compute defaults and tags (#364)

### Fixed

* [23b9cf1](https://github.com/runvoy/runvoy/commit/23b9cf197cc3404b89bda8e771f553c11fbae356) fix(webapp): consolidate api configuration (#363)
* [2ee4e4b](https://github.com/runvoy/runvoy/commit/2ee4e4b1850a5b3620fdaf34ad2d059da9f696dd) fix(web): remove useless index fallback
* [35dffa1](https://github.com/runvoy/runvoy/commit/35dffa1264ba606c3d31b072e8d310d0c9a945c2) fix(webapp): improve modal accessibility (#361)
* [5ac1463](https://github.com/runvoy/runvoy/commit/5ac14639c9f7b21c854660291a5ce7fa0e30c582) fix(webapp): remove lint disables from log rendering (#366)
* [5eddfd3](https://github.com/runvoy/runvoy/commit/5eddfd3b6fd0ca0ca5e64bd07476e37c2fca8f4f) fix(webapp): cleanup logs flow
* [71d9abb](https://github.com/runvoy/runvoy/commit/71d9abb99ff573d0dd90b4bdc6b9069ff7433b07) fix(webapp): load persisted config and enforce redirects
* [8683896](https://github.com/runvoy/runvoy/commit/86838964b6d85399030a4072b5b561e03edf66f1) fix(webapp): use sveltekit version directly (#362)
* [9026807](https://github.com/runvoy/runvoy/commit/902680721963f7ad1c2866d886c376c2583ffb77) fix(webapp): simplify ws flow
* [afa5de7](https://github.com/runvoy/runvoy/commit/afa5de796001d606fc92240928068486a11b818d) fix(webapp): cleanup settings onboarding
* [c9190dd](https://github.com/runvoy/runvoy/commit/c9190dd14647a69b05c6b577b413165d95915ba3) fix(webapp): reorder nav buttons
* [d34758f](https://github.com/runvoy/runvoy/commit/d34758f66e9addd2b2e825d8fd54a69ff0cc5592) fix(webapp): redirect claim to configure endpoint (#367)
* [eab2247](https://github.com/runvoy/runvoy/commit/eab2247db176fb930d6fb28f836916ef3eb3fe6b) fix(webapp): remove obsolete normalize func
* [fe10e79](https://github.com/runvoy/runvoy/commit/fe10e790dfa5a1ce02f1fec143c6fa1d27e467af) fix(webapp): refresh health after saving settings (#368)

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

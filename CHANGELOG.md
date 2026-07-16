
## [0.6.0] - 2026-07-16

### Bug Fixes

- Strip \r from diff lines to prevent CRLF corruption in split view ([04066a1](https://github.com/nospor/gitlab-tui/commit/04066a136a2d8118114c710d84d33c83a0dbe105))

            Diff content from files with Windows-style CRLF line endings left a
            trailing \r in each DiffLine.Content. When rendered to the terminal the
            \r caused a carriage-return, moving the cursor back to column 0 and
            overwriting the vertical separator and left-panel content with
            right-panel
            diff text — producing visible "holes" in the border.
- Improve MR details scrolling behavior ([9c377bd](https://github.com/nospor/gitlab-tui/commit/9c377bd801441c4ed6a6e7a9c3a86656b574c7d0))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.9 [skip ci] ([8a1bea0](https://github.com/nospor/gitlab-tui/commit/8a1bea09ee6afd9b4cb7d3371435da873e226835))

## [0.5.9] - 2026-07-16

### Refactor

- *(gitlab)* Replace hand-rolled HTML parser with html-to-markdown library ([3bcdd4d](https://github.com/nospor/gitlab-tui/commit/3bcdd4dac4f6b6b1d59a39a4d91fbb1749c6d163))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.8 [skip ci] ([ac4c877](https://github.com/nospor/gitlab-tui/commit/ac4c8772d4ba298ffa063338e558712ac400bc52))

## [0.5.8] - 2026-07-16

### Features

- *(tui)* Support triggering manual pipeline jobs with the `r` key ([2f56d1e](https://github.com/nospor/gitlab-tui/commit/2f56d1e187002d5c115cfd1da9fa8a6ade4962cb))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.7 [skip ci] ([5ae0fad](https://github.com/nospor/gitlab-tui/commit/5ae0fadb24d1be4fb8b204759fcb3e7ff1707e07))

## [0.5.7] - 2026-07-09

### Bug Fixes

- *(tui)* Show inline comments created as individual notes in diff view ([20af146](https://github.com/nospor/gitlab-tui/commit/20af146e7ad6fd9c4dca9f6b066541e962d859f7))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.6 [skip ci] ([2b8a087](https://github.com/nospor/gitlab-tui/commit/2b8a087f67c7ecd963bc2015df605d0daf4150b9))

## [0.5.6] - 2026-07-08

### Bug Fixes

- *(tui)* Prevent footer help bar from disappearing after popups ([0e3ce50](https://github.com/nospor/gitlab-tui/commit/0e3ce5073d6bf586e61f67db444792fbdd7c0dc1))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.5 [skip ci] ([f1cab6d](https://github.com/nospor/gitlab-tui/commit/f1cab6dfcf2d6970a6982b8ae6ecc92cf2a52b25))

## [0.5.5] - 2026-07-07

### Bug Fixes

- *(theme)* Use green background with dark text for Teams theme titles ([91f88ec](https://github.com/nospor/gitlab-tui/commit/91f88ec3ec51ec3d982dbdeb03c4e90dabffb535))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.4 [skip ci] ([1e3500f](https://github.com/nospor/gitlab-tui/commit/1e3500f463126c1ef74997793d1a955a130e0000))

## [0.5.4] - 2026-07-07

### Features

- New theme ([aabce4a](https://github.com/nospor/gitlab-tui/commit/aabce4ad52a771008cc93ff69dc6dffbe082dcb1))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.3 [skip ci] ([10039a9](https://github.com/nospor/gitlab-tui/commit/10039a924f9fdd14cecbeae7021d9d1d7f630b9d))

## [0.5.3] - 2026-07-07

### Features

- Support opening pipelines and jobs via URL parameters ([f42f523](https://github.com/nospor/gitlab-tui/commit/f42f523817842a46c1bdbfb17ec8e36382337129))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.2 [skip ci] ([642cd1c](https://github.com/nospor/gitlab-tui/commit/642cd1c7a9a34b6766c4d3ae0b99aada869d9c40))

## [0.5.2] - 2026-07-07

### Features

- *(pipelines)* Add job list, statuses, trace logs, external editor, and auto-refresh ([5e8109f](https://github.com/nospor/gitlab-tui/commit/5e8109f02e05b59433c2e342c7dd90eae56ad893))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.0 [skip ci] ([68b6976](https://github.com/nospor/gitlab-tui/commit/68b6976d7ce98cfcda571999b65e58edb6a59eb1))
- Update CHANGELOG.md for v0.5.1 [skip ci] ([2e2733a](https://github.com/nospor/gitlab-tui/commit/2e2733a46f9db4e1be44a3839732e9509284704b))

## [0.5.1] - 2026-07-06

### Documentation

- Link yt-tui to its GitHub repository in README ([b3a38aa](https://github.com/nospor/gitlab-tui/commit/b3a38aa3dc688f7ffe55a170592db81497009ccf))
- Update readme and license ([8cb972e](https://github.com/nospor/gitlab-tui/commit/8cb972e24ea50130b6d4ad6102d5ce6bf760bf23))

## [0.5.0] - 2026-07-06

### Features

- *(tui)* Add number prefixes to tabs and display navigation key hints ([becffbf](https://github.com/nospor/gitlab-tui/commit/becffbf9cac6402310a631f8976c285d89a65f85))
- Add toggleable vote up/down on MR detail view ([8aaf2af](https://github.com/nospor/gitlab-tui/commit/8aaf2af0204504c4832a18a35399cfb207b69af9))
- Add side-by-side MR diff panel with scrollable hunks and inline commenting ([dfa772e](https://github.com/nospor/gitlab-tui/commit/dfa772ebdec6bf930eaeb04dc4554ae9f71e211e))
- *(tui)* Add merge request discussions and inline comments support ([4be52db](https://github.com/nospor/gitlab-tui/commit/4be52dba53c272ac4c03c925c2e73cd4ced733fc))
- *(tui)* Overlay loading and error dialogs on background layout ([0ba99a7](https://github.com/nospor/gitlab-tui/commit/0ba99a76f221dd1999081773a6049378cf2d4b1e))
- *(tui)* Skip confirmation popup for MR vote up/down (+/-) ([f551767](https://github.com/nospor/gitlab-tui/commit/f5517672e8f5efede1cdced8e65d2992ce4141ed))
- Support opening MR directly via URL parameter and add --help flag ([38b5446](https://github.com/nospor/gitlab-tui/commit/38b54468ce203c8bafa48de20ddea89e5020bcfc))
- *(gitlab)* Parse and convert HTML discussion comments to markdown/text ([abe50c2](https://github.com/nospor/gitlab-tui/commit/abe50c28d31e834b2773d862a159c3e2b87e4385))
- *(tui)* Expand comment composer to multiline textarea and fix overlay transparency ([75f690c](https://github.com/nospor/gitlab-tui/commit/75f690cc4de9c1229aadd778118a474427157707))
- Add link selector overlay and browser_command config ([3e7d948](https://github.com/nospor/gitlab-tui/commit/3e7d94887039610d6504513aafcb4a3af65fd730))
- Add YouTrack issue URL recognition in details view ([f129e3d](https://github.com/nospor/gitlab-tui/commit/f129e3d298bf0408678845290562983eb800912e))
- Support opening YouTrack URLs via custom command in foreground ([0c4eaa2](https://github.com/nospor/gitlab-tui/commit/0c4eaa23026575c321fd76683863020064aa3ebe))

### Bug Fixes

- Proper server comparing ([85029e6](https://github.com/nospor/gitlab-tui/commit/85029e66b316be827b347888c31ebd1004c5b015))
- *(gitlab)* Enable search_namespaces when searching projects ([21529a2](https://github.com/nospor/gitlab-tui/commit/21529a237a74ec85062289974a7eb68c27e5a228))
- *(gitlab)* Resolve diff loading issues for large merge requests ([f56189a](https://github.com/nospor/gitlab-tui/commit/f56189a7d9bcc7610394cccbb692cb3384be3b43))
- *(tui)* Resolve terminal viewport overflow and scroll issues in split diff view ([74c6643](https://github.com/nospor/gitlab-tui/commit/74c6643b45cbe319f367735429fb77395ee663db))

### Refactor

- *(tui)* Remove project select popup and integrate inline search in Projects tab ([c940ab6](https://github.com/nospor/gitlab-tui/commit/c940ab63ff09a30d99bd5fcca6a63e2b4cc231a5))

### Miscellaneous Tasks

- Add git-cliff configuration and changelog workflow ([2d461c4](https://github.com/nospor/gitlab-tui/commit/2d461c42d307d12d7801cbc4034b142f904ea431))

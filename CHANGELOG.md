
## [0.6.5] - 2026-07-19

### Features

- *(tui)* Add Branches tab with delete, create MR, commits, and compare features ([6fa0a0b](https://github.com/nospor/gitlab-tui/commit/6fa0a0bfe49cca6c264de585f0156d3fc7d02588))

            - Add a new Branches tab (hotkey 2) between Merge Requests and
            Pipelines.
            - Implement branch listing with windowed scrolling for large
            repositories.
            - Add branch deletion with a confirmation overlay dialog.
            - Implement skip-source branch wizard for creating Merge Requests
            directly from a branch.
            - Add commits view for branches showing commit history (Short SHA,
            Author, Date, Title).
            - Implement side-by-side branch comparison view showing unique commits
            and changed files, with file diff expansion.
- *(tui)* Add interactive commit diff panel in branch view ([9b6b566](https://github.com/nospor/gitlab-tui/commit/9b6b56696caeb6c00a5f0d7a60e51b511d8791cd))

            - Add Tab key support in branch commits view to open/close the diff
            panel for the selected commit.
             - Implement Gitlab API endpoint wrapper to fetch commit diffs with
            paging support.
            - Support scrolling diff lines (j/k), switching modified files (n/p),
            and hunk jumping (J/K) when the panel is open.
            - Add split layout view for branch commits and dynamic key hints in the
            footer.
- *(tui)* Align branch compare view behavior with branch commits view ([65dbe5f](https://github.com/nospor/gitlab-tui/commit/65dbe5ffcf32b77bcb8847bcbd30125dc9bcc59a))

            - Replace the side-by-side commits/changed files panes in the compare
            view with a single commits list.
            - Allow pressing Tab on a commit in the compare view to load and open
            its diff panel, matching the standard branch commits view.
            - Enable full diff panel navigation (j/k to scroll lines, n/p for files,
            J/K for hunks) in compare view.
- *(tui)* Add capability to reopen closed merge requests ([d646818](https://github.com/nospor/gitlab-tui/commit/d646818e40c357850c6366a11a35e52086df46ed))
- *(tui)* Auto-refresh lists after detail actions & preserve cursor position ([13e124b](https://github.com/nospor/gitlab-tui/commit/13e124b1de5d69264bd00ad2d34b03361f8d2507))

### Bug Fixes

- *(tui)* Allow space character in merge request title input ([2e00cb9](https://github.com/nospor/gitlab-tui/commit/2e00cb93ea396326f21b1d3b14b4ec96d2ebcf9c))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.4 [skip ci] ([5d7eb90](https://github.com/nospor/gitlab-tui/commit/5d7eb90225edce71073a42ec4e9edff4d3046846))

## [0.6.4] - 2026-07-17

### Other

- Make install path configurable ([08e3bce](https://github.com/nospor/gitlab-tui/commit/08e3bce59c0ed56e92cbaa7f7b1624c63d578c1a))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.3 [skip ci] ([1a0253a](https://github.com/nospor/gitlab-tui/commit/1a0253a0021a2da72383888d50e55c4c17e8ebb7))

## [0.6.3] - 2026-07-17

### Features

- Add Create MR wizard to MR tab ([f468336](https://github.com/nospor/gitlab-tui/commit/f4683369a48daa8e684c0633dbf1073f9b30b1dd))
- Add merge request editing in MR list and detail views ([7f66758](https://github.com/nospor/gitlab-tui/commit/7f667584931fea4d2164ca52ab8047d9c489c9e0))
- *(tui)* Add MR pipelines display and jump navigation ([c56bb37](https://github.com/nospor/gitlab-tui/commit/c56bb37054d9663ae572e16d9fb3981c2832a24b))

            - Fetch and display pipelines (MR pipelines, source branch pipelines,
            head commit pipelines, and merge commit pipelines) on the Merge Request
            details screen.
            - Press 'p' to open a selection popup containing the list of pipelines.

### Bug Fixes

- Restore footer help bar in main view after closing edit popup ([128feb1](https://github.com/nospor/gitlab-tui/commit/128feb18c2e643d2f133b5b35953e4d78f25691b))
- *(tui)* Align list columns by padding status badges to a fixed width ([6e7890d](https://github.com/nospor/gitlab-tui/commit/6e7890d8e9dc253b8333161831b35996ffa4308d))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.2 [skip ci] ([071712a](https://github.com/nospor/gitlab-tui/commit/071712a2a87b5c96c30146c8f923b481e3d5434b))

## [0.6.2] - 2026-07-17

### Bug Fixes

- Anchor binary ignore pattern in gitignore ([c5dc0a9](https://github.com/nospor/gitlab-tui/commit/c5dc0a964f746bbeb6e0387c8803b0b13c826c1e))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.1 [skip ci] ([aa2e8f2](https://github.com/nospor/gitlab-tui/commit/aa2e8f278f682cdef90bec7eefa7432f966e524e))

## [0.6.1] - 2026-07-17

### Other

- Point Makefile build target directly to main.go file ([aff2d90](https://github.com/nospor/gitlab-tui/commit/aff2d9062999222e09c0ac480e889db1b96a002f))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.0 [skip ci] ([a110a54](https://github.com/nospor/gitlab-tui/commit/a110a5430e702e35d44102445db17ef4ad9d3b44))

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

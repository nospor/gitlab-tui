
## [unreleased]

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

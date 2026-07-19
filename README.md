# gitlab-tui

A terminal UI (TUI) application for managing GitLab projects — Merge Requests, Pipelines, and Issues — from your terminal.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea) and the official [GitLab API client](https://gitlab.com/gitlab-org/api/client-go).

## Features

- 🔀 **Merge Requests** — list, filter by state (open/merged/closed), view details, approve, merge, close, edit existing MRs, and **create new MRs** via an interactive wizard
- 🌿 **Branches** — list branches, delete branch (with confirmation), create MR directly from a branch, view commits history, and compare branches (showing commit differences and changed files diff)
- 🚀 **Pipelines** — list, view details (with dual-pane layout showing jobs and statuses), automatic background refresh (every 5s) for active pipelines, retry/cancel pipelines, restart individual jobs, scroll/search job trace logs, and open traces directly in your external editor
- 🐛 **Issues** — list, view details
- 📁 **Projects** — browse and switch projects on the current server
- 🔌 **Multiple servers** — configure several GitLab instances, switch between them
- 🧠 **Auto-detection** — detects server and project from the current directory's `git remote`
- 🔗 **Open links** — press `o` on MR, pipeline, or issue detail to see all links (WebURL, links in descriptions and comments) and open them in your browser
- 🎫 **YouTrack integration** — automatically parses issue tracker keys (like `PROJ-XXXX`) in descriptions and comments, resolving them to YouTrack URLs inside the link selection menu
- 🎨 **Themes** — support for `"catppuccin"` (default dark theme with purple/indigo accents) and `"teams"` (green borders, dark grey panels, purple highlights)

## Installation

To build and install the binary:

```bash
# Install to /usr/local/bin (may require sudo)
make install

# Or install to a custom path (e.g. ~/.local/bin) without sudo
make install PREFIX=$HOME/.local
```

Or just build without installing:

```bash
make build
./gitlab-tui
```

## Usage

You can start `gitlab-tui` in the current directory's auto-detected project (or select a project inside the TUI):

```bash
gitlab-tui
```

### Opening specific resources directly

You can pass a GitLab URL (Merge Request, Pipeline, or Job) as an argument to open that resource directly on startup:

```bash
# Open a merge request directly
gitlab-tui https://gitlab.com/group/project/-/merge_requests/123

# Open a pipeline directly
gitlab-tui https://gitlab.com/group/project/-/pipelines/33780

# Open a job trace directly
gitlab-tui https://gitlab.com/group/project/-/jobs/155933
```

## Configuration

On first run, a sample config is created at:

```
~/.config/gitlab-tui/config.json
```

Edit it to add your servers:

```json
{
  "servers": [
    {
      "name": "gitlab.com",
      "url": "https://gitlab.com",
      "token": "glpat-xxxxxxxxxxxxxxxxxxxx",
      "default": true
    },
    {
      "name": "company-gitlab",
      "url": "https://gitlab.company.com",
      "token": "glpat-xxxxxxxxxxxxxxxxxxxx"
    }
  ],
  "youtrack_servers": [
    {
      "name": "company-youtrack",
      "url": "https://youtrack.company.com",
      "projects": ["PROJ1", "PROJ2"]
    }
  ],
  "browser_command": "xdg-open",
  "youtrack_command": "yt-tui",
  "theme": "catppuccin"
}
```

| Option             | Default        | Description                                                                                                                                                                                                          |
| ------------------ | -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `servers`          | —              | List of GitLab server configurations                                                                                                                                                                                 |
| `youtrack_servers` | —              | List of YouTrack server configurations to resolve ticket keys to URLs (each server has `name`, `url`, and a list of `projects`)                                                                                      |
| `browser_command`  | `"xdg-open"`   | Command to open URLs (e.g. `"firefox"`, `"google-chrome"`, `"brave-browser"`). Leave as `"xdg-open"` to use your system default browser.                                                                             |
| `youtrack_command` | —              | Command to open YouTrack URLs specifically (e.g. [`yt-tui`](https://github.com/nospor/yt-tui)). When set, any URL belonging to a configured YouTrack server is opened via this command instead of `browser_command`. |
| `theme`            | `"catppuccin"` | TUI theme. Supported values: `"catppuccin"` (default) or `"teams"` (green borders, dark grey panels, purple highlights).                                                                                             |

You need a **Personal Access Token** with at least `api` scope. Create one at:
`https://gitlab.com/-/user_settings/personal_access_tokens`

## Auto-detection

When you run `gitlab-tui` from inside a git repository, it reads `git remote get-url origin` and matches it against your configured servers.

Both SSH and HTTPS remotes are supported:
- `git@gitlab.com:mygroup/myproject.git`
- `https://gitlab.com/mygroup/myproject.git`

## Keyboard Shortcuts

### Main view

| Key               | Action                                                      |
| ----------------- | ----------------------------------------------------------- |
| `Tab` / `→`       | Next tab                                                    |
| `Shift+Tab` / `←` | Previous tab                                                |
| `1–5`             | Jump to tab (MRs, Branches, Pipelines, Issues, Projects)    |
| `j` / `↓`         | Move down                                                   |
| `k` / `↑`         | Move up                                                     |
| `Enter`           | Open detail / select project / show branch commits          |
| `s`               | Cycle MR state filter (open→merged→closed→all)              |
| `c`               | Create new MR (MR and Branches tabs)                        |
| `C`               | Compare branch with another (Branches tab only)             |
| `d`               | Delete branch (Branches tab only)                           |
| `e`               | Edit selected MR (MR tab only)                              |
| `x`               | Close selected MR (MR tab only)                             |
| `O`               | Reopen selected MR (MR tab only)                            |
| `r`               | Refresh current tab                                         |
| `n` / `p`         | Next / previous page                                        |
| `P`               | Switch project                                              |
| `S`               | Switch server                                               |
| `q` / `Ctrl+C`    | Quit                                                        |

### MR detail

| Key       | Action                 |
| --------- | ---------------------- |
| `j` / `k` | Scroll detail          |
| `Tab`     | Toggle changes panel   |
| `C`       | Comment                |
| `e`       | Edit MR                |
| `a`       | Approve MR             |
| `m`       | Merge MR               |
| `x`       | Close MR               |
| `O`       | Reopen MR (closed MRs) |
| `+` / `-` | Vote up / down         |
| `o`       | Open link selector     |
| `p`       | Open pipeline selector |
| `Esc`     | Back to list           |

### Branch commits & Compare detail

| Key       | Action                                                             |
| --------- | ------------------------------------------------------------------ |
| `j` / `k` | Scroll commits list / changed files list / diff lines              |
| `Tab`     | Toggle focus between commits panel and files panel (compare view)  |
| `Enter`   | View branch commits (main list) / expand file diff (compare files) |
| `Esc`     | Go back (exit diff, exit comparison list, or return to branch list)|


### Create MR wizard

Press `c` on the MR list tab to open the Create MR wizard. It guides you through three steps:

### Edit MR popup

Press `e` on either the MR list tab (for the highlighted MR) or the MR detail page to open the Edit MR popup. It loads the MR's current fields:
- **Title** (excluding any `Draft:` or `WIP:` prefix)
- **Mark as Draft** checkbox
- **Description**
- **Delete source branch** checkbox
- **Squash commits** checkbox

Using the same form navigation keys as the Create MR wizard (`Tab`, `Shift+Tab`, `Space`, `Ctrl+S`, `Esc`), you can modify these details and save your changes.

**Step 1 — Source branch**
Pick the branch you want to merge from. All project branches are shown in a scrollable list.

**Step 2 — Target branch**
Pick the destination branch. The source branch is excluded from the list. The repository's default branch is pre-selected.

**Step 3 — Details form**

| Field                | Default                 | Notes                                               |
| -------------------- | ----------------------- | --------------------------------------------------- |
| Title                | Branch name (humanized) | Pre-filled from source branch name                  |
| Mark as Draft        | ☐ Off                   | Prefixes title with `Draft:`                        |
| Description          | Last commit body        | Auto-fetched from the source branch's latest commit |
| Delete source branch | ✓ On                    | Removes branch after merge                          |
| Squash commits       | ☐ Off                   | Squashes all commits on merge                       |

**Form navigation:**

| Key               | Action                       |
| ----------------- | ---------------------------- |
| `Tab`             | Next field                   |
| `Shift+Tab`       | Previous field               |
| `Space` / `Enter` | Toggle focused checkbox      |
| `Ctrl+S`          | Submit and create the MR     |
| `Esc`             | Go back one step (or cancel) |

> **Note:** When the Description textarea is focused, `Enter` inserts a newline. Use `Ctrl+S` to submit, or `Tab` / `Shift+Tab` to leave the textarea.

### Pipeline detail

| Key       | Action                                             |
| --------- | -------------------------------------------------- |
| `j` / `k` | Select job / Navigate jobs list                    |
| `Enter`   | View trace/log output for the selected job         |
| `r`       | Restart/retry selected job (requires confirmation) |
| `R`       | Retry entire pipeline                              |
| `c`       | Cancel pipeline                                    |
| `o`       | Open link selector                                 |
| `Esc`     | Back to list (or close trace view if open)         |

**Inside trace view:**
- `j` / `k` (or down/up): Scroll trace log lines
- `g` / `G`: Scroll trace to top / bottom
- `Ctrl+G`: Open trace in external editor (reads `$EDITOR`, falls back to `vim`)
- `Esc` / `Enter` / `Tab`: Close trace view

### Issue detail

| Key   | Action             |
| ----- | ------------------ |
| `o`   | Open link selector |
| `Esc` | Back to list       |

## Project structure

```
.
├── cmd/gitlab-tui/main.go     # Entrypoint
├── internal/
│   ├── config/config.go       # Config loading + git auto-detection
│   ├── gitlab/client.go       # GitLab API wrapper
│   └── tui/
│       ├── model.go           # BubbleTea model (update + view)
│       └── styles.go          # Lipgloss styles + colors
└── Makefile
```

## License

See [LICENSE](LICENSE).

## Thanks For Visiting
Hope you liked it. Wanna **[buy Me a coffee](https://www.buymeacoffee.com/nospor)**?


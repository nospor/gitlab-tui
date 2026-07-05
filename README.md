# gitlab-tui

A terminal UI (TUI) application for managing GitLab projects — Merge Requests, Pipelines, and Issues — from your terminal.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea) and the official [GitLab API client](https://gitlab.com/gitlab-org/api/client-go).

## Features

- 🔀 **Merge Requests** — list, filter by state (open/merged/closed), view details, approve, merge, close
- 🚀 **Pipelines** — list, view details, retry, cancel
- 🐛 **Issues** — list, view details
- 📁 **Projects** — browse and switch projects on the current server
- 🔌 **Multiple servers** — configure several GitLab instances, switch between them
- 🧠 **Auto-detection** — detects server and project from the current directory's `git remote`
- 🔗 **Open links** — press `o` on MR, pipeline, or issue detail to see all links (WebURL, links in descriptions and comments) and open them in your browser

## Installation

```bash
make install
```

Or just build:

```bash
make build
./gitlab-tui
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
  "browser_command": "xdg-open"
}
```

| Option            | Default      | Description                                                                                                                              |
| ----------------- | ------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `servers`         | —            | List of GitLab server configurations                                                                                                     |
| `browser_command` | `"xdg-open"` | Command to open URLs (e.g. `"firefox"`, `"google-chrome"`, `"brave-browser"`). Leave as `"xdg-open"` to use your system default browser. |

You need a **Personal Access Token** with at least `api` scope. Create one at:
`https://gitlab.com/-/user_settings/personal_access_tokens`

## Auto-detection

When you run `gitlab-tui` from inside a git repository, it reads `git remote get-url origin` and matches it against your configured servers.

Both SSH and HTTPS remotes are supported:
- `git@gitlab.com:mygroup/myproject.git`
- `https://gitlab.com/mygroup/myproject.git`

## Keyboard Shortcuts

### Main view

| Key               | Action                                         |
| ----------------- | ---------------------------------------------- |
| `Tab` / `→`       | Next tab                                       |
| `Shift+Tab` / `←` | Previous tab                                   |
| `1–4`             | Jump to tab (MRs, Pipelines, Issues, Projects) |
| `j` / `↓`         | Move down                                      |
| `k` / `↑`         | Move up                                        |
| `Enter`           | Open detail / select project                   |
| `s`               | Cycle MR state filter (open→merged→closed→all) |
| `r`               | Refresh current tab                            |
| `n` / `p`         | Next / previous page                           |
| `P`               | Switch project                                 |
| `S`               | Switch server                                  |
| `q` / `Ctrl+C`    | Quit                                           |

### MR detail

| Key       | Action               |
| --------- | -------------------- |
| `j` / `k` | Scroll detail        |
| `Tab`     | Toggle changes panel |
| `C`       | Comment              |
| `a`       | Approve MR           |
| `m`       | Merge MR             |
| `x`       | Close MR             |
| `+` / `-` | Vote up / down       |
| `o`       | Open link selector   |
| `Esc`     | Back to list         |

### Pipeline detail

| Key   | Action             |
| ----- | ------------------ |
| `r`   | Retry pipeline     |
| `c`   | Cancel pipeline    |
| `o`   | Open link selector |
| `Esc` | Back to list       |

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

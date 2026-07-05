package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Server represents a configured GitLab server entry.
type Server struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Token   string `json:"token"`
	Default bool   `json:"default,omitempty"`
}

// Config is the top-level configuration structure.
type Config struct {
	Servers        []Server `json:"servers"`
	BrowserCommand string   `json:"browser_command,omitempty"`
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gitlab-tui", "config.json")
}

// defaultBrowserCommand is the fallback command for opening URLs.
const defaultBrowserCommand = "xdg-open"

// Load reads config from disk. Returns an empty config if the file doesn't exist.
func Load() (*Config, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Fill in defaults and persist so the user can see available options.
	if cfg.BrowserCommand == "" {
		cfg.BrowserCommand = defaultBrowserCommand
		if err := Save(&cfg); err != nil {
			return nil, fmt.Errorf("saving defaults: %w", err)
		}
	}

	return &cfg, nil
}

// Save writes config to disk.
func Save(cfg *Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// EnsureDefaultConfig creates a sample config if none exists.
func EnsureDefaultConfig() error {
	path := ConfigPath()
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	sample := Config{
		Servers: []Server{
			{
				Name:    "gitlab.com",
				URL:     "https://gitlab.com",
				Token:   "YOUR_PERSONAL_ACCESS_TOKEN",
				Default: true,
			},
		},
		BrowserCommand: defaultBrowserCommand,
	}
	return Save(&sample)
}

// DetectedProject holds info auto-detected from git remote.
type DetectedProject struct {
	ServerName  string
	ServerURL   string
	ProjectPath string // e.g. "group/project"
}

// DetectFromGit tries to find a matching server+project from any git remote
// in the current working directory. Returns detected info + the raw remote URL for debugging.
func DetectFromGit(cfg *Config) (*DetectedProject, string) {
	// Collect all remotes, prefer "origin" first
	out, err := exec.Command("git", "remote").Output()
	if err != nil {
		return nil, ""
	}
	remotes := strings.Fields(strings.TrimSpace(string(out)))
	// Sort so "origin" is first
	sort.Slice(remotes, func(i, _ int) bool { return remotes[i] == "origin" })

	for _, remoteName := range remotes {
		urlOut, err := exec.Command("git", "remote", "get-url", remoteName).Output()
		if err != nil {
			continue
		}
		rawRemote := strings.TrimSpace(string(urlOut))
		if rawRemote == "" {
			continue
		}

		if p := matchRemote(rawRemote, cfg); p != nil {
			return p, rawRemote
		}
	}
	return nil, ""
}

// matchRemote checks a single remote URL against all configured servers.
func matchRemote(remote string, cfg *Config) *DetectedProject {
	// Parse the remote URL to normalise it
	parsedURL, _ := url.Parse(remote)

	for _, srv := range cfg.Servers {
		srvParsed, err := url.Parse(srv.URL)
		if err != nil {
			continue
		}
		// srvHost includes port if present (e.g. "gitlab.example.com:10443")
		srvHost := strings.ToLower(strings.TrimRight(srvParsed.Host, "/"))
		// srvHostname is just the hostname without port (for SSH remote matching)
		srvHostname := strings.ToLower(srvParsed.Hostname())

		var projectPath string

		// ── SSH formats ──────────────────────────────────────────────────────
		// git@gitlab.com:group/project.git
		// ssh://git@gitlab.com/group/project.git
		if parsedURL != nil && parsedURL.Scheme == "ssh" {
			host := strings.ToLower(parsedURL.Hostname())
			if host == srvHostname || host == srvHost {
				projectPath = strings.TrimPrefix(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
			}
		}
		// SCP-like: git@gitlab.com:group/project.git
		if strings.Contains(remote, "@") && strings.Contains(remote, ":") && !strings.Contains(remote, "://") {
			parts := strings.SplitN(remote, "@", 2)
			if len(parts) == 2 {
				hostAndPath := parts[1]
				colonIdx := strings.Index(hostAndPath, ":")
				if colonIdx >= 0 {
					host := strings.ToLower(hostAndPath[:colonIdx])
					path := strings.TrimSuffix(hostAndPath[colonIdx+1:], ".git")
					// Match against both "host:port" and just "host"
					if host == srvHostname || host == srvHost {
						projectPath = path
					}
				}
			}
		}

		// ── HTTPS formats ────────────────────────────────────────────────────
		// https://gitlab.com/group/project.git
		// https://user:token@gitlab.com/group/project.git
		if parsedURL != nil && (parsedURL.Scheme == "https" || parsedURL.Scheme == "http") {
			host := strings.ToLower(parsedURL.Host)
			// Strip userinfo (credentials embedded in URL)
			if hi := strings.Index(host, "@"); hi >= 0 {
				host = host[hi+1:]
			}
			if host == srvHost || host == srvHostname {
				projectPath = strings.TrimPrefix(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
			}
		}

		if projectPath != "" {
			return &DetectedProject{
				ServerName:  srv.Name,
				ServerURL:   srv.URL,
				ProjectPath: projectPath,
			}
		}
	}
	return nil
}


// DefaultServer returns the first server marked default, or the first server.
func (c *Config) DefaultServer() *Server {
	for i, s := range c.Servers {
		if s.Default {
			return &c.Servers[i]
		}
	}
	if len(c.Servers) > 0 {
		return &c.Servers[0]
	}
	return nil
}

// ParseGitLabMRURL parses a GitLab merge request URL and matches it against configured servers.
// It returns the server index, project path, and MR IID.
func ParseGitLabMRURL(cfg *Config, rawURL string) (int, string, int, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return -1, "", 0, fmt.Errorf("parsing URL: %w", err)
	}

	// 1. Find the server match
	serverIdx := -1
	uHost := strings.ToLower(u.Host)
	uHostname := strings.ToLower(u.Hostname())

	for i, s := range cfg.Servers {
		srvParsed, err := url.Parse(s.URL)
		if err != nil {
			continue
		}
		srvHost := strings.ToLower(strings.TrimRight(srvParsed.Host, "/"))
		srvHostname := strings.ToLower(srvParsed.Hostname())

		if uHost == srvHost || uHostname == srvHostname {
			serverIdx = i
			break
		}
	}

	if serverIdx == -1 {
		return -1, "", 0, fmt.Errorf("no configured server URL or host matches %q", u.Host)
	}

	// 2. Extract project path and MR IID
	// Formats expected: /<project-path>/-/merge_requests/<iid> or /<project-path>/merge_requests/<iid>
	mrIdx := strings.Index(u.Path, "/-/merge_requests/")
	var delimiterLen int
	if mrIdx >= 0 {
		delimiterLen = len("/-/merge_requests/")
	} else {
		mrIdx = strings.Index(u.Path, "/merge_requests/")
		if mrIdx >= 0 {
			delimiterLen = len("/merge_requests/")
		}
	}

	if mrIdx < 0 {
		return -1, "", 0, fmt.Errorf("URL does not appear to be a GitLab merge request URL (missing '/merge_requests/')")
	}

	projectPath := strings.TrimPrefix(u.Path[:mrIdx], "/")
	if projectPath == "" {
		return -1, "", 0, fmt.Errorf("could not extract project path from URL")
	}

	mrPart := u.Path[mrIdx+delimiterLen:]
	if qIdx := strings.IndexAny(mrPart, "?#"); qIdx >= 0 {
		mrPart = mrPart[:qIdx]
	}
	mrPart = strings.Trim(mrPart, "/")
	if slashIdx := strings.Index(mrPart, "/"); slashIdx >= 0 {
		mrPart = mrPart[:slashIdx]
	}

	if mrPart == "" {
		return -1, "", 0, fmt.Errorf("could not extract merge request IID from URL")
	}

	mrIID, err := strconv.Atoi(mrPart)
	if err != nil {
		return -1, "", 0, fmt.Errorf("invalid merge request IID %q: %w", mrPart, err)
	}

	return serverIdx, projectPath, mrIID, nil
}


package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	Servers []Server `json:"servers"`
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gitlab-tui", "config.json")
}

// Load reads config from disk. Creates a default if not present.
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
		srvHost := strings.ToLower(strings.TrimRight(srvParsed.Host, "/"))

		var projectPath string

		// ── SSH formats ──────────────────────────────────────────────────────
		// git@gitlab.com:group/project.git
		// ssh://git@gitlab.com/group/project.git
		if parsedURL != nil && parsedURL.Scheme == "ssh" {
			host := strings.ToLower(parsedURL.Host)
			if host == srvHost {
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
					if host == srvHost {
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
			if host == srvHost {
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

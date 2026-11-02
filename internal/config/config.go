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

// YouTrackServer represents a configured YouTrack server entry.
type YouTrackServer struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Projects []string `json:"projects"`
}

// Config is the top-level configuration structure.
type Config struct {
	Servers         []Server         `json:"servers"`
	YouTrackServers []YouTrackServer `json:"youtrack_servers,omitempty"`
	BrowserCommand  string           `json:"browser_command,omitempty"`
	YouTrackCommand string           `json:"youtrack_command,omitempty"`
	Theme           string           `json:"theme,omitempty"`
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
	dirty := false
	if cfg.BrowserCommand == "" {
		cfg.BrowserCommand = defaultBrowserCommand
		dirty = true
	}
	if cfg.Theme == "" {
		cfg.Theme = "catppuccin"
		dirty = true
	}
	if dirty {
		if err := Save(&cfg); err != nil {
			return nil, fmt.Errorf("saving defaults: %w", err)
		}
	}

	// Expand $VAR tokens after saving defaults so secrets are not written back.
	if err := resolveTokens(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// dotenvPath is the env file next to config.json.
func dotenvPath() string {
	return filepath.Join(filepath.Dir(ConfigPath()), ".env")
}

// envRef reports whether token is a whole-value reference ($NAME or ${NAME}).
func envRef(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if len(token) < 2 || token[0] != '$' {
		return "", false
	}
	name := token[1:]
	if strings.HasPrefix(name, "{") && strings.HasSuffix(name, "}") {
		name = name[1 : len(name)-1]
	}
	if !validEnvName(name) {
		return "", false
	}
	return name, true
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// loadDotEnv reads a simple KEY=VALUE file. Missing file is reported as os.ErrNotExist.
func loadDotEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !validEnvName(key) {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		env[key] = val
	}
	return env, nil
}

// resolveTokens replaces $VAR server tokens from the .env file beside the config,
// then from the process environment. A value present in the file wins.
// Literal tokens are left unchanged.
func resolveTokens(cfg *Config) error {
	path := dotenvPath()
	fileEnv, err := loadDotEnv(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return resolveServerTokens(cfg.Servers, fileEnv, path, os.Getenv)
}

func resolveServerTokens(servers []Server, fileEnv map[string]string, envPath string, lookup func(string) string) error {
	for i := range servers {
		name, ok := envRef(servers[i].Token)
		if !ok {
			continue
		}
		if fileEnv != nil {
			if val, found := fileEnv[name]; found {
				if val == "" {
					return fmt.Errorf("server %q: token $%s is empty in %s", servers[i].Name, name, envPath)
				}
				servers[i].Token = val
				continue
			}
		}
		val := ""
		if lookup != nil {
			val = lookup(name)
		}
		if val == "" {
			return fmt.Errorf("server %q: token $%s is not set in %s or the environment", servers[i].Name, name, envPath)
		}
		servers[i].Token = val
	}
	return nil
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
		Theme:          "catppuccin",
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

// ParseGitLabURL parses a GitLab URL (MR, pipeline, or job) and matches it against configured servers.
// It returns the server index, project path, MR IID, pipeline ID, job ID, and error.
func ParseGitLabURL(cfg *Config, rawURL string) (int, string, int, int, int64, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return -1, "", 0, 0, 0, fmt.Errorf("parsing URL: %w", err)
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
		return -1, "", 0, 0, 0, fmt.Errorf("no configured server URL or host matches %q", u.Host)
	}

	// 2. Extract project path and resource type/ID
	// Formats expected:
	//   /<project-path>/-/merge_requests/<iid> or /<project-path>/merge_requests/<iid>
	//   /<project-path>/-/pipelines/<id> or /<project-path>/pipelines/<id>
	//   /<project-path>/-/jobs/<id> or /<project-path>/jobs/<id>
	var resourceType string
	var delimiterLen int
	var resourceIdx int

	delimiters := []struct {
		resType string
		delims  []string
	}{
		{"merge_requests", []string{"/-/merge_requests/", "/merge_requests/"}},
		{"pipelines", []string{"/-/pipelines/", "/pipelines/"}},
		{"jobs", []string{"/-/jobs/", "/jobs/"}},
	}

	for _, d := range delimiters {
		for _, delim := range d.delims {
			idx := strings.Index(u.Path, delim)
			if idx >= 0 {
				resourceType = d.resType
				delimiterLen = len(delim)
				resourceIdx = idx
				break
			}
		}
		if resourceType != "" {
			break
		}
	}

	if resourceType == "" {
		return -1, "", 0, 0, 0, fmt.Errorf("URL does not appear to be a GitLab merge request, pipeline, or job URL")
	}

	projectPath := strings.TrimPrefix(u.Path[:resourceIdx], "/")
	if projectPath == "" {
		return -1, "", 0, 0, 0, fmt.Errorf("could not extract project path from URL")
	}

	part := u.Path[resourceIdx+delimiterLen:]
	if qIdx := strings.IndexAny(part, "?#"); qIdx >= 0 {
		part = part[:qIdx]
	}
	part = strings.Trim(part, "/")
	if slashIdx := strings.Index(part, "/"); slashIdx >= 0 {
		part = part[:slashIdx]
	}

	if part == "" {
		return -1, "", 0, 0, 0, fmt.Errorf("could not extract ID from URL")
	}

	idVal, err := strconv.ParseInt(part, 10, 64)
	if err != nil {
		return -1, "", 0, 0, 0, fmt.Errorf("invalid ID %q: %w", part, err)
	}

	var mrIID int
	var pipelineID int
	var jobID int64

	switch resourceType {
	case "merge_requests":
		mrIID = int(idVal)
	case "pipelines":
		pipelineID = int(idVal)
	case "jobs":
		jobID = idVal
	}

	return serverIdx, projectPath, mrIID, pipelineID, jobID, nil
}

// GetYouTrackURL checks if a project key is configured for a YouTrack server,
// and if so, returns the built issue URL (e.g. https://youtrack.example.com/issue/PROJ-123).
func (c *Config) GetYouTrackURL(issueKey string) (string, bool) {
	if c == nil {
		return "", false
	}
	parts := strings.Split(issueKey, "-")
	if len(parts) != 2 {
		return "", false
	}
	proj := strings.ToUpper(parts[0])
	num := parts[1]

	// Verify that num is non-empty and consists only of digits
	if len(num) == 0 {
		return "", false
	}
	for _, char := range num {
		if char < '0' || char > '9' {
			return "", false
		}
	}

	for _, srv := range c.YouTrackServers {
		for _, p := range srv.Projects {
			if strings.ToUpper(p) == proj {
				serverURL := strings.TrimRight(srv.URL, "/")
				return fmt.Sprintf("%s/issue/%s-%s", serverURL, proj, num), true
			}
		}
	}
	return "", false
}

// IsYouTrackURL checks if the given URL belongs to any configured YouTrack server.
func (c *Config) IsYouTrackURL(rawURL string) bool {
	if c == nil {
		return false
	}
	rawURL = strings.ToLower(rawURL)
	for _, srv := range c.YouTrackServers {
		srvURL := strings.ToLower(strings.TrimRight(srv.URL, "/"))
		if srvURL != "" && (strings.HasPrefix(rawURL, srvURL+"/") || rawURL == srvURL) {
			return true
		}
	}
	return false
}



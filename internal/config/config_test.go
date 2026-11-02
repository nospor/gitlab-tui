package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitLabURL(t *testing.T) {
	cfg := &Config{
		Servers: []Server{
			{
				Name: "gitlab.mediatel.co.uk",
				URL:  "https://gitlab.mediatel.co.uk",
			},
			{
				Name: "gitlab.com",
				URL:  "https://gitlab.com",
			},
		},
	}

	tests := []struct {
		name             string
		url              string
		expectedIdx      int
		expectedPrj      string
		expectedMRIID    int
		expectedPipeline int
		expectedJob      int64
		expectError      bool
	}{
		{
			name:          "Standard URL mediatel MR",
			url:           "https://gitlab.mediatel.co.uk/audio/audiotrack-admin-hub/-/merge_requests/25",
			expectedIdx:   0,
			expectedPrj:   "audio/audiotrack-admin-hub",
			expectedMRIID: 25,
			expectError:   false,
		},
		{
			name:          "URL without HTTPS prefix MR",
			url:           "gitlab.mediatel.co.uk/audio/audiotrack-admin-hub/-/merge_requests/25",
			expectedIdx:   0,
			expectedPrj:   "audio/audiotrack-admin-hub",
			expectedMRIID: 25,
			expectError:   false,
		},
		{
			name:          "Standard URL gitlab.com MR",
			url:           "https://gitlab.com/gitlab-org/gitlab/-/merge_requests/12345/diffs?view=inline",
			expectedIdx:   1,
			expectedPrj:   "gitlab-org/gitlab",
			expectedMRIID: 12345,
			expectError:   false,
		},
		{
			name:          "Old style merge request URL (no /-/ )",
			url:           "https://gitlab.com/group/subgroup/project/merge_requests/99",
			expectedIdx:   1,
			expectedPrj:   "group/subgroup/project",
			expectedMRIID: 99,
			expectError:   false,
		},
		{
			name:             "Pipeline URL",
			url:              "https://gitlab.mediatel.co.uk/adwanted/srds/-/pipelines/33780",
			expectedIdx:      0,
			expectedPrj:      "adwanted/srds",
			expectedPipeline: 33780,
			expectError:      false,
		},
		{
			name:        "Job URL",
			url:         "https://gitlab.mediatel.co.uk/adwanted/srds/-/jobs/155933",
			expectedIdx: 0,
			expectedPrj: "adwanted/srds",
			expectedJob: 155933,
			expectError: false,
		},
		{
			name:        "Invalid server",
			url:         "https://github.com/group/project/-/merge_requests/25",
			expectError: true,
		},
		{
			name:        "Not a valid URL",
			url:         "https://gitlab.com/group/project",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, prj, mrIID, pipelineID, jobID, err := ParseGitLabURL(cfg, tt.url)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if idx != tt.expectedIdx {
					t.Errorf("expected server index %d, got %d", tt.expectedIdx, idx)
				}
				if prj != tt.expectedPrj {
					t.Errorf("expected project path %q, got %q", tt.expectedPrj, prj)
				}
				if mrIID != tt.expectedMRIID {
					t.Errorf("expected MR IID %d, got %d", tt.expectedMRIID, mrIID)
				}
				if pipelineID != tt.expectedPipeline {
					t.Errorf("expected Pipeline ID %d, got %d", tt.expectedPipeline, pipelineID)
				}
				if jobID != tt.expectedJob {
					t.Errorf("expected Job ID %d, got %d", tt.expectedJob, jobID)
				}
			}
		})
	}
}

func TestGetYouTrackURL(t *testing.T) {
	cfg := &Config{
		YouTrackServers: []YouTrackServer{
			{
				Name:     "Mediatel YouTrack",
				URL:      "https://youtrack.mediatel.co.uk/",
				Projects: []string{"MTEL", "BARB"},
			},
			{
				Name:     "Other YouTrack",
				URL:      "http://youtrack.other.org",
				Projects: []string{"FOO", "BAR"},
			},
		},
	}

	tests := []struct {
		name        string
		key         string
		expectedURL string
		expectOK    bool
	}{
		{
			name:        "Match MTEL exact uppercase",
			key:         "MTEL-22122",
			expectedURL: "https://youtrack.mediatel.co.uk/issue/MTEL-22122",
			expectOK:    true,
		},
		{
			name:        "Match BARB lowercase key",
			key:         "barb-123",
			expectedURL: "https://youtrack.mediatel.co.uk/issue/BARB-123",
			expectOK:    true,
		},
		{
			name:        "Match FOO other server",
			key:         "FOO-99",
			expectedURL: "http://youtrack.other.org/issue/FOO-99",
			expectOK:    true,
		},
		{
			name:        "No match project",
			key:         "XYZ-123",
			expectedURL: "",
			expectOK:    false,
		},
		{
			name:        "Invalid format no hyphen",
			key:         "MTEL22122",
			expectedURL: "",
			expectOK:    false,
		},
		{
			name:        "Invalid format non-numeric suffix",
			key:         "MTEL-abc",
			expectedURL: "",
			expectOK:    false,
		},
		{
			name:        "Invalid format multiple hyphens",
			key:         "MTEL-123-45",
			expectedURL: "",
			expectOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, ok := cfg.GetYouTrackURL(tt.key)
			if ok != tt.expectOK {
				t.Fatalf("expected ok=%v, got %v", tt.expectOK, ok)
			}
			if ok && url != tt.expectedURL {
				t.Errorf("expected URL %q, got %q", tt.expectedURL, url)
			}
		})
	}
}

func TestIsYouTrackURL(t *testing.T) {
	cfg := &Config{
		YouTrackServers: []YouTrackServer{
			{
				Name:     "Mediatel YouTrack",
				URL:      "https://youtrack.mediatel.co.uk/",
				Projects: []string{"MTEL", "BARB"},
			},
			{
				Name:     "Other YouTrack",
				URL:      "http://youtrack.other.org",
				Projects: []string{"FOO", "BAR"},
			},
		},
	}

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Match exact base URL with trailing slash",
			url:      "https://youtrack.mediatel.co.uk/",
			expected: true,
		},
		{
			name:     "Match issue URL under base with trailing slash",
			url:      "https://youtrack.mediatel.co.uk/issue/MTEL-22122",
			expected: true,
		},
		{
			name:     "Match issue URL under base without trailing slash",
			url:      "http://youtrack.other.org/issue/FOO-99",
			expected: true,
		},
		{
			name:     "Match exact base URL without trailing slash",
			url:      "http://youtrack.other.org",
			expected: true,
		},
		{
			name:     "Case insensitive matching",
			url:      "HTTPS://YOUTRACK.MEDIATEL.CO.UK/issue/MTEL-22122",
			expected: true,
		},
		{
			name:     "No match different host",
			url:      "https://gitlab.com/group/project",
			expected: false,
		},
		{
			name:     "No match partial domain name",
			url:      "https://youtrack.mediatel.co.uk.attacker.com/issue/MTEL-22122",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.IsYouTrackURL(tt.url)
			if got != tt.expected {
				t.Errorf("IsYouTrackURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

func TestEnvRef(t *testing.T) {
	tests := []struct {
		token string
		name  string
		ok    bool
	}{
		{token: "$GITLAB_TOKEN", name: "GITLAB_TOKEN", ok: true},
		{token: "${GITLAB_TOKEN}", name: "GITLAB_TOKEN", ok: true},
		{token: "  $GITLAB_TOKEN  ", name: "GITLAB_TOKEN", ok: true},
		{token: "glpat-abc", ok: false},
		{token: "$", ok: false},
		{token: "${}", ok: false},
		{token: "$1TOKEN", ok: false},
		{token: "prefix-$GITLAB_TOKEN", ok: false},
	}
	for _, tt := range tests {
		name, ok := envRef(tt.token)
		if ok != tt.ok || name != tt.name {
			t.Errorf("envRef(%q) = %q, %v; want %q, %v", tt.token, name, ok, tt.name, tt.ok)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	contents := strings.Join([]string{
		"# comment",
		"",
		"GITLAB_COM_TOKEN=glpat-from-file",
		`COMPANY_TOKEN="glpat-quoted"`,
		"export SPACED_TOKEN = glpat-spaced",
		"not a pair",
		"EMPTY=",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	env, err := loadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if env["GITLAB_COM_TOKEN"] != "glpat-from-file" {
		t.Errorf("GITLAB_COM_TOKEN = %q", env["GITLAB_COM_TOKEN"])
	}
	if env["COMPANY_TOKEN"] != "glpat-quoted" {
		t.Errorf("COMPANY_TOKEN = %q", env["COMPANY_TOKEN"])
	}
	if env["SPACED_TOKEN"] != "glpat-spaced" {
		t.Errorf("SPACED_TOKEN = %q", env["SPACED_TOKEN"])
	}
	if _, ok := env["EMPTY"]; !ok || env["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, present %v", env["EMPTY"], ok)
	}
}

func TestResolveServerTokens(t *testing.T) {
	servers := []Server{
		{Name: "file", Token: "$FROM_FILE"},
		{Name: "env", Token: "${FROM_ENV}"},
		{Name: "literal", Token: "glpat-literal"},
	}
	err := resolveServerTokens(servers, map[string]string{"FROM_FILE": "file-secret"}, "/cfg/.env", func(name string) string {
		if name == "FROM_ENV" {
			return "env-secret"
		}
		if name == "FROM_FILE" {
			return "should-not-win"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if servers[0].Token != "file-secret" {
		t.Errorf("file token = %q", servers[0].Token)
	}
	if servers[1].Token != "env-secret" {
		t.Errorf("env token = %q", servers[1].Token)
	}
	if servers[2].Token != "glpat-literal" {
		t.Errorf("literal token = %q", servers[2].Token)
	}

	missing := []Server{{Name: "gitlab.com", Token: "$MISSING"}}
	err = resolveServerTokens(missing, nil, "/cfg/.env", func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "$MISSING") || !strings.Contains(err.Error(), "gitlab.com") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestLoadResolvesTokenWithoutPersisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GITLAB_COM_TOKEN", "from-process")

	dir := filepath.Join(home, ".config", "gitlab-tui")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\n  \"servers\": [\n    {\n      \"name\": \"gitlab.com\",\n      \"url\": \"https://gitlab.com\",\n      \"token\": \"$GITLAB_COM_TOKEN\"\n    }\n  ]\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GITLAB_COM_TOKEN=glpat-from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Servers[0].Token != "glpat-from-file" {
		t.Errorf("loaded token = %q", cfg.Servers[0].Token)
	}
	if cfg.BrowserCommand != defaultBrowserCommand || cfg.Theme != "catppuccin" {
		t.Errorf("defaults not applied: browser %q theme %q", cfg.BrowserCommand, cfg.Theme)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(saved)
	if !strings.Contains(text, `"token": "$GITLAB_COM_TOKEN"`) {
		t.Errorf("config.json lost the token reference:\n%s", text)
	}
	if strings.Contains(text, "glpat-from-file") || strings.Contains(text, "from-process") {
		t.Errorf("config.json contains a resolved token:\n%s", text)
	}
}



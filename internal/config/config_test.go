package config

import (
	"testing"
)

func TestParseGitLabMRURL(t *testing.T) {
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
		name        string
		url         string
		expectedIdx int
		expectedPrj string
		expectedIID int
		expectError bool
	}{
		{
			name:        "Standard URL mediatel",
			url:         "https://gitlab.mediatel.co.uk/audio/audiotrack-admin-hub/-/merge_requests/25",
			expectedIdx: 0,
			expectedPrj: "audio/audiotrack-admin-hub",
			expectedIID: 25,
			expectError: false,
		},
		{
			name:        "URL without HTTPS prefix",
			url:         "gitlab.mediatel.co.uk/audio/audiotrack-admin-hub/-/merge_requests/25",
			expectedIdx: 0,
			expectedPrj: "audio/audiotrack-admin-hub",
			expectedIID: 25,
			expectError: false,
		},
		{
			name:        "Standard URL gitlab.com",
			url:         "https://gitlab.com/gitlab-org/gitlab/-/merge_requests/12345/diffs?view=inline",
			expectedIdx: 1,
			expectedPrj: "gitlab-org/gitlab",
			expectedIID: 12345,
			expectError: false,
		},
		{
			name:        "Old style merge request URL (no /-/ )",
			url:         "https://gitlab.com/group/subgroup/project/merge_requests/99",
			expectedIdx: 1,
			expectedPrj: "group/subgroup/project",
			expectedIID: 99,
			expectError: false,
		},
		{
			name:        "Invalid server",
			url:         "https://github.com/group/project/-/merge_requests/25",
			expectError: true,
		},
		{
			name:        "Not a merge request URL",
			url:         "https://gitlab.com/group/project",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, prj, iid, err := ParseGitLabMRURL(cfg, tt.url)
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
				if iid != tt.expectedIID {
					t.Errorf("expected MR IID %d, got %d", tt.expectedIID, iid)
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


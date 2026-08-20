package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gl "gitlab.com/gitlab-org/api/client-go"
)

func TestGetMRDiffsUsesAccessRawDiffs(t *testing.T) {
	var gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		if r.URL.Path != "/api/v4/projects/7/merge_requests/42/changes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("access_raw_diffs") != "true" {
			t.Errorf("expected access_raw_diffs=true query param, got %q", r.URL.RawQuery)
		}

		// Build a new-file diff with 3 added lines.
		var diff strings.Builder
		diff.WriteString("@@ -0,0 +1,3 @@\n")
		diff.WriteString("+line one\n")
		diff.WriteString("+line two\n")
		diff.WriteString("+line three\n")

		resp := mrChangesResponse{
			Overflow: true,
			Changes: []struct {
				OldPath     string `json:"old_path"`
				NewPath     string `json:"new_path"`
				AMode       string `json:"a_mode"`
				BMode       string `json:"b_mode"`
				Diff        string `json:"diff"`
				NewFile     bool   `json:"new_file"`
				RenamedFile bool   `json:"renamed_file"`
				DeletedFile bool   `json:"deleted_file"`
				TooLarge    bool   `json:"too_large"`
				Collapsed   bool   `json:"collapsed"`
			}{
				{
					OldPath: "new_file.ts",
					NewPath: "new_file.ts",
					AMode:   "0",
					BMode:   "100644",
					Diff:    diff.String(),
					NewFile: true,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	raw, err := gl.NewClient("test-token", gl.WithBaseURL(ts.URL))
	if err != nil {
		t.Fatalf("creating gitlab client: %v", err)
	}
	c := &Client{raw: raw}

	files, err := c.getMRChangesFallback(7, 42)
	if err != nil {
		t.Fatalf("getMRChangesFallback: %v", err)
	}
	if gotPath == "" {
		t.Fatal("server was not called")
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Added != 3 {
		t.Errorf("expected Added=3, got %d", f.Added)
	}
	if f.Deleted != 0 {
		t.Errorf("expected Deleted=0, got %d", f.Deleted)
	}
	if !f.NewFile {
		t.Error("expected NewFile=true")
	}
	if !f.Overflow {
		t.Error("expected Overflow=true")
	}
	if f.AMode != "0" || f.BMode != "100644" {
		t.Errorf("unexpected modes: %q -> %q", f.AMode, f.BMode)
	}
}
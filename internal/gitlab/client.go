package gitlab

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	gl "gitlab.com/gitlab-org/api/client-go"
)

// ─── MR Diffs ─────────────────────────────────────────────────────────────────

// DiffLine represents a single line in a diff.
type DiffLine struct {
	OldLine int    // 0 if line is added
	NewLine int    // 0 if line is deleted
	Type    string // "added", "removed", "context"
	Content string // raw diff line text (with leading +/-/ )
}

// DiffFile represents a single changed file in an MR.
type DiffFile struct {
	OldPath   string
	NewPath   string
	Added     int
	Deleted   int
	Lines     []DiffLine
	TooLarge  bool // diff was too large to return inline
	Collapsed bool // diff was collapsed by GitLab
}

// GetMRDiffs fetches the list of changed files (with diff hunks) for an MR.
func (c *Client) GetMRDiffs(projectID, mriid int) ([]*DiffFile, error) {
	result, err := c.getMRChangesFallback(projectID, mriid)
	if err == nil {
		return result, nil
	}

	// If changes endpoint fails, try ListMergeRequestDiffs as a fallback
	var fallbackResult []*DiffFile
	page := int64(1)
	for {
		opts := &gl.ListMergeRequestDiffsOptions{
			ListOptions: gl.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}
		diffs, resp, err := c.raw.MergeRequests.ListMergeRequestDiffs(projectID, int64(mriid), opts)
		if err != nil {
			return nil, fmt.Errorf("listing MR diffs (page %d): %w", page, err)
		}

		for _, d := range diffs {
			f := &DiffFile{
				OldPath:   d.OldPath,
				NewPath:   d.NewPath,
				TooLarge:  d.TooLarge,
				Collapsed: d.Collapsed,
			}
			lines := parseDiffLines(d.Diff)
			for _, l := range lines {
				if l.Type == "added" {
					f.Added++
				} else if l.Type == "removed" {
					f.Deleted++
				}
			}
			f.Lines = lines
			fallbackResult = append(fallbackResult, f)
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return fallbackResult, nil
}

type mrChangesResponse struct {
	Changes []struct {
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
	} `json:"changes"`
}

func (c *Client) getMRChangesFallback(projectID, mriid int) ([]*DiffFile, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/changes", projectID, mriid)
	req, err := c.raw.NewRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating raw request for fallback: %w", err)
	}

	var respObj mrChangesResponse
	_, err = c.raw.Do(req, &respObj)
	if err != nil {
		return nil, fmt.Errorf("executing raw request for fallback: %w", err)
	}

	var result []*DiffFile
	for _, d := range respObj.Changes {
		f := &DiffFile{
			OldPath:   d.OldPath,
			NewPath:   d.NewPath,
			TooLarge:  d.TooLarge,
			Collapsed: d.Collapsed,
		}
		lines := parseDiffLines(d.Diff)
		for _, l := range lines {
			if l.Type == "added" {
				f.Added++
			} else if l.Type == "removed" {
				f.Deleted++
			}
		}
		f.Lines = lines
		result = append(result, f)
	}
	return result, nil
}


// parseDiffLines parses the raw unified diff string into DiffLine entries.
func parseDiffLines(raw string) []DiffLine {
	var lines []DiffLine
	oldLine, newLine := 0, 0
	for _, l := range splitLines(raw) {
		if len(l) == 0 {
			continue
		}
		switch l[0] {
		case '@':
			// hunk header — parse starting line numbers
			var oStart, oLen, nStart, nLen int
			// format: @@ -oStart,oLen +nStart,nLen @@
			if _, err := fmt.Sscanf(l, "@@ -%d,%d +%d,%d", &oStart, &oLen, &nStart, &nLen); err != nil {
				// try without len
				fmt.Sscanf(l, "@@ -%d +%d", &oStart, &nStart) //nolint
			}
			oldLine = oStart
			newLine = nStart
			lines = append(lines, DiffLine{Type: "hunk", Content: l})
		case '+':
			lines = append(lines, DiffLine{NewLine: newLine, Type: "added", Content: l})
			newLine++
		case '-':
			lines = append(lines, DiffLine{OldLine: oldLine, Type: "removed", Content: l})
			oldLine++
		default:
			lines = append(lines, DiffLine{OldLine: oldLine, NewLine: newLine, Type: "context", Content: l})
			oldLine++
			newLine++
		}
	}
	return lines
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			// Strip \r so that CRLF line-endings in diffs don't emit a
			// carriage-return to the terminal and overwrite other columns.
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	if start < len(s) {
		line := s[start:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		out = append(out, line)
	}
	return out
}

// ─── MR Comments ──────────────────────────────────────────────────────────────

// MRDiscussion represents a GitLab discussion thread on a merge request.
type MRDiscussion struct {
	ID             string
	IndividualNote bool
	Notes          []*MRNote
}

// MRNote represents a single comment or event in a discussion thread.
type MRNote struct {
	ID        int64
	Body      string
	Author    string
	System    bool
	CreatedAt string
	Position  *MRNotePosition
}

// MRNotePosition holds path and line indicators for inline comments.
type MRNotePosition struct {
	NewPath string
	NewLine int
	OldPath string
	OldLine int
}

// GetMRDiscussions fetches all discussion threads (general and inline comments) for an MR.
func (c *Client) GetMRDiscussions(projectID, mriid int) ([]*MRDiscussion, error) {
	opts := &gl.ListMergeRequestDiscussionsOptions{
		ListOptions: gl.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}
	var allDiscussions []*gl.Discussion
	for {
		discussions, resp, err := c.raw.Discussions.ListMergeRequestDiscussions(projectID, int64(mriid), opts)
		if err != nil {
			return nil, fmt.Errorf("listing MR discussions: %w", err)
		}
		allDiscussions = append(allDiscussions, discussions...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = int64(resp.NextPage)
	}

	var result []*MRDiscussion
	for _, d := range allDiscussions {
		disc := &MRDiscussion{
			ID:             d.ID,
			IndividualNote: d.IndividualNote,
		}
		for _, n := range d.Notes {
			note := &MRNote{
				ID:     n.ID,
				Body:   ConvertHTMLToMarkdown(n.Body),
				System: n.System,
			}
			note.Author = n.Author.Username
			if n.CreatedAt != nil {
				note.CreatedAt = n.CreatedAt.Format("2006-01-02 15:04")
			}
			if n.Position != nil {
				note.Position = &MRNotePosition{
					NewPath: n.Position.NewPath,
					NewLine: int(n.Position.NewLine),
					OldPath: n.Position.OldPath,
					OldLine: int(n.Position.OldLine),
				}
			}
			disc.Notes = append(disc.Notes, note)
		}
		result = append(result, disc)
	}
	return result, nil
}

// ReplyToMRDiscussion replies to an existing discussion thread on an MR.
func (c *Client) ReplyToMRDiscussion(projectID, mriid int, discussionID string, body string) error {
	opt := &gl.AddMergeRequestDiscussionNoteOptions{
		Body: &body,
	}
	_, _, err := c.raw.Discussions.AddMergeRequestDiscussionNote(projectID, int64(mriid), discussionID, opt)
	if err != nil {
		return fmt.Errorf("replying to MR discussion thread: %w", err)
	}
	return nil
}

// CreateMRComment posts a general (non-inline) note on an MR.
func (c *Client) CreateMRComment(projectID, mriid int, body string) error {
	_, _, err := c.raw.Notes.CreateMergeRequestNote(projectID, int64(mriid), &gl.CreateMergeRequestNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("creating MR comment: %w", err)
	}
	return nil
}

// EditMRComment updates the body of an existing MR comment.
func (c *Client) EditMRComment(projectID, mriid int, noteID int64, body string) error {
	_, _, err := c.raw.Notes.UpdateMergeRequestNote(projectID, int64(mriid), noteID, &gl.UpdateMergeRequestNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("editing MR comment %d: %w", noteID, err)
	}
	return nil
}

// DeleteMRComment deletes an existing MR comment.
func (c *Client) DeleteMRComment(projectID, mriid int, noteID int64) error {
	_, err := c.raw.Notes.DeleteMergeRequestNote(projectID, int64(mriid), noteID)
	if err != nil {
		return fmt.Errorf("deleting MR comment %d: %w", noteID, err)
	}
	return nil
}

// CreateMRInlineComment posts an inline comment on a specific diff line.
func (c *Client) CreateMRInlineComment(projectID, mriid int, body, baseSHA, startSHA, headSHA, oldPath, newPath string, oldLine, newLine int) error {
	posType := "text"
	pos := &gl.CreateMergeRequestDiscussionOptions{
		Body: &body,
		Position: &gl.PositionOptions{
			BaseSHA:      gl.Ptr(baseSHA),
			StartSHA:     gl.Ptr(startSHA),
			HeadSHA:      gl.Ptr(headSHA),
			PositionType: gl.Ptr(posType),
			OldPath:      gl.Ptr(oldPath),
			NewPath:      gl.Ptr(newPath),
		},
	}
	if newLine > 0 {
		nl := int64(newLine)
		pos.Position.NewLine = &nl
	} else if oldLine > 0 {
		ol := int64(oldLine)
		pos.Position.OldLine = &ol
	}
	_, _, err := c.raw.Discussions.CreateMergeRequestDiscussion(projectID, int64(mriid), pos)
	if err != nil {
		return fmt.Errorf("creating inline comment: %w", err)
	}
	return nil
}

// MRVersion holds the SHA information needed to post inline comments.
type MRVersion struct {
	BaseSHA  string
	StartSHA string
	HeadSHA  string
}

// GetMRVersion fetches the latest diff version SHAs for an MR.
func (c *Client) GetMRVersion(projectID, mriid int) (*MRVersion, error) {
	versions, _, err := c.raw.MergeRequests.GetMergeRequestDiffVersions(projectID, int64(mriid), nil)
	if err != nil {
		return nil, fmt.Errorf("getting MR version: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no diff versions found for MR !%d", mriid)
	}
	v := versions[0] // most recent version
	return &MRVersion{
		BaseSHA:  v.BaseCommitSHA,
		StartSHA: v.StartCommitSHA,
		HeadSHA:  v.HeadCommitSHA,
	}, nil
}

// Client wraps the GitLab API client.
type Client struct {
	raw     *gl.Client
	baseURL string
}

// NewClient creates an authenticated GitLab API client.
func NewClient(baseURL, token string) (*Client, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	c, err := gl.NewClient(token,
		gl.WithBaseURL(baseURL),
		gl.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("creating gitlab client: %w", err)
	}
	return &Client{raw: c, baseURL: baseURL}, nil
}

// i64 safely converts int64 to int.
func i64(v int64) int { return int(v) }

// ─── Projects ────────────────────────────────────────────────────────────────

// ProjectInfo is a simplified project representation.
type ProjectInfo struct {
	ID                int
	Name              string
	NameWithNamespace string
	PathWithNamespace string
	WebURL            string
	Description       string
	DefaultBranch     string
	OpenIssuesCount   int
}

// ListProjects lists all accessible projects for the authenticated user.
func (c *Client) ListProjects(search string, page int) ([]*ProjectInfo, int, error) {
	opts := &gl.ListProjectsOptions{
		Membership: gl.Ptr(true),
		OrderBy:    gl.Ptr("last_activity_at"),
		Sort:       gl.Ptr("desc"),
		ListOptions: gl.ListOptions{
			Page:    int64(page),
			PerPage: 25,
		},
	}
	if search != "" {
		opts.Search = gl.Ptr(search)
		opts.SearchNamespaces = gl.Ptr(true)
	}

	projects, resp, err := c.raw.Projects.ListProjects(opts)
	if err != nil {
		return nil, 0, fmt.Errorf("listing projects: %w", err)
	}

	var result []*ProjectInfo
	for _, p := range projects {
		result = append(result, &ProjectInfo{
			ID:                i64(p.ID),
			Name:              p.Name,
			NameWithNamespace: p.NameWithNamespace,
			PathWithNamespace: p.PathWithNamespace,
			WebURL:            p.WebURL,
			Description:       p.Description,
			DefaultBranch:     p.DefaultBranch,
			OpenIssuesCount:   i64(p.OpenIssuesCount),
		})
	}
	return result, i64(resp.TotalPages), nil
}

// GetProject fetches a single project by path.
func (c *Client) GetProject(path string) (*ProjectInfo, error) {
	p, _, err := c.raw.Projects.GetProject(path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting project %q: %w", path, err)
	}
	return &ProjectInfo{
		ID:                i64(p.ID),
		Name:              p.Name,
		NameWithNamespace: p.NameWithNamespace,
		PathWithNamespace: p.PathWithNamespace,
		WebURL:            p.WebURL,
		Description:       p.Description,
		DefaultBranch:     p.DefaultBranch,
		OpenIssuesCount:   i64(p.OpenIssuesCount),
	}, nil
}

// ─── Merge Requests ──────────────────────────────────────────────────────────

// MRState represents the state filter for MRs.
type MRState string

const (
	MRStateOpened MRState = "opened"
	MRStateMerged MRState = "merged"
	MRStateClosed MRState = "closed"
	MRStateAll    MRState = "all"
)

// MRInfo holds a summary of a merge request.
type MRInfo struct {
	IID            int
	Title          string
	State          string
	Author         string
	TargetBranch   string
	SourceBranch   string
	WebURL         string
	CreatedAt      string
	UpdatedAt      string
	Upvotes        int
	Downvotes      int
	UserNotesCount int
	Labels         []string
	Draft          bool
	Description             string
	Assignees               []string
	Reviewers               []string
	ForceRemoveSourceBranch bool
	Squash                  bool
	SHA                     string
	MergeCommitSHA          string
}

// ListMRs lists merge requests for a project.
func (c *Client) ListMRs(projectID int, state MRState, page int) ([]*MRInfo, int, error) {
	stateStr := string(state)
	opts := &gl.ListProjectMergeRequestsOptions{
		State:   &stateStr,
		OrderBy: gl.Ptr("updated_at"),
		Sort:    gl.Ptr("desc"),
		ListOptions: gl.ListOptions{
			Page:    int64(page),
			PerPage: 25,
		},
	}

	mrs, resp, err := c.raw.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("listing MRs: %w", err)
	}

	var result []*MRInfo
	for _, mr := range mrs {
		info := &MRInfo{
			IID:            i64(mr.IID),
			Title:          mr.Title,
			State:          mr.State,
			TargetBranch:   mr.TargetBranch,
			SourceBranch:   mr.SourceBranch,
			WebURL:         mr.WebURL,
			Upvotes:        i64(mr.Upvotes),
			Downvotes:      i64(mr.Downvotes),
			UserNotesCount: i64(mr.UserNotesCount),
			Description:             mr.Description,
			Draft:                   mr.Draft,
			ForceRemoveSourceBranch: mr.ForceRemoveSourceBranch,
			Squash:                  mr.Squash,
			SHA:                     mr.SHA,
			MergeCommitSHA:          mr.MergeCommitSHA,
		}
		if mr.Author != nil {
			info.Author = mr.Author.Username
		}
		if mr.CreatedAt != nil {
			info.CreatedAt = mr.CreatedAt.Format("2006-01-02 15:04")
		}
		if mr.UpdatedAt != nil {
			info.UpdatedAt = mr.UpdatedAt.Format("2006-01-02 15:04")
		}
		for _, l := range mr.Labels {
			info.Labels = append(info.Labels, l)
		}
		for _, a := range mr.Assignees {
			info.Assignees = append(info.Assignees, a.Username)
		}
		for _, r := range mr.Reviewers {
			info.Reviewers = append(info.Reviewers, r.Username)
		}
		result = append(result, info)
	}
	return result, i64(resp.TotalPages), nil
}

// GetMR fetches a single merge request by IID with up-to-date field values
// (vote counts on the list endpoint can be stale).
func (c *Client) GetMR(projectID, mriid int) (*MRInfo, error) {
	mr, _, err := c.raw.MergeRequests.GetMergeRequest(projectID, int64(mriid), nil)
	if err != nil {
		return nil, fmt.Errorf("getting MR !%d: %w", mriid, err)
	}
	info := &MRInfo{
		IID:            i64(mr.IID),
		Title:          mr.Title,
		State:          mr.State,
		TargetBranch:   mr.TargetBranch,
		SourceBranch:   mr.SourceBranch,
		WebURL:         mr.WebURL,
		Upvotes:        i64(mr.Upvotes),
		Downvotes:      i64(mr.Downvotes),
		UserNotesCount: i64(mr.UserNotesCount),
		Description:             mr.Description,
		Draft:                   mr.Draft,
		ForceRemoveSourceBranch: mr.ForceRemoveSourceBranch,
		Squash:                  mr.Squash,
		SHA:                     mr.SHA,
		MergeCommitSHA:          mr.MergeCommitSHA,
	}
	if mr.Author != nil {
		info.Author = mr.Author.Username
	}
	if mr.CreatedAt != nil {
		info.CreatedAt = mr.CreatedAt.Format("2006-01-02 15:04")
	}
	if mr.UpdatedAt != nil {
		info.UpdatedAt = mr.UpdatedAt.Format("2006-01-02 15:04")
	}
	for _, l := range mr.Labels {
		info.Labels = append(info.Labels, l)
	}
	for _, a := range mr.Assignees {
		info.Assignees = append(info.Assignees, a.Username)
	}
	for _, r := range mr.Reviewers {
		info.Reviewers = append(info.Reviewers, r.Username)
	}
	return info, nil
}


func (c *Client) ApproveMR(projectID, mriid int) error {
	_, _, err := c.raw.MergeRequestApprovals.ApproveMergeRequest(projectID, int64(mriid), nil)
	return err
}

// MergeMR merges a merge request.
func (c *Client) MergeMR(projectID, mriid int) error {
	_, _, err := c.raw.MergeRequests.AcceptMergeRequest(projectID, int64(mriid), nil)
	return err
}

// CloseMR closes a merge request.
func (c *Client) CloseMR(projectID, mriid int) error {
	state := "close"
	_, _, err := c.raw.MergeRequests.UpdateMergeRequest(projectID, int64(mriid), &gl.UpdateMergeRequestOptions{
		StateEvent: &state,
	})
	return err
}

// ReopenMR reopens a merge request.
func (c *Client) ReopenMR(projectID, mriid int) error {
	state := "reopen"
	_, _, err := c.raw.MergeRequests.UpdateMergeRequest(projectID, int64(mriid), &gl.UpdateMergeRequestOptions{
		StateEvent: &state,
	})
	return err
}


// ToggleVoteMR adds or removes a thumbsup/thumbsdown award emoji on a merge request.
// If the authenticated user (username) has already awarded the same emoji, it is
// deleted (toggle off) and the function returns false. Otherwise the emoji is created
// and the function returns true.
// vote must be either "thumbsup" or "thumbsdown".
func (c *Client) ToggleVoteMR(projectID, mriid int, vote, username string) (added bool, err error) {
	emojis, _, err := c.raw.AwardEmoji.ListMergeRequestAwardEmoji(projectID, int64(mriid), nil)
	if err != nil {
		return false, fmt.Errorf("listing award emoji: %w", err)
	}
	for _, e := range emojis {
		if e.Name == vote && e.User.Username == username {
			// Already voted — remove it.
			_, err = c.raw.AwardEmoji.DeleteMergeRequestAwardEmoji(projectID, int64(mriid), int64(e.ID))
			return false, err
		}
	}
	// Not yet voted — add it.
	_, _, err = c.raw.AwardEmoji.CreateMergeRequestAwardEmoji(projectID, int64(mriid), &gl.CreateAwardEmojiOptions{
		Name: vote,
	})
	return true, err
}

// ─── Pipelines ───────────────────────────────────────────────────────────────

// PipelineInfo holds a summary of a pipeline.
type PipelineInfo struct {
	ID        int
	Ref       string
	Status    string
	WebURL    string
	CreatedAt string
	UpdatedAt string
	Duration  int
	Source    string
	User      string // set from separate field if available
}

// ListPipelines lists pipelines for a project.
func (c *Client) ListPipelines(projectID int, page int) ([]*PipelineInfo, int, error) {
	opts := &gl.ListProjectPipelinesOptions{
		OrderBy: gl.Ptr("updated_at"),
		Sort:    gl.Ptr("desc"),
		ListOptions: gl.ListOptions{
			Page:    int64(page),
			PerPage: 25,
		},
	}

	pipelines, resp, err := c.raw.Pipelines.ListProjectPipelines(projectID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("listing pipelines: %w", err)
	}

	var result []*PipelineInfo
	for _, p := range pipelines {
		info := &PipelineInfo{
			ID:     i64(p.ID),
			Ref:    p.Ref,
			Status: p.Status,
			WebURL: p.WebURL,
			Source: p.Source,
		}
		if p.CreatedAt != nil {
			info.CreatedAt = p.CreatedAt.Format("2006-01-02 15:04")
		}
		if p.UpdatedAt != nil {
			info.UpdatedAt = p.UpdatedAt.Format("2006-01-02 15:04")
		}
		result = append(result, info)
	}
	return result, i64(resp.TotalPages), nil
}

// RetryPipeline retries a failed pipeline.
func (c *Client) RetryPipeline(projectID, pipelineID int) error {
	_, _, err := c.raw.Pipelines.RetryPipelineBuild(projectID, int64(pipelineID))
	return err
}

// CancelPipeline cancels a running pipeline.
func (c *Client) CancelPipeline(projectID, pipelineID int) error {
	_, _, err := c.raw.Pipelines.CancelPipelineBuild(projectID, int64(pipelineID))
	return err
}

// GetMRPipelines fetches pipelines for a merge request.
func (c *Client) GetMRPipelines(projectID, mriid int, sourceBranch, sha, mergeCommitSHA string) ([]*PipelineInfo, error) {
	var allPipelines []*gl.PipelineInfo

	// 1. Try to fetch merge request pipelines
	mrPipelines, _, err := c.raw.MergeRequests.ListMergeRequestPipelines(projectID, int64(mriid))
	if err == nil {
		allPipelines = append(allPipelines, mrPipelines...)
	}

	// 2. Fetch project pipelines for the source branch
	if sourceBranch != "" {
		branchPipelines, _, err := c.raw.Pipelines.ListProjectPipelines(projectID, &gl.ListProjectPipelinesOptions{
			Ref: gl.Ptr(sourceBranch),
		})
		if err == nil {
			allPipelines = append(allPipelines, branchPipelines...)
		}
	}

	// 3. Fetch project pipelines for the commit head SHA
	if sha != "" {
		shaPipelines, _, err := c.raw.Pipelines.ListProjectPipelines(projectID, &gl.ListProjectPipelinesOptions{
			SHA: gl.Ptr(sha),
		})
		if err == nil {
			allPipelines = append(allPipelines, shaPipelines...)
		}
	}

	// 4. Fetch project pipelines for the merge commit SHA
	if mergeCommitSHA != "" {
		mergePipelines, _, err := c.raw.Pipelines.ListProjectPipelines(projectID, &gl.ListProjectPipelinesOptions{
			SHA: gl.Ptr(mergeCommitSHA),
		})
		if err == nil {
			allPipelines = append(allPipelines, mergePipelines...)
		}
	}

	// 5. Deduplicate by ID and preserve order (descending ID)
	seen := make(map[int64]bool)
	var result []*PipelineInfo
	for _, p := range allPipelines {
		if p == nil || seen[p.ID] {
			continue
		}
		seen[p.ID] = true

		info := &PipelineInfo{
			ID:     i64(p.ID),
			Ref:    p.Ref,
			Status: p.Status,
			WebURL: p.WebURL,
			Source: p.Source,
		}
		if p.CreatedAt != nil {
			info.CreatedAt = p.CreatedAt.Format("2006-01-02 15:04")
		}
		if p.UpdatedAt != nil {
			info.UpdatedAt = p.UpdatedAt.Format("2006-01-02 15:04")
		}
		result = append(result, info)
	}

	// Sort by ID descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID > result[j].ID
	})

	return result, nil
}

// GetPipeline fetches details of a single pipeline.
func (c *Client) GetPipeline(projectID, pipelineID int) (*PipelineInfo, error) {
	p, _, err := c.raw.Pipelines.GetPipeline(projectID, int64(pipelineID))
	if err != nil {
		return nil, err
	}
	info := &PipelineInfo{
		ID:     i64(p.ID),
		Ref:    p.Ref,
		Status: p.Status,
		WebURL: p.WebURL,
		Source: string(p.Source),
	}
	if p.CreatedAt != nil {
		info.CreatedAt = p.CreatedAt.Format("2006-01-02 15:04")
	}
	if p.UpdatedAt != nil {
		info.UpdatedAt = p.UpdatedAt.Format("2006-01-02 15:04")
	}
	return info, nil
}

// TriggerPipeline triggers a new pipeline on a ref.
func (c *Client) TriggerPipeline(projectID int, ref string) (*PipelineInfo, error) {
	p, _, err := c.raw.Pipelines.CreatePipeline(projectID, &gl.CreatePipelineOptions{
		Ref: gl.Ptr(ref),
	})
	if err != nil {
		return nil, err
	}
	info := &PipelineInfo{
		ID:     i64(p.ID),
		Ref:    p.Ref,
		Status: p.Status,
		WebURL: p.WebURL,
	}
	return info, nil
}

// ─── Jobs ────────────────────────────────────────────────────────────────────

// JobInfo holds details of a pipeline job.
type JobInfo struct {
	ID            int64
	Name          string
	Stage         string
	Status        string
	AllowFailure  bool
	CreatedAt     string
	StartedAt     string
	FinishedAt    string
	Duration      int // duration in seconds
	FailureReason string
}

// ListPipelineJobs lists jobs for a specific pipeline in a project.
func (c *Client) ListPipelineJobs(projectID int, pipelineID int) ([]*JobInfo, error) {
	var result []*JobInfo
	page := int64(1)
	for {
		opts := &gl.ListJobsOptions{
			ListOptions: gl.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}
		jobs, resp, err := c.raw.Jobs.ListPipelineJobs(projectID, int64(pipelineID), opts)
		if err != nil {
			return nil, fmt.Errorf("listing pipeline jobs: %w", err)
		}
		for _, j := range jobs {
			info := &JobInfo{
				ID:           j.ID,
				Name:         j.Name,
				Stage:        j.Stage,
				Status:       j.Status,
				AllowFailure: j.AllowFailure,
				Duration:     int(j.Duration),
			}
			if j.CreatedAt != nil {
				info.CreatedAt = j.CreatedAt.Format("2006-01-02 15:04")
			}
			if j.StartedAt != nil {
				info.StartedAt = j.StartedAt.Format("2006-01-02 15:04")
			}
			if j.FinishedAt != nil {
				info.FinishedAt = j.FinishedAt.Format("2006-01-02 15:04")
			}
			if j.FailureReason != "" {
				info.FailureReason = j.FailureReason
			}
			result = append(result, info)
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return result, nil
}

// RetryJob retries a single job of a project.
func (c *Client) RetryJob(projectID int, jobID int64) error {
	_, _, err := c.raw.Jobs.RetryJob(projectID, jobID)
	return err
}

// PlayJob plays/triggers a manual job of a project.
func (c *Client) PlayJob(projectID int, jobID int64) error {
	_, _, err := c.raw.Jobs.PlayJob(projectID, jobID, nil)
	return err
}

// GetJobPipelineID fetches the pipeline ID for a given job.
func (c *Client) GetJobPipelineID(projectID int, jobID int64) (int, error) {
	job, _, err := c.raw.Jobs.GetJob(projectID, jobID)
	if err != nil {
		return 0, err
	}
	if job.Pipeline.ID == 0 {
		return 0, fmt.Errorf("job has no pipeline information")
	}
	return int(job.Pipeline.ID), nil
}


// GetJobTrace fetches the log/trace file of a job.
func (c *Client) GetJobTrace(projectID int, jobID int64) (string, error) {
	reader, _, err := c.raw.Jobs.GetTraceFile(projectID, jobID)
	if err != nil {
		return "", fmt.Errorf("getting job trace: %w", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("reading job trace: %w", err)
	}
	return string(data), nil
}

// ─── Issues ──────────────────────────────────────────────────────────────────

// IssueState is the state filter for issues.
type IssueState string

const (
	IssueStateOpened IssueState = "opened"
	IssueStateClosed IssueState = "closed"
	IssueStateAll    IssueState = "all"
)

// IssueInfo holds a summary of an issue.
type IssueInfo struct {
	IID            int
	Title          string
	IssueType      string
	State          string
	Author         string
	WebURL         string
	CreatedAt      string
	UpdatedAt      string
	Labels         []string
	Description    string
	Assignees      []string
	Upvotes        int
	Downvotes      int
	UserNotesCount int
}

// ListIssues lists issues for a project.
func (c *Client) ListIssues(projectID int, state IssueState, page int) ([]*IssueInfo, int, error) {
	stateStr := string(state)
	opts := &gl.ListProjectIssuesOptions{
		State:   &stateStr,
		OrderBy: gl.Ptr("updated_at"),
		Sort:    gl.Ptr("desc"),
		ListOptions: gl.ListOptions{
			Page:    int64(page),
			PerPage: 25,
		},
	}

	issues, resp, err := c.raw.Issues.ListProjectIssues(projectID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("listing issues: %w", err)
	}

	var result []*IssueInfo
	for _, iss := range issues {
		info := &IssueInfo{
			IID:            i64(iss.IID),
			Title:          iss.Title,
			State:          iss.State,
			WebURL:         iss.WebURL,
			Description:    iss.Description,
			Upvotes:        i64(iss.Upvotes),
			Downvotes:      i64(iss.Downvotes),
			UserNotesCount: i64(iss.UserNotesCount),
		}
		if iss.IssueType != nil {
			info.IssueType = *iss.IssueType
		} else {
			info.IssueType = "issue"
		}
		if iss.Author != nil {
			info.Author = iss.Author.Username
		}
		if iss.CreatedAt != nil {
			info.CreatedAt = iss.CreatedAt.Format("2006-01-02 15:04")
		}
		if iss.UpdatedAt != nil {
			info.UpdatedAt = iss.UpdatedAt.Format("2006-01-02 15:04")
		}
		for _, l := range iss.Labels {
			info.Labels = append(info.Labels, l)
		}
		for _, a := range iss.Assignees {
			info.Assignees = append(info.Assignees, a.Username)
		}
		result = append(result, info)
	}
	return result, i64(resp.TotalPages), nil
}

// GetIssue fetches a single issue by IID with up-to-date field values.
func (c *Client) GetIssue(projectID, issueIID int) (*IssueInfo, error) {
	iss, _, err := c.raw.Issues.GetIssue(projectID, int64(issueIID))
	if err != nil {
		return nil, fmt.Errorf("getting issue #%d: %w", issueIID, err)
	}
	info := &IssueInfo{
		IID:            i64(iss.IID),
		Title:          iss.Title,
		State:          iss.State,
		WebURL:         iss.WebURL,
		Description:    iss.Description,
		Upvotes:        i64(iss.Upvotes),
		Downvotes:      i64(iss.Downvotes),
		UserNotesCount: i64(iss.UserNotesCount),
	}
	if iss.IssueType != nil {
		info.IssueType = *iss.IssueType
	} else {
		info.IssueType = "issue"
	}
	if iss.Author != nil {
		info.Author = iss.Author.Username
	}
	if iss.CreatedAt != nil {
		info.CreatedAt = iss.CreatedAt.Format("2006-01-02 15:04")
	}
	if iss.UpdatedAt != nil {
		info.UpdatedAt = iss.UpdatedAt.Format("2006-01-02 15:04")
	}
	for _, l := range iss.Labels {
		info.Labels = append(info.Labels, l)
	}
	for _, a := range iss.Assignees {
		info.Assignees = append(info.Assignees, a.Username)
	}
	return info, nil
}

// CreateIssueOptions holds optional parameters for creating an issue.
type CreateIssueOptions struct {
	Description string
	IssueType   string
}

// CreateIssue creates a new issue and returns basic info about it.
func (c *Client) CreateIssue(projectID int, title string, opts CreateIssueOptions) (*IssueInfo, error) {
	o := &gl.CreateIssueOptions{
		Title: &title,
	}
	if opts.Description != "" {
		o.Description = &opts.Description
	}
	if opts.IssueType != "" {
		o.IssueType = &opts.IssueType
	}

	iss, _, err := c.raw.Issues.CreateIssue(projectID, o)
	if err != nil {
		return nil, fmt.Errorf("creating issue: %w", err)
	}

	info := &IssueInfo{
		IID:         i64(iss.IID),
		Title:       iss.Title,
		State:       iss.State,
		WebURL:      iss.WebURL,
		Description: iss.Description,
	}
	if iss.IssueType != nil {
		info.IssueType = *iss.IssueType
	} else {
		info.IssueType = "issue"
	}
	if iss.Author != nil {
		info.Author = iss.Author.Username
	}
	if iss.CreatedAt != nil {
		info.CreatedAt = iss.CreatedAt.Format("2006-01-02 15:04")
	}
	if iss.UpdatedAt != nil {
		info.UpdatedAt = iss.UpdatedAt.Format("2006-01-02 15:04")
	}
	return info, nil
}

// UpdateIssueOptions holds optional parameters for updating an issue.
type UpdateIssueOptions struct {
	Title       string
	Description string
	IssueType   string
}

// UpdateIssue updates an existing issue and returns basic info about it.
func (c *Client) UpdateIssue(projectID, issueIID int, opts UpdateIssueOptions) (*IssueInfo, error) {
	o := &gl.UpdateIssueOptions{}
	if opts.Title != "" {
		o.Title = &opts.Title
	}
	o.Description = &opts.Description
	if opts.IssueType != "" {
		o.IssueType = &opts.IssueType
	}

	iss, _, err := c.raw.Issues.UpdateIssue(projectID, int64(issueIID), o)
	if err != nil {
		return nil, fmt.Errorf("updating issue: %w", err)
	}

	info := &IssueInfo{
		IID:         i64(iss.IID),
		Title:       iss.Title,
		State:       iss.State,
		WebURL:      iss.WebURL,
		Description: iss.Description,
	}
	if iss.IssueType != nil {
		info.IssueType = *iss.IssueType
	} else {
		info.IssueType = "issue"
	}
	if iss.Author != nil {
		info.Author = iss.Author.Username
	}
	if iss.CreatedAt != nil {
		info.CreatedAt = iss.CreatedAt.Format("2006-01-02 15:04")
	}
	if iss.UpdatedAt != nil {
		info.UpdatedAt = iss.UpdatedAt.Format("2006-01-02 15:04")
	}
	return info, nil
}

// CloseIssue closes an issue.
func (c *Client) CloseIssue(projectID, issueIID int) error {
	state := "close"
	_, _, err := c.raw.Issues.UpdateIssue(projectID, int64(issueIID), &gl.UpdateIssueOptions{
		StateEvent: &state,
	})
	return err
}

// ReopenIssue reopens an issue.
func (c *Client) ReopenIssue(projectID, issueIID int) error {
	state := "reopen"
	_, _, err := c.raw.Issues.UpdateIssue(projectID, int64(issueIID), &gl.UpdateIssueOptions{
		StateEvent: &state,
	})
	return err
}

// IssueDiscussion represents a GitLab discussion thread on an issue.
type IssueDiscussion struct {
	ID             string
	IndividualNote bool
	Notes          []*IssueNote
}

// IssueNote represents a single comment or event in an issue discussion thread.
type IssueNote struct {
	ID        int64
	Body      string
	Author    string
	System    bool
	CreatedAt string
}

// GetIssueDiscussions fetches all discussion threads for an issue.
func (c *Client) GetIssueDiscussions(projectID, issueIID int) ([]*IssueDiscussion, error) {
	opts := &gl.ListIssueDiscussionsOptions{
		ListOptions: gl.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}
	var allDiscussions []*gl.Discussion
	for {
		discussions, resp, err := c.raw.Discussions.ListIssueDiscussions(projectID, int64(issueIID), opts)
		if err != nil {
			return nil, fmt.Errorf("listing issue discussions: %w", err)
		}
		allDiscussions = append(allDiscussions, discussions...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = int64(resp.NextPage)
	}

	var result []*IssueDiscussion
	for _, d := range allDiscussions {
		disc := &IssueDiscussion{
			ID:             d.ID,
			IndividualNote: d.IndividualNote,
		}
		for _, n := range d.Notes {
			note := &IssueNote{
				ID:     n.ID,
				Body:   ConvertHTMLToMarkdown(n.Body),
				System: n.System,
			}
			note.Author = n.Author.Username
			if n.CreatedAt != nil {
				note.CreatedAt = n.CreatedAt.Format("2006-01-02 15:04")
			}
			disc.Notes = append(disc.Notes, note)
		}
		result = append(result, disc)
	}
	return result, nil
}

// ReplyToIssueDiscussion replies to an existing discussion thread on an issue.
func (c *Client) ReplyToIssueDiscussion(projectID, issueIID int, discussionID string, body string) error {
	opt := &gl.AddIssueDiscussionNoteOptions{
		Body: &body,
	}
	_, _, err := c.raw.Discussions.AddIssueDiscussionNote(projectID, int64(issueIID), discussionID, opt)
	if err != nil {
		return fmt.Errorf("replying to issue discussion thread: %w", err)
	}
	return nil
}

// CreateIssueComment posts a general note on an issue.
func (c *Client) CreateIssueComment(projectID, issueIID int, body string) error {
	_, _, err := c.raw.Notes.CreateIssueNote(projectID, int64(issueIID), &gl.CreateIssueNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("creating issue comment: %w", err)
	}
	return nil
}

// EditIssueComment updates the body of an existing issue comment.
func (c *Client) EditIssueComment(projectID, issueIID int, noteID int64, body string) error {
	_, _, err := c.raw.Notes.UpdateIssueNote(projectID, int64(issueIID), noteID, &gl.UpdateIssueNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("editing issue comment %d: %w", noteID, err)
	}
	return nil
}

// DeleteIssueComment deletes an existing issue comment.
func (c *Client) DeleteIssueComment(projectID, issueIID int, noteID int64) error {
	_, err := c.raw.Notes.DeleteIssueNote(projectID, int64(issueIID), noteID)
	if err != nil {
		return fmt.Errorf("deleting issue comment %d: %w", noteID, err)
	}
	return nil
}

// ToggleVoteIssue adds or removes a thumbsup/thumbsdown award emoji on an issue.
func (c *Client) ToggleVoteIssue(projectID, issueIID int, vote, username string) (added bool, err error) {
	emojis, _, err := c.raw.AwardEmoji.ListIssueAwardEmoji(projectID, int64(issueIID), nil)
	if err != nil {
		return false, fmt.Errorf("listing issue award emoji: %w", err)
	}
	for _, e := range emojis {
		if e.Name == vote && e.User.Username == username {
			// Already voted — remove it.
			_, err = c.raw.AwardEmoji.DeleteIssueAwardEmoji(projectID, int64(issueIID), int64(e.ID))
			return false, err
		}
	}
	// Not yet voted — add it.
	_, _, err = c.raw.AwardEmoji.CreateIssueAwardEmoji(projectID, int64(issueIID), &gl.CreateAwardEmojiOptions{
		Name: vote,
	})
	return true, err
}

// ─── Branches ────────────────────────────────────────────────────────────────

// GetBranchLastCommit returns the title and body of the most recent commit on a branch.
func (c *Client) GetBranchLastCommit(projectID int, branch string) (title, body string, err error) {
	b, _, err := c.raw.Branches.GetBranch(projectID, branch)
	if err != nil {
		return "", "", fmt.Errorf("getting branch %q: %w", branch, err)
	}
	if b.Commit == nil {
		return "", "", nil
	}
	return b.Commit.Title, b.Commit.Message, nil
}

// ListBranches returns all branch names for a project, ordered by last commit date.
func (c *Client) ListBranches(projectID int) ([]string, error) {
	var names []string
	page := int64(1)
	for {
		opts := &gl.ListBranchesOptions{
			ListOptions: gl.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}
		branches, resp, err := c.raw.Branches.ListBranches(projectID, opts)
		if err != nil {
			return nil, fmt.Errorf("listing branches: %w", err)
		}
		for _, b := range branches {
			names = append(names, b.Name)
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return names, nil
}

// CreateMROptions holds all optional parameters for creating a merge request.
type CreateMROptions struct {
	Description        string
	Draft              bool
	RemoveSourceBranch bool
	Squash             bool
}

// CreateMR creates a new merge request and returns basic info about it.
func (c *Client) CreateMR(projectID int, sourceBranch, targetBranch, title string, opts CreateMROptions) (*MRInfo, error) {
	finalTitle := title
	if opts.Draft {
		finalTitle = "Draft: " + title
	}
	o := &gl.CreateMergeRequestOptions{
		Title:              &finalTitle,
		SourceBranch:       &sourceBranch,
		TargetBranch:       &targetBranch,
		RemoveSourceBranch: &opts.RemoveSourceBranch,
		Squash:             &opts.Squash,
	}
	if opts.Description != "" {
		o.Description = &opts.Description
	}
	mr, _, err := c.raw.MergeRequests.CreateMergeRequest(projectID, o)
	if err != nil {
		return nil, fmt.Errorf("creating MR: %w", err)
	}
	info := &MRInfo{
		IID:          int(mr.IID),
		Title:        mr.Title,
		State:        mr.State,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		WebURL:       mr.WebURL,
	}
	return info, nil
}

// UpdateMROptions holds optional parameters for updating a merge request.
type UpdateMROptions struct {
	Description        string
	TargetBranch       string
	Draft              bool
	RemoveSourceBranch bool
	Squash             bool
}

// UpdateMR updates an existing merge request and returns basic info about it.
func (c *Client) UpdateMR(projectID, mriid int, title string, opts UpdateMROptions) (*MRInfo, error) {
	// Strip existing draft prefixes from title
	cleanTitle := title
	for {
		if strings.HasPrefix(strings.ToLower(cleanTitle), "draft:") {
			cleanTitle = strings.TrimSpace(cleanTitle[6:])
			continue
		}
		if strings.HasPrefix(strings.ToLower(cleanTitle), "wip:") {
			cleanTitle = strings.TrimSpace(cleanTitle[4:])
			continue
		}
		break
	}

	finalTitle := cleanTitle
	if opts.Draft {
		finalTitle = "Draft: " + cleanTitle
	}

	o := &gl.UpdateMergeRequestOptions{
		Title:              &finalTitle,
		RemoveSourceBranch: &opts.RemoveSourceBranch,
		Squash:             &opts.Squash,
	}
	if opts.TargetBranch != "" {
		o.TargetBranch = &opts.TargetBranch
	}
	o.Description = &opts.Description

	mr, _, err := c.raw.MergeRequests.UpdateMergeRequest(projectID, int64(mriid), o)
	if err != nil {
		return nil, fmt.Errorf("updating MR: %w", err)
	}

	info := &MRInfo{
		IID:          int(mr.IID),
		Title:        mr.Title,
		State:        mr.State,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		WebURL:       mr.WebURL,
	}
	return info, nil
}

// ─── Current User ─────────────────────────────────────────────────────────────

// WhoAmI returns the username of the authenticated user.
func (c *Client) WhoAmI() (string, error) {
	u, _, err := c.raw.Users.CurrentUser()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// CommitInfo represents a simplified commit representation.
type CommitInfo struct {
	ID         string
	ShortID    string
	Title      string
	AuthorName string
	Date       string
}

// CompareInfo represents the results of comparing two branches/refs.
type CompareInfo struct {
	Commits []*CommitInfo
	Diffs   []*DiffFile
}

// CreateBranch creates a new branch in the GitLab project.
func (c *Client) CreateBranch(projectID int, branch, ref string) error {
	opts := &gl.CreateBranchOptions{
		Branch: gl.Ptr(branch),
		Ref:    gl.Ptr(ref),
	}
	_, _, err := c.raw.Branches.CreateBranch(projectID, opts)
	if err != nil {
		return fmt.Errorf("creating branch %q from %q: %w", branch, ref, err)
	}
	return nil
}

// DeleteBranch deletes a branch in the GitLab project.
func (c *Client) DeleteBranch(projectID int, branch string) error {
	_, err := c.raw.Branches.DeleteBranch(projectID, branch)
	return err
}

// ListCommits fetches commits for a project ref (branch or tag).
func (c *Client) ListCommits(projectID int, ref string) ([]*CommitInfo, error) {
	opts := &gl.ListCommitsOptions{
		RefName: gl.Ptr(ref),
		ListOptions: gl.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}
	commits, _, err := c.raw.Commits.ListCommits(projectID, opts)
	if err != nil {
		return nil, err
	}
	var result []*CommitInfo
	for _, comm := range commits {
		dateStr := ""
		if comm.CommittedDate != nil {
			dateStr = comm.CommittedDate.Format("2006-01-02 15:04")
		}
		result = append(result, &CommitInfo{
			ID:         comm.ID,
			ShortID:    comm.ShortID,
			Title:      comm.Title,
			AuthorName: comm.AuthorName,
			Date:       dateStr,
		})
	}
	return result, nil
}

// Compare compares two branches/refs in the GitLab project.
func (c *Client) Compare(projectID int, from, to string) (*CompareInfo, error) {
	opts := &gl.CompareOptions{
		From: gl.Ptr(from),
		To:   gl.Ptr(to),
	}
	comp, _, err := c.raw.Repositories.Compare(projectID, opts)
	if err != nil {
		return nil, err
	}
	var commits []*CommitInfo
	for _, comm := range comp.Commits {
		dateStr := ""
		if comm.CommittedDate != nil {
			dateStr = comm.CommittedDate.Format("2006-01-02 15:04")
		}
		commits = append(commits, &CommitInfo{
			ID:         comm.ID,
			ShortID:    comm.ShortID,
			Title:      comm.Title,
			AuthorName: comm.AuthorName,
			Date:       dateStr,
		})
	}
	var diffs []*DiffFile
	for _, d := range comp.Diffs {
		f := &DiffFile{
			OldPath:   d.OldPath,
			NewPath:   d.NewPath,
			TooLarge:  false,
			Collapsed: false,
		}
		lines := parseDiffLines(d.Diff)
		for _, l := range lines {
			if l.Type == "added" {
				f.Added++
			} else if l.Type == "removed" {
				f.Deleted++
			}
		}
		f.Lines = lines
		diffs = append(diffs, f)
	}
	return &CompareInfo{
		Commits: commits,
		Diffs:   diffs,
	}, nil
}

// GetCommitDiffs fetches the diffs for a specific commit.
func (c *Client) GetCommitDiffs(projectID int, sha string) ([]*DiffFile, error) {
	var result []*DiffFile
	page := int64(1)
	for {
		opts := &gl.GetCommitDiffOptions{
			ListOptions: gl.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}
		diffs, resp, err := c.raw.Commits.GetCommitDiff(projectID, sha, opts)
		if err != nil {
			return nil, fmt.Errorf("getting commit diffs (page %d): %w", page, err)
		}

		for _, d := range diffs {
			f := &DiffFile{
				OldPath:   d.OldPath,
				NewPath:   d.NewPath,
				TooLarge:  false,
				Collapsed: false,
			}
			lines := parseDiffLines(d.Diff)
			for _, l := range lines {
				if l.Type == "added" {
					f.Added++
				} else if l.Type == "removed" {
					f.Deleted++
				}
			}
			f.Lines = lines
			result = append(result, f)
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return result, nil
}

// ─── Tags ────────────────────────────────────────────────────────────────────

// TagInfo represents a simplified tag structure.
type TagInfo struct {
	Name        string
	Target      string
	Message     string
	CommitTitle string
	CommitID    string
	ShortID     string
	AuthorName  string
	Date        string
	ReleaseDesc string
}

// ListTags lists repository tags for a project.
func (c *Client) ListTags(projectID int) ([]*TagInfo, error) {
	var tags []*TagInfo
	page := int64(1)
	for {
		opts := &gl.ListTagsOptions{
			ListOptions: gl.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}
		res, resp, err := c.raw.Tags.ListTags(projectID, opts)
		if err != nil {
			return nil, fmt.Errorf("listing tags: %w", err)
		}
		for _, t := range res {
			ti := &TagInfo{
				Name:    t.Name,
				Target:  t.Target,
				Message: t.Message,
			}
			if t.Commit != nil {
				ti.CommitTitle = t.Commit.Title
				ti.CommitID = t.Commit.ID
				ti.ShortID = t.Commit.ShortID
				ti.AuthorName = t.Commit.AuthorName
				if t.Commit.CommittedDate != nil {
					ti.Date = t.Commit.CommittedDate.Format("2006-01-02 15:04")
				}
			}
			if t.Release != nil {
				ti.ReleaseDesc = t.Release.Description
			}
			tags = append(tags, ti)
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return tags, nil
}

// CreateTag creates a new tag in the repository.
func (c *Client) CreateTag(projectID int, name, ref, message string) (*TagInfo, error) {
	opts := &gl.CreateTagOptions{
		TagName: gl.Ptr(name),
		Ref:     gl.Ptr(ref),
	}
	if message != "" {
		opts.Message = gl.Ptr(message)
	}
	t, _, err := c.raw.Tags.CreateTag(projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("creating tag %q from %q: %w", name, ref, err)
	}
	ti := &TagInfo{
		Name:    t.Name,
		Target:  t.Target,
		Message: t.Message,
	}
	if t.Commit != nil {
		ti.CommitTitle = t.Commit.Title
		ti.CommitID = t.Commit.ID
		ti.ShortID = t.Commit.ShortID
		ti.AuthorName = t.Commit.AuthorName
		if t.Commit.CommittedDate != nil {
			ti.Date = t.Commit.CommittedDate.Format("2006-01-02 15:04")
		}
	}
	if t.Release != nil {
		ti.ReleaseDesc = t.Release.Description
	}
	return ti, nil
}

// DeleteTag deletes a tag in the repository.
func (c *Client) DeleteTag(projectID int, tag string) error {
	_, err := c.raw.Tags.DeleteTag(projectID, tag)
	if err != nil {
		return fmt.Errorf("deleting tag %q: %w", tag, err)
	}
	return nil
}

// UpdateTagRelease creates or updates the GitLab Release description for a tag.
// If the tag already has a release, it is updated; otherwise a new release is created.
func (c *Client) UpdateTagRelease(projectID int, tagName, description string) error {
	// Try updating first
	_, _, err := c.raw.Releases.UpdateRelease(projectID, tagName, &gl.UpdateReleaseOptions{
		Description: gl.Ptr(description),
	})
	if err == nil {
		return nil
	}
	// If update failed (release does not exist), create it
	_, _, err2 := c.raw.Releases.CreateRelease(projectID, &gl.CreateReleaseOptions{
		TagName:     gl.Ptr(tagName),
		Description: gl.Ptr(description),
	})
	if err2 != nil {
		return fmt.Errorf("updating release for tag %q: %w", tagName, err2)
	}
	return nil
}

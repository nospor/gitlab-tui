package gitlab

import (
	"fmt"
	"net/http"
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
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
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
	Description    string
	Assignees      []string
	Reviewers      []string
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
			Description:    mr.Description,
			Draft:          mr.Draft,
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
		Description:    mr.Description,
		Draft:          mr.Draft,
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
	IID         int
	Title       string
	State       string
	Author      string
	WebURL      string
	CreatedAt   string
	UpdatedAt   string
	Labels      []string
	Description string
	Assignees   []string
	Upvotes     int
	Downvotes   int
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
			IID:         i64(iss.IID),
			Title:       iss.Title,
			State:       iss.State,
			WebURL:      iss.WebURL,
			Description: iss.Description,
			Upvotes:     i64(iss.Upvotes),
			Downvotes:   i64(iss.Downvotes),
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

// ─── Current User ─────────────────────────────────────────────────────────────

// WhoAmI returns the username of the authenticated user.
func (c *Client) WhoAmI() (string, error) {
	u, _, err := c.raw.Users.CurrentUser()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

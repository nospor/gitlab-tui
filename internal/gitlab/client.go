package gitlab

import (
	"fmt"
	"net/http"
	"time"

	gl "gitlab.com/gitlab-org/api/client-go"
)

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

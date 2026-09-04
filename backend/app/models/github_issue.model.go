package models

import (
	"time"

	"github.com/google/uuid"
)

// GithubIssue records an issue the GitHub channel opened, so archiving the
// exception it tracks can close it again. ClosedAt doubles as the state: an
// issue still open in GitHub has none.
//
// Rows are keyed per channel rather than per rule: several rules can point at
// the same GitHub channel, and the issue belongs to the repository it landed
// in, which is the channel's configuration. A regression opens a second issue
// under the same IssueKey, so a key can carry more than one row over time.
type GithubIssue struct {
	Id          int        `json:"id" lit:"id"`
	ProjectId   uuid.UUID  `json:"projectId" lit:"project_id"`
	ChannelId   int        `json:"channelId" lit:"channel_id"`
	IssueKey    string     `json:"issueKey" lit:"issue_key"`
	Owner       string     `json:"owner" lit:"owner"`
	Repo        string     `json:"repo" lit:"repo"`
	IssueNumber int        `json:"issueNumber" lit:"issue_number"`
	CreatedAt   time.Time  `json:"createdAt" lit:"created_at"`
	ClosedAt    *time.Time `json:"closedAt" lit:"closed_at"`
}

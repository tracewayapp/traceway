//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type githubIssueRepository struct{}

const githubIssueColumns = "id, project_id, channel_id, issue_key, owner, repo, issue_number, created_at, closed_at"

// Create records an issue the GitHub channel just opened.
func (r *githubIssueRepository) Create(tx *sql.Tx, issue *models.GithubIssue) (int, error) {
	return lit.Insert[models.GithubIssue](tx, issue)
}

// FindOpenByIssueKey returns every issue still open for one exception hash.
// A hash can hold more than one: several GitHub channels can watch the same
// project, and a regression opens a fresh issue after the last one closed.
func (r *githubIssueRepository) FindOpenByIssueKey(tx *sql.Tx, projectId uuid.UUID, issueKey string) ([]*models.GithubIssue, error) {
	return lit.SelectNamed[models.GithubIssue](
		tx,
		"SELECT "+githubIssueColumns+" FROM github_issues WHERE project_id = :project_id AND issue_key = :issue_key AND closed_at IS NULL ORDER BY id ASC",
		lit.P{"project_id": projectId, "issue_key": issueKey},
	)
}

// MarkClosed stops tracking an issue. The closed_at guard makes a second
// archive of the same hash a no-op instead of a duplicate close.
func (r *githubIssueRepository) MarkClosed(tx *sql.Tx, id int, closedAt time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE github_issues SET closed_at = :closed_at WHERE id = :id AND closed_at IS NULL",
		lit.P{"closed_at": closedAt.UTC(), "id": id},
	)
}

var GithubIssueRepository = githubIssueRepository{}

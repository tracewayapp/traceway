package reports

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"example.com/reports-api/internal/httpx"
)

// Summary is one entry of the reports list.
type Summary struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	OwnerID   int64     `json:"ownerId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Catalog lists the saved reports of a workspace.
type Catalog struct{ db *sql.DB }

// NewCatalog returns a Catalog.
func NewCatalog(db *sql.DB) *Catalog { return &Catalog{db: db} }

// List serves GET /api/reports for the workspace in X-Workspace-Id.
func (c *Catalog) List(w http.ResponseWriter, r *http.Request) {
	workspace, err := strconv.ParseInt(r.Header.Get("X-Workspace-Id"), 10, 64)
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "bad workspace id"})
		return
	}
	rs, err := c.db.QueryContext(r.Context(), `SELECT id, name, owner_id, updated_at FROM reports WHERE workspace_id = $1 ORDER BY updated_at DESC LIMIT 50`, workspace)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	defer rs.Close()
	out := []Summary{}
	for rs.Next() {
		var s Summary
		if err := rs.Scan(&s.ID, &s.Name, &s.OwnerID, &s.UpdatedAt); err != nil {
			httpx.Error(w, r, err)
			return
		}
		out = append(out, s)
	}
	if err := rs.Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

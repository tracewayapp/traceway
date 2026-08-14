//go:build transactional_pg

package pg

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type statusPageRepository struct{}

const statusPageColumns = "id, organization_id, slug, name, is_public, check_ids, description, logo_key, custom_domain, created_at, updated_at"

func (r *statusPageRepository) Create(tx *sql.Tx, page *models.StatusPage) (int, error) {
	return lit.Insert[models.StatusPage](tx, page)
}

func (r *statusPageRepository) Update(tx *sql.Tx, page *models.StatusPage) error {
	return lit.UpdateNamed(tx, page, "id = :id", lit.P{"id": page.Id})
}

func (r *statusPageRepository) Delete(tx *sql.Tx, id int, organizationId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM status_pages WHERE id = :id AND organization_id = :organization_id", lit.P{"id": id, "organization_id": organizationId})
}

func (r *statusPageRepository) FindByIdForOrganization(tx *sql.Tx, id int, organizationId int) (*models.StatusPage, error) {
	return lit.SelectSingleNamed[models.StatusPage](
		tx,
		"SELECT "+statusPageColumns+" FROM status_pages WHERE id = :id AND organization_id = :organization_id",
		lit.P{"id": id, "organization_id": organizationId},
	)
}

func (r *statusPageRepository) ListByOrganization(tx *sql.Tx, organizationId int) ([]*models.StatusPage, error) {
	return lit.SelectNamed[models.StatusPage](
		tx,
		"SELECT "+statusPageColumns+" FROM status_pages WHERE organization_id = :organization_id ORDER BY name ASC, id ASC",
		lit.P{"organization_id": organizationId},
	)
}

func (r *statusPageRepository) FindBySlug(tx *sql.Tx, slug string) (*models.StatusPage, error) {
	return lit.SelectSingleNamed[models.StatusPage](
		tx,
		"SELECT "+statusPageColumns+" FROM status_pages WHERE slug = :slug",
		lit.P{"slug": slug},
	)
}

// FindPublicBySlug backs the anonymous status endpoint: private pages are
// indistinguishable from missing ones.
func (r *statusPageRepository) FindPublicBySlug(tx *sql.Tx, slug string) (*models.StatusPage, error) {
	return lit.SelectSingleNamed[models.StatusPage](
		tx,
		"SELECT "+statusPageColumns+" FROM status_pages WHERE slug = :slug AND is_public = true",
		lit.P{"slug": slug},
	)
}

// FindByCustomDomain backs the create/update uniqueness check.
func (r *statusPageRepository) FindByCustomDomain(tx *sql.Tx, domain string) (*models.StatusPage, error) {
	return lit.SelectSingleNamed[models.StatusPage](
		tx,
		"SELECT "+statusPageColumns+" FROM status_pages WHERE custom_domain <> '' AND custom_domain = :domain",
		lit.P{"domain": domain},
	)
}

// FindPublicByCustomDomain resolves a vanity host (CNAME to this instance)
// to its status page.
func (r *statusPageRepository) FindPublicByCustomDomain(tx *sql.Tx, domain string) (*models.StatusPage, error) {
	return lit.SelectSingleNamed[models.StatusPage](
		tx,
		"SELECT "+statusPageColumns+" FROM status_pages WHERE custom_domain <> '' AND custom_domain = :domain AND is_public = true",
		lit.P{"domain": domain},
	)
}

var StatusPageRepository = statusPageRepository{}

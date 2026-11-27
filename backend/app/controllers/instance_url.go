package controllers

import (
	"github.com/gin-gonic/gin"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/services/authserver"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// instanceUrl is the address SDKs, upload tools and the CLI reach this server at: APP_BASE_URL, or the origin the
// request came in on when the operator never set it (most self-hosted servers). models.GetBackendUrl alone would
// hand a self-hosted server's users the cloud address, which the SDKs now use by default when no baseUrl is given.
// The value goes through the SDKs' own URL rules, so an APP_BASE_URL without a scheme or with a trailing /api
// still names the server the SDKs reach.
func instanceUrl(c *gin.Context) string {
	url := authserver.IssuerBaseURLFromRequest(c)
	if url == "" {
		url = models.GetBackendUrl()
	}
	if normalized, err := traceway.NormalizeBaseURL(url); err == nil {
		return normalized
	}
	return url
}

func withInstanceUrl(c *gin.Context, projects ...*models.ProjectWithBackendUrl) {
	url := instanceUrl(c)
	for _, p := range projects {
		if p != nil {
			p.BackendUrl = url
		}
	}
}

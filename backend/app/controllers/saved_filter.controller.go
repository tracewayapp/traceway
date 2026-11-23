package controllers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

type savedFilterController struct{}

var SavedFilterController = savedFilterController{}

// savedFilterPageParams lists, per page, the URL params a saved filter keeps.
// Anything else in the submitted query (the project id, a stray param) is
// dropped, so applying a filter never moves the user to another project.
var savedFilterPageParams = map[string]map[string]bool{
	"logs": {
		"preset": true, "from": true, "to": true,
		"search": true, "searchType": true, "minSeverity": true,
		"service": true, "traceId": true, "spanId": true, "scopeName": true, "body": true,
		"attr":  true,
		"chart": true, "chartView": true, "chartAgg": true,
	},
}

const (
	maxSavedFiltersPerPage    = 100
	maxSavedFilterNameLength  = 100
	maxSavedFilterQueryLength = 8 * 1024
)

type savedFilterCreateRequest struct {
	Page  string `json:"page"`
	Name  string `json:"name"`
	Query string `json:"query"`
}

type savedFilterUpdateRequest struct {
	Name  *string `json:"name"`
	Query *string `json:"query"`
}

type savedFilterError string

func (e savedFilterError) Error() string { return string(e) }

func validSavedFilterPage(page string) bool {
	_, ok := savedFilterPageParams[page]
	return ok
}

func savedFilterName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", savedFilterError("Name is required")
	}
	if utf8.RuneCountInString(name) > maxSavedFilterNameLength {
		return "", savedFilterError("Name must be at most 100 characters")
	}
	return name, nil
}

// canonicalSavedFilterQuery keeps the params the page knows, in a stable order.
func canonicalSavedFilterQuery(page, raw string) (string, error) {
	values, err := url.ParseQuery(strings.TrimPrefix(strings.TrimSpace(raw), "?"))
	if err != nil {
		return "", savedFilterError("Query is not a valid URL query string")
	}
	allowed := savedFilterPageParams[page]
	kept := url.Values{}
	for key, list := range values {
		if !allowed[key] {
			continue
		}
		for _, v := range list {
			if v != "" {
				kept.Add(key, v)
			}
		}
	}
	if len(kept) == 0 {
		return "", savedFilterError("There is nothing to save: add a filter, a search or a chart first")
	}
	query := kept.Encode()
	if len(query) > maxSavedFilterQueryLength {
		return "", savedFilterError("These filters are too long to save (at most 8 KB)")
	}
	return query, nil
}

func savedFilterProjectId(c *gin.Context) (uuid.UUID, bool) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return uuid.Nil, false
	}
	return projectId, true
}

func savedFilterIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid saved filter id"})
		return 0, false
	}
	return id, true
}

// savedFilterNameTaken compares case-insensitively, so "Slow requests" and "slow requests"
// cannot both appear in the menu.
func savedFilterNameTaken(existing []*models.SavedFilterListItem, name string, exceptId int) bool {
	for _, f := range existing {
		if f.Id != exceptId && strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

func (savedFilterController) List(c *gin.Context) {
	projectId, ok := savedFilterProjectId(c)
	if !ok {
		return
	}
	page := c.Query("page")
	if !validSavedFilterPage(page) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be logs"})
		return
	}
	filters, err := transactional.SavedFilterRepository.List(db.GetTx(c), projectId, page)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list saved filters: %w", err))
		return
	}
	if filters == nil {
		filters = []*models.SavedFilterListItem{}
	}
	c.JSON(http.StatusOK, gin.H{"data": filters})
}

func (savedFilterController) Create(c *gin.Context) {
	projectId, ok := savedFilterProjectId(c)
	if !ok {
		return
	}
	var req savedFilterCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	if !validSavedFilterPage(req.Page) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "page must be logs"})
		return
	}
	name, err := savedFilterName(req.Name)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	query, err := canonicalSavedFilterQuery(req.Page, req.Query)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	tx := db.GetTx(c)
	existing, err := transactional.SavedFilterRepository.List(tx, projectId, req.Page)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list saved filters: %w", err))
		return
	}
	if len(existing) >= maxSavedFiltersPerPage {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A project can have at most 100 saved filters per page"})
		return
	}
	if savedFilterNameTaken(existing, name, 0) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A saved filter with this name already exists"})
		return
	}
	userId := middleware.GetUserId(c)
	now := ruleTimestamp()
	filter := &models.SavedFilter{
		ProjectId: projectId,
		Page:      req.Page,
		Name:      name,
		Query:     query,
		CreatedBy: &userId,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := transactional.SavedFilterRepository.Create(tx, filter)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create saved filter: %w", err))
		return
	}
	filter.Id = id
	c.JSON(http.StatusCreated, filter)
}

func (savedFilterController) Update(c *gin.Context) {
	projectId, ok := savedFilterProjectId(c)
	if !ok {
		return
	}
	id, ok := savedFilterIdParam(c)
	if !ok {
		return
	}
	var req savedFilterUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(c, err, "Invalid request body")
		return
	}
	tx := db.GetTx(c)
	filter, err := transactional.SavedFilterRepository.Find(tx, projectId, id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load saved filter: %w", err))
		return
	}
	if filter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved filter not found"})
		return
	}
	if req.Name != nil {
		name, err := savedFilterName(*req.Name)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		existing, err := transactional.SavedFilterRepository.List(tx, projectId, filter.Page)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list saved filters: %w", err))
			return
		}
		if savedFilterNameTaken(existing, name, filter.Id) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A saved filter with this name already exists"})
			return
		}
		filter.Name = name
	}
	if req.Query != nil {
		query, err := canonicalSavedFilterQuery(filter.Page, *req.Query)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		filter.Query = query
	}
	filter.UpdatedAt = ruleTimestamp()
	if err := transactional.SavedFilterRepository.Update(tx, filter); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update saved filter: %w", err))
		return
	}
	c.JSON(http.StatusOK, filter)
}

func (savedFilterController) Delete(c *gin.Context) {
	projectId, ok := savedFilterProjectId(c)
	if !ok {
		return
	}
	id, ok := savedFilterIdParam(c)
	if !ok {
		return
	}
	tx := db.GetTx(c)
	filter, err := transactional.SavedFilterRepository.Find(tx, projectId, id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load saved filter: %w", err))
		return
	}
	if filter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved filter not found"})
		return
	}
	if err := transactional.SavedFilterRepository.Delete(tx, projectId, id); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete saved filter: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

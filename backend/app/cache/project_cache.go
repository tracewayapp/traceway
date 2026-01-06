package cache

import (
	"backend/app/models"
	"backend/app/repositories"
	"context"
	"sort"
	"sync"
	"time"
)

type projectCache struct {
	projects     map[string]*models.Project // key: token
	projectsById map[string]*models.Project // key: id
	mu           sync.RWMutex
	lastRefresh  time.Time
}

// ProjectCache is the global project cache instance
var ProjectCache = &projectCache{
	projects:     make(map[string]*models.Project),
	projectsById: make(map[string]*models.Project),
}

// Init initializes the cache by loading all projects from the database
func (c *projectCache) Init(ctx context.Context) error {
	return c.Refresh(ctx)
}

// Refresh reloads all projects from the database into the cache
func (c *projectCache) Refresh(ctx context.Context) error {
	projects, err := repositories.ProjectRepository.FindAll(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.projects = make(map[string]*models.Project)
	c.projectsById = make(map[string]*models.Project)

	for i := range projects {
		proj := &projects[i]
		c.projects[proj.Token] = proj
		c.projectsById[proj.Id] = proj
	}
	c.lastRefresh = time.Now()

	return nil
}

// GetByToken returns a project by its token, or nil if not found
func (c *projectCache) GetByToken(token string) *models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projects[token]
}

// GetById returns a project by its ID, or nil if not found
func (c *projectCache) GetById(id string) *models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projectsById[id]
}

// GetAll returns all cached projects sorted by created_at (oldest first)
func (c *projectCache) GetAll() []*models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*models.Project, 0, len(c.projectsById))
	for _, proj := range c.projectsById {
		result = append(result, proj)
	}

	// Sort by CreatedAt ascending (oldest first, newest last)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}

// AddProject adds a project to the cache
func (c *projectCache) AddProject(proj *models.Project) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.projects[proj.Token] = proj
	c.projectsById[proj.Id] = proj
}

// LastRefresh returns the time of the last cache refresh
func (c *projectCache) LastRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastRefresh
}

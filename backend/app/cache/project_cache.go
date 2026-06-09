package cache

import (
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"context"
	"database/sql"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type projectCache struct {
	projects                 map[string]*models.Project    // key: token
	projectsById             map[uuid.UUID]*models.Project // key: id
	projectsBySourceMapToken map[string]*models.Project    // key: source_map_token
	mu                       sync.RWMutex
	lastRefresh              time.Time
}

var ProjectCache = &projectCache{
	projects:                 make(map[string]*models.Project),
	projectsById:             make(map[uuid.UUID]*models.Project),
	projectsBySourceMapToken: make(map[string]*models.Project),
}

func (c *projectCache) Init(ctx context.Context) error {
	return c.Refresh(ctx)
}

func (c *projectCache) Refresh(ctx context.Context) error {
	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return repositories.ProjectRepository.FindAll(tx)
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.projects = make(map[string]*models.Project)
	c.projectsById = make(map[uuid.UUID]*models.Project)
	c.projectsBySourceMapToken = make(map[string]*models.Project)

	for i := range projects {
		proj := projects[i]
		c.projects[proj.Token] = proj
		c.projectsById[proj.Id] = proj
		if proj.SourceMapToken != nil {
			c.projectsBySourceMapToken[*proj.SourceMapToken] = proj
		}
	}
	c.lastRefresh = time.Now()

	return nil
}

func (c *projectCache) GetByToken(token string) *models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projects[token]
}

func (c *projectCache) GetById(id uuid.UUID) *models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projectsById[id]
}

func (c *projectCache) GetAll() []*models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*models.Project, 0, len(c.projectsById))
	for _, proj := range c.projectsById {
		result = append(result, proj)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}

func (c *projectCache) AddProject(proj *models.Project) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.projects[proj.Token] = proj
	c.projectsById[proj.Id] = proj
	if proj.SourceMapToken != nil {
		c.projectsBySourceMapToken[*proj.SourceMapToken] = proj
	}
}

func (c *projectCache) UpdateProject(proj *models.Project) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.projectsById[proj.Id]
	if !ok {
		return
	}
	cached.Name = proj.Name
	cached.Framework = proj.Framework
	cached.DropHealthyHealthchecks = proj.DropHealthyHealthchecks
	cached.HealthcheckPaths = proj.HealthcheckPaths
}

func (c *projectCache) RemoveProject(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	proj, ok := c.projectsById[id]
	if !ok {
		return
	}
	delete(c.projects, proj.Token)
	delete(c.projectsById, id)
	if proj.SourceMapToken != nil {
		delete(c.projectsBySourceMapToken, *proj.SourceMapToken)
	}
}

func (c *projectCache) GetBySourceMapToken(token string) *models.Project {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projectsBySourceMapToken[token]
}

func (c *projectCache) UpdateSourceMapToken(projectId uuid.UUID, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	proj, ok := c.projectsById[projectId]
	if !ok {
		return
	}
	if proj.SourceMapToken != nil {
		delete(c.projectsBySourceMapToken, *proj.SourceMapToken)
	}
	proj.SourceMapToken = &token
	c.projectsBySourceMapToken[token] = proj
}

func (c *projectCache) LastRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastRefresh
}

func (c *projectCache) Entries() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.projectsById)
}

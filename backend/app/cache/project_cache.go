package cache

import (
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
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
	projects, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
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

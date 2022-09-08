package cache

import (
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/db/types"
	"sync"
	"time"
)

type Cache interface {
	AddSession(session *types.Session)
	HasSession(sessionID uint64) bool
	GetSession(sessionID uint64) (*types.Session, error)
	DeleteSession(sessionID uint64)
	GetProject(projectID uint32) (*types.Project, error)
}

func (c *cacheImpl) AddSession(session *types.Session) {
	c.sessions[session.SessionID] = session
}

func (c *cacheImpl) HasSession(sessionID uint64) bool {
	return c.sessions[sessionID] != nil
}

type cacheImpl struct {
	conn                     postgres.Pool
	sessions                 map[uint64]*types.Session //remove by timeout to avoid memory leak
	projects                 map[uint32]*ProjectMeta
	projectsByKeys           sync.Map
	projectExpirationTimeout time.Duration
}

func New(pgConn postgres.Pool, projectExpirationTimeoutMs int64) (Cache, error) {
	return &cacheImpl{
		conn:                     pgConn,
		sessions:                 make(map[uint64]*types.Session),
		projects:                 make(map[uint32]*ProjectMeta),
		projectExpirationTimeout: time.Duration(1000 * projectExpirationTimeoutMs),
	}, nil
}

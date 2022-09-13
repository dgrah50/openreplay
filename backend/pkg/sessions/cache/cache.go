package cache

import (
	"fmt"
	"log"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/sessions/model"
	"sync"
	"time"
)

type Sessions interface {
	AddSession(session *model.Session)
	HasSession(sessionID uint64) bool
	GetSession(sessionID uint64) (*model.Session, error)
	DeleteSession(sessionID uint64)
	GetProject(projectID uint32) (*model.Project, error)
	GetProjectByKey(projectKey string) (*model.Project, error)
	InsertSessionEnd(sessionID uint64, e *messages.SessionEnd) error
	InsertMetadata(sessionID uint64, metadata *messages.Metadata) error
}

type cacheImpl struct {
	conn                     postgres.Pool
	sessions                 map[uint64]*model.Session //remove by timeout to avoid memory leak
	projects                 map[uint32]*ProjectMeta
	projectsByKeys           sync.Map
	projectExpirationTimeout time.Duration
}

func New(pgConn postgres.Pool, projectExpirationTimeoutMs int64) (Sessions, error) {
	return &cacheImpl{
		conn:                     pgConn,
		sessions:                 make(map[uint64]*model.Session),
		projects:                 make(map[uint32]*ProjectMeta),
		projectExpirationTimeout: time.Duration(1000 * projectExpirationTimeoutMs),
	}, nil
}

func (c *cacheImpl) InsertMetadata(sessionID uint64, metadata *messages.Metadata) error {
	session, err := c.GetSession(sessionID)
	if err != nil {
		return err
	}
	project, err := c.GetProject(session.ProjectID)
	if err != nil {
		return err
	}

	keyNo := project.GetMetadataNo(metadata.Key)

	if keyNo == 0 {
		// TODO: insert project metadata
		return nil
	}
	if err := c.insertMetadata(sessionID, keyNo, metadata.Value); err != nil {
		// Try to insert metadata after one minute
		time.AfterFunc(time.Minute, func() {
			if err := c.insertMetadata(sessionID, keyNo, metadata.Value); err != nil {
				log.Printf("metadata retry err: %s", err)
			}
		})
		return err
	}
	session.SetMetadata(keyNo, metadata.Value)
	return nil
}

func (c *cacheImpl) insertMetadata(sessionID uint64, keyNo uint, value string) error {
	sqlRequest := `
		UPDATE sessions-builder SET  metadata_%v = $1
		WHERE session_id = $2`
	return c.conn.Exec(fmt.Sprintf(sqlRequest, keyNo), value, sessionID)
}

func (c *cacheImpl) GetSessionDuration(sessionID uint64) (uint64, error) {
	var dur uint64
	if err := c.conn.QueryRow("SELECT COALESCE( duration, 0 ) FROM sessions-builder WHERE session_id=$1", sessionID).Scan(&dur); err != nil {
		return 0, err
	}
	return dur, nil
}

func (c *cacheImpl) InsertSessionEnd(sessionID uint64, e *messages.SessionEnd) error {
	currDuration, err := c.GetSessionDuration(sessionID)
	if err != nil {
		log.Printf("getSessionDuration failed, sessID: %d, err: %s", sessionID, err)
	}
	var newDuration uint64
	if err := c.conn.QueryRow(`
		UPDATE sessions-builder SET duration=$2 - start_ts
		WHERE session_id=$1
		RETURNING duration
	`,
		sessionID, e.Timestamp,
	).Scan(&newDuration); err != nil {
		return err
	}
	if currDuration == newDuration {
		return fmt.Errorf("sessionEnd duplicate, sessID: %d, prevDur: %d, newDur: %d", sessionID,
			currDuration, newDuration)
	}
	return nil
}

func (c *cacheImpl) AddSession(session *model.Session) {
	c.sessions[session.SessionID] = session
}

func (c *cacheImpl) HasSession(sessionID uint64) bool {
	return c.sessions[sessionID] != nil
}

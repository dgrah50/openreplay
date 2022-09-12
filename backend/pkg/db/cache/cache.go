package cache

import (
	"errors"
	"fmt"
	"log"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/db/types"
	"openreplay/backend/pkg/messages"
	"sync"
	"time"
)

type Sessions interface {
	AddSession(session *types.Session)
	HandleSessionStart(sessionID uint64, s *messages.SessionStart) (*types.Session, error)
	InsertSession(sessionID uint64, s *messages.SessionStart) error
	HasSession(sessionID uint64) bool
	GetSession(sessionID uint64) (*types.Session, error)
	DeleteSession(sessionID uint64)
	GetProject(projectID uint32) (*types.Project, error)
	GetProjectByKey(projectKey string) (*types.Project, error)
	InsertSessionEnd(sessionID uint64, e *messages.SessionEnd) error
	InsertUnstartedSession(s *UnstartedSession) error
	InsertMetadata(sessionID uint64, metadata *messages.Metadata) error
}

type cacheImpl struct {
	conn                     postgres.Pool
	sessions                 map[uint64]*types.Session //remove by timeout to avoid memory leak
	projects                 map[uint32]*ProjectMeta
	projectsByKeys           sync.Map
	projectExpirationTimeout time.Duration
}

func New(pgConn postgres.Pool, projectExpirationTimeoutMs int64) (Sessions, error) {
	return &cacheImpl{
		conn:                     pgConn,
		sessions:                 make(map[uint64]*types.Session),
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

func (c *cacheImpl) HandleSessionStart(sessionID uint64, s *messages.SessionStart) (*types.Session, error) {
	if c.HasSession(sessionID) {
		return nil, errors.New("this session already in cache")
	}
	newSession := &types.Session{
		SessionID:      sessionID,
		Platform:       "web",
		Timestamp:      s.Timestamp,
		ProjectID:      uint32(s.ProjectID),
		TrackerVersion: s.TrackerVersion,
		RevID:          s.RevID,
		UserUUID:       s.UserUUID,
		UserOS:         s.UserOS,
		UserOSVersion:  s.UserOSVersion,
		UserDevice:     s.UserDevice,
		UserCountry:    s.UserCountry,
		// web properties (TODO: unite different platform types)
		UserAgent:            s.UserAgent,
		UserBrowser:          s.UserBrowser,
		UserBrowserVersion:   s.UserBrowserVersion,
		UserDeviceType:       s.UserDeviceType,
		UserDeviceMemorySize: s.UserDeviceMemorySize,
		UserDeviceHeapSize:   s.UserDeviceHeapSize,
		UserID:               &s.UserID,
	}
	c.AddSession(newSession)
	return newSession, nil
}

type UnstartedSession struct {
	ProjectKey         string
	TrackerVersion     string
	DoNotTrack         bool
	Platform           string
	UserAgent          string
	UserOS             string
	UserOSVersion      string
	UserBrowser        string
	UserBrowserVersion string
	UserDevice         string
	UserDeviceType     string
	UserCountry        string
}

func (c *cacheImpl) InsertUnstartedSession(s *UnstartedSession) error {
	return c.conn.Exec(`
		INSERT INTO unstarted_sessions (
			project_id, 
			tracker_version, do_not_track, 
			platform, user_agent, 
			user_os, user_os_version, 
			user_browser, user_browser_version,
			user_device, user_device_type, 
			user_country
		) VALUES (
			(SELECT project_id FROM projects WHERE project_key = $1), 
			$2, $3,
			$4, $5, 
			$6, $7, 
			$8, $9,
			$10, $11,
			$12
		)`,
		s.ProjectKey,
		s.TrackerVersion, s.DoNotTrack,
		s.Platform, s.UserAgent,
		s.UserOS, s.UserOSVersion,
		s.UserBrowser, s.UserBrowserVersion,
		s.UserDevice, s.UserDeviceType,
		s.UserCountry,
	)
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

func (c *cacheImpl) AddSession(session *types.Session) {
	c.sessions[session.SessionID] = session
}

func (c *cacheImpl) HasSession(sessionID uint64) bool {
	return c.sessions[sessionID] != nil
}

func (c *cacheImpl) InsertSession(sessionID uint64, s *messages.SessionStart) error {
	return c.insertSessionStart(sessionID, &types.Session{
		SessionID:      sessionID,
		Platform:       "web",
		Timestamp:      s.Timestamp,
		ProjectID:      uint32(s.ProjectID),
		TrackerVersion: s.TrackerVersion,
		RevID:          s.RevID,
		UserUUID:       s.UserUUID,
		UserOS:         s.UserOS,
		UserOSVersion:  s.UserOSVersion,
		UserDevice:     s.UserDevice,
		UserCountry:    s.UserCountry,
		// web properties (TODO: unite different platform types)
		UserAgent:            s.UserAgent,
		UserBrowser:          s.UserBrowser,
		UserBrowserVersion:   s.UserBrowserVersion,
		UserDeviceType:       s.UserDeviceType,
		UserDeviceMemorySize: s.UserDeviceMemorySize,
		UserDeviceHeapSize:   s.UserDeviceHeapSize,
		UserID:               &s.UserID,
	})
}

func (c *cacheImpl) insertSessionStart(sessionID uint64, s *types.Session) error {
	return c.conn.Exec(`
		INSERT INTO sessions-builder (
			session_id, project_id, start_ts,
			user_uuid, user_device, user_device_type, user_country,
			user_os, user_os_version,
			rev_id, 
			tracker_version, issue_score,
			platform,
			user_agent, user_browser, user_browser_version, user_device_memory_size, user_device_heap_size,
			user_id
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7, 
			$8, NULLIF($9, ''),
			NULLIF($10, ''), 
			$11, $12,
			$13,
			NULLIF($14, ''), NULLIF($15, ''), NULLIF($16, ''), NULLIF($17, 0), NULLIF($18, 0::bigint),
			NULLIF($19, '')
		)`,
		sessionID, s.ProjectID, s.Timestamp,
		s.UserUUID, s.UserDevice, s.UserDeviceType, s.UserCountry,
		s.UserOS, s.UserOSVersion,
		s.RevID,
		s.TrackerVersion, s.Timestamp/1000,
		s.Platform,
		s.UserAgent, s.UserBrowser, s.UserBrowserVersion, s.UserDeviceMemorySize, s.UserDeviceHeapSize,
		s.UserID,
	)
}

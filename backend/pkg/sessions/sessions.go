package sessions

import (
	"errors"
	"openreplay/backend/pkg/db/autocomplete"
	"openreplay/backend/pkg/db/batch"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/sessions/cache"
	"openreplay/backend/pkg/sessions/model"
	"openreplay/backend/pkg/url"
	"strings"
)

type Sessions interface {
	InsertSession(sessionID uint64, s *messages.SessionStart) error
	InsertSessionReferrer(sessionID uint64, referrer string) error
	HandleWebSessionStart(sessionID uint64, s *messages.SessionStart) error
	HandleWebSessionEnd(sessionID uint64, e *messages.SessionEnd) error
	InsertWebUserID(sessionID uint64, userID *messages.UserID) error
	InsertWebUserAnonymousID(sessionID uint64, userAnonymousID *messages.UserAnonymousID) error
	HandleSessionStart(sessionID uint64, s *messages.SessionStart) (*model.Session, error)
	GetProjectByKey(projectKey string) (*model.Project, error)
	InsertUnstartedSession(s *model.UnstartedSession) error
}

type sessionsImpl struct {
	db            postgres.Pool
	cache         cache.Sessions
	batches       batch.Batches
	autocompletes autocomplete.Autocompletes
}

func New(db postgres.Pool, cache cache.Sessions) (Sessions, error) {
	return &sessionsImpl{
		db:    db,
		cache: cache,
	}, nil
}

func (s *sessionsImpl) InsertUnstartedSession(sess *model.UnstartedSession) error {
	return s.db.Exec(`
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
		sess.ProjectKey,
		sess.TrackerVersion, sess.DoNotTrack,
		sess.Platform, sess.UserAgent,
		sess.UserOS, sess.UserOSVersion,
		sess.UserBrowser, sess.UserBrowserVersion,
		sess.UserDevice, sess.UserDeviceType,
		sess.UserCountry,
	)
}

func (s *sessionsImpl) GetProjectByKey(projectKey string) (*model.Project, error) {
	return s.cache.GetProjectByKey(projectKey)
}

func getAutocompleteType(baseType string, platform string) string {
	if platform == "web" {
		return baseType
	}
	return baseType + "_" + strings.ToUpper(platform)
}

func (s *sessionsImpl) InsertSession(sessionID uint64, msg *messages.SessionStart) error {
	return s.db.Exec(`
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
		sessionID, uint32(msg.ProjectID), msg.Timestamp,
		msg.UserUUID, msg.UserDevice, msg.UserDeviceType, msg.UserCountry,
		msg.UserOS, msg.UserOSVersion,
		msg.RevID,
		msg.TrackerVersion, msg.Timestamp/1000,
		"web",
		msg.UserAgent, msg.UserBrowser, msg.UserBrowserVersion, msg.UserDeviceMemorySize, msg.UserDeviceHeapSize,
		msg.UserID,
	)
}

func (s *sessionsImpl) HandleSessionStart(sessionID uint64, msg *messages.SessionStart) (*model.Session, error) {
	if s.cache.HasSession(sessionID) {
		return nil, errors.New("this session already in cache")
	}
	newSession := &model.Session{
		SessionID:            sessionID,
		Platform:             "web",
		Timestamp:            msg.Timestamp,
		ProjectID:            uint32(msg.ProjectID),
		TrackerVersion:       msg.TrackerVersion,
		RevID:                msg.RevID,
		UserUUID:             msg.UserUUID,
		UserOS:               msg.UserOS,
		UserOSVersion:        msg.UserOSVersion,
		UserDevice:           msg.UserDevice,
		UserCountry:          msg.UserCountry,
		UserAgent:            msg.UserAgent,
		UserBrowser:          msg.UserBrowser,
		UserBrowserVersion:   msg.UserBrowserVersion,
		UserDeviceType:       msg.UserDeviceType,
		UserDeviceMemorySize: msg.UserDeviceMemorySize,
		UserDeviceHeapSize:   msg.UserDeviceHeapSize,
		UserID:               &msg.UserID,
	}
	s.cache.AddSession(newSession)
	return newSession, nil
}

func (s *sessionsImpl) HandleWebSessionStart(sessionID uint64, sess *messages.SessionStart) error {
	session, err := s.HandleSessionStart(sessionID, sess)
	if err != nil {
		return err
	}
	s.autocompletes.InsertValue(sessionID, session.ProjectID, getAutocompleteType("USEROS", session.Platform), session.UserOS)
	s.autocompletes.InsertValue(sessionID, session.ProjectID, getAutocompleteType("USERDEVICE", session.Platform), session.UserDevice)
	s.autocompletes.InsertValue(sessionID, session.ProjectID, getAutocompleteType("USERCOUNTRY", session.Platform), session.UserCountry)
	s.autocompletes.InsertValue(sessionID, session.ProjectID, getAutocompleteType("REVID", session.Platform), session.RevID)
	s.autocompletes.InsertValue(sessionID, session.ProjectID, "USERBROWSER", session.UserBrowser)
	return nil
}

func (s *sessionsImpl) HandleWebSessionEnd(sessionID uint64, e *messages.SessionEnd) error {
	sqlRequest := `
	UPDATE sessions-builder
		SET issue_types=(SELECT 
			CASE WHEN errors_count > 0 THEN
			  (COALESCE(ARRAY_AGG(DISTINCT ps.type), '{}') || 'js_exception'::issue_type)::issue_type[]
			ELSE
				(COALESCE(ARRAY_AGG(DISTINCT ps.type), '{}'))::issue_type[]
			END
    FROM events_common.issues
      INNER JOIN issues AS ps USING (issue_id)
                WHERE session_id = $1)
		WHERE session_id = $1`
	err := s.db.Exec(sqlRequest, sessionID)
	if err != nil {
		return err
	}
	s.cache.DeleteSession(sessionID)
	return nil
}

func (s *sessionsImpl) InsertSessionReferrer(sessionID uint64, referrer string) error {
	_, err := s.cache.GetSession(sessionID)
	if err != nil {
		return err
	}
	if referrer == "" {
		return nil
	}
	return s.db.Exec(`
		UPDATE sessions-builder 
		SET referrer = $1, base_referrer = $2
		WHERE session_id = $3 AND referrer IS NULL`,
		referrer, url.DiscardURLQuery(referrer), sessionID)
}

func (s *sessionsImpl) InsertWebUserID(sessionID uint64, userID *messages.UserID) error {
	session, err := s.cache.GetSession(sessionID)
	if err != nil {
		return err
	}
	err = s.InsertUserID(sessionID, userID.ID)
	if err == nil {
		s.autocompletes.InsertValue(sessionID, session.ProjectID, "USERID", userID.ID)
	}
	return err
}

func (s *sessionsImpl) InsertWebUserAnonymousID(sessionID uint64, userAnonymousID *messages.UserAnonymousID) error {
	session, err := s.cache.GetSession(sessionID)
	if err != nil {
		return err
	}
	err = s.InsertUserAnonymousID(sessionID, userAnonymousID.ID)
	if err == nil {
		s.autocompletes.InsertValue(sessionID, session.ProjectID, "USERANONYMOUSID", userAnonymousID.ID)
	}
	return err
}

func (s *sessionsImpl) InsertUserID(sessionID uint64, userID string) error {
	sqlRequest := `
		UPDATE sessions-builder SET  user_id = $1
		WHERE session_id = $2`
	s.batches.Queue(sessionID, sqlRequest, userID, sessionID)

	// Record approximate message size
	s.batches.UpdateSize(sessionID, len(sqlRequest)+len(userID)+8)
	return nil
}

func (s *sessionsImpl) InsertUserAnonymousID(sessionID uint64, userAnonymousID string) error {
	sqlRequest := `
		UPDATE sessions-builder SET  user_anonymous_id = $1
		WHERE session_id = $2`
	s.batches.Queue(sessionID, sqlRequest, userAnonymousID, sessionID)

	// Record approximate message size
	s.batches.UpdateSize(sessionID, len(sqlRequest)+len(userAnonymousID)+8)
	return nil
}

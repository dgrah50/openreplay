package postgres

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"openreplay/backend/pkg/db/types"
	"openreplay/backend/pkg/hashid"
	"openreplay/backend/pkg/messages"
)

func getAutocompleteType(baseType string, platform string) string {
	if platform == "web" {
		return baseType
	}
	return baseType + "_" + strings.ToUpper(platform)

}

func (conn *dbImpl) InsertWebSessionStart(sessionID uint64, s *messages.SessionStart) error {
	return conn.insertSessionStart(sessionID, &types.Session{
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

func (conn *dbImpl) insertSessionStart(sessionID uint64, s *types.Session) error {
	return conn.c.Exec(`
		INSERT INTO sessions (
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

func (conn *dbImpl) HandleWebSessionStart(sessionID uint64, s *messages.SessionStart) error {
	if conn.cacher.HasSession(sessionID) {
		return errors.New("this session already in cache")
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
	conn.cacher.AddSession(newSession)
	conn.handleSessionStart(sessionID, newSession)
	return nil
}

func (conn *dbImpl) handleSessionStart(sessionID uint64, s *types.Session) {
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("USEROS", s.Platform), s.UserOS)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("USERDEVICE", s.Platform), s.UserDevice)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("USERCOUNTRY", s.Platform), s.UserCountry)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("REVID", s.Platform), s.RevID)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, "USERBROWSER", s.UserBrowser)
}

func (conn *dbImpl) GetSessionDuration(sessionID uint64) (uint64, error) {
	var dur uint64
	if err := conn.c.QueryRow("SELECT COALESCE( duration, 0 ) FROM sessions WHERE session_id=$1", sessionID).Scan(&dur); err != nil {
		return 0, err
	}
	return dur, nil
}

func (conn *dbImpl) InsertSessionEnd(sessionID uint64, e *messages.SessionEnd) error {
	currDuration, err := conn.GetSessionDuration(sessionID)
	if err != nil {
		log.Printf("getSessionDuration failed, sessID: %d, err: %s", sessionID, err)
	}
	var newDuration uint64
	if err := conn.c.QueryRow(`
		UPDATE sessions SET duration=$2 - start_ts
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

func (conn *dbImpl) HandleWebSessionEnd(sessionID uint64, e *messages.SessionEnd) error {
	sqlRequest := `
	UPDATE sessions
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
	err := conn.c.Exec(sqlRequest, sessionID)
	if err != nil {
		return err
	}
	conn.cacher.DeleteSession(sessionID)
	return nil
}

func (conn *dbImpl) InsertRequest(sessionID uint64, timestamp uint64, index uint64, url string, duration uint64, success bool) error {
	if err := conn.requests.Append(sessionID, timestamp, getSqIdx(index), url, duration, success); err != nil {
		return fmt.Errorf("insert request in bulk err: %s", err)
	}
	return nil
}

func (conn *dbImpl) InsertCustomEvent(sessionID uint64, timestamp uint64, index uint64, name string, payload string) error {
	if err := conn.customEvents.Append(sessionID, timestamp, getSqIdx(index), name, payload); err != nil {
		return fmt.Errorf("insert custom event in bulk err: %s", err)
	}
	return nil
}

func (conn *dbImpl) InsertUserID(sessionID uint64, userID string) error {
	sqlRequest := `
		UPDATE sessions SET  user_id = $1
		WHERE session_id = $2`
	conn.batchQueue(sessionID, sqlRequest, userID, sessionID)

	// Record approximate message size
	conn.updateBatchSize(sessionID, len(sqlRequest)+len(userID)+8)
	return nil
}

func (conn *dbImpl) InsertUserAnonymousID(sessionID uint64, userAnonymousID string) error {
	sqlRequest := `
		UPDATE sessions SET  user_anonymous_id = $1
		WHERE session_id = $2`
	conn.batchQueue(sessionID, sqlRequest, userAnonymousID, sessionID)

	// Record approximate message size
	conn.updateBatchSize(sessionID, len(sqlRequest)+len(userAnonymousID)+8)
	return nil
}

func (conn *dbImpl) InsertMetadata(sessionID uint64, metadata *messages.Metadata) error {
	session, err := conn.cacher.GetSession(sessionID)
	if err != nil {
		return err
	}
	project, err := conn.cacher.GetProject(session.ProjectID)
	if err != nil {
		return err
	}

	keyNo := project.GetMetadataNo(metadata.Key)

	if keyNo == 0 {
		// TODO: insert project metadata
		return nil
	}
	if err := conn.insertMetadata(sessionID, keyNo, metadata.Value); err != nil {
		// Try to insert metadata after one minute
		time.AfterFunc(time.Minute, func() {
			if err := conn.insertMetadata(sessionID, keyNo, metadata.Value); err != nil {
				log.Printf("metadata retry err: %s", err)
			}
		})
		return err
	}
	session.SetMetadata(keyNo, metadata.Value)
	return nil
}

func (conn *dbImpl) insertMetadata(sessionID uint64, keyNo uint, value string) error {
	sqlRequest := `
		UPDATE sessions SET  metadata_%v = $1
		WHERE session_id = $2`
	return conn.c.Exec(fmt.Sprintf(sqlRequest, keyNo), value, sessionID)
}

func (conn *dbImpl) InsertIssueEvent(sessionID uint64, e *messages.IssueEvent) (err error) {
	session, err := conn.cacher.GetSession(sessionID)
	if err != nil {
		return err
	}
	tx, err := conn.c.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.rollback(); rollbackErr != nil {
				log.Printf("rollback err: %s", rollbackErr)
			}
		}
	}()
	issueID := hashid.IssueID(session.ProjectID, e)

	// TEMP. TODO: nullable & json message field type
	payload := &e.Payload
	if *payload == "" || *payload == "{}" {
		payload = nil
	}
	context := &e.Context
	if *context == "" || *context == "{}" {
		context = nil
	}

	if err = tx.exec(`
		INSERT INTO issues (
			project_id, issue_id, type, context_string, context
		) (SELECT
			project_id, $2, $3, $4, CAST($5 AS jsonb)
			FROM sessions
			WHERE session_id = $1
		)ON CONFLICT DO NOTHING`,
		sessionID, issueID, e.Type, e.ContextString, context,
	); err != nil {
		return err
	}
	if err = tx.exec(`
		INSERT INTO events_common.issues (
			session_id, issue_id, timestamp, seq_index, payload
		) VALUES (
			$1, $2, $3, $4, CAST($5 AS jsonb)
		)`,
		sessionID, issueID, e.Timestamp,
		getSqIdx(e.MessageID),
		payload,
	); err != nil {
		return err
	}
	if err = tx.exec(`
		UPDATE sessions SET issue_score = issue_score + $2
		WHERE session_id = $1`,
		sessionID, getIssueScore(e),
	); err != nil {
		return err
	}
	// TODO: no redundancy. Deliver to UI in a different way
	if e.Type == "custom" {
		if err = tx.exec(`
			INSERT INTO events_common.customs
				(session_id, seq_index, timestamp, name, payload, level)
			VALUES
				($1, $2, $3, left($4, 2700), $5, 'error')
			`,
			sessionID, getSqIdx(e.MessageID), e.Timestamp, e.ContextString, e.Payload,
		); err != nil {
			return err
		}
	}
	err = tx.commit()
	return
}

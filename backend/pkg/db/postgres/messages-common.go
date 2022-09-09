package postgres

import (
	"fmt"
	"log"
	"openreplay/backend/pkg/hashid"
	"openreplay/backend/pkg/messages"
	"strings"
)

func getAutocompleteType(baseType string, platform string) string {
	if platform == "web" {
		return baseType
	}
	return baseType + "_" + strings.ToUpper(platform)

}

func (conn *dbImpl) HandleWebSessionStart(sessionID uint64, sess *messages.SessionStart) error {
	s, err := conn.cacher.HandleSessionStart(sessionID, sess)
	if err != nil {
		return err
	}
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("USEROS", s.Platform), s.UserOS)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("USERDEVICE", s.Platform), s.UserDevice)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("USERCOUNTRY", s.Platform), s.UserCountry)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, getAutocompleteType("REVID", s.Platform), s.RevID)
	conn.insertAutocompleteValue(sessionID, s.ProjectID, "USERBROWSER", s.UserBrowser)
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

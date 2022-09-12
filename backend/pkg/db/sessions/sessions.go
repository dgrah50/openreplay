package sessions

import (
	"openreplay/backend/pkg/db/autocomplete"
	"openreplay/backend/pkg/db/batch"
	"openreplay/backend/pkg/db/cache"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/url"
	"strings"
)

type Sessions interface {
	InsertSessionReferrer(sessionID uint64, referrer string) error
	HandleWebSessionStart(sessionID uint64, s *messages.SessionStart) error
	HandleWebSessionEnd(sessionID uint64, e *messages.SessionEnd) error
	InsertWebUserID(sessionID uint64, userID *messages.UserID) error
	InsertWebUserAnonymousID(sessionID uint64, userAnonymousID *messages.UserAnonymousID) error
}

type sessionsImpl struct {
	db            postgres.Pool
	sessions      cache.Sessions
	batches       batch.Batches
	autocompletes autocomplete.Autocompletes
}

func New(db postgres.Pool, sessions cache.Sessions) (Sessions, error) {
	return &sessionsImpl{
		db:       db,
		sessions: sessions,
	}, nil
}

func getAutocompleteType(baseType string, platform string) string {
	if platform == "web" {
		return baseType
	}
	return baseType + "_" + strings.ToUpper(platform)

}

func (s *sessionsImpl) HandleWebSessionStart(sessionID uint64, sess *messages.SessionStart) error {
	session, err := s.sessions.HandleSessionStart(sessionID, sess)
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
	s.sessions.DeleteSession(sessionID)
	return nil
}

func (s *sessionsImpl) InsertSessionReferrer(sessionID uint64, referrer string) error {
	_, err := s.sessions.GetSession(sessionID)
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
	session, err := s.sessions.GetSession(sessionID)
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
	session, err := s.sessions.GetSession(sessionID)
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

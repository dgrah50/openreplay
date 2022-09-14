package cache

import (
	"errors"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"log"
	"openreplay/backend/pkg/sessions/model"
)

var NilSessionInCacheError = errors.New("nil session in error")

func (c *sessionsImpl) AddSession(session *model.Session) {
	c.sessions[session.SessionID] = session
}

func (c *sessionsImpl) HasSession(sessionID uint64) bool {
	return c.sessions[sessionID] != nil
}

func (c *sessionsImpl) GetSession(sessionID uint64) (*model.Session, error) {
	if s, inCache := c.sessions[sessionID]; inCache {
		if s == nil {
			return s, NilSessionInCacheError
		}
		return s, nil
	}
	s, err := c.getSession(sessionID)
	if err == pgx.ErrNoRows {
		c.sessions[sessionID] = nil
	}
	if err != nil {
		return nil, err
	}
	c.sessions[sessionID] = s
	// get project information
	p, err := c.GetProject(s.ProjectID)
	if err != nil {
		return nil, err
	}
	s.SaveRequestPayload = p.SaveRequestPayloads
	return s, nil
}

func (c *sessionsImpl) getSession(sessionID uint64) (*model.Session, error) {
	s := &model.Session{SessionID: sessionID}
	var revID, userOSVersion, userBrowserVersion *string
	var issueTypes pgtype.EnumArray
	if err := c.conn.QueryRow(`
		SELECT platform,
			duration, project_id, start_ts,
			user_uuid, user_os, user_os_version, 
			user_device, user_device_type, user_country,
			rev_id, tracker_version,
			user_id, user_anonymous_id, referrer,
			pages_count, events_count, errors_count, issue_types,
			user_browser, user_browser_version, issue_score,
			metadata_1, metadata_2, metadata_3, metadata_4, metadata_5,
			metadata_6, metadata_7, metadata_8, metadata_9, metadata_10
		FROM sessions
		WHERE session_id=$1 
	`,
		sessionID,
	).Scan(&s.Platform,
		&s.Duration, &s.ProjectID, &s.Timestamp,
		&s.UserUUID, &s.UserOS, &userOSVersion,
		&s.UserDevice, &s.UserDeviceType, &s.UserCountry,
		&revID, &s.TrackerVersion,
		&s.UserID, &s.UserAnonymousID, &s.Referrer,
		&s.PagesCount, &s.EventsCount, &s.ErrorsCount, &issueTypes,
		&s.UserBrowser, &userBrowserVersion, &s.IssueScore,
		&s.Metadata1, &s.Metadata2, &s.Metadata3, &s.Metadata4, &s.Metadata5,
		&s.Metadata6, &s.Metadata7, &s.Metadata8, &s.Metadata9, &s.Metadata10); err != nil {
		return nil, err
	}
	if userOSVersion != nil {
		s.UserOSVersion = *userOSVersion
	}
	if userBrowserVersion != nil {
		s.UserBrowserVersion = *userBrowserVersion
	}
	if revID != nil {
		s.RevID = *revID
	}
	if err := issueTypes.AssignTo(&s.IssueTypes); err != nil {
		log.Printf("can't scan IssueTypes, err: %s", err)
	}
	return s, nil
}

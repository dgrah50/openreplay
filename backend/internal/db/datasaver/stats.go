package datasaver

import (
	"errors"
	"log"
	"openreplay/backend/pkg/db/cache"
	. "openreplay/backend/pkg/db/types"
	. "openreplay/backend/pkg/messages"
)

func (si *Saver) InitStats() {
	// noop
}

func (si *Saver) InsertStats(sessionID uint64, msg Message) error {
	session, err := si.cacher.GetSession(sessionID)
	if session == nil {
		if err != nil && !errors.Is(err, cache.NilSessionInCacheError) {
			log.Printf("Error on session retrieving from cache: %v, SessionID: %v, Message: %v", err, sessionID, msg)
		}
		return err
	}
	switch m := msg.(type) {
	// Web
	case *PerformanceTrackAggr:
		return si.pg.InsertWebStatsPerformance(session.SessionID, m)
	case *ResourceEvent:
		return si.pg.InsertWebStatsResourceEvent(session.SessionID, m)
	}
	return nil
}

func (si *Saver) CommitStats() error {
	return nil
}

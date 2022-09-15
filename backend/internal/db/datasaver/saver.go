package datasaver

import (
	"errors"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/db/stats"
	"openreplay/backend/pkg/queue/types"
	"openreplay/backend/pkg/sessions/cache"
)

type Saver struct {
	sessions cache.Sessions
	events   postgres.Events
	stats    stats.Stats
	producer types.Producer
}

func New(sessions cache.Sessions, events postgres.Events, stats stats.Stats, producer types.Producer) (*Saver, error) {
	switch {
	case sessions == nil:
		return nil, errors.New("sessions is empty")
	case events == nil:
		return nil, errors.New("events is empty")
	case stats == nil:
		return nil, errors.New("stats is empty")
	}
	return &Saver{
		sessions: sessions,
		events:   events,
		stats:    stats,
		producer: producer,
	}, nil
}

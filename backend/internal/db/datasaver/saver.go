package datasaver

import (
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/db/stats"
	"openreplay/backend/pkg/queue/types"
	"openreplay/backend/pkg/sessions"
	"openreplay/backend/pkg/sessions/cache"
)

type Saver struct {
	cache    cache.Sessions
	sessions sessions.Sessions
	events   postgres.Events
	stats    stats.Stats
	producer types.Producer
}

func New(pg postgres.Events, cache cache.Sessions, producer types.Producer) *Saver {
	return &Saver{events: pg, cache: cache, producer: producer}
}

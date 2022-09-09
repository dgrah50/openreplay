package datasaver

import (
	"openreplay/backend/pkg/db/cache"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/queue/types"
)

type Saver struct {
	pg       postgres.DB
	cacher   cache.Cache
	producer types.Producer
}

func New(pg postgres.DB, cacher cache.Cache, producer types.Producer) *Saver {
	return &Saver{pg: pg, cacher: cacher, producer: producer}
}

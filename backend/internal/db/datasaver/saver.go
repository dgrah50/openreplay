package datasaver

import (
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/queue/types"
)

type Saver struct {
	pg       postgres.DB
	producer types.Producer
}

func New(pg postgres.DB, producer types.Producer) *Saver {
	return &Saver{pg: pg, producer: producer}
}

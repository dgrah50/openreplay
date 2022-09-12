package main

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	config "openreplay/backend/internal/config/integrations"
	"openreplay/backend/internal/integrations/clientManager"
	integrations2 "openreplay/backend/pkg/db/integrations"
	"openreplay/backend/pkg/monitoring"
	"time"

	"os"
	"os/signal"
	"syscall"

	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/intervals"
	"openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/queue"
	"openreplay/backend/pkg/token"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC | log.Llongfile)
	metrics := monitoring.New("integrations")
	cfg := config.New()

	// Create pool of connections to DB (postgres)
	conn, err := pgxpool.Connect(context.Background(), cfg.Postgres)
	if err != nil {
		log.Fatalf("pgxpool.Connect err: %s", err)
	}
	// Create pool wrapper
	connWrapper, err := postgres.NewPool(conn, metrics)
	if err != nil {
		log.Fatalf("can't create new pool wrapper: %s", err)
	}

	dbService, err := integrations2.New(connWrapper)
	if err != nil {
		log.Fatalf("can't init integrations")
	}

	tokenizer := token.NewTokenizer(cfg.TokenSecret)

	manager := clientManager.NewManager()

	dbService.IterateIntegrationsOrdered(func(i *integrations2.Integration, err error) {
		if err != nil {
			log.Printf("Postgres error: %v\n", err)
			return
		}
		log.Printf("Integration initialization: %v\n", *i)
		err = manager.Update(i)
		if err != nil {
			log.Printf("Integration parse error: %v | Integration: %v\n", err, *i)
			return
		}
	})

	producer := queue.NewProducer(cfg.MessageSizeLimit, true)
	defer producer.Close(15000)

	// TODO: check it
	listener, err := integrations2.NewIntegrationsListener(cfg.Postgres)
	if err != nil {
		log.Printf("Postgres listener error: %v\n", err)
		log.Fatalf("Postgres listener error")
	}
	defer listener.Close()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	tick := time.Tick(intervals.INTEGRATIONS_REQUEST_INTERVAL * time.Millisecond)

	log.Printf("Integration service started\n")
	manager.RequestAll()
	for {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal %v: terminating\n", sig)
			listener.Close()
			dbService.Close()
			os.Exit(0)
		case <-tick:
			log.Printf("Requesting all...\n")
			manager.RequestAll()
		case event := <-manager.Events:
			log.Printf("New integration event: %+v\n", *event.IntegrationEvent)
			sessionID := event.SessionID
			if sessionID == 0 {
				sessData, err := tokenizer.Parse(event.Token)
				if err != nil && err != token.EXPIRED {
					log.Printf("Error on token parsing: %v; Token: %v", err, event.Token)
					continue
				}
				sessionID = sessData.ID
			}
			producer.Produce(cfg.TopicAnalytics, sessionID, messages.Encode(event.IntegrationEvent))
		case err := <-manager.Errors:
			log.Printf("Integration error: %v\n", err)
		case i := <-manager.RequestDataUpdates:
			// log.Printf("Last request integration update: %v || %v\n", i, string(i.RequestData))
			if err := dbService.UpdateIntegrationRequestData(&i); err != nil {
				log.Printf("Postgres Update request_data error: %v\n", err)
			}
		case err := <-listener.Errors:
			log.Printf("Postgres listen error: %v\n", err)
			listener.Close()
			dbService.Close()
			os.Exit(0)
		case iPointer := <-listener.Integrations:
			log.Printf("Integration update: %v\n", *iPointer)
			err := manager.Update(iPointer)
			if err != nil {
				log.Printf("Integration parse error: %v | Integration: %v\n", err, *iPointer)
			}
		}
	}
}

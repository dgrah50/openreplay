package main

import (
	"encoding/binary"
	"log"
	"openreplay/backend/internal/sink/assetscache"
	"openreplay/backend/internal/sink/oswriter"
	"openreplay/backend/internal/storage"
	"time"

	"os"
	"os/signal"
	"syscall"

	"openreplay/backend/internal/config/sink"
	. "openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/queue"
	"openreplay/backend/pkg/queue/types"
	"openreplay/backend/pkg/url/assets"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC | log.Llongfile)

	cfg := sink.New()

	if _, err := os.Stat(cfg.FsDir); os.IsNotExist(err) {
		log.Fatalf("%v doesn't exist. %v", cfg.FsDir, err)
	}

	writer := oswriter.NewWriter(cfg.FsUlimit, cfg.FsDir)

	producer := queue.NewProducer()
	defer producer.Close(cfg.ProducerCloseTimeout)
	rewriter := assets.NewRewriter(cfg.AssetsOrigin)
	assetMessageHandler := assetscache.New(cfg, rewriter, producer)

	counter := storage.NewLogCounter()

	consumer := queue.NewMessageConsumer(
		cfg.GroupSink,
		[]string{
			cfg.TopicRawIOS,
			cfg.TopicRawWeb,
		},
		func(sessionID uint64, message Message, _ *types.Meta) {
			// Process assets
			message = assetMessageHandler.ParseAssets(sessionID, message)

			// Filter message
			typeID := message.TypeID()
			if !IsReplayerType(typeID) {
				return
			}

			// If message timestamp is empty, use at least ts of session start
			ts := message.Meta().Timestamp
			if ts == 0 {
				log.Printf("zero ts; sessID: %d, msg: %+v", sessionID, message)
			} else {
				// Log ts of last processed message
				counter.Update(sessionID, time.UnixMilli(ts))
			}

			value := message.Encode()
			var data []byte
			if IsIOSType(typeID) {
				data = value
			} else {
				data = make([]byte, len(value)+8)
				copy(data[8:], value[:])
				binary.LittleEndian.PutUint64(data[0:], message.Meta().Index)
			}
			if err := writer.Write(sessionID, data); err != nil {
				log.Printf("Writer error: %v\n", err)
			}
		},
		false,
	)
	log.Printf("Sink service started\n")

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	tick := time.Tick(30 * time.Second)
	for {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal %v: terminating\n", sig)
			if err := consumer.Commit(); err != nil {
				log.Printf("can't commit messages: %s", err)
			}
			consumer.Close()
			os.Exit(0)
		case <-tick:
			if err := writer.SyncAll(); err != nil {
				log.Fatalf("Sync error: %v\n", err)
			}
			counter.Print()
			if err := consumer.Commit(); err != nil {
				log.Printf("can't commit messages: %s", err)
			}
		default:
			err := consumer.ConsumeNext()
			if err != nil {
				log.Fatalf("Error on consumption: %v", err)
			}
		}
	}

}

package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/SocialRoots/sr-email/pkg/email"
	"github.com/SocialRoots/sr-email/settings"
)

func main() {
	interval := settings.CronInterval
	log.Printf("Starting sr-email cron [interval=%s store=%s]",
		interval, settings.EmailStoreDir)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately on startup.
	email.ProcessInbox()

	for {
		select {
		case <-ticker.C:
			email.ProcessInbox()
		case <-ctx.Done():
			log.Println("sr-email cron shutting down")
			return
		}
	}
}
// cf-email-monitor — Cloudflare Email Routing Monitor
//
// Polls the Cloudflare Email Routing GraphQL API every minute,
// stores new messages to PostgreSQL, and serves a password-protected
// web dashboard (Gin) with full-text search and pagination.
//
// Required env vars:
//   CF_API_TOKEN    – Cloudflare API token (Email Routing / Analytics read)
//   CF_ZONE_TAG     – Zone tag (domain ID) from the Cloudflare dashboard
//   DATABASE_URL    – PostgreSQL DSN  e.g. postgres://user:pass@host/db?sslmode=require
//   UI_PASSWORD     – Dashboard access password
//
// Optional env vars:
//   PORT             – HTTP listen port    (default: 8080)
//   SESSION_SECRET   – Cookie signing key  (default: auto-generated)
//   LOOKBACK_MINUTES – Polling window      (default: 5)

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"src/internals"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// ─────────────────────────────────────────────────────────────────────────────
// Deduplication (in-memory; PostgreSQL ON CONFLICT is the durable layer)
// ─────────────────────────────────────────────────────────────────────────────

type seenSet struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

func newSeenSet() *seenSet { return &seenSet{ids: make(map[string]struct{})} }

func (s *seenSet) markNew(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ids[id]; ok {
		return false
	}
	s.ids[id] = struct{}{}
	return true
}

// ─────────────────────────────────────────────────────────────────────────────
// Poll
// ─────────────────────────────────────────────────────────────────────────────

func poll(cfg internals.Config, db *internals.DB, seen *seenSet) {
	now := time.Now().UTC()
	from := now.Add(-cfg.Lookback)

	log.Printf("[poll] tick=%s  window=%s → %s",
		now.Format("15:04:05 UTC"),
		from.Format("15:04:05"),
		now.Format("15:04:05"),
	)

	records, err := internals.FetchEmailActivity(cfg, from, now)
	if err != nil {
		log.Printf("[poll] ERROR fetching from Cloudflare: %v", err)
		return
	}

	// Pre-filter with in-memory cache to avoid redundant DB round-trips
	var fresh []internals.CFEmailRecord
	for _, r := range records {
		if seen.markNew(r.ID) {
			fresh = append(fresh, r)
		}
	}

	if len(fresh) == 0 {
		log.Printf("[poll] 0 new messages")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	inserted, err := db.UpsertBatch(ctx, fresh)
	if err != nil {
		log.Printf("[db] ERROR upserting %d records: %v", len(fresh), err)
		return
	}

	log.Printf("[poll] %d seen, %d inserted into DB", len(fresh), inserted)
	for _, r := range fresh {
		log.Printf("[new]  time=%-22s  to=%-35s  from=%-50s  subject=%q",
			r.Datetime, r.To, r.From, r.Subject)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	// Load .env if present (dev convenience — not required in production)
	_ = godotenv.Load()

	cfg := internals.LoadConfig()

	// Connect to PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := internals.OpenDB(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatalf("DB connect: %v", err)
	}

	// Run migrations
	mCtx, mCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := db.Migrate(mCtx); err != nil {
		log.Fatalf("DB migrate: %v", err)
	}
	mCancel()

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("  Cloudflare Email Routing Monitor")
	log.Printf("  Zone     : %s", cfg.CFZoneTag)
	log.Printf("  Poll     : every %s  (lookback %s)", cfg.PollInterval, cfg.Lookback)
	log.Printf("  Web      : http://0.0.0.0:%s", cfg.Port)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	seen := newSeenSet()

	// Start web server in background
	go internals.StartWebServer(cfg, db)
	go func() {
		// loop http get request url "https://cloudflare-log-email.onrender.com/healthz" every 5 minutes to keep the connection alive
		
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				resp, err := http.Get("https://cloudflare-log-email.onrender.com/healthz")
				if err != nil {
					log.Printf("[healthz] ERROR: %v", err)
					continue
				}
				resp.Body.Close()
				log.Printf("[healthz] pinged successfully")
			}
		}
	}()
		 
	}

	// Initial poll immediately
	poll(cfg, db, seen)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			poll(cfg, db, seen)
		case sig := <-quit:
			log.Printf("signal %v — shutting down", sig)
			return
		}
	}
}

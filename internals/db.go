package internals

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// EmailLog is the stored representation of one routing event.
type EmailLog struct {
	ID            int64     `json:"id"`
	SessionID     string    `json:"session_id"`
	MessageID     string    `json:"message_id"`
	ReceivedAt    time.Time `json:"received_at"`
	FromAddr      string    `json:"from_addr"`
	ToAddr        string    `json:"to_addr"`
	Subject       string    `json:"subject"`
	Status        string    `json:"status"`
	Action        string    `json:"action"`
	SPF           string    `json:"spf"`
	DKIM          string    `json:"dkim"`
	DMARC         string    `json:"dmarc"`
	ARC           string    `json:"arc"`
	ErrorDetail   string    `json:"error_detail"`
	IsNDR         bool      `json:"is_ndr"`
	IsSpam        bool      `json:"is_spam"`
	SpamScore     int       `json:"spam_score"`
	SpamThreshold int       `json:"spam_threshold"`
	InsertedAt    time.Time `json:"inserted_at"`
}

// LogQuery holds search/pagination params.
type LogQuery struct {
	Search   string
	FromAddr string
	ToAddr   string
	Status   string
	Action   string
	IsSpam   *bool
	DateFrom string
	DateTo   string
	Page     int
	PageSize int
}

// LogPage is the paginated result.
type LogPage struct {
	Rows       []EmailLog
	TotalCount int
	Page       int
	PageSize   int
	TotalPages int
}

func OpenDB(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Migrate(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS email_logs (
    id             BIGSERIAL PRIMARY KEY,
    session_id     TEXT        NOT NULL UNIQUE,
    message_id     TEXT        NOT NULL DEFAULT '',
    received_at    TIMESTAMPTZ NOT NULL,
    from_addr      TEXT        NOT NULL DEFAULT '',
    to_addr        TEXT        NOT NULL DEFAULT '',
    subject        TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL DEFAULT '',
    action         TEXT        NOT NULL DEFAULT '',
    spf            TEXT        NOT NULL DEFAULT '',
    dkim           TEXT        NOT NULL DEFAULT '',
    dmarc          TEXT        NOT NULL DEFAULT '',
    arc            TEXT        NOT NULL DEFAULT '',
    error_detail   TEXT        NOT NULL DEFAULT '',
    is_ndr         BOOLEAN     NOT NULL DEFAULT FALSE,
    is_spam        BOOLEAN     NOT NULL DEFAULT FALSE,
    spam_score     INT         NOT NULL DEFAULT 0,
    spam_threshold INT         NOT NULL DEFAULT 0,
    inserted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_logs_received_at   ON email_logs (received_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_logs_from_addr     ON email_logs (from_addr);
CREATE INDEX IF NOT EXISTS idx_email_logs_to_addr       ON email_logs (to_addr);
CREATE INDEX IF NOT EXISTS idx_email_logs_status        ON email_logs (status);
CREATE INDEX IF NOT EXISTS idx_email_logs_is_spam       ON email_logs (is_spam);
`)
	return err
}

// UpsertBatch inserts records, ignoring duplicates by session_id.
// Returns the count of actually inserted rows.
func (db *DB) UpsertBatch(ctx context.Context, records []CFEmailRecord) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	rows := make([][]interface{}, 0, len(records))
	for _, r := range records {
		t, err := time.Parse(time.RFC3339, r.Datetime)
		if err != nil {
			t = time.Now().UTC()
		}
		rows = append(rows, []interface{}{
			r.ID, r.MessageID, t,
			r.From, r.To, r.Subject,
			r.Status, r.Action,
			r.SPF, r.DKIM, r.DMARC, r.ARC,
			r.ErrorDetail,
			r.IsNDR != 0, r.IsSpam != 0,
			r.SpamScore, r.SpamThreshold,
		})
	}

	cols := []string{
		"session_id", "message_id", "received_at",
		"from_addr", "to_addr", "subject",
		"status", "action",
		"spf", "dkim", "dmarc", "arc",
		"error_detail",
		"is_ndr", "is_spam",
		"spam_score", "spam_threshold",
	}

	copyCount, err := db.pool.CopyFrom(
		ctx,
		pgx.Identifier{"email_logs"},
		cols,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		// On conflict we fall back to individual inserts to skip dupes
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return db.upsertOneByOne(ctx, records)
		}
		return 0, fmt.Errorf("CopyFrom: %w", err)
	}
	return int(copyCount), nil
}

func (db *DB) upsertOneByOne(ctx context.Context, records []CFEmailRecord) (int, error) {
	inserted := 0
	for _, r := range records {
		t, err := time.Parse(time.RFC3339, r.Datetime)
		if err != nil {
			t = time.Now().UTC()
		}
		tag, err := db.pool.Exec(ctx, `
INSERT INTO email_logs
    (session_id, message_id, received_at, from_addr, to_addr, subject,
     status, action, spf, dkim, dmarc, arc, error_detail,
     is_ndr, is_spam, spam_score, spam_threshold)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
ON CONFLICT (session_id) DO NOTHING`,
			r.ID, r.MessageID, t,
			r.From, r.To, r.Subject,
			r.Status, r.Action,
			r.SPF, r.DKIM, r.DMARC, r.ARC,
			r.ErrorDetail,
			r.IsNDR != 0, r.IsSpam != 0,
			r.SpamScore, r.SpamThreshold,
		)
		if err != nil {
			log.Printf("[db] upsert error for session %s: %v", r.ID, err)
			continue
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

// QueryLogs returns a paginated, filtered list of email logs.
func (db *DB) QueryLogs(ctx context.Context, q LogQuery) (LogPage, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 200 {
		q.PageSize = 50
	}

	var (
		where []string
		args  []interface{}
		n     = 1
	)

	addArg := func(clause string, val interface{}) {
		where = append(where, fmt.Sprintf(clause, n))
		args = append(args, val)
		n++
	}

	if q.Search != "" {
		like := "%" + strings.ToLower(q.Search) + "%"
		where = append(where, fmt.Sprintf(
			"(LOWER(from_addr) LIKE $%d OR LOWER(to_addr) LIKE $%d OR LOWER(subject) LIKE $%d OR LOWER(message_id) LIKE $%d)",
			n, n+1, n+2, n+3,
		))
		args = append(args, like, like, like, like)
		n += 4
	}
	if q.FromAddr != "" {
		addArg("LOWER(from_addr) LIKE $%d", "%"+strings.ToLower(q.FromAddr)+"%")
	}
	if q.ToAddr != "" {
		addArg("LOWER(to_addr) LIKE $%d", "%"+strings.ToLower(q.ToAddr)+"%")
	}
	if q.Status != "" {
		addArg("status = $%d", q.Status)
	}
	if q.Action != "" {
		addArg("action = $%d", q.Action)
	}
	if q.IsSpam != nil {
		addArg("is_spam = $%d", *q.IsSpam)
	}
	if q.DateFrom != "" {
		addArg("received_at >= $%d", q.DateFrom)
	}
	if q.DateTo != "" {
		addArg("received_at <= $%d", q.DateTo)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	// Count
	var total int
	countSQL := "SELECT COUNT(*) FROM email_logs " + whereSQL
	if err := db.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return LogPage{}, fmt.Errorf("count: %w", err)
	}

	totalPages := (total + q.PageSize - 1) / q.PageSize
	if totalPages == 0 {
		totalPages = 1
	}

	offset := (q.Page - 1) * q.PageSize
	dataSQL := fmt.Sprintf(`
SELECT id, session_id, message_id, received_at,
       from_addr, to_addr, subject, status, action,
       spf, dkim, dmarc, arc, error_detail,
       is_ndr, is_spam, spam_score, spam_threshold, inserted_at
FROM email_logs
%s
ORDER BY received_at DESC
LIMIT $%d OFFSET $%d`, whereSQL, n, n+1)

	dataArgs := append(args, q.PageSize, offset)
	rows, err := db.pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return LogPage{}, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var logs []EmailLog
	for rows.Next() {
		var e EmailLog
		if err := rows.Scan(
			&e.ID, &e.SessionID, &e.MessageID, &e.ReceivedAt,
			&e.FromAddr, &e.ToAddr, &e.Subject, &e.Status, &e.Action,
			&e.SPF, &e.DKIM, &e.DMARC, &e.ARC, &e.ErrorDetail,
			&e.IsNDR, &e.IsSpam, &e.SpamScore, &e.SpamThreshold, &e.InsertedAt,
		); err != nil {
			return LogPage{}, fmt.Errorf("scan: %w", err)
		}
		logs = append(logs, e)
	}

	return LogPage{
		Rows:       logs,
		TotalCount: total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// DistinctValues returns unique non-empty values of a column for filter dropdowns.
func (db *DB) DistinctValues(ctx context.Context, col string) ([]string, error) {
	safe := map[string]bool{"status": true, "action": true}
	if !safe[col] {
		return nil, fmt.Errorf("unsupported column: %s", col)
	}
	rows, err := db.pool.Query(ctx,
		fmt.Sprintf(`SELECT DISTINCT %s FROM email_logs WHERE %s <> '' ORDER BY %s`, col, col, col))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err == nil && v != "" {
			vals = append(vals, v)
		}
	}
	return vals, nil
}

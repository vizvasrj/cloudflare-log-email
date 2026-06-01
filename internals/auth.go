package internals

import (
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────────────────────
// Brute-force rate limiter
// ─────────────────────────────────────────────────────────────────────────────

const (
	maxAttempts   = 5
	lockoutPeriod = 15 * time.Minute
	windowPeriod  = 10 * time.Minute
)

type attemptRecord struct {
	count    int
	firstAt  time.Time
	lockedAt time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	records map[string]*attemptRecord
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{records: make(map[string]*attemptRecord)}
	go rl.gc()
	return rl
}

// check returns (allowed, retryAfterSeconds).
func (rl *rateLimiter) check(ip string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	r, ok := rl.records[ip]
	if !ok {
		return true, 0
	}
	now := time.Now()
	if !r.lockedAt.IsZero() {
		remaining := lockoutPeriod - now.Sub(r.lockedAt)
		if remaining > 0 {
			return false, int(remaining.Seconds())
		}
		delete(rl.records, ip)
		return true, 0
	}
	if now.Sub(r.firstAt) > windowPeriod {
		delete(rl.records, ip)
		return true, 0
	}
	if r.count >= maxAttempts {
		r.lockedAt = now
		return false, int(lockoutPeriod.Seconds())
	}
	return true, 0
}

func (rl *rateLimiter) record(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if r, ok := rl.records[ip]; ok {
		r.count++
		return
	}
	rl.records[ip] = &attemptRecord{count: 1, firstAt: time.Now()}
}

func (rl *rateLimiter) reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.records, ip)
}

func (rl *rateLimiter) gc() {
	for range time.Tick(5 * time.Minute) {
		rl.mu.Lock()
		now := time.Now()
		for ip, r := range rl.records {
			if (!r.lockedAt.IsZero() && now.Sub(r.lockedAt) > lockoutPeriod) ||
				(r.lockedAt.IsZero() && now.Sub(r.firstAt) > windowPeriod) {
				delete(rl.records, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Session store
// ─────────────────────────────────────────────────────────────────────────────

const sessionCookieName = "cf_monitor_session"

type sessionStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time
}

func newSessionStore() *sessionStore {
	ss := &sessionStore{tokens: make(map[string]time.Time)}
	go ss.gc()
	return ss
}

func (ss *sessionStore) create() string {
	token := generateSecret()
	ss.mu.Lock()
	ss.tokens[token] = time.Now().Add(24 * time.Hour)
	ss.mu.Unlock()
	return token
}

func (ss *sessionStore) valid(token string) bool {
	ss.mu.RLock()
	exp, ok := ss.tokens[token]
	ss.mu.RUnlock()
	return ok && time.Now().Before(exp)
}

func (ss *sessionStore) revoke(token string) {
	ss.mu.Lock()
	delete(ss.tokens, token)
	ss.mu.Unlock()
}

func (ss *sessionStore) gc() {
	for range time.Tick(time.Hour) {
		ss.mu.Lock()
		now := time.Now()
		for t, exp := range ss.tokens {
			if now.After(exp) {
				delete(ss.tokens, t)
			}
		}
		ss.mu.Unlock()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Gin middleware & handlers
// ─────────────────────────────────────────────────────────────────────────────

func requireAuth(sessions *sessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err != nil || !sessions.valid(token) {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func loginHandler(cfg Config, sessions *sessionStore, rl *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ok, retry := rl.check(ip); !ok {
			c.HTML(http.StatusTooManyRequests, "login.html", gin.H{
				"Error": "Too many failed attempts. Please wait before trying again.",
				"Retry": retry,
			})
			return
		}

		if c.Request.Method == http.MethodGet {
			c.HTML(http.StatusOK, "login.html", gin.H{})
			return
		}

		password := c.PostForm("password")
		if subtle.ConstantTimeCompare([]byte(password), []byte(cfg.UIPassword)) != 1 {
			rl.record(ip)
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"Error": "Incorrect password.",
			})
			return
		}

		rl.reset(ip)
		token := sessions.create()
		c.SetCookie(sessionCookieName, token, 86400, "/", "", false, true)
		c.Redirect(http.StatusFound, "/")
	}
}

func logoutHandler(sessions *sessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := c.Cookie(sessionCookieName); err == nil {
			sessions.revoke(token)
		}
		c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
	}
}

package internals

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func StartWebServer(cfg Config, db *DB) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	sessions := newSessionStore()
	rl := newRateLimiter()

	// Templates
	r.SetHTMLTemplate(buildTemplates())

	// Auth routes
	r.GET("/login", loginHandler(cfg, sessions, rl))
	r.POST("/login", loginHandler(cfg, sessions, rl))
	r.GET("/logout", logoutHandler(sessions))

	// Protected routes
	auth := r.Group("/", requireAuth(sessions))
	{
		auth.GET("/", logsHandler(cfg, db))
		auth.GET("/api/logs", logsAPIHandler(db))
	}

	addr := ":" + cfg.Port
	if err := r.Run(addr); err != nil {
		panic(err)
	}
}

// logsHandler renders the main HTML page (server-side with initial data).
func logsHandler(cfg Config, db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := parseLogQuery(c)
		page, err := db.QueryLogs(context.Background(), q)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
			return
		}
		statuses, _ := db.DistinctValues(context.Background(), "status")
		actions, _ := db.DistinctValues(context.Background(), "action")

		c.HTML(http.StatusOK, "logs.html", gin.H{
			"Page":     page,
			"Query":    q,
			"Statuses": statuses,
			"Actions":  actions,
			"ZoneTag":  cfg.CFZoneTag,
		})
	}
}

// logsAPIHandler returns JSON for optional JS-driven reload.
func logsAPIHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := parseLogQuery(c)
		page, err := db.QueryLogs(context.Background(), q)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, page)
	}
}

func parseLogQuery(c *gin.Context) LogQuery {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	q := LogQuery{
		Search:   c.Query("q"),
		FromAddr: c.Query("from"),
		ToAddr:   c.Query("to"),
		Status:   c.Query("status"),
		Action:   c.Query("action"),
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
		Page:     pageNum,
		PageSize: pageSize,
	}

	switch c.Query("spam") {
	case "true":
		t := true
		q.IsSpam = &t
	case "false":
		f := false
		q.IsSpam = &f
	}
	return q
}

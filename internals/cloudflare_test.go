package internals

import (
	"testing"
	"time"
)

func TestFetchEmailActivity(t *testing.T) {
	cfg := Config{}

	now := time.Now().UTC()
	from := now.Add(-time.Minute * 60)
	to := now

	records, err := FetchEmailActivity(cfg, from, to)
	if err != nil {
		t.Fatalf("FetchEmailActivity error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected some records but got none")
	}
	for _, r := range records {
		if r.ID == "" {
			t.Error("record missing ID")
		}
		t.Logf("%#v\n", r)

	}

}

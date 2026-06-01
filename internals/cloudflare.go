package internals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const cfGraphQLEndpoint = "https://api.cloudflare.com/client/v4/graphql"

const gqlQuery = `query EmailRoutingActivity($zoneTag: string, $filter: EmailRoutingAdaptiveFilter_InputObject) {
  viewer {
    zones(filter: { zoneTag: $zoneTag }) {
      emailRoutingAdaptive(
        filter: $filter
        limit: 100
        orderBy: [datetime_DESC]
      ) {
        datetime
        id: sessionId
        messageId
        from
        to
        subject
        status
        action
        spf
        dkim
        dmarc
        arc
        errorDetail
        isNDR
        isSpam
        spamScore
        spamThreshold
      }
    }
  }
}`

// CFEmailRecord mirrors the Cloudflare emailRoutingAdaptive response shape.
type CFEmailRecord struct {
	Datetime      string `json:"datetime"`
	ID            string `json:"id"`
	MessageID     string `json:"messageId"`
	From          string `json:"from"`
	To            string `json:"to"`
	Subject       string `json:"subject"`
	Status        string `json:"status"`
	Action        string `json:"action"`
	SPF           string `json:"spf"`
	DKIM          string `json:"dkim"`
	DMARC         string `json:"dmarc"`
	ARC           string `json:"arc"`
	ErrorDetail   string `json:"errorDetail"`
	IsNDR         int    `json:"isNDR"`
	IsSpam        int    `json:"isSpam"`
	SpamScore     int    `json:"spamScore"`
	SpamThreshold int    `json:"spamThreshold"`
}

type cfGQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type cfGQLResponse struct {
	Data struct {
		Viewer struct {
			Zones []struct {
				EmailRoutingAdaptive []CFEmailRecord `json:"emailRoutingAdaptive"`
			} `json:"zones"`
		} `json:"viewer"`
	} `json:"data"`
	Errors json.RawMessage `json:"errors"`
}

var cfHTTPClient = &http.Client{Timeout: 30 * time.Second}

func FetchEmailActivity(cfg Config, from, to time.Time) ([]CFEmailRecord, error) {
	payload := cfGQLRequest{
		Query: gqlQuery,
		Variables: map[string]interface{}{
			"zoneTag": cfg.CFZoneTag,
			"filter": map[string]string{
				"datetime_geq": from.UTC().Format(time.RFC3339),
				"datetime_leq": to.UTC().Format(time.RFC3339),
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, cfGraphQLEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.CFAPIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := cfHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %s", resp.Status)
	}
	var gqlResp cfGQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(gqlResp.Errors) > 0 && string(gqlResp.Errors) != "null" {
		return nil, fmt.Errorf("GraphQL errors: %s", gqlResp.Errors)
	}
	if len(gqlResp.Data.Viewer.Zones) == 0 {
		return nil, nil
	}
	return gqlResp.Data.Viewer.Zones[0].EmailRoutingAdaptive, nil
}

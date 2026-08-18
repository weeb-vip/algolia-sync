package catalogue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Entry is the minimum needed to decide whether a record belongs in the index.
type Entry struct {
	ID   string  `json:"id"`
	Slug *string `json:"slug"`
}

// Client reads the catalogue from the GraphQL gateway.
//
// algolia-sync has no database of its own, and giving it one would mean
// spreading database credentials to a service that only needs to know which
// anime exist. The gateway already answers that, and it is the same source the
// sitemap enumerates from.
type Client struct {
	Endpoint string
	HTTP     *http.Client
}

func New(endpoint string) *Client {
	return &Client{
		Endpoint: endpoint,
		// Generous: this asks for the entire catalogue in one request.
		HTTP: &http.Client{Timeout: 120 * time.Second},
	}
}

// Deliberately only id and slug. Requesting the full document for ~30,000 anime
// would be a multi-megabyte response, and reconcile only needs identity.
const allAnimeQuery = `query ReconcileCatalogue($limit: Int!) {
  newestAnime(limit: $limit) {
    id
    slug
  }
}`

// catalogueCeiling asks for more than the catalogue holds and takes what comes
// back; newestAnime has no pagination, only a limit.
const catalogueCeiling = 100000

// All returns every anime the source knows about.
func (c *Client) All(ctx context.Context) ([]Entry, error) {
	body, err := json.Marshal(map[string]any{
		"query":     allAnimeQuery,
		"variables": map[string]any{"limit": catalogueCeiling},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalogue query returned %d", resp.StatusCode)
	}

	var out struct {
		Data struct {
			NewestAnime []Entry `json:"newestAnime"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("catalogue query failed: %s", out.Errors[0].Message)
	}
	// An empty catalogue is treated as an error rather than "delete everything".
	// A gateway that answers 200 with no data must never be read as the
	// instruction to empty the search index.
	if len(out.Data.NewestAnime) == 0 {
		return nil, fmt.Errorf("catalogue query returned no anime; refusing to treat that as an empty catalogue")
	}
	return out.Data.NewestAnime, nil
}

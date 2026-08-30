package work_index

import (
	"encoding/json"
	"strings"
	"time"
)

// WorkDocument is a work as Algolia stores it.
//
// Shaped like AnimeDocument where the two overlap, so a frontend rendering both
// kinds of result reads the same field names for title, slug, image and
// description. Where they differ they differ honestly: a work has volumes,
// chapters, authors and a magazine; it has no episodes, studios or broadcast.
// That difference is the reason this is a separate index rather than a type
// column on the anime one.
type WorkDocument struct {
	ObjectID string `json:"objectID"`
	ID       string `json:"id"`
	// Without a slug a result cannot link to /manga/<slug>, and unlike anime
	// there is no id-based route to fall back on -- workBySlug is the only
	// lookup the schema exposes. A work with no slug is unreachable, so it is
	// worth being able to spot them in the index.
	Slug *string `json:"slug,omitempty"`

	TitleEn       *string  `json:"title_en,omitempty"`
	TitleJp       *string  `json:"title_jp,omitempty"`
	TitleSynonyms []string `json:"title_synonyms,omitempty"`

	// Type is MANGA, LIGHT_NOVEL, MANHWA and so on -- the facet that makes this
	// index browsable, since it is the one thing every work has.
	Type   string  `json:"type"`
	Status *string `json:"status,omitempty"`

	// Year is extracted so it can be a facet; a date string cannot be one.
	Year          *int    `json:"year,omitempty"`
	PublishedFrom *string `json:"published_from,omitempty"`
	PublishedTo   *string `json:"published_to,omitempty"`
	// DateRank is unix seconds, for sorting newest-first.
	DateRank *int64 `json:"date_rank,omitempty"`

	Volumes  *int `json:"volumes,omitempty"`
	Chapters *int `json:"chapters,omitempty"`

	Authors       []string `json:"authors,omitempty"`
	Serialization *string  `json:"serialization,omitempty"`
	Demographic   *string  `json:"demographic,omitempty"`

	Score   *float64 `json:"score,omitempty"`
	Ranking *int     `json:"ranking,omitempty"`
	// RankSort is what customRanking sorts on, and unlike Ranking it is always
	// present. An omitted attribute does not sort last in Algolia -- it scores
	// better than any real value -- so sorting on a nullable ranking puts every
	// unranked entry ahead of every ranked one. That bug is already documented
	// on the anime index, where it buried the actual One Piece at position 92;
	// this index would hit it far harder, since most of MyAnimeList's manga are
	// unranked.
	RankSort int `json:"rank_sort"`

	ImageURL *string `json:"image_url,omitempty"`
	// Stored for display, kept out of searchableAttributes: matching on synopsis
	// text makes every result look plausible and ranks them by coincidence.
	Description *string `json:"description,omitempty"`
}

// Beyond any real MyAnimeList position, so unranked entries sort last.
const unrankedSortValue = 9_999_999

// ToDocument maps a work row onto the search document.
func (s *Schema) ToDocument() WorkDocument {
	doc := WorkDocument{
		ObjectID:      s.Id,
		ID:            s.Id,
		Slug:          s.UrlSlug,
		TitleEn:       s.TitleEn,
		TitleJp:       s.TitleJp,
		TitleSynonyms: jsonList(s.TitleSynonyms),
		Type:          s.Type,
		Status:        s.Status,
		PublishedFrom: s.PublishedFrom,
		PublishedTo:   s.PublishedTo,
		Volumes:       s.Volumes,
		Chapters:      s.Chapters,
		Authors:       jsonList(s.Authors),
		Serialization: s.Serialization,
		Demographic:   s.Demographic,
		Score:         s.Score,
		Ranking:       s.Ranking,
		RankSort:      unrankedSortValue,
		ImageURL:      s.ImageUrl,
		Description:   s.Synopsis,
	}

	if s.Ranking != nil {
		doc.RankSort = *s.Ranking
	}

	if from := parseTimestamp(s.PublishedFrom); from != nil {
		year := from.Year()
		doc.Year = &year
		unix := from.Unix()
		doc.DateRank = &unix
	}

	return doc
}

// jsonList decodes the JSON-in-a-text-column the scraper writes for authors and
// title synonyms.
//
// Returns nil rather than an error on anything unparseable, matching how
// anime-api reads the same columns: one malformed row should cost that row its
// author list, not drop the record from search.
func jsonList(raw *string) []string {
	if raw == nil || *raw == "" {
		return nil
	}

	var out []string
	if err := json.Unmarshal([]byte(*raw), &out); err != nil {
		return nil
	}

	cleaned := make([]string, 0, len(out))
	for _, v := range out {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}

	return cleaned
}

// parseTimestamp reads the publication dates.
//
// These are timestamptz columns, which Debezium encodes as ISO 8601 strings
// rather than the epoch integers it uses for plain timestamps -- the same
// difference that made the work consumer fail on its first event. Both layouts
// are accepted anyway, because a value that does not parse should cost the
// record its year facet and nothing more.
func parseTimestamp(v *string) *time.Time {
	if v == nil || *v == "" {
		return nil
	}

	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, *v); err == nil {
			return &parsed
		}
	}

	return nil
}

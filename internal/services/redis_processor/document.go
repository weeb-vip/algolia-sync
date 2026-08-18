package redis_processor

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AnimeDocument is what actually gets indexed.
//
// Deliberately not the database row. The previous document was the CDC payload
// forwarded verbatim, which meant Algolia held microsecond timestamps nothing
// could sort on, scraper artifacts like licensors: ["None found", " add some"],
// and a rating stored as the string "6.01" so no numeric filter worked. It also
// did not hold the slug at all, so search results could only link to /show/<id>.
//
// Field names match what the frontend already reads (tags, episode_count,
// description, year, slug) rather than what postgres happens to call them.
type AnimeDocument struct {
	ObjectID string `json:"objectID"`
	ID       string `json:"id"`
	// Slug is the whole point of this rework: without it a search result cannot
	// link to /anime/<slug> and has to fall back to the id, which redirects.
	Slug *string `json:"slug,omitempty"`

	TitleEn       *string  `json:"title_en,omitempty"`
	TitleJp       *string  `json:"title_jp,omitempty"`
	TitleRomaji   *string  `json:"title_romaji,omitempty"`
	TitleSynonyms []string `json:"title_synonyms,omitempty"`

	Type   *string `json:"type,omitempty"`
	Status *string `json:"status,omitempty"`

	// Year is extracted so it can be a facet. A date string cannot be one.
	Year      *int    `json:"year,omitempty"`
	StartDate *string `json:"start_date,omitempty"`
	EndDate   *string `json:"end_date,omitempty"`
	// DateRank is real unix seconds, for sorting newest-first.
	DateRank *int64 `json:"date_rank,omitempty"`

	EpisodeCount    *int `json:"episode_count,omitempty"`
	DurationMinutes *int `json:"duration_minutes,omitempty"`

	Tags    []string `json:"tags,omitempty"`
	Studios []string `json:"studios,omitempty"`

	// Rating as a number, so numeric filters and sorts work on it.
	Rating  *float64 `json:"rating,omitempty"`
	Ranking *int     `json:"ranking,omitempty"`

	ImageURL *string `json:"image_url,omitempty"`
	// Stored for display, excluded from searchableAttributes. See ApplySettings.
	Description *string `json:"description,omitempty"`
}

// ToDocument maps a CDC row onto the search document.
func (s *Schema) ToDocument() AnimeDocument {
	doc := AnimeDocument{
		ObjectID:      s.Id,
		ID:            s.Id,
		Slug:          s.UrlSlug,
		TitleEn:       s.TitleEn,
		TitleJp:       s.TitleJp,
		TitleRomaji:   s.TitleRomaji,
		Type:          s.Type,
		Status:        s.Status,
		EpisodeCount:  s.Episodes,
		Ranking:       s.Ranking,
		ImageURL:      s.ImageUrl,
		Description:   s.Synopsis,
		TitleSynonyms: parseJSONStringArray(s.TitleSynonyms),
		Tags:          cleanList(parseJSONStringArray(s.Genres)),
		Studios:       cleanList(parseJSONStringArray(s.Studios)),
	}

	if t := parseTimestamp(s.StartDate); t != nil {
		unix := t.Unix()
		year := t.Year()
		doc.DateRank = &unix
		doc.Year = &year
		iso := t.Format("2006-01-02")
		doc.StartDate = &iso
	}
	if t := parseTimestamp(s.EndDate); t != nil {
		iso := t.Format("2006-01-02")
		doc.EndDate = &iso
	}

	if s.Rating != nil {
		if f, err := strconv.ParseFloat(strings.TrimSpace(*s.Rating), 64); err == nil {
			doc.Rating = &f
		}
	}
	if m := parseDurationMinutes(s.Duration); m != nil {
		doc.DurationMinutes = m
	}

	return doc
}

// Two timestamp formats reach us, and only one used to be handled.
//
// The old parser accepted "2006-01-02 15:04:05" only, so every row arriving as
// ISO 8601 silently lost its date_rank -- the code logged a warning and carried
// on, which is why the index ended up with a sorting field that was present on
// some records and absent on others.
var timestampLayouts = []string{
	"2006-01-02T15:04:05.000000Z",
	"2006-01-02T15:04:05Z",
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTimestamp(v *string) *time.Time {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	raw := strings.TrimSpace(*v)
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}

var durationRe = regexp.MustCompile(`(\d+)\s*(hr|hour|min)`)

// parseDurationMinutes turns "24 min. per ep." or "1 hr. 58 min." into minutes.
// Sorting or filtering on the raw string is meaningless -- "9 min" sorts after
// "10 min" lexically.
func parseDurationMinutes(v *string) *int {
	if v == nil {
		return nil
	}
	matches := durationRe.FindAllStringSubmatch(strings.ToLower(*v), -1)
	if len(matches) == 0 {
		return nil
	}
	total := 0
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if strings.HasPrefix(m[2], "h") {
			total += n * 60
		} else {
			total += n
		}
	}
	if total == 0 {
		return nil
	}
	return &total
}

// cleanList drops the scraper's placeholder entries and tidies whitespace.
// MyAnimeList renders "None found, add some" when a field is empty, and that
// was being scraped and indexed as if it were two real studio names.
func cleanList(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		lower := strings.ToLower(v)
		if v == "" || lower == "none found" || lower == "add some" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

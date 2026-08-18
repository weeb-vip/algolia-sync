package redis_processor

import "testing"

func str(s string) *string { return &s }
func integer(i int) *int   { return &i }

func TestToDocumentMapsTheFieldsSearchNeeds(t *testing.T) {
	s := Schema{
		Id:            "00057440-b6f2-438b-a296-e90ccabd00a0",
		UrlSlug:       str("platinum-end"),
		TitleEn:       str("Platinum End"),
		Type:          str("TV"),
		Status:        str("Finished Airing"),
		Episodes:      integer(24),
		Rating:        str("6.01"),
		Ranking:       integer(10921),
		Duration:      str("24 min. per ep."),
		StartDate:     str("2021-10-08T04:00:00.000000Z"),
		Genres:        str(`["Drama","Supernatural"]`),
		Studios:       str(`["Signal.MD"]`),
		TitleSynonyms: str(`["Platinum End"]`),
	}

	doc := s.ToDocument()

	if doc.Slug == nil || *doc.Slug != "platinum-end" {
		t.Fatalf("slug not carried through: %v", doc.Slug)
	}
	if doc.ObjectID != s.Id || doc.ID != s.Id {
		t.Errorf("objectID/id mismatch: %q %q", doc.ObjectID, doc.ID)
	}
	// Was a string, so no numeric filter or sort could touch it.
	if doc.Rating == nil || *doc.Rating != 6.01 {
		t.Errorf("rating should be numeric, got %v", doc.Rating)
	}
	if doc.Year == nil || *doc.Year != 2021 {
		t.Errorf("year should be extracted for faceting, got %v", doc.Year)
	}
	if doc.StartDate == nil || *doc.StartDate != "2021-10-08" {
		t.Errorf("start_date should be a plain date, got %v", doc.StartDate)
	}
	if doc.DurationMinutes == nil || *doc.DurationMinutes != 24 {
		t.Errorf("duration should be minutes, got %v", doc.DurationMinutes)
	}
	if len(doc.Tags) != 2 || doc.Tags[0] != "Drama" {
		t.Errorf("genres should map to tags: %v", doc.Tags)
	}
}

// The old parser handled "2006-01-02 15:04:05" only. Debezium emits ISO 8601,
// so those rows silently lost date_rank and sorted unpredictably.
func TestDateRankHandlesBothTimestampFormats(t *testing.T) {
	for _, raw := range []string{
		"2021-10-08T04:00:00.000000Z",
		"2021-10-08 04:00:00",
	} {
		doc := (&Schema{Id: "x", StartDate: str(raw)}).ToDocument()
		if doc.DateRank == nil {
			t.Fatalf("no date_rank for %q", raw)
		}
		if *doc.DateRank != 1633665600 {
			t.Errorf("%q -> %d, want real unix seconds 1633665600", raw, *doc.DateRank)
		}
	}
}

func TestUnparseableDateLeavesFieldsUnset(t *testing.T) {
	doc := (&Schema{Id: "x", StartDate: str("not a date")}).ToDocument()
	if doc.DateRank != nil || doc.Year != nil || doc.StartDate != nil {
		t.Errorf("a bad date should yield no date fields, got %v %v %v",
			doc.DateRank, doc.Year, doc.StartDate)
	}
}

// MyAnimeList renders "None found, add some" for empty fields, and it was being
// scraped and indexed as two studio names.
func TestScraperPlaceholdersAreDropped(t *testing.T) {
	doc := (&Schema{Id: "x", Studios: str(`["None found"," add some"]`)}).ToDocument()
	if doc.Studios != nil {
		t.Errorf("placeholder studios should be dropped, got %v", doc.Studios)
	}

	doc = (&Schema{Id: "x", Studios: str(`["None found","Bones"]`)}).ToDocument()
	if len(doc.Studios) != 1 || doc.Studios[0] != "Bones" {
		t.Errorf("real studios must survive alongside placeholders: %v", doc.Studios)
	}
}

func TestDurationParsing(t *testing.T) {
	cases := map[string]*int{
		"24 min. per ep.": integer(24),
		"1 hr. 58 min.":   integer(118),
		"2 hr.":           integer(120),
		"Unknown":         nil,
	}
	for in, want := range cases {
		got := (&Schema{Id: "x", Duration: str(in)}).ToDocument().DurationMinutes
		switch {
		case want == nil && got != nil:
			t.Errorf("%q -> %d, want nil", in, *got)
		case want != nil && (got == nil || *got != *want):
			t.Errorf("%q -> %v, want %d", in, got, *want)
		}
	}
}

// A row that predates the url_slug column must still index, just without a slug.
func TestMissingSlugStillProducesADocument(t *testing.T) {
	doc := (&Schema{Id: "abc", TitleEn: str("Some Anime")}).ToDocument()
	if doc.ObjectID != "abc" {
		t.Errorf("objectID should still be set, got %q", doc.ObjectID)
	}
	if doc.Slug != nil {
		t.Errorf("slug should be absent, got %v", doc.Slug)
	}
}

package work_index

import "testing"

func str(s string) *string   { return &s }
func integer(i int) *int     { return &i }
func flt(f float64) *float64 { return &f }

// Hellsing, as the read store actually holds it -- the list columns are JSON in
// a text column and the dates are the ISO strings Debezium writes for
// timestamptz, not the epoch integers it writes for plain timestamp.
func hellsing() Schema {
	return Schema{
		Id:            "39802931-6519-4de5-a2e3-4ebab1e50f40",
		MalId:         integer(267),
		Type:          "MANGA",
		UrlSlug:       str("hellsing"),
		TitleEn:       str("Hellsing"),
		TitleJp:       str("ヘルシング"),
		TitleSynonyms: str(`["Hellsing Cross Fire","Crossfire"]`),
		Synopsis:      str("For centuries, many secret organizations..."),
		ImageUrl:      str("https://cdn.myanimelist.net/images/manga/3/267321.jpg"),
		Status:        str("Finished"),
		Volumes:       integer(10),
		Chapters:      integer(92),
		PublishedFrom: str("1997-04-30T00:00:00Z"),
		PublishedTo:   str("2008-09-30T00:00:00Z"),
		Demographic:   str("Seinen"),
		Serialization: str("Young King OURs"),
		Authors:       str(`["Hirano, Kouta"]`),
		Score:         flt(8.29),
		Ranking:       integer(366),
	}
}

func TestToDocumentMapsTheFieldsSearchNeeds(t *testing.T) {
	w := hellsing()
	doc := w.ToDocument()

	if doc.ObjectID != doc.ID || doc.ObjectID == "" {
		t.Fatalf("objectID must be the id: %q vs %q", doc.ObjectID, doc.ID)
	}
	// Without a slug a result cannot link anywhere: workBySlug is the only
	// lookup the schema exposes, so there is no id fallback as there is for
	// anime.
	if doc.Slug == nil || *doc.Slug != "hellsing" {
		t.Fatalf("slug not carried through: %v", doc.Slug)
	}
	if doc.Type != "MANGA" {
		t.Fatalf("type is the one facet every work has, got %q", doc.Type)
	}
	if doc.Volumes == nil || *doc.Volumes != 10 {
		t.Fatalf("volumes not carried: %v", doc.Volumes)
	}
	if doc.Serialization == nil || *doc.Serialization != "Young King OURs" {
		t.Fatalf("serialization not carried: %v", doc.Serialization)
	}
}

func TestAuthorsAndSynonymsAreDecodedFromJSON(t *testing.T) {
	w := hellsing()
	doc := w.ToDocument()

	// These arrive as JSON inside a text column. Left as a raw string they
	// would be one unsearchable blob, and authors is the field people look a
	// manga up by.
	if len(doc.Authors) != 1 || doc.Authors[0] != "Hirano, Kouta" {
		t.Fatalf("authors not decoded: %#v", doc.Authors)
	}
	if len(doc.TitleSynonyms) != 2 || doc.TitleSynonyms[1] != "Crossfire" {
		t.Fatalf("synonyms not decoded: %#v", doc.TitleSynonyms)
	}
}

func TestMalformedListsCostTheirFieldNotTheRecord(t *testing.T) {
	s := hellsing()
	s.Authors = str("Hirano, Kouta") // not JSON

	doc := s.ToDocument()

	if doc.Authors != nil {
		t.Fatalf("unparseable list should be dropped, got %#v", doc.Authors)
	}
	// The record still indexes -- one bad column must not remove a work from
	// search entirely.
	if doc.ObjectID == "" || doc.TitleEn == nil {
		t.Fatal("record should survive a malformed list")
	}
}

func TestYearAndDateRankComeFromTheISOTimestamp(t *testing.T) {
	w := hellsing()
	doc := w.ToDocument()

	// timestamptz reaches us as an ISO string, not epoch millis. Parsing it as
	// the latter is what broke the work consumer on its very first event.
	if doc.Year == nil || *doc.Year != 1997 {
		t.Fatalf("year should be extracted for faceting, got %v", doc.Year)
	}
	if doc.DateRank == nil || *doc.DateRank <= 0 {
		t.Fatalf("date_rank should be unix seconds, got %v", doc.DateRank)
	}
}

func TestUnrankedWorksSortLast(t *testing.T) {
	rankedWork := hellsing()
	ranked := rankedWork.ToDocument()
	if ranked.RankSort != 366 {
		t.Fatalf("a ranked work should sort on its ranking, got %d", ranked.RankSort)
	}

	s := hellsing()
	s.Ranking = nil
	unranked := s.ToDocument()

	// An omitted attribute scores better than any real value in Algolia, so a
	// nullable ranking would put every unranked work ahead of every ranked one.
	// Most of MyAnimeList's manga are unranked, so that would bury the entire
	// recognisable catalogue.
	if unranked.RankSort != unrankedSortValue {
		t.Fatalf("unranked work must carry the sentinel, got %d", unranked.RankSort)
	}
	if unranked.RankSort <= ranked.RankSort {
		t.Fatal("unranked work must sort after a ranked one")
	}
	// Still nullable for display.
	if unranked.Ranking != nil {
		t.Fatal("ranking should stay absent for display")
	}
}

func TestSynopsisIsStoredForDisplay(t *testing.T) {
	w := hellsing()
	doc := w.ToDocument()

	// Kept out of searchableAttributes by ApplyWorkSettings, but it still has
	// to reach the index or results have no text to show.
	if doc.Description == nil || *doc.Description == "" {
		t.Fatal("synopsis should be carried as description")
	}
}

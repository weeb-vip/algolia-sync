package algolia_processor

import "encoding/json"

type Action = string

const (
	CreateAction Action = "create"
	UpdateAction Action = "update"
	DeleteAction Action = "delete"
)

// Schema is the input schema from the source (with JSON string fields)
type Schema struct {
	Id            string  `json:"id"`
	AnidbID       *string `json:"anidbid"`
	TitleEn       *string `json:"title_en"`
	TitleJp       *string `json:"title_jp"`
	TitleRomaji   *string `json:"title_romaji"`
	TitleKanji    *string `json:"title_kanji"`
	Type          *string `json:"type"`
	ImageUrl      *string `json:"image_url"`
	Synopsis      *string `json:"synopsis"`
	Episodes      *int    `json:"episodes"`
	Status        *string `json:"status"`
	Duration      *string `json:"duration"`
	Broadcast     *string `json:"broadcast"`
	Source        *string `json:"source"`
	CreatedAt     *int64  `json:"created_at"`
	UpdatedAt     *int64  `json:"updated_at"`
	Rating        *string `json:"rating"`
	StartDate     *string `json:"start_date"`
	EndDate       *string `json:"end_date"`
	TitleSynonyms *string `json:"title_synonyms"`
	Genres        *string `json:"genres"`
	Licensors     *string `json:"licensors"`
	Studios       *string `json:"studios"`
	Ranking       *int    `json:"ranking"`
	ObjectId      *string `json:"objectID"`
	DateRank      *int64  `json:"date_rank"`
}

// AlgoliaSchema is the output schema for Algolia (with array fields for faceting)
type AlgoliaSchema struct {
	Id            string   `json:"id"`
	AnidbID       *string  `json:"anidbid,omitempty"`
	TitleEn       *string  `json:"title_en,omitempty"`
	TitleJp       *string  `json:"title_jp,omitempty"`
	TitleRomaji   *string  `json:"title_romaji,omitempty"`
	TitleKanji    *string  `json:"title_kanji,omitempty"`
	Type          *string  `json:"type,omitempty"`
	ImageUrl      *string  `json:"image_url,omitempty"`
	Synopsis      *string  `json:"synopsis,omitempty"`
	Episodes      *int     `json:"episodes,omitempty"`
	Status        *string  `json:"status,omitempty"`
	Duration      *string  `json:"duration,omitempty"`
	Broadcast     *string  `json:"broadcast,omitempty"`
	Source        *string  `json:"source,omitempty"`
	CreatedAt     *int64   `json:"created_at,omitempty"`
	UpdatedAt     *int64   `json:"updated_at,omitempty"`
	Rating        *string  `json:"rating,omitempty"`
	StartDate     *string  `json:"start_date,omitempty"`
	EndDate       *string  `json:"end_date,omitempty"`
	TitleSynonyms []string `json:"title_synonyms,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Licensors     []string `json:"licensors,omitempty"`
	Studios       []string `json:"studios,omitempty"`
	Ranking       *int     `json:"ranking,omitempty"`
	ObjectId      string   `json:"objectID"`
	DateRank      *int64   `json:"date_rank,omitempty"`
}

type Payload struct {
	Action Action `json:"action"`
	Data   Schema `json:"data"`
}

// ToAlgoliaSchema converts Schema (with JSON strings) to AlgoliaSchema (with arrays)
func (s *Schema) ToAlgoliaSchema() AlgoliaSchema {
	result := AlgoliaSchema{
		Id:          s.Id,
		AnidbID:     s.AnidbID,
		TitleEn:     s.TitleEn,
		TitleJp:     s.TitleJp,
		TitleRomaji: s.TitleRomaji,
		TitleKanji:  s.TitleKanji,
		Type:        s.Type,
		ImageUrl:    s.ImageUrl,
		Synopsis:    s.Synopsis,
		Episodes:    s.Episodes,
		Status:      s.Status,
		Duration:    s.Duration,
		Broadcast:   s.Broadcast,
		Source:      s.Source,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		Rating:      s.Rating,
		StartDate:   s.StartDate,
		EndDate:     s.EndDate,
		Ranking:     s.Ranking,
		DateRank:    s.DateRank,
	}

	if s.ObjectId != nil {
		result.ObjectId = *s.ObjectId
	} else {
		result.ObjectId = s.Id
	}

	// Parse JSON string fields into arrays
	result.Genres = parseJSONStringArray(s.Genres)
	result.Studios = parseJSONStringArray(s.Studios)
	result.Licensors = parseJSONStringArray(s.Licensors)
	result.TitleSynonyms = parseJSONStringArray(s.TitleSynonyms)

	return result
}

// parseJSONStringArray parses a JSON string containing an array into []string
func parseJSONStringArray(jsonStr *string) []string {
	if jsonStr == nil || *jsonStr == "" {
		return nil
	}

	var result []string
	if err := json.Unmarshal([]byte(*jsonStr), &result); err != nil {
		return nil
	}

	return result
}

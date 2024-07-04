package algolia_processor

type Action = string

const (
	CreateAction Action = "create"
	UpdateAction Action = "update"
	DeleteAction Action = "delete"
)

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
}

type Payload struct {
	Action Action `json:"action"`
	Data   Schema `json:"data"`
}

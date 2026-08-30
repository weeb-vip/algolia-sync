// Package work_index holds everything the works search index needs: the record
// as it arrives from anime-sync, the queued form it takes in Redis, and the
// document that reaches Algolia.
//
// One package rather than the two the anime path uses. There, redis_processor
// and redis_event_processor each declare their own copy of the anime schema and
// the two have to be kept identical by hand -- a field added to one and missed
// on the other is dropped silently in transit, which is how url_slug went
// missing and left search results unable to link anywhere. Works get one
// declaration that both halves share.
package work_index

type Action = string

const (
	CreateAction Action = "create"
	UpdateAction Action = "update"
	DeleteAction Action = "delete"
)

// Schema is a work row as anime-sync publishes it, which is the read store's
// columns unchanged. The list-valued columns arrive as JSON held in text,
// exactly as the scraper writes them.
type Schema struct {
	Id            string   `json:"id"`
	MalId         *int     `json:"mal_id"`
	Type          string   `json:"type"`
	UrlSlug       *string  `json:"url_slug"`
	TitleEn       *string  `json:"title_en"`
	TitleJp       *string  `json:"title_jp"`
	TitleSynonyms *string  `json:"title_synonyms"`
	Synopsis      *string  `json:"synopsis"`
	ImageUrl      *string  `json:"image_url"`
	Status        *string  `json:"status"`
	Volumes       *int     `json:"volumes"`
	Chapters      *int     `json:"chapters"`
	PublishedFrom *string  `json:"published_from"`
	PublishedTo   *string  `json:"published_to"`
	Demographic   *string  `json:"demographic"`
	Serialization *string  `json:"serialization"`
	Authors       *string  `json:"authors"`
	Score         *float64 `json:"score"`
	Ranking       *int     `json:"ranking"`
	Members       *int     `json:"members"`
	Favorites     *int     `json:"favorites"`
	ObjectId      *string  `json:"objectID"`
}

type Payload struct {
	Action Action `json:"action"`
	Data   Schema `json:"data"`
}

// QueuedItem is one pending index operation as it sits in Redis, between the
// consumer that received the event and the job that ships batches to Algolia.
type QueuedItem struct {
	Action    Action `json:"action"`
	Data      Schema `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

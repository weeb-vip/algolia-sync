package redis_processor_kafka

import (
	"context"
	"testing"

	"github.com/ThatCatDev/ep/v2/event"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/weeb-vip/algolia-sync/internal/logger"
)

// MockRedisService implements redis.RedisService for testing
type MockRedisService struct {
	StoredItems []QueuedItem
	StoreErr    error
}

func (m *MockRedisService) StoreData(ctx context.Context, data QueuedItem) error {
	if m.StoreErr != nil {
		return m.StoreErr
	}
	m.StoredItems = append(m.StoredItems, data)
	return nil
}

func (m *MockRedisService) GetAllData(ctx context.Context) ([]QueuedItem, error) {
	return m.StoredItems, nil
}

func (m *MockRedisService) ClearData(ctx context.Context) error {
	m.StoredItems = nil
	return nil
}

func setupTestContext() context.Context {
	log := logger.Get()
	return logger.WithCtx(context.Background(), log)
}

func stringPtr(s string) *string {
	return &s
}

func TestProcess_ValidStartDate(t *testing.T) {
	mockRedis := &MockRedisService{}
	processor := NewRedisProcessor(mockRedis)
	ctx := setupTestContext()

	payload := Payload{
		Action: CreateAction,
		Data: Schema{
			Id:        "test-id",
			TitleEn:   stringPtr("Test Anime"),
			StartDate: stringPtr("2007-04-02 04:00:00"),
		},
	}

	evt := event.Event[*kafka.Message, Payload]{
		Payload: payload,
	}

	result, err := processor.Process(ctx, evt)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(mockRedis.StoredItems) != 1 {
		t.Fatalf("Expected 1 stored item, got %d", len(mockRedis.StoredItems))
	}

	storedItem := mockRedis.StoredItems[0]

	// date_rank is deliberately NOT computed here any more. This test used to
	// assert the value 1175486, which is unix seconds divided by 1000 -- not a
	// timestamp in any unit, just the bug written down. Worse, the parser
	// accepted a single layout, so the ISO 8601 timestamps Debezium actually
	// emits produced no date_rank at all.
	//
	// The queue now carries the raw start_date and the sync job derives
	// date_rank once, correctly; see document_test.go in redis_processor.
	if storedItem.Data.DateRank != nil {
		t.Errorf("processor should not compute DateRank, got %d", *storedItem.Data.DateRank)
	}
	if storedItem.Data.StartDate == nil {
		t.Fatal("start_date must be preserved for the sync job to parse")
	}

	// ObjectID is set regardless of action, which is what stops a delete from
	// panicking on a nil dereference.
	if storedItem.Data.ObjectId == nil || *storedItem.Data.ObjectId != storedItem.Data.Id {
		t.Errorf("ObjectId should mirror Id, got %v", storedItem.Data.ObjectId)
	}

	_ = result
}

func TestProcess_InvalidStartDate_ContinuesWithoutDateRank(t *testing.T) {
	mockRedis := &MockRedisService{}
	processor := NewRedisProcessor(mockRedis)
	ctx := setupTestContext()

	payload := Payload{
		Action: CreateAction,
		Data: Schema{
			Id:        "test-id",
			TitleEn:   stringPtr("Test Anime"),
			StartDate: stringPtr("invalid-date-format"),
		},
	}

	evt := event.Event[*kafka.Message, Payload]{
		Payload: payload,
	}

	result, err := processor.Process(ctx, evt)
	if err != nil {
		t.Fatalf("Expected no error even with invalid date, got: %v", err)
	}

	if len(mockRedis.StoredItems) != 1 {
		t.Fatalf("Expected 1 stored item, got %d", len(mockRedis.StoredItems))
	}

	storedItem := mockRedis.StoredItems[0]
	if storedItem.Data.DateRank != nil {
		t.Errorf("Expected DateRank to be nil for invalid date, got %d", *storedItem.Data.DateRank)
	}

	// Verify the item was still stored
	if storedItem.Data.Id != "test-id" {
		t.Errorf("Expected Id 'test-id', got '%s'", storedItem.Data.Id)
	}

	_ = result
}

func TestProcess_NoStartDate(t *testing.T) {
	mockRedis := &MockRedisService{}
	processor := NewRedisProcessor(mockRedis)
	ctx := setupTestContext()

	payload := Payload{
		Action: CreateAction,
		Data: Schema{
			Id:        "test-id",
			TitleEn:   stringPtr("Test Anime"),
			StartDate: nil,
		},
	}

	evt := event.Event[*kafka.Message, Payload]{
		Payload: payload,
	}

	_, err := processor.Process(ctx, evt)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(mockRedis.StoredItems) != 1 {
		t.Fatalf("Expected 1 stored item, got %d", len(mockRedis.StoredItems))
	}

	storedItem := mockRedis.StoredItems[0]
	if storedItem.Data.DateRank != nil {
		t.Errorf("Expected DateRank to be nil when StartDate is nil, got %d", *storedItem.Data.DateRank)
	}
}

func TestProcess_WrongDateFormat_ContinuesProcessing(t *testing.T) {
	mockRedis := &MockRedisService{}
	processor := NewRedisProcessor(mockRedis)
	ctx := setupTestContext()

	// Test various invalid date formats
	invalidDates := []string{
		"2007/04/02",           // wrong separator
		"04-02-2007",           // wrong order
		"2007-04-02",           // missing time
		"2007-04-02T04:00:00",  // ISO format with T
		"April 2, 2007",        // text format
		"",                     // empty string
	}

	for _, invalidDate := range invalidDates {
		mockRedis.StoredItems = nil // Reset

		payload := Payload{
			Action: CreateAction,
			Data: Schema{
				Id:        "test-id",
				TitleEn:   stringPtr("Test Anime"),
				StartDate: stringPtr(invalidDate),
			},
		}

		evt := event.Event[*kafka.Message, Payload]{
			Payload: payload,
		}

		_, err := processor.Process(ctx, evt)
		if err != nil {
			t.Errorf("Expected no error for date '%s', got: %v", invalidDate, err)
		}

		if len(mockRedis.StoredItems) != 1 {
			t.Errorf("Expected 1 stored item for date '%s', got %d", invalidDate, len(mockRedis.StoredItems))
		}

		storedItem := mockRedis.StoredItems[0]
		if storedItem.Data.DateRank != nil {
			t.Errorf("Expected DateRank to be nil for invalid date '%s', got %d", invalidDate, *storedItem.Data.DateRank)
		}
	}
}

func TestProcess_UpdateAction_SkipsDateProcessing(t *testing.T) {
	mockRedis := &MockRedisService{}
	processor := NewRedisProcessor(mockRedis)
	ctx := setupTestContext()

	objectId := "existing-object-id"
	payload := Payload{
		Action: UpdateAction,
		Data: Schema{
			Id:        "test-id",
			ObjectId:  &objectId,
			TitleEn:   stringPtr("Test Anime"),
			StartDate: stringPtr("2007-04-02 04:00:00"),
		},
	}

	evt := event.Event[*kafka.Message, Payload]{
		Payload: payload,
	}

	_, err := processor.Process(ctx, evt)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(mockRedis.StoredItems) != 1 {
		t.Fatalf("Expected 1 stored item, got %d", len(mockRedis.StoredItems))
	}

	storedItem := mockRedis.StoredItems[0]
	// For update action, date processing is skipped, so DateRank should be nil
	if storedItem.Data.DateRank != nil {
		t.Errorf("Expected DateRank to be nil for update action, got %d", *storedItem.Data.DateRank)
	}
}

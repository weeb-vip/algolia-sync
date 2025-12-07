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
	if storedItem.Data.DateRank == nil {
		t.Fatal("Expected DateRank to be set")
	}

	// Verify DateRank was calculated (2007-04-02 04:00:00 UTC = 1175486400 / 1000 = 1175486)
	expectedDateRank := int64(1175486)
	if *storedItem.Data.DateRank != expectedDateRank {
		t.Errorf("Expected DateRank %d, got %d", expectedDateRank, *storedItem.Data.DateRank)
	}

	// Note: result.Payload.Data.DateRank may be nil since payload is modified locally
	// The important thing is that the stored item has the correct DateRank
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

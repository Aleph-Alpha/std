package qdrant

import (
	"testing"
	"time"
)

func TestBuildFilter_NilFilterSet(t *testing.T) {
	result := buildFilter(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestBuildFilter_EmptyFilterSet(t *testing.T) {
	filters := &FilterSet{}
	result := buildFilter(filters)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestBuildFilter_EmptyConditionSet(t *testing.T) {
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{},
		},
	}
	result := buildFilter(filters)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestBuildFilter_MustWithTextCondition(t *testing.T) {
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "city", Value: "London"},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
	if len(result.Should) != 0 {
		t.Errorf("expected 0 Should conditions, got %d", len(result.Should))
	}
	if len(result.MustNot) != 0 {
		t.Errorf("expected 0 MustNot conditions, got %d", len(result.MustNot))
	}
}

func TestBuildFilter_ShouldWithMultipleTextConditions(t *testing.T) {
	// city = "London" OR city = "Berlin"
	filters := &FilterSet{
		Should: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "city", Value: "London"},
				TextCondition{Key: "city", Value: "Berlin"},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Should) != 2 {
		t.Errorf("expected 2 Should conditions, got %d", len(result.Should))
	}
}

func TestBuildFilter_MustNotWithBoolCondition(t *testing.T) {
	filters := &FilterSet{
		MustNot: &ConditionSet{
			Conditions: []FilterCondition{
				BoolCondition{Key: "archived", Value: true},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.MustNot) != 1 {
		t.Errorf("expected 1 MustNot condition, got %d", len(result.MustNot))
	}
}

func TestBuildFilter_MixedConditionTypes(t *testing.T) {
	// city = "London" AND active = true AND priority = 1
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "city", Value: "London"},
				BoolCondition{Key: "active", Value: true},
				IntCondition{Key: "priority", Value: 1},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 3 {
		t.Errorf("expected 3 Must conditions, got %d", len(result.Must))
	}
}

func TestBuildFilter_CombinedClauses(t *testing.T) {
	// (city = "London" AND active = true) AND NOT archived
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "city", Value: "London"},
				BoolCondition{Key: "active", Value: true},
			},
		},
		MustNot: &ConditionSet{
			Conditions: []FilterCondition{
				BoolCondition{Key: "archived", Value: true},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 2 {
		t.Errorf("expected 2 Must conditions, got %d", len(result.Must))
	}
	if len(result.MustNot) != 1 {
		t.Errorf("expected 1 MustNot condition, got %d", len(result.MustNot))
	}
}

func TestBuildFilter_AllThreeClauses(t *testing.T) {
	// Must AND Should AND MustNot
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "status", Value: "active"},
			},
		},
		Should: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "city", Value: "London"},
				TextCondition{Key: "city", Value: "Berlin"},
			},
		},
		MustNot: &ConditionSet{
			Conditions: []FilterCondition{
				BoolCondition{Key: "deleted", Value: true},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
	if len(result.Should) != 2 {
		t.Errorf("expected 2 Should conditions, got %d", len(result.Should))
	}
	if len(result.MustNot) != 1 {
		t.Errorf("expected 1 MustNot condition, got %d", len(result.MustNot))
	}
}

func TestBuildFilter_TimeRangeCondition(t *testing.T) {
	startTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TimeRangeCondition{
					Key: "created_at",
					Value: TimeRange{
						Gte: &startTime,
						Lt:  &endTime,
					},
				},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
}

func TestBuildFilter_TimeRangeAllBounds(t *testing.T) {
	gt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	gte := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	lt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lte := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TimeRangeCondition{
					Key: "updated_at",
					Value: TimeRange{
						Gt:  &gt,
						Gte: &gte,
						Lt:  &lt,
						Lte: &lte,
					},
				},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
}

func TestBuildFilter_EmptyTimeRange(t *testing.T) {
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TimeRangeCondition{
					Key:   "created_at",
					Value: TimeRange{}, // All nil
				},
			},
		},
	}
	result := buildFilter(filters)

	// Empty TimeRange returns nil condition, so filter should be nil
	if result != nil {
		t.Errorf("expected nil for empty time range, got %v", result)
	}
}

func TestBuildConditions_NilConditionSet(t *testing.T) {
	result := buildConditions(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTextCondition_ToQdrantCondition(t *testing.T) {
	c := TextCondition{Key: "city", Value: "London"}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBoolCondition_ToQdrantCondition(t *testing.T) {
	c := BoolCondition{Key: "active", Value: true}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestIntCondition_ToQdrantCondition(t *testing.T) {
	c := IntCondition{Key: "priority", Value: 42}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestTimeRangeCondition_ToQdrantCondition(t *testing.T) {
	now := time.Now()
	c := TimeRangeCondition{
		Key:   "created_at",
		Value: TimeRange{Gte: &now},
	}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestTimeRangeCondition_EmptyRange(t *testing.T) {
	c := TimeRangeCondition{
		Key:   "created_at",
		Value: TimeRange{}, // All nil
	}
	result := c.ToQdrantCondition()

	if result != nil {
		t.Errorf("expected nil for empty time range, got %v", result)
	}
}

func TestToTimestamp_Nil(t *testing.T) {
	result := toTimestamp(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToTimestamp_ValidTime(t *testing.T) {
	now := time.Now()
	result := toTimestamp(&now)

	if result == nil {
		t.Fatal("expected timestamp, got nil")
	}
	if result.AsTime().Unix() != now.Unix() {
		t.Errorf("timestamp mismatch: expected %v, got %v", now.Unix(), result.AsTime().Unix())
	}
}

// === FieldType Tests ===

func TestResolveFieldKey_InternalField(t *testing.T) {
	key := resolveFieldKey("search_store_id", InternalField)
	expected := "search_store_id"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestResolveFieldKey_UserField(t *testing.T) {
	key := resolveFieldKey("document_id", UserField)
	expected := "custom.document_id"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestResolveFieldKey_UserField_PreventDoublePrefix(t *testing.T) {
	// If key already has prefix, don't add again
	key := resolveFieldKey("custom.document_id", UserField)
	expected := "custom.document_id"
	if key != expected {
		t.Errorf("expected %q, got %q (double prefix detected)", expected, key)
	}
}

func TestTextCondition_InternalField(t *testing.T) {
	c := TextCondition{Key: "search_store_id", Value: "store-123", FieldType: InternalField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
	// Internal field should NOT have prefix
}

func TestTextCondition_UserField(t *testing.T) {
	c := TextCondition{Key: "document_id", Value: "doc-456", FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
	// User field should have "custom." prefix
}

func TestBoolCondition_UserField(t *testing.T) {
	c := BoolCondition{Key: "is_reviewed", Value: true, FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestIntCondition_UserField(t *testing.T) {
	c := IntCondition{Key: "version", Value: 2, FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestTimeRangeCondition_UserField(t *testing.T) {
	now := time.Now()
	c := TimeRangeCondition{
		Key:       "uploaded_at",
		Value:     TimeRange{Gte: &now},
		FieldType: UserField,
	}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBuildFilter_MixedInternalAndUserFields(t *testing.T) {
	// search_store_id = "store-123" (internal) AND custom.category = "reports" (user)
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "search_store_id", Value: "store-123", FieldType: InternalField},
				TextCondition{Key: "category", Value: "reports", FieldType: UserField},
				BoolCondition{Key: "is_published", Value: true, FieldType: UserField},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 3 {
		t.Errorf("expected 3 Must conditions, got %d", len(result.Must))
	}
}

// === BuildPayload Tests ===

func TestBuildPayload_OnlyInternal(t *testing.T) {
	internal := map[string]any{
		"search_store_id": "store-123",
		"modalities":      []string{"text"},
	}
	payload := BuildPayload(internal, nil)

	if payload["search_store_id"] != "store-123" {
		t.Errorf("expected search_store_id at top-level")
	}
	if _, exists := payload["custom"]; exists {
		t.Errorf("custom should not exist when user is nil")
	}
}

func TestBuildPayload_OnlyUser(t *testing.T) {
	user := map[string]any{
		"document_id": "doc-456",
		"author":      "John",
	}
	payload := BuildPayload(nil, user)

	custom, ok := payload["custom"].(map[string]any)
	if !ok {
		t.Fatal("expected custom field")
	}
	if custom["document_id"] != "doc-456" {
		t.Errorf("expected document_id in custom")
	}
	if custom["author"] != "John" {
		t.Errorf("expected author in custom")
	}
}

func TestBuildPayload_BothInternalAndUser(t *testing.T) {
	internal := map[string]any{
		"search_store_id": "store-123",
	}
	user := map[string]any{
		"document_id": "doc-456",
		"category":    "reports",
	}
	payload := BuildPayload(internal, user)

	// Check internal at top-level
	if payload["search_store_id"] != "store-123" {
		t.Errorf("expected search_store_id at top-level")
	}

	// Check user under custom
	custom, ok := payload["custom"].(map[string]any)
	if !ok {
		t.Fatal("expected custom field")
	}
	if custom["document_id"] != "doc-456" {
		t.Errorf("expected document_id in custom")
	}
	if custom["category"] != "reports" {
		t.Errorf("expected category in custom")
	}
}

func TestBuildPayload_EmptyUser(t *testing.T) {
	internal := map[string]any{
		"search_store_id": "store-123",
	}
	user := map[string]any{} // Empty, not nil
	payload := BuildPayload(internal, user)

	if _, exists := payload["custom"]; exists {
		t.Errorf("custom should not exist when user is empty")
	}
}

func TestResolveFieldKey_ActualPath(t *testing.T) {
	tests := []struct {
		key       string
		fieldType FieldType
		expected  string
	}{
		{"city", InternalField, "city"},
		{"city", UserField, "custom.city"},
		{"custom.city", UserField, "custom.city"}, // No double prefix
	}

	for _, tt := range tests {
		result := resolveFieldKey(tt.key, tt.fieldType)
		if result != tt.expected {
			t.Errorf("resolveFieldKey(%q, %v) = %q, want %q",
				tt.key, tt.fieldType, result, tt.expected)
		}
	}
}

// === MatchAnyCondition Tests ===

func TestTextAnyCondition_ToQdrantCondition(t *testing.T) {
	c := TextAnyCondition{Key: "city", Values: []string{"London", "Berlin", "Paris"}}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestIntAnyCondition_ToQdrantCondition(t *testing.T) {
	c := IntAnyCondition{Key: "priority", Values: []int64{1, 2, 3}}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestTextAnyCondition_UserField(t *testing.T) {
	c := TextAnyCondition{Key: "category", Values: []string{"tech", "science"}, FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBuildFilter_WithTextAnyCondition(t *testing.T) {
	// city IN ("London", "Berlin")
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextAnyCondition{Key: "city", Values: []string{"London", "Berlin"}},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
}

// === MatchExceptCondition Tests ===

func TestTextExceptCondition_ToQdrantCondition(t *testing.T) {
	c := TextExceptCondition{Key: "city", Values: []string{"Paris", "Madrid"}}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestIntExceptCondition_ToQdrantCondition(t *testing.T) {
	c := IntExceptCondition{Key: "priority", Values: []int64{0, -1}}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestTextExceptCondition_UserField(t *testing.T) {
	c := TextExceptCondition{Key: "status", Values: []string{"draft", "deleted"}, FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBuildFilter_WithTextExceptCondition(t *testing.T) {
	// city NOT IN ("Paris", "Madrid")
	filters := &FilterSet{
		MustNot: &ConditionSet{
			Conditions: []FilterCondition{
				TextExceptCondition{Key: "city", Values: []string{"Paris", "Madrid"}},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.MustNot) != 1 {
		t.Errorf("expected 1 MustNot condition, got %d", len(result.MustNot))
	}
}

// === NumericRangeCondition Tests ===

func TestNumericRangeCondition_ToQdrantCondition(t *testing.T) {
	minPrice := 100.0
	maxPrice := 500.0
	c := NumericRangeCondition{
		Key: "price",
		Value: NumericRange{
			Gte: &minPrice,
			Lte: &maxPrice,
		},
	}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestNumericRangeCondition_AllBounds(t *testing.T) {
	gt := 10.0
	gte := 20.0
	lt := 100.0
	lte := 90.0
	c := NumericRangeCondition{
		Key: "score",
		Value: NumericRange{
			Gt:  &gt,
			Gte: &gte,
			Lt:  &lt,
			Lte: &lte,
		},
	}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestNumericRangeCondition_EmptyRange(t *testing.T) {
	c := NumericRangeCondition{
		Key:   "price",
		Value: NumericRange{}, // All nil
	}
	result := c.ToQdrantCondition()

	if result != nil {
		t.Errorf("expected nil for empty numeric range, got %v", result)
	}
}

func TestNumericRangeCondition_UserField(t *testing.T) {
	minPrice := 50.0
	c := NumericRangeCondition{
		Key:       "price",
		Value:     NumericRange{Gte: &minPrice},
		FieldType: UserField,
	}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBuildFilter_WithNumericRangeCondition(t *testing.T) {
	minPrice := 100.0
	maxPrice := 500.0
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				NumericRangeCondition{
					Key: "price",
					Value: NumericRange{
						Gte: &minPrice,
						Lte: &maxPrice,
					},
				},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
}

// === IsNullCondition Tests ===

func TestIsNullCondition_ToQdrantCondition(t *testing.T) {
	c := IsNullCondition{Key: "deleted_at"}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestIsNullCondition_UserField(t *testing.T) {
	c := IsNullCondition{Key: "review_date", FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBuildFilter_WithIsNullCondition(t *testing.T) {
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				IsNullCondition{Key: "deleted_at"},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 Must condition, got %d", len(result.Must))
	}
}

// === IsEmptyCondition Tests ===

func TestIsEmptyCondition_ToQdrantCondition(t *testing.T) {
	c := IsEmptyCondition{Key: "tags"}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestIsEmptyCondition_UserField(t *testing.T) {
	c := IsEmptyCondition{Key: "categories", FieldType: UserField}
	result := c.ToQdrantCondition()

	if len(result) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result))
	}
}

func TestBuildFilter_WithIsEmptyCondition(t *testing.T) {
	// Find documents where tags is NOT empty (using MustNot)
	filters := &FilterSet{
		MustNot: &ConditionSet{
			Conditions: []FilterCondition{
				IsEmptyCondition{Key: "tags"},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.MustNot) != 1 {
		t.Errorf("expected 1 MustNot condition, got %d", len(result.MustNot))
	}
}

// === MarshalJSON Tests ===

func TestTimeRangeCondition_MarshalJSON(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	c := TimeRangeCondition{
		Key: "created_at",
		Value: TimeRange{
			Gte: &startTime,
			Lt:  &endTime,
		},
	}

	data, err := c.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	// Check that it contains expected fields
	if !contains(jsonStr, `"field":"created_at"`) {
		t.Errorf("expected field in JSON, got %s", jsonStr)
	}
	if !contains(jsonStr, `"atOrAfter"`) {
		t.Errorf("expected atOrAfter in JSON, got %s", jsonStr)
	}
	if !contains(jsonStr, `"before"`) {
		t.Errorf("expected before in JSON, got %s", jsonStr)
	}
}

func TestNumericRangeCondition_MarshalJSON(t *testing.T) {
	minPrice := 100.0
	maxPrice := 500.0

	c := NumericRangeCondition{
		Key: "price",
		Value: NumericRange{
			Gte: &minPrice,
			Lte: &maxPrice,
		},
	}

	data, err := c.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"field":"price"`) {
		t.Errorf("expected field in JSON, got %s", jsonStr)
	}
	if !contains(jsonStr, `"greaterThanOrEqualTo"`) {
		t.Errorf("expected greaterThanOrEqualTo in JSON, got %s", jsonStr)
	}
	if !contains(jsonStr, `"lessThanOrEqualTo"`) {
		t.Errorf("expected lessThanOrEqualTo in JSON, got %s", jsonStr)
	}
}

// === Complex Filter Tests ===

func TestBuildFilter_ComplexCombination(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	minPrice := 100.0

	// Complex filter:
	// (search_store_id = "store-123" AND created_at >= 2024-01-01 AND price >= 100)
	// AND (city IN ("London", "Berlin"))
	// AND NOT (deleted = true OR status IN ("draft", "archived"))
	filters := &FilterSet{
		Must: &ConditionSet{
			Conditions: []FilterCondition{
				TextCondition{Key: "search_store_id", Value: "store-123"},
				TimeRangeCondition{
					Key:   "created_at",
					Value: TimeRange{Gte: &startTime},
				},
				NumericRangeCondition{
					Key:       "price",
					Value:     NumericRange{Gte: &minPrice},
					FieldType: UserField,
				},
			},
		},
		Should: &ConditionSet{
			Conditions: []FilterCondition{
				TextAnyCondition{Key: "city", Values: []string{"London", "Berlin"}},
			},
		},
		MustNot: &ConditionSet{
			Conditions: []FilterCondition{
				BoolCondition{Key: "deleted", Value: true},
				TextAnyCondition{Key: "status", Values: []string{"draft", "archived"}},
			},
		},
	}
	result := buildFilter(filters)

	if result == nil {
		t.Fatal("expected filter, got nil")
	}
	if len(result.Must) != 3 {
		t.Errorf("expected 3 Must conditions, got %d", len(result.Must))
	}
	if len(result.Should) != 1 {
		t.Errorf("expected 1 Should condition, got %d", len(result.Should))
	}
	if len(result.MustNot) != 2 {
		t.Errorf("expected 2 MustNot conditions, got %d", len(result.MustNot))
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

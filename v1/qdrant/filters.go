package qdrant

import (
	"encoding/json"
	"strings"
	"time"

	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserPayloadPrefix is the prefix for user-defined metadata fields
const UserPayloadPrefix = "custom"

// FilterCondition is the interface for all filter conditions
type FilterCondition interface {
	ToQdrantCondition() []*qdrant.Condition
}

// FieldType indicates whether a field is internal or user-defined
type FieldType int

const (
	// InternalField - system-managed fields stored at top-level
	InternalField FieldType = iota
	// UserField - user-defined fields stored under "custom." prefix
	UserField
)

// TimeRange represents a time-based filter condition
type TimeRange struct {
	Gt  *time.Time `json:"after,omitempty"`      // Greater than this time
	Gte *time.Time `json:"atOrAfter,omitempty"`  // Greater than or equal to this time
	Lt  *time.Time `json:"before,omitempty"`     // Less than this time
	Lte *time.Time `json:"atOrBefore,omitempty"` // Less than or equal to this time
}

// NumericRange represents a numeric range filter condition
type NumericRange struct {
	Gt  *float64 `json:"greaterThan,omitempty"`          // Greater than
	Gte *float64 `json:"greaterThanOrEqualTo,omitempty"` // Greater than or equal
	Lt  *float64 `json:"lessThan,omitempty"`             // Less than
	Lte *float64 `json:"lessThanOrEqualTo,omitempty"`    // Less than or equal
}

type MatchCondition[T comparable] struct {
	Key       string    `json:"field"`
	Value     T         `json:"equalTo"`
	FieldType FieldType `json:"-"` // Internal or User field (default: InternalField)
}

func (c MatchCondition[T]) ToQdrantCondition() []*qdrant.Condition {
	key := resolveFieldKey(c.Key, c.FieldType)
	switch v := any(c.Value).(type) {
	case string:
		return []*qdrant.Condition{qdrant.NewMatch(key, v)}
	case bool:
		return []*qdrant.Condition{qdrant.NewMatchBool(key, v)}
	case int64:
		return []*qdrant.Condition{qdrant.NewMatchInt(key, v)}
	default:
		//Unsupported type
		return nil
	}

}

// MatchAnyCondition matches if value is one of the given values (IN operator)
// Applicable to keyword (string) and integer payloads
type MatchAnyCondition[T string | int64] struct {
	Key       string    `json:"field"`
	Values    []T       `json:"anyOf"`
	FieldType FieldType `json:"-"` // Internal or User field (default: InternalField)
}

func (c MatchAnyCondition[T]) ToQdrantCondition() []*qdrant.Condition {
	key := resolveFieldKey(c.Key, c.FieldType)
	switch v := any(c.Values).(type) {
	case []string:
		return []*qdrant.Condition{qdrant.NewMatchKeywords(key, v...)}
	case []int64:
		return []*qdrant.Condition{qdrant.NewMatchInts(key, v...)}
	default:
		return nil
	}
}

// MatchExceptCondition matches if value is NOT one of the given values (NOT IN operator)
// Applicable to keyword (string) and integer payloads
type MatchExceptCondition[T string | int64] struct {
	Key       string    `json:"field"`
	Values    []T       `json:"noneOf"`
	FieldType FieldType `json:"-"` // Internal or User field (default: InternalField)
}

func (c MatchExceptCondition[T]) ToQdrantCondition() []*qdrant.Condition {
	key := resolveFieldKey(c.Key, c.FieldType)
	switch v := any(c.Values).(type) {
	case []string:
		return []*qdrant.Condition{qdrant.NewMatchExceptKeywords(key, v...)}
	case []int64:
		return []*qdrant.Condition{qdrant.NewMatchExceptInts(key, v...)}
	default:
		return nil
	}
}

type TextCondition = MatchCondition[string]             // Exact string match
type BoolCondition = MatchCondition[bool]               // Exact boolean match
type IntCondition = MatchCondition[int64]               // Exact integer match
type TextAnyCondition = MatchAnyCondition[string]       // String IN operator
type IntAnyCondition = MatchAnyCondition[int64]         // Integer IN operator
type TextExceptCondition = MatchExceptCondition[string] // String NOT IN
type IntExceptCondition = MatchExceptCondition[int64]   // Integer NOT IN

// TimeRangeCondition represents a time range filter condition
type TimeRangeCondition struct {
	Key       string    `json:"field"`
	Value     TimeRange `json:"-"`
	FieldType FieldType `json:"-"`
}

func (c TimeRangeCondition) ToQdrantCondition() []*qdrant.Condition {
	return buildDateTimeRangeConditions(resolveFieldKey(c.Key, c.FieldType), c.Value)
}

func (c TimeRangeCondition) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Field      string     `json:"field"`
		After      *time.Time `json:"after,omitempty"`
		AtOrAfter  *time.Time `json:"atOrAfter,omitempty"`
		Before     *time.Time `json:"before,omitempty"`
		AtOrBefore *time.Time `json:"atOrBefore,omitempty"`
	}
	return json.Marshal(Alias{
		Field:      c.Key,
		After:      c.Value.Gt,
		AtOrAfter:  c.Value.Gte,
		Before:     c.Value.Lt,
		AtOrBefore: c.Value.Lte,
	})
}

// NumericRangeCondition represents a numeric range filter
type NumericRangeCondition struct {
	Key       string       `json:"field"`
	Value     NumericRange `json:"-"`
	FieldType FieldType    `json:"-"`
}

func (c NumericRangeCondition) ToQdrantCondition() []*qdrant.Condition {
	return buildNumericRangeConditions(resolveFieldKey(c.Key, c.FieldType), c.Value)
}

func (c NumericRangeCondition) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Field                string   `json:"field"`
		GreaterThan          *float64 `json:"greaterThan,omitempty"`
		GreaterThanOrEqualTo *float64 `json:"greaterThanOrEqualTo,omitempty"`
		LessThan             *float64 `json:"lessThan,omitempty"`
		LessThanOrEqualTo    *float64 `json:"lessThanOrEqualTo,omitempty"`
	}
	return json.Marshal(Alias{
		Field:                c.Key,
		GreaterThan:          c.Value.Gt,
		GreaterThanOrEqualTo: c.Value.Gte,
		LessThan:             c.Value.Lt,
		LessThanOrEqualTo:    c.Value.Lte,
	})
}

// resolveFieldKey returns the full field path based on FieldType
// Internal fields: "search_store_id" -> "search_store_id"
// User fields: "document_id" -> "custom.document_id"
func resolveFieldKey(key string, fieldType FieldType) string {
	if fieldType == UserField {
		// Prevent double-prefixing
		if strings.HasPrefix(key, UserPayloadPrefix+".") {
			return key
		}
		return UserPayloadPrefix + "." + key
	}
	return key
}

// ConditionSet holds conditions for a single clause
type ConditionSet struct {
	Conditions []FilterCondition `json:"conditions,omitempty"`
}

// FilterSet supports Must (AND), Should (OR), and MustNot (NOT) clauses.
// Use with SearchRequest.Filters to filter search results.
//
// Example:
//
//	filters := &FilterSet{
//	    Must: &ConditionSet{
//	        Conditions: []FilterCondition{
//	            TextCondition{Key: "city", Value: "London"},
//	        },
//	    },
//	}
type FilterSet struct {
	Must    *ConditionSet `json:"with,omitempty"`      // AND - all conditions must match
	Should  *ConditionSet `json:"withOneOf,omitempty"` // OR - at least one condition must match
	MustNot *ConditionSet `json:"without,omitempty"`   // NOT - none of the conditions should match
}

// buildFilter constructs a Qdrant filter from FilterSet
func buildFilter(filters *FilterSet) *qdrant.Filter {
	if filters == nil {
		return nil
	}

	filter := &qdrant.Filter{}

	if filters.Must != nil {
		filter.Must = buildConditions(filters.Must)
	}

	if filters.Should != nil {
		filter.Should = buildConditions(filters.Should)
	}

	if filters.MustNot != nil {
		filter.MustNot = buildConditions(filters.MustNot)
	}

	// Return nil if no conditions were added
	if len(filter.Must) == 0 && len(filter.Should) == 0 && len(filter.MustNot) == 0 {
		return nil
	}

	return filter
}

// buildConditions converts a ConditionSet to Qdrant conditions
func buildConditions(cs *ConditionSet) []*qdrant.Condition {
	if cs == nil {
		return nil
	}

	var conditions []*qdrant.Condition
	for _, c := range cs.Conditions {
		conditions = append(conditions, c.ToQdrantCondition()...)
	}
	return conditions
}

// buildDateTimeRangeConditions creates datetime range conditions
func buildDateTimeRangeConditions(key string, tr TimeRange) []*qdrant.Condition {
	dateRange := &qdrant.DatetimeRange{
		Gt:  toTimestamp(tr.Gt),
		Gte: toTimestamp(tr.Gte),
		Lt:  toTimestamp(tr.Lt),
		Lte: toTimestamp(tr.Lte),
	}

	// Check if any field is set
	if dateRange.Gt == nil && dateRange.Gte == nil && dateRange.Lt == nil && dateRange.Lte == nil {
		return nil
	}

	return []*qdrant.Condition{qdrant.NewDatetimeRange(key, dateRange)}
}

// buildNumericRangeConditions creates numeric range conditions
func buildNumericRangeConditions(key string, nr NumericRange) []*qdrant.Condition {
	rangeFilter := &qdrant.Range{
		Gt:  nr.Gt,
		Gte: nr.Gte,
		Lt:  nr.Lt,
		Lte: nr.Lte,
	}

	// Check if any field is set
	if rangeFilter.Gt == nil && rangeFilter.Gte == nil && rangeFilter.Lt == nil && rangeFilter.Lte == nil {
		return nil
	}

	return []*qdrant.Condition{qdrant.NewRange(key, rangeFilter)}
}

// toTimestamp converts a *time.Time to *timestamppb.Timestamp (nil-safe)
func toTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// IsNullCondition checks if a field is null
type IsNullCondition struct {
	Key       string    `json:"field"`
	FieldType FieldType `json:"-"` // Internal or User field (default: InternalField)
}

func (c IsNullCondition) ToQdrantCondition() []*qdrant.Condition {
	key := resolveFieldKey(c.Key, c.FieldType)
	return []*qdrant.Condition{qdrant.NewIsNull(key)}
}

// IsEmptyCondition checks if a field is empty (does not exist, null, or [])
type IsEmptyCondition struct {
	Key       string    `json:"field"`
	FieldType FieldType `json:"-"` // Internal or User field (default: InternalField)
}

func (c IsEmptyCondition) ToQdrantCondition() []*qdrant.Condition {
	key := resolveFieldKey(c.Key, c.FieldType)
	return []*qdrant.Condition{qdrant.NewIsEmpty(key)}
}

// === Payload Helpers ===

// BuildPayload creates a Qdrant payload with separated internal and user fields
func BuildPayload(internal map[string]any, user map[string]any) map[string]any {
	payload := make(map[string]any)

	// Add internal fields at top-level
	for k, v := range internal {
		payload[k] = v
	}

	// Add user fields under "custom" prefix
	if len(user) > 0 {
		payload[UserPayloadPrefix] = user
	}

	return payload
}

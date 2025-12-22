package qdrant

import (
	"fmt"
	"strings"
	"time"

	"github.com/Aleph-Alpha/std/v1/vectordb"
	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserPayloadPrefix is the prefix for user-defined metadata fields.
// User fields are stored under "custom." in the Qdrant payload.
const UserPayloadPrefix = "custom"

// BuildPayload creates a Qdrant payload with separated internal and user fields.
// Internal fields are stored at the top level, while user fields are stored under
// the "custom" prefix.
//
// Example:
//
//	payload := BuildPayload(
//	    map[string]any{"search_store_id": "store123"},
//	    map[string]any{"document_id": "doc456"},
//	)
//	// Result: {"search_store_id": "store123", "custom": {"document_id": "doc456"}}
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

// ── Filter Conversion ────────────────────────────────────────────────────────

// convertVectorDBFilterSet converts a vectordb.FilterSet to a Qdrant filter.
func convertVectorDBFilterSet(filters *vectordb.FilterSet) *qdrant.Filter {
	if filters == nil {
		return nil
	}

	filter := &qdrant.Filter{}

	if filters.Must != nil {
		filter.Must = convertVectorDBConditionSet(filters.Must)
	}
	if filters.Should != nil {
		filter.Should = convertVectorDBConditionSet(filters.Should)
	}
	if filters.MustNot != nil {
		filter.MustNot = convertVectorDBConditionSet(filters.MustNot)
	}

	// Return nil if no conditions were added
	if len(filter.Must) == 0 && len(filter.Should) == 0 && len(filter.MustNot) == 0 {
		return nil
	}

	return filter
}

// convertVectorDBConditionSet converts a vectordb.ConditionSet to Qdrant conditions.
func convertVectorDBConditionSet(cs *vectordb.ConditionSet) []*qdrant.Condition {
	if cs == nil {
		return nil
	}

	var conditions []*qdrant.Condition
	for _, c := range cs.Conditions {
		conds := convertVectorDBCondition(c)
		for _, cond := range conds {
			if cond != nil {
				conditions = append(conditions, cond)
			}
		}
	}
	return conditions
}

// convertVectorDBCondition converts a single vectordb.FilterCondition to Qdrant conditions.
func convertVectorDBCondition(c vectordb.FilterCondition) []*qdrant.Condition {
	switch cond := c.(type) {
	case *vectordb.MatchCondition:
		return convertVectorDBMatchCondition(cond)
	case *vectordb.MatchAnyCondition:
		return convertVectorDBMatchAnyCondition(cond)
	case *vectordb.MatchExceptCondition:
		return convertVectorDBMatchExceptCondition(cond)
	case *vectordb.NumericRangeCondition:
		return convertVectorDBNumericRangeCondition(cond)
	case *vectordb.TimeRangeCondition:
		return convertVectorDBTimeRangeCondition(cond)
	case *vectordb.IsNullCondition:
		return convertVectorDBIsNullCondition(cond)
	case *vectordb.IsEmptyCondition:
		return convertVectorDBIsEmptyCondition(cond)
	default:
		return nil
	}
}

func convertVectorDBMatchCondition(c *vectordb.MatchCondition) []*qdrant.Condition {
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)
	switch v := c.Value.(type) {
	case string:
		return []*qdrant.Condition{qdrant.NewMatch(key, v)}
	case bool:
		return []*qdrant.Condition{qdrant.NewMatchBool(key, v)}
	case int:
		return []*qdrant.Condition{qdrant.NewMatchInt(key, int64(v))}
	case int64:
		return []*qdrant.Condition{qdrant.NewMatchInt(key, v)}
	case float64:
		// Handle JSON numbers which are float64 by default
		return []*qdrant.Condition{qdrant.NewMatchInt(key, int64(v))}
	default:
		return nil
	}
}

func convertVectorDBMatchAnyCondition(c *vectordb.MatchAnyCondition) []*qdrant.Condition {
	if len(c.Values) == 0 {
		return nil
	}
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)

	// Detect type from first value
	switch c.Values[0].(type) {
	case string:
		strs := make([]string, len(c.Values))
		for i, v := range c.Values {
			if s, ok := v.(string); ok {
				strs[i] = s
			}
		}
		return []*qdrant.Condition{qdrant.NewMatchKeywords(key, strs...)}
	case int, int64, float64:
		ints := make([]int64, len(c.Values))
		for i, v := range c.Values {
			switch n := v.(type) {
			case int:
				ints[i] = int64(n)
			case int64:
				ints[i] = n
			case float64:
				ints[i] = int64(n)
			}
		}
		return []*qdrant.Condition{qdrant.NewMatchInts(key, ints...)}
	}
	return nil
}

func convertVectorDBMatchExceptCondition(c *vectordb.MatchExceptCondition) []*qdrant.Condition {
	if len(c.Values) == 0 {
		return nil
	}
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)

	// Detect type from first value
	switch c.Values[0].(type) {
	case string:
		strs := make([]string, len(c.Values))
		for i, v := range c.Values {
			if s, ok := v.(string); ok {
				strs[i] = s
			}
		}
		return []*qdrant.Condition{qdrant.NewMatchExceptKeywords(key, strs...)}
	case int, int64, float64:
		ints := make([]int64, len(c.Values))
		for i, v := range c.Values {
			switch n := v.(type) {
			case int:
				ints[i] = int64(n)
			case int64:
				ints[i] = n
			case float64:
				ints[i] = int64(n)
			}
		}
		return []*qdrant.Condition{qdrant.NewMatchExceptInts(key, ints...)}
	}
	return nil
}

func convertVectorDBNumericRangeCondition(c *vectordb.NumericRangeCondition) []*qdrant.Condition {
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)
	rangeFilter := &qdrant.Range{
		Gt:  c.Range.Gt,
		Gte: c.Range.Gte,
		Lt:  c.Range.Lt,
		Lte: c.Range.Lte,
	}

	if rangeFilter.Gt == nil && rangeFilter.Gte == nil &&
		rangeFilter.Lt == nil && rangeFilter.Lte == nil {
		return nil
	}

	return []*qdrant.Condition{qdrant.NewRange(key, rangeFilter)}
}

func convertVectorDBTimeRangeCondition(c *vectordb.TimeRangeCondition) []*qdrant.Condition {
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)
	dateRange := &qdrant.DatetimeRange{
		Gt:  toVectorDBTimestamp(c.Range.Gt),
		Gte: toVectorDBTimestamp(c.Range.Gte),
		Lt:  toVectorDBTimestamp(c.Range.Lt),
		Lte: toVectorDBTimestamp(c.Range.Lte),
	}

	if dateRange.Gt == nil && dateRange.Gte == nil &&
		dateRange.Lt == nil && dateRange.Lte == nil {
		return nil
	}

	return []*qdrant.Condition{qdrant.NewDatetimeRange(key, dateRange)}
}

func convertVectorDBIsNullCondition(c *vectordb.IsNullCondition) []*qdrant.Condition {
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)
	return []*qdrant.Condition{qdrant.NewIsNull(key)}
}

func convertVectorDBIsEmptyCondition(c *vectordb.IsEmptyCondition) []*qdrant.Condition {
	key := resolveVectorDBFieldKey(c.Field, c.FieldType)
	return []*qdrant.Condition{qdrant.NewIsEmpty(key)}
}

// resolveVectorDBFieldKey returns the full field path based on FieldType.
// Internal fields: "search_store_id" -> "search_store_id"
// User fields: "document_id" -> "custom.document_id"
func resolveVectorDBFieldKey(key string, fieldType vectordb.FieldType) string {
	if fieldType == vectordb.UserField {
		if strings.HasPrefix(key, UserPayloadPrefix+".") {
			return key
		}
		return UserPayloadPrefix + "." + key
	}
	return key
}

func toVectorDBTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// ── Result Conversion ────────────────────────────────────────────────────────

// parseVectorDBSearchResults converts Qdrant response to vectordb.SearchResult slice.
func parseVectorDBSearchResults(resp []*qdrant.ScoredPoint) ([]vectordb.SearchResult, error) {
	results := make([]vectordb.SearchResult, 0, len(resp))
	for _, r := range resp {
		id, err := extractVectorDBPointID(r.Id)
		if err != nil {
			return nil, err
		}

		results = append(results, vectordb.SearchResult{
			ID:      id,
			Score:   r.Score,
			Payload: convertVectorDBPayload(r.Payload),
		})
	}
	return results, nil
}

// extractVectorDBPointID extracts a string ID from Qdrant's PointId type.
func extractVectorDBPointID(id *qdrant.PointId) (string, error) {
	if id == nil {
		return "", fmt.Errorf("nil point ID")
	}
	switch v := id.PointIdOptions.(type) {
	case *qdrant.PointId_Num:
		return fmt.Sprintf("%d", v.Num), nil
	case *qdrant.PointId_Uuid:
		return v.Uuid, nil
	default:
		return "", fmt.Errorf("unexpected PointId type: %T", v)
	}
}

// convertVectorDBPayload converts Qdrant's protobuf payload to a generic map.
func convertVectorDBPayload(payload map[string]*qdrant.Value) map[string]any {
	if payload == nil {
		return nil
	}
	result := make(map[string]any, len(payload))
	for k, v := range payload {
		result[k] = extractVectorDBValue(v)
	}
	return result
}

// extractVectorDBValue recursively converts a Qdrant Value to a Go native type.
func extractVectorDBValue(v *qdrant.Value) any {
	if v == nil {
		return nil
	}
	switch val := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return val.StringValue
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	case *qdrant.Value_DoubleValue:
		return val.DoubleValue
	case *qdrant.Value_BoolValue:
		return val.BoolValue
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_StructValue:
		if val.StructValue == nil {
			return nil
		}
		return convertVectorDBPayload(val.StructValue.Fields)
	case *qdrant.Value_ListValue:
		if val.ListValue == nil {
			return nil
		}
		items := make([]any, len(val.ListValue.Values))
		for i, item := range val.ListValue.Values {
			items[i] = extractVectorDBValue(item)
		}
		return items
	default:
		return nil
	}
}

// convertVectorDBFilterSets converts an array of FilterSets to a single Qdrant filter.
// Multiple filter sets are combined with AND logic (all must match).
func convertVectorDBFilterSets(filters []*vectordb.FilterSet) *qdrant.Filter {
	if len(filters) == 0 {
		return nil
	}

	// Single filter set - convert directly
	if len(filters) == 1 {
		return convertVectorDBFilterSet(filters[0])
	}

	// Multiple filter sets - combine with AND
	var allConditions []*qdrant.Condition
	for _, fs := range filters {
		converted := convertVectorDBFilterSet(fs)
		if converted != nil {
			allConditions = append(allConditions, &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Filter{Filter: converted},
			})
		}
	}

	if len(allConditions) == 0 {
		return nil
	}

	return &qdrant.Filter{Must: allConditions}
}

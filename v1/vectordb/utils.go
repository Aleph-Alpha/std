package vectordb

import (
	"encoding/json"
	"fmt"
)

// ── FilterSet Constructors ───────────────────────────────────────────────────

// NewFilterSet creates a FilterSet with the given clauses.
// Use with Must(), Should(), and MustNot() helpers.
//
// Example:
//
//	vectordb.NewFilterSet(
//	    vectordb.Must(vectordb.NewMatch("status", "published")),
//	    vectordb.Should(vectordb.NewMatch("tag", "ml"), vectordb.NewMatch("tag", "ai")),
//	)
func NewFilterSet(clauses ...func(*FilterSet)) *FilterSet {
	fs := &FilterSet{}
	for _, clause := range clauses {
		clause(fs)
	}
	return fs
}

// Must creates a Must clause (AND logic) with the given conditions.
// All conditions must match for a document to be included.
func Must(conditions ...FilterCondition) func(*FilterSet) {
	return func(fs *FilterSet) {
		fs.Must = &ConditionSet{Conditions: conditions}
	}
}

// Should creates a Should clause (OR logic) with the given conditions.
// At least one condition must match for a document to be included.
func Should(conditions ...FilterCondition) func(*FilterSet) {
	return func(fs *FilterSet) {
		fs.Should = &ConditionSet{Conditions: conditions}
	}
}

// MustNot creates a MustNot clause (NOT logic) with the given conditions.
// Documents matching any of these conditions are excluded.
func MustNot(conditions ...FilterCondition) func(*FilterSet) {
	return func(fs *FilterSet) {
		fs.MustNot = &ConditionSet{Conditions: conditions}
	}
}

// ── Condition Constructors ───────────────────────────────────────────────────

// NewMatch creates a match condition for internal fields.
func NewMatch(field string, value any) *MatchCondition {
	return &MatchCondition{Field: field, Value: value, FieldType: InternalField}
}

// NewUserMatch creates a match condition for user-defined fields.
func NewUserMatch(field string, value any) *MatchCondition {
	return &MatchCondition{Field: field, Value: value, FieldType: UserField}
}

// NewMatchAny creates an IN condition for internal fields.
func NewMatchAny(field string, values ...any) *MatchAnyCondition {
	validateHomogeneousTypes(values)
	return &MatchAnyCondition{Field: field, Values: values, FieldType: InternalField}
}

// NewUserMatchAny creates an IN condition for user-defined fields.
func NewUserMatchAny(field string, values ...any) *MatchAnyCondition {
	validateHomogeneousTypes(values)
	return &MatchAnyCondition{Field: field, Values: values, FieldType: UserField}
}

// NewMatchExcept creates a NOT IN condition for internal fields.
func NewMatchExcept(field string, values ...any) *MatchExceptCondition {
	validateHomogeneousTypes(values)
	return &MatchExceptCondition{Field: field, Values: values, FieldType: InternalField}
}

// NewUserMatchExcept creates a NOT IN condition for user-defined fields.
func NewUserMatchExcept(field string, values ...any) *MatchExceptCondition {
	validateHomogeneousTypes(values)
	return &MatchExceptCondition{Field: field, Values: values, FieldType: UserField}
}

// NewNumericRange creates a numeric range condition for internal fields.
func NewNumericRange(field string, r NumericRange) *NumericRangeCondition {
	return &NumericRangeCondition{
		Field:     field,
		Range:     r,
		FieldType: InternalField,
	}
}

// NewUserNumericRange creates a numeric range condition for user-defined fields.
func NewUserNumericRange(field string, r NumericRange) *NumericRangeCondition {
	return &NumericRangeCondition{
		Field:     field,
		Range:     r,
		FieldType: UserField,
	}
}

// NewTimeRange creates a time range condition for internal fields.
func NewTimeRange(field string, t TimeRange) *TimeRangeCondition {
	return &TimeRangeCondition{
		Field:     field,
		Range:     t,
		FieldType: InternalField,
	}
}

// NewUserTimeRange creates a time range condition for user-defined fields.
func NewUserTimeRange(field string, t TimeRange) *TimeRangeCondition {
	return &TimeRangeCondition{
		Field:     field,
		Range:     t,
		FieldType: UserField,
	}
}

// NewIsNull creates an IS NULL condition for internal fields.
func NewIsNull(field string) *IsNullCondition {
	return &IsNullCondition{Field: field, FieldType: InternalField}
}

// NewUserIsNull creates an IS NULL condition for user-defined fields.
func NewUserIsNull(field string) *IsNullCondition {
	return &IsNullCondition{Field: field, FieldType: UserField}
}

// NewIsEmpty creates an IS EMPTY condition for internal fields.
func NewIsEmpty(field string) *IsEmptyCondition {
	return &IsEmptyCondition{Field: field, FieldType: InternalField}
}

// NewUserIsEmpty creates an IS EMPTY condition for user-defined fields.
func NewUserIsEmpty(field string) *IsEmptyCondition {
	return &IsEmptyCondition{Field: field, FieldType: UserField}
}

// ── JSON Serialization ───────────────────────────────────────────────────────

// MarshalJSON implements custom JSON marshaling for ConditionSet.
// This is needed because FilterCondition is an interface.
func (cs *ConditionSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(cs.Conditions)
}

// UnmarshalJSON implements custom JSON unmarshaling for ConditionSet.
// It detects the condition type based on JSON keys and deserializes
// into the appropriate concrete type (MatchCondition, NumericRangeCondition, etc.)
func (cs *ConditionSet) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	cs.Conditions = make([]FilterCondition, 0, len(raw))

	for _, r := range raw {
		cond, err := parseCondition(r)
		if err != nil {
			return err
		}
		cs.Conditions = append(cs.Conditions, cond)
	}

	return nil
}

// parseCondition detects and parses a single FilterCondition from JSON.
// It examines the JSON keys to determine the condition type:
//   - "equalTo" → MatchCondition
//   - "anyOf" → MatchAnyCondition
//   - "noneOf" → MatchExceptCondition
//   - "greaterThan", "lessThan", etc. → NumericRangeCondition
//   - "after", "before", etc. → TimeRangeCondition
func parseCondition(data []byte) (FilterCondition, error) {
	// Extract field names to determine condition type
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}

	switch {
	case hasKey(fields, "equalTo"):
		var c MatchCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil

	case hasKey(fields, "anyOf"):
		var c MatchAnyCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil

	case hasKey(fields, "noneOf"):
		var c MatchExceptCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil

	case hasKey(fields, "greaterThan"), hasKey(fields, "greaterThanOrEqualTo"),
		hasKey(fields, "lessThan"), hasKey(fields, "lessThanOrEqualTo"):
		var c NumericRangeCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil

	case hasKey(fields, "after"), hasKey(fields, "atOrAfter"),
		hasKey(fields, "before"), hasKey(fields, "atOrBefore"):
		var c TimeRangeCondition
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil

	default:
		return nil, fmt.Errorf("unknown filter condition type: %s", string(data))
	}
}

// hasKey checks if a JSON object contains a specific key.
// Used by parseCondition to detect filter condition types.
func hasKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

// validateHomogeneousTypes ensures all values are of the same type category.
// Panics if mixed types are detected - this catches programming errors early.
//
// TODO: Consider whether panic is appropriate here, or if we should:
//   - Return an error instead (for runtime data validation)
//   - Add a separate NewMatchAnyChecked() that returns error
//   - Keep panic for constructor calls, error for JSON unmarshaling
func validateHomogeneousTypes(values []any) {
	if len(values) <= 1 {
		return
	}

	expectedType := getType(values[0])
	if expectedType == "" {
		panic(fmt.Sprintf("vectordb: unsupported value type: %T", values[0]))
	}

	// Validate all values match expected type
	for i, v := range values[1:] {
		actualType := getType(v)
		if actualType == "" {
			panic(fmt.Sprintf("vectordb: unsupported value type at index %d: %T", i+1, v))
		}
		if actualType != expectedType {
			panic(fmt.Sprintf("vectordb: mixed types not allowed in MatchAny/MatchExcept: expected %s but got %s at index %d", expectedType, actualType, i+1))
		}
	}
}

func getType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int64, float64:
		return "numeric"
	case bool:
		return "boolean"
	}
	return ""
}

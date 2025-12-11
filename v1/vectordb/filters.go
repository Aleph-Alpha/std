package vectordb

import (
	"encoding/json"
	"time"
)

// FieldType indicates whether a field is internal (system-managed)
// or user-defined (stored under a prefix like "custom.").
type FieldType int

const (
	// InternalField - system-managed fields stored at top-level
	InternalField FieldType = iota
	// UserField - user-defined fields stored under a prefix (e.g., "custom.")
	UserField
)

// FilterCondition is the interface all filter conditions must implement.
// Each database adapter converts these to its native filter format.
type FilterCondition interface {
	// isFilterCondition is a marker method to ensure type safety
	isFilterCondition()
}

// FilterSet supports Must (AND), Should (OR), and MustNot (NOT) clauses.
// Use with SearchRequest.Filters to filter search results.
//
// Example:
//
//	filters := &FilterSet{
//	    Must: &ConditionSet{
//	        Conditions: []FilterCondition{
//	            &MatchCondition{Field: "city", Value: "London"},
//	        },
//	    },
//	}
type FilterSet struct {
	// Must: All conditions must match (AND)
	Must *ConditionSet `json:"must,omitempty"`
	// Should: At least one condition must match (OR)
	Should *ConditionSet `json:"should,omitempty"`
	// MustNot: None of the conditions should match (NOT)
	MustNot *ConditionSet `json:"mustNot,omitempty"`
}

// ConditionSet holds a group of conditions for a single clause.
type ConditionSet struct {
	Conditions []FilterCondition `json:"conditions,omitempty"`
}

// ── Match Conditions ─────────────────────────────────────────────────────────

// MatchCondition represents an exact match filter (WHERE field = value).
// Supports string, bool, and int64 values.
type MatchCondition struct {
	Field     string    `json:"field"`
	Value     any       `json:"equalTo"`
	FieldType FieldType `json:"-"`
}

func (c *MatchCondition) isFilterCondition() {}

// MatchAnyCondition matches if value is one of the given values (IN operator).
// SQL equivalent: WHERE field IN (value1, value2, ...)
type MatchAnyCondition struct {
	Field     string    `json:"field"`
	Values    []any     `json:"anyOf"`
	FieldType FieldType `json:"-"`
}

func (c *MatchAnyCondition) isFilterCondition() {}

// MatchExceptCondition matches if value is NOT one of the given values (NOT IN).
// SQL equivalent: WHERE field NOT IN (value1, value2, ...)
type MatchExceptCondition struct {
	Field     string    `json:"field"`
	Values    []any     `json:"noneOf"`
	FieldType FieldType `json:"-"`
}

func (c *MatchExceptCondition) isFilterCondition() {}

// ── Range Conditions ─────────────────────────────────────────────────────────

// NumericRangeCondition filters by numeric range.
// SQL equivalent: WHERE field >= min AND field <= max
type NumericRangeCondition struct {
	Field                string    `json:"field"`
	GreaterThan          *float64  `json:"greaterThan,omitempty"`
	GreaterThanOrEqualTo *float64  `json:"greaterThanOrEqualTo,omitempty"`
	LessThan             *float64  `json:"lessThan,omitempty"`
	LessThanOrEqualTo    *float64  `json:"lessThanOrEqualTo,omitempty"`
	FieldType            FieldType `json:"-"`
}

func (c *NumericRangeCondition) isFilterCondition() {}

// TimeRangeCondition filters by datetime range.
// SQL equivalent: WHERE created_at >= '2024-01-01' AND created_at < '2025-01-01'
type TimeRangeCondition struct {
	Field      string     `json:"field"`
	After      *time.Time `json:"after,omitempty"`
	AtOrAfter  *time.Time `json:"atOrAfter,omitempty"`
	Before     *time.Time `json:"before,omitempty"`
	AtOrBefore *time.Time `json:"atOrBefore,omitempty"`
	FieldType  FieldType  `json:"-"`
}

func (c *TimeRangeCondition) isFilterCondition() {}

// ── Null/Empty Conditions ────────────────────────────────────────────────────

// IsNullCondition checks if a field has a NULL value.
// SQL equivalent: WHERE field IS NULL
type IsNullCondition struct {
	Field     string    `json:"field"`
	FieldType FieldType `json:"-"`
}

func (c *IsNullCondition) isFilterCondition() {}

// IsEmptyCondition checks if a field is empty (doesn't exist, null, or []).
// SQL equivalent: WHERE field IS NULL OR field = ” OR field = []
type IsEmptyCondition struct {
	Field     string    `json:"field"`
	FieldType FieldType `json:"-"`
}

func (c *IsEmptyCondition) isFilterCondition() {}

// ── Convenience Constructors ─────────────────────────────────────────────────

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
	return &MatchAnyCondition{Field: field, Values: values, FieldType: InternalField}
}

// NewUserMatchAny creates an IN condition for user-defined fields.
func NewUserMatchAny(field string, values ...any) *MatchAnyCondition {
	return &MatchAnyCondition{Field: field, Values: values, FieldType: UserField}
}

// NewMatchExcept creates a NOT IN condition for internal fields.
func NewMatchExcept(field string, values ...any) *MatchExceptCondition {
	return &MatchExceptCondition{Field: field, Values: values, FieldType: InternalField}
}

// NewUserMatchExcept creates a NOT IN condition for user-defined fields.
func NewUserMatchExcept(field string, values ...any) *MatchExceptCondition {
	return &MatchExceptCondition{Field: field, Values: values, FieldType: UserField}
}

// NewNumericRange creates a numeric range condition for internal fields.
func NewNumericRange(field string, gte, lte *float64) *NumericRangeCondition {
	return &NumericRangeCondition{
		Field:                field,
		GreaterThanOrEqualTo: gte,
		LessThanOrEqualTo:    lte,
		FieldType:            InternalField,
	}
}

// NewUserNumericRange creates a numeric range condition for user-defined fields.
func NewUserNumericRange(field string, gte, lte *float64) *NumericRangeCondition {
	return &NumericRangeCondition{
		Field:                field,
		GreaterThanOrEqualTo: gte,
		LessThanOrEqualTo:    lte,
		FieldType:            UserField,
	}
}

// NewTimeRange creates a time range condition for internal fields.
func NewTimeRange(field string, atOrAfter, before *time.Time) *TimeRangeCondition {
	return &TimeRangeCondition{
		Field:     field,
		AtOrAfter: atOrAfter,
		Before:    before,
		FieldType: InternalField,
	}
}

// NewUserTimeRange creates a time range condition for user-defined fields.
func NewUserTimeRange(field string, atOrAfter, before *time.Time) *TimeRangeCondition {
	return &TimeRangeCondition{
		Field:     field,
		AtOrAfter: atOrAfter,
		Before:    before,
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
func (cs *ConditionSet) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	cs.Conditions = make([]FilterCondition, 0, len(raw))
	return nil
}

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
	IsFilterCondition()
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

func (c *MatchCondition) IsFilterCondition() {}

// MatchAnyCondition matches if value is one of the given values (IN operator).
// SQL equivalent: WHERE field IN (value1, value2, ...)
type MatchAnyCondition struct {
	Field     string    `json:"field"`
	Values    []any     `json:"anyOf"`
	FieldType FieldType `json:"-"`
}

func (c *MatchAnyCondition) IsFilterCondition() {}

// MatchExceptCondition matches if value is NOT one of the given values (NOT IN).
// SQL equivalent: WHERE field NOT IN (value1, value2, ...)
type MatchExceptCondition struct {
	Field     string    `json:"field"`
	Values    []any     `json:"noneOf"`
	FieldType FieldType `json:"-"`
}

func (c *MatchExceptCondition) IsFilterCondition() {}

// ── Range Types ──────────────────────────────────────────────────────────────

// NumericRange defines bounds for numeric filtering.
// Used with NewNumericRange for cleaner constructor calls.
type NumericRange struct {
	Gt  *float64 `json:"greaterThan,omitempty"`          // GreaterThan (exclusive)
	Gte *float64 `json:"greaterThanOrEqualTo,omitempty"` // GreaterThanOrEqualTo (inclusive)
	Lt  *float64 `json:"lessThan,omitempty"`             // LessThan (exclusive)
	Lte *float64 `json:"lessThanOrEqualTo,omitempty"`    // LessThanOrEqualTo (inclusive)
}

// TimeRange defines bounds for time filtering.
// Used with NewTimeRange for cleaner constructor calls.
type TimeRange struct {
	Gt  *time.Time `json:"after,omitempty"`      // After (exclusive)
	Gte *time.Time `json:"atOrAfter,omitempty"`  // AtOrAfter (inclusive)
	Lt  *time.Time `json:"before,omitempty"`     // Before (exclusive)
	Lte *time.Time `json:"atOrBefore,omitempty"` // AtOrBefore (inclusive)
}

// ── Range Conditions ─────────────────────────────────────────────────────────

// NumericRangeCondition filters by numeric range.
// SQL equivalent: WHERE field >= min AND field <= max
type NumericRangeCondition struct {
	Field     string       `json:"field"`
	Range     NumericRange `json:"-"`
	FieldType FieldType    `json:"-"`
}

func (c *NumericRangeCondition) IsFilterCondition() {}

func (c *NumericRangeCondition) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Field                string   `json:"field"`
		GreaterThan          *float64 `json:"greaterThan,omitempty"`
		GreaterThanOrEqualTo *float64 `json:"greaterThanOrEqualTo,omitempty"`
		LessThan             *float64 `json:"lessThan,omitempty"`
		LessThanOrEqualTo    *float64 `json:"lessThanOrEqualTo,omitempty"`
	}
	return json.Marshal(Alias{
		Field:                c.Field,
		GreaterThan:          c.Range.Gt,
		GreaterThanOrEqualTo: c.Range.Gte,
		LessThan:             c.Range.Lt,
		LessThanOrEqualTo:    c.Range.Lte,
	})
}

func (c *NumericRangeCondition) UnmarshalJSON(data []byte) error {
	type Alias struct {
		Field                string   `json:"field"`
		GreaterThan          *float64 `json:"greaterThan,omitempty"`
		GreaterThanOrEqualTo *float64 `json:"greaterThanOrEqualTo,omitempty"`
		LessThan             *float64 `json:"lessThan,omitempty"`
		LessThanOrEqualTo    *float64 `json:"lessThanOrEqualTo,omitempty"`
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	c.Field = alias.Field
	c.Range = NumericRange{
		Gt:  alias.GreaterThan,
		Gte: alias.GreaterThanOrEqualTo,
		Lt:  alias.LessThan,
		Lte: alias.LessThanOrEqualTo,
	}
	return nil
}

// TimeRangeCondition filters by datetime range.
// SQL equivalent: WHERE created_at >= '2024-01-01' AND created_at < '2025-01-01'
type TimeRangeCondition struct {
	Field     string    `json:"field"`
	Range     TimeRange `json:"-"`
	FieldType FieldType `json:"-"`
}

func (c *TimeRangeCondition) IsFilterCondition() {}

func (c TimeRangeCondition) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Field      string     `json:"field"`
		After      *time.Time `json:"after,omitempty"`
		AtOrAfter  *time.Time `json:"atOrAfter,omitempty"`
		Before     *time.Time `json:"before,omitempty"`
		AtOrBefore *time.Time `json:"atOrBefore,omitempty"`
	}
	return json.Marshal(Alias{
		Field:      c.Field,
		After:      c.Range.Gt,
		AtOrAfter:  c.Range.Gte,
		Before:     c.Range.Lt,
		AtOrBefore: c.Range.Lte,
	})
}

func (c *TimeRangeCondition) UnmarshalJSON(data []byte) error {
	type Alias struct {
		Field      string     `json:"field"`
		After      *time.Time `json:"after,omitempty"`
		AtOrAfter  *time.Time `json:"atOrAfter,omitempty"`
		Before     *time.Time `json:"before,omitempty"`
		AtOrBefore *time.Time `json:"atOrBefore,omitempty"`
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	c.Field = alias.Field
	c.Range = TimeRange{
		Gt:  alias.After,
		Gte: alias.AtOrAfter,
		Lt:  alias.Before,
		Lte: alias.AtOrBefore,
	}
	return nil
}

// ── Null/Empty Conditions ────────────────────────────────────────────────────

// IsNullCondition checks if a field has a NULL value.
// SQL equivalent: WHERE field IS NULL
type IsNullCondition struct {
	Field     string    `json:"field"`
	FieldType FieldType `json:"-"`
}

func (c *IsNullCondition) IsFilterCondition() {}

// IsEmptyCondition checks if a field is empty (doesn't exist, null, or []).
// SQL equivalent: WHERE field IS NULL OR field = ” OR field = []
type IsEmptyCondition struct {
	Field     string    `json:"field"`
	FieldType FieldType `json:"-"`
}

func (c *IsEmptyCondition) IsFilterCondition() {}

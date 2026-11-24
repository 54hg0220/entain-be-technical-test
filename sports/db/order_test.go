package db

import (
	"errors"
	"testing"
)

func TestBuildOrderBy(t *testing.T) {
	tests := []struct {
		name    string
		orderBy string
		want    string
	}{
		{name: "empty uses default", orderBy: "", want: " ORDER BY advertised_start_time ASC, id ASC"},
		{name: "whitespace uses default", orderBy: "   ", want: " ORDER BY advertised_start_time ASC, id ASC"},
		{name: "single field", orderBy: "name", want: " ORDER BY name ASC, id ASC"},
		{name: "desc", orderBy: "advertised_start_time desc", want: " ORDER BY advertised_start_time DESC, id ASC"},
		{name: "direction is case insensitive", orderBy: "sport DESC", want: " ORDER BY sport DESC, id ASC"},
		{name: "multiple fields with extra whitespace", orderBy: " sport ,  competition desc ", want: " ORDER BY sport ASC, competition DESC, id ASC"},
		{name: "id is not duplicated", orderBy: "id desc", want: " ORDER BY id DESC"},
		{name: "id may appear before other fields", orderBy: "id, name", want: " ORDER BY id ASC, name ASC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildOrderBy(tt.orderBy)
			if err != nil {
				t.Fatalf("buildOrderBy(%q) error = %v", tt.orderBy, err)
			}
			if got != tt.want {
				t.Errorf("buildOrderBy(%q) = %q, want %q", tt.orderBy, got, tt.want)
			}
		})
	}
}

func TestBuildOrderBy_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		orderBy string
	}{
		{name: "unknown field", orderBy: "colour"},
		{name: "unknown direction", orderBy: "name sideways"},
		{name: "too many tokens", orderBy: "name desc nulls"},
		{name: "empty term", orderBy: "name,,sport"},
		{name: "trailing comma", orderBy: "name,"},
		{name: "duplicate field", orderBy: "name, name desc"},
		{name: "SQL injection attempt", orderBy: "id; DROP TABLE events"},
		{name: "field names are case sensitive", orderBy: "NAME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := buildOrderBy(tt.orderBy); !errors.Is(err, ErrInvalidOrderBy) {
				t.Errorf("buildOrderBy(%q) error = %v, want ErrInvalidOrderBy", tt.orderBy, err)
			}
		})
	}
}

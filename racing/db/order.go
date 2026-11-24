package db

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidOrderBy is returned when a requested order_by cannot be parsed or
// references a field that races cannot be ordered by.
var ErrInvalidOrderBy = errors.New("invalid order_by")

// defaultRaceOrderBy is applied when the caller does not specify an order.
const defaultRaceOrderBy = "advertised_start_time"

// raceOrderColumns maps the API field names callers may order by to their
// database columns. Only fields in this allow-list ever reach the SQL, which
// is what keeps the ORDER BY clause safe from injection.
var raceOrderColumns = map[string]string{
	"id":                    "id",
	"meeting_id":            "meeting_id",
	"name":                  "name",
	"number":                "number",
	"visible":               "visible",
	"advertised_start_time": "advertised_start_time",
}

// buildOrderBy converts an AIP-132 style order_by string such as
// "meeting_id, advertised_start_time desc" into an SQL ORDER BY clause.
//
// An empty orderBy falls back to defaultRaceOrderBy. The race id is always
// appended as a final tie-breaker (unless already present) so that results
// are returned in a deterministic order.
func buildOrderBy(orderBy string) (string, error) {
	if strings.TrimSpace(orderBy) == "" {
		orderBy = defaultRaceOrderBy
	}

	var (
		terms []string
		seen  = make(map[string]bool)
		parts = strings.Split(orderBy, ",")
	)

	for _, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 || len(fields) > 2 {
			return "", fmt.Errorf("%w: malformed term %q", ErrInvalidOrderBy, strings.TrimSpace(part))
		}

		column, ok := raceOrderColumns[fields[0]]
		if !ok {
			return "", fmt.Errorf("%w: unsupported field %q", ErrInvalidOrderBy, fields[0])
		}

		if seen[column] {
			return "", fmt.Errorf("%w: field %q specified more than once", ErrInvalidOrderBy, fields[0])
		}
		seen[column] = true

		direction := "ASC"
		if len(fields) == 2 {
			switch strings.ToLower(fields[1]) {
			case "asc":
				// Already the default.
			case "desc":
				direction = "DESC"
			default:
				return "", fmt.Errorf("%w: unsupported direction %q for field %q", ErrInvalidOrderBy, fields[1], fields[0])
			}
		}

		terms = append(terms, column+" "+direction)
	}

	if !seen["id"] {
		terms = append(terms, "id ASC")
	}

	return " ORDER BY " + strings.Join(terms, ", "), nil
}

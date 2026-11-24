// Package db provides SQLite-backed storage for sports events.
package db

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/types/known/timestamppb"

	"git.neds.sh/matty/entain/sports/proto/sports"
)

// EventsRepo provides repository access to sports events.
type EventsRepo interface {
	// Init initialises the events repository, seeding it on first use.
	Init() error

	// List returns the events matching the filter, ordered by
	// orderBy (see ListEventsRequest.order_by). It returns an error wrapping
	// ErrInvalidOrderBy if orderBy is not valid.
	List(filter *sports.ListEventsRequestFilter, orderBy string) ([]*sports.Event, error)
}

type eventsRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewEventsRepo creates a new events repository.
func NewEventsRepo(db *sql.DB) EventsRepo {
	return &eventsRepo{db: db}
}

// Init prepares the events repository dummy data.
func (r *eventsRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy events.
		err = r.seed()
	})

	return err
}

func (r *eventsRepo) List(filter *sports.ListEventsRequestFilter, orderBy string) ([]*sports.Event, error) {
	// Validate the ordering before touching the database.
	orderClause, err := buildOrderBy(orderBy)
	if err != nil {
		return nil, err
	}

	query := getEventQueries()[eventsList]

	query, args := r.applyFilter(query, filter)

	query += orderClause

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

func (r *eventsRepo) applyFilter(query string, filter *sports.ListEventsRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.Sports) > 0 {
		clauses = append(clauses, "sport IN ("+strings.Repeat("?,", len(filter.Sports)-1)+"?)")

		for _, sport := range filter.Sports {
			args = append(args, sport)
		}
	}

	// Visible is optional so that "not set" (all events) can be told apart
	// from "false" (hidden events only).
	if filter.Visible != nil {
		clauses = append(clauses, "visible = ?")
		args = append(args, filter.GetVisible())
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}

func (r *eventsRepo) scanEvents(rows *sql.Rows) ([]*sports.Event, error) {
	var events []*sports.Event

	for rows.Next() {
		var event sports.Event
		var advertisedStart time.Time

		if err := rows.Scan(&event.Id, &event.Name, &event.Sport, &event.Competition, &event.Visible, &advertisedStart); err != nil {
			return nil, err
		}

		event.AdvertisedStartTime = timestamppb.New(advertisedStart)

		events = append(events, &event)
	}

	// rows.Next returns false on iteration errors as well as at the end of
	// the result set; only rows.Err tells them apart.
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

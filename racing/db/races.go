package db

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"

	"git.neds.sh/matty/entain/racing/proto/racing"
)

// RacesRepo provides repository access to races.
type RacesRepo interface {
	// Init initialises the races repository, seeding it on first use.
	Init() error

	// List returns the races matching the filter, ordered by
	// orderBy (see ListRacesRequest.order_by). It returns an error wrapping
	// ErrInvalidOrderBy if orderBy is not valid.
	List(filter *racing.ListRacesRequestFilter, orderBy string) ([]*racing.Race, error)

	// Get returns the race with the given ID, or ErrNotFound if there is no
	// such race.
	Get(id int64) (*racing.Race, error)
}

// ErrNotFound is returned when a requested race does not exist.
var ErrNotFound = errors.New("race not found")

type racesRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewRacesRepo creates a new races repository.
func NewRacesRepo(db *sql.DB) RacesRepo {
	return &racesRepo{db: db}
}

// Init prepares the race repository dummy data.
func (r *racesRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy races.
		err = r.seed()
	})

	return err
}

func (r *racesRepo) List(filter *racing.ListRacesRequestFilter, orderBy string) ([]*racing.Race, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	// Validate the ordering before touching the database.
	orderClause, err := buildOrderBy(orderBy)
	if err != nil {
		return nil, err
	}

	query = getRaceQueries()[racesList]

	query, args = r.applyFilter(query, filter)

	query += orderClause

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRaces(rows)
}

func (r *racesRepo) Get(id int64) (*racing.Race, error) {
	query := getRaceQueries()[racesList] + " WHERE id = ?"

	rows, err := r.db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Reuse scanRaces so a race is built identically whether it is listed or
	// fetched on its own; id is the primary key so at most one row matches.
	races, err := r.scanRaces(rows)
	if err != nil {
		return nil, err
	}

	if len(races) == 0 {
		return nil, ErrNotFound
	}

	return races[0], nil
}

func (r *racesRepo) applyFilter(query string, filter *racing.ListRacesRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.MeetingIds) > 0 {
		clauses = append(clauses, "meeting_id IN ("+strings.Repeat("?,", len(filter.MeetingIds)-1)+"?)")

		for _, meetingID := range filter.MeetingIds {
			args = append(args, meetingID)
		}
	}

	// Visible is optional so that "not set" (all races) can be told apart from
	// "false" (hidden races only).
	if filter.Visible != nil {
		clauses = append(clauses, "visible = ?")
		args = append(args, filter.GetVisible())
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}

func (r *racesRepo) scanRaces(
	rows *sql.Rows,
) ([]*racing.Race, error) {
	var races []*racing.Race

	for rows.Next() {
		var race racing.Race
		var advertisedStart time.Time

		if err := rows.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}

			return nil, err
		}

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, err
		}

		race.AdvertisedStartTime = ts

		races = append(races, &race)
	}

	// rows.Next returns false on iteration errors as well as at the end of
	// the result set; only rows.Err tells them apart.
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return races, nil
}

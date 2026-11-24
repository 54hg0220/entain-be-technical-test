package db

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"git.neds.sh/matty/entain/sports/proto/sports"
)

// testEvent is a row inserted into the test database.
type testEvent struct {
	id              int64
	sport           string
	visible         bool
	advertisedStart time.Time
}

// newTestDB returns a fresh, empty in-memory SQLite database.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Every new connection to ":memory:" gets its own empty database, so pin
	// the pool to a single connection.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// newTestRepo returns an eventsRepo backed by an in-memory database
// containing only the given events.
func newTestRepo(t *testing.T, events []testEvent) *eventsRepo {
	t.Helper()

	db := newTestDB(t)

	if _, err := db.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY, name TEXT, sport TEXT, competition TEXT, visible INTEGER, advertised_start_time DATETIME)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, e := range events {
		if _, err := db.Exec(
			`INSERT INTO events(id, name, sport, competition, visible, advertised_start_time) VALUES (?,?,?,?,?,?)`,
			e.id, "Home vs Away", e.sport, e.sport+" League", e.visible, e.advertisedStart.Format(time.RFC3339),
		); err != nil {
			t.Fatalf("insert event %d: %v", e.id, err)
		}
	}

	return &eventsRepo{db: db}
}

func eventIDs(events []*sports.Event) []int64 {
	ids := make([]int64, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.Id)
	}
	return ids
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEventsRepo_List_Filter(t *testing.T) {
	start := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)
	repo := newTestRepo(t, []testEvent{
		{id: 1, sport: "AFL", visible: true, advertisedStart: start},
		{id: 2, sport: "AFL", visible: false, advertisedStart: start},
		{id: 3, sport: "Soccer", visible: true, advertisedStart: start},
		{id: 4, sport: "Cricket", visible: false, advertisedStart: start},
	})

	tests := []struct {
		name    string
		filter  *sports.ListEventsRequestFilter
		wantIDs []int64
	}{
		{name: "nil filter returns all events", filter: nil, wantIDs: []int64{1, 2, 3, 4}},
		{name: "empty filter returns all events", filter: &sports.ListEventsRequestFilter{}, wantIDs: []int64{1, 2, 3, 4}},
		{name: "visible true", filter: &sports.ListEventsRequestFilter{Visible: proto.Bool(true)}, wantIDs: []int64{1, 3}},
		{name: "visible false", filter: &sports.ListEventsRequestFilter{Visible: proto.Bool(false)}, wantIDs: []int64{2, 4}},
		{name: "sports", filter: &sports.ListEventsRequestFilter{Sports: []string{"AFL", "Cricket"}}, wantIDs: []int64{1, 2, 4}},
		{name: "unknown sport matches nothing", filter: &sports.ListEventsRequestFilter{Sports: []string{"Curling"}}, wantIDs: []int64{}},
		{
			name:    "sports combined with visible",
			filter:  &sports.ListEventsRequestFilter{Sports: []string{"AFL", "Cricket"}, Visible: proto.Bool(true)},
			wantIDs: []int64{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := repo.List(context.Background(), tt.filter, "")
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			// Ordering is not under test here, only which events match.
			got := eventIDs(events)
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

			if !equalIDs(got, tt.wantIDs) {
				t.Errorf("List() ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestEventsRepo_List_Order(t *testing.T) {
	base := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)
	repo := newTestRepo(t, []testEvent{
		{id: 1, sport: "Soccer", visible: true, advertisedStart: base.Add(3 * time.Hour)},
		{id: 2, sport: "AFL", visible: true, advertisedStart: base.Add(1 * time.Hour)},
		{id: 3, sport: "Soccer", visible: false, advertisedStart: base.Add(2 * time.Hour)},
		// Same start time as event 2, to exercise the id tie-breaker.
		{id: 4, sport: "AFL", visible: true, advertisedStart: base.Add(1 * time.Hour)},
	})

	tests := []struct {
		name    string
		orderBy string
		wantIDs []int64
	}{
		{name: "defaults to advertised_start_time with id tie-breaker", wantIDs: []int64{2, 4, 3, 1}},
		{name: "descending", orderBy: "advertised_start_time desc", wantIDs: []int64{1, 3, 2, 4}},
		{name: "multiple fields", orderBy: "sport desc, advertised_start_time", wantIDs: []int64{3, 1, 2, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := repo.List(context.Background(), nil, tt.orderBy)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if got := eventIDs(events); !equalIDs(got, tt.wantIDs) {
				t.Errorf("List() ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestEventsRepo_List_InvalidOrder(t *testing.T) {
	repo := newTestRepo(t, nil)

	if _, err := repo.List(context.Background(), nil, "meeting_id"); !errors.Is(err, ErrInvalidOrderBy) {
		t.Fatalf("List() error = %v, want ErrInvalidOrderBy", err)
	}
}

func TestEventsRepo_Init_Seeds(t *testing.T) {
	repo := &eventsRepo{db: newTestDB(t)}

	if err := repo.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	events, err := repo.List(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(events) != 100 {
		t.Fatalf("seeded %d events, want 100", len(events))
	}

	for _, e := range events {
		competition, ok := seedCompetitions[e.Sport]
		if !ok {
			t.Errorf("event %d has unknown sport %q", e.Id, e.Sport)
		}
		if e.Competition != competition {
			t.Errorf("event %d competition = %q, want %q", e.Id, e.Competition, competition)
		}
		if e.Name == "" || e.AdvertisedStartTime == nil {
			t.Errorf("event %d is missing a name or start time: %+v", e.Id, e)
		}
	}
}

func TestEventsRepo_List_CancelledContext(t *testing.T) {
	repo := newTestRepo(t, []testEvent{{id: 1, sport: "AFL", advertisedStart: time.Now()}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repo.List(ctx, nil, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("List() error = %v, want context.Canceled", err)
	}
}

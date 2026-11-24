package db

import (
	"database/sql"
	"sort"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"git.neds.sh/matty/entain/racing/proto/racing"
)

// testRace is a row inserted into the test database.
type testRace struct {
	id              int64
	meetingID       int64
	visible         bool
	advertisedStart time.Time
}

// newTestRepo returns a racesRepo backed by a fresh in-memory SQLite database
// containing only the given races.
func newTestRepo(t *testing.T, races []testRace) *racesRepo {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Every new connection to ":memory:" gets its own empty database, so pin
	// the pool to a single connection.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE races (id INTEGER PRIMARY KEY, meeting_id INTEGER, name TEXT, number INTEGER, visible INTEGER, advertised_start_time DATETIME)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, r := range races {
		if _, err := db.Exec(
			`INSERT INTO races(id, meeting_id, name, number, visible, advertised_start_time) VALUES (?,?,?,?,?,?)`,
			r.id, r.meetingID, "race", 1, r.visible, r.advertisedStart.Format(time.RFC3339),
		); err != nil {
			t.Fatalf("insert race %d: %v", r.id, err)
		}
	}

	return &racesRepo{db: db}
}

func raceIDs(races []*racing.Race) []int64 {
	ids := make([]int64, 0, len(races))
	for _, r := range races {
		ids = append(ids, r.Id)
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

func TestRacesRepo_List_Filter(t *testing.T) {
	start := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)
	repo := newTestRepo(t, []testRace{
		{id: 1, meetingID: 1, visible: true, advertisedStart: start},
		{id: 2, meetingID: 1, visible: false, advertisedStart: start},
		{id: 3, meetingID: 2, visible: true, advertisedStart: start},
		{id: 4, meetingID: 3, visible: false, advertisedStart: start},
	})

	tests := []struct {
		name    string
		filter  *racing.ListRacesRequestFilter
		wantIDs []int64
	}{
		{
			name:    "nil filter returns all races",
			filter:  nil,
			wantIDs: []int64{1, 2, 3, 4},
		},
		{
			name:    "empty filter returns all races regardless of visibility",
			filter:  &racing.ListRacesRequestFilter{},
			wantIDs: []int64{1, 2, 3, 4},
		},
		{
			name:    "visible true returns only visible races",
			filter:  &racing.ListRacesRequestFilter{Visible: proto.Bool(true)},
			wantIDs: []int64{1, 3},
		},
		{
			name:    "visible false returns only hidden races",
			filter:  &racing.ListRacesRequestFilter{Visible: proto.Bool(false)},
			wantIDs: []int64{2, 4},
		},
		{
			name:    "meeting ids only",
			filter:  &racing.ListRacesRequestFilter{MeetingIds: []int64{1, 3}},
			wantIDs: []int64{1, 2, 4},
		},
		{
			name: "meeting ids combined with visible",
			filter: &racing.ListRacesRequestFilter{
				MeetingIds: []int64{1, 3},
				Visible:    proto.Bool(true),
			},
			wantIDs: []int64{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			races, err := repo.List(tt.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			// Ordering is not under test here, only which races match.
			got := raceIDs(races)
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

			if !equalIDs(got, tt.wantIDs) {
				t.Errorf("List() ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"git.neds.sh/matty/entain/racing/db"
	"git.neds.sh/matty/entain/racing/proto/racing"
)

// fakeRacesRepo is an in-memory db.RacesRepo that records the arguments it
// was called with and returns canned results.
type fakeRacesRepo struct {
	races []*racing.Race
	err   error

	gotFilter  *racing.ListRacesRequestFilter
	gotOrderBy string
	gotID      int64
}

func (f *fakeRacesRepo) Init() error { return nil }

func (f *fakeRacesRepo) List(filter *racing.ListRacesRequestFilter, orderBy string) ([]*racing.Race, error) {
	f.gotFilter = filter
	f.gotOrderBy = orderBy
	return f.races, f.err
}

// Get returns the first canned race, or ErrNotFound if there are none.
func (f *fakeRacesRepo) Get(id int64) (*racing.Race, error) {
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	if len(f.races) == 0 {
		return nil, db.ErrNotFound
	}
	return f.races[0], nil
}

func TestRacingService_ListRaces(t *testing.T) {
	repo := &fakeRacesRepo{races: []*racing.Race{{Id: 1}, {Id: 2}}}
	svc := NewRacingService(repo)

	req := &racing.ListRacesRequest{
		Filter:  &racing.ListRacesRequestFilter{Visible: proto.Bool(true)},
		OrderBy: "name desc",
	}

	resp, err := svc.ListRaces(context.Background(), req)
	if err != nil {
		t.Fatalf("ListRaces() error = %v", err)
	}

	if len(resp.Races) != 2 {
		t.Errorf("ListRaces() returned %d races, want 2", len(resp.Races))
	}
	if repo.gotFilter != req.Filter {
		t.Errorf("repo received filter %v, want %v", repo.gotFilter, req.Filter)
	}
	if repo.gotOrderBy != req.OrderBy {
		t.Errorf("repo received orderBy %q, want %q", repo.gotOrderBy, req.OrderBy)
	}
}

func TestRacingService_ListRaces_Errors(t *testing.T) {
	tests := []struct {
		name     string
		repoErr  error
		wantCode codes.Code
	}{
		{
			name:     "invalid order_by maps to InvalidArgument",
			repoErr:  fmt.Errorf("%w: unsupported field %q", db.ErrInvalidOrderBy, "colour"),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "other errors are not reported as caller errors",
			repoErr:  errors.New("database is locked"),
			wantCode: codes.Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewRacingService(&fakeRacesRepo{err: tt.repoErr})

			_, err := svc.ListRaces(context.Background(), &racing.ListRacesRequest{})
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("ListRaces() code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}
		})
	}
}

func TestRaceStatus(t *testing.T) {
	now := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		start *timestamppb.Timestamp
		want  racing.Race_Status
	}{
		{name: "start in the future is open", start: timestamppb.New(now.Add(time.Second)), want: racing.Race_OPEN},
		{name: "start in the past is closed", start: timestamppb.New(now.Add(-time.Second)), want: racing.Race_CLOSED},
		{name: "start exactly now is closed", start: timestamppb.New(now), want: racing.Race_CLOSED},
		{name: "missing start time is unspecified", start: nil, want: racing.Race_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := raceStatus(&racing.Race{AdvertisedStartTime: tt.start}, now)
			if got != tt.want {
				t.Errorf("raceStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRacingService_ListRaces_Status(t *testing.T) {
	now := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)
	repo := &fakeRacesRepo{races: []*racing.Race{
		{Id: 1, AdvertisedStartTime: timestamppb.New(now.Add(-time.Hour))},
		{Id: 2, AdvertisedStartTime: timestamppb.New(now.Add(time.Hour))},
	}}
	svc := &racingService{racesRepo: repo, now: func() time.Time { return now }}

	resp, err := svc.ListRaces(context.Background(), &racing.ListRacesRequest{})
	if err != nil {
		t.Fatalf("ListRaces() error = %v", err)
	}

	want := map[int64]racing.Race_Status{1: racing.Race_CLOSED, 2: racing.Race_OPEN}
	for _, race := range resp.Races {
		if race.Status != want[race.Id] {
			t.Errorf("race %d status = %v, want %v", race.Id, race.Status, want[race.Id])
		}
	}
}

func TestRacingService_GetRace(t *testing.T) {
	now := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)
	repo := &fakeRacesRepo{races: []*racing.Race{
		{Id: 7, AdvertisedStartTime: timestamppb.New(now.Add(time.Hour))},
	}}
	svc := &racingService{racesRepo: repo, now: func() time.Time { return now }}

	race, err := svc.GetRace(context.Background(), &racing.GetRaceRequest{Id: 7})
	if err != nil {
		t.Fatalf("GetRace() error = %v", err)
	}

	if repo.gotID != 7 {
		t.Errorf("repo received id %d, want 7", repo.gotID)
	}
	if race.Id != 7 {
		t.Errorf("GetRace() id = %d, want 7", race.Id)
	}
	if race.Status != racing.Race_OPEN {
		t.Errorf("GetRace() status = %v, want %v", race.Status, racing.Race_OPEN)
	}
}

func TestRacingService_GetRace_Errors(t *testing.T) {
	tests := []struct {
		name     string
		id       int64
		repo     *fakeRacesRepo
		wantCode codes.Code
	}{
		{name: "zero id is invalid", id: 0, repo: &fakeRacesRepo{}, wantCode: codes.InvalidArgument},
		{name: "negative id is invalid", id: -1, repo: &fakeRacesRepo{}, wantCode: codes.InvalidArgument},
		{name: "missing race is not found", id: 1, repo: &fakeRacesRepo{}, wantCode: codes.NotFound},
		{name: "other errors are not reported as caller errors", id: 1, repo: &fakeRacesRepo{err: errors.New("database is locked")}, wantCode: codes.Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRacingService(tt.repo).GetRace(context.Background(), &racing.GetRaceRequest{Id: tt.id})
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("GetRace() code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}
		})
	}
}

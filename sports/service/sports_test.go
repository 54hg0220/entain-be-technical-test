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

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
)

// fakeEventsRepo is an in-memory db.EventsRepo that records the arguments it
// was called with and returns canned results.
type fakeEventsRepo struct {
	events []*sports.Event
	err    error

	gotFilter  *sports.ListEventsRequestFilter
	gotOrderBy string
}

func (f *fakeEventsRepo) Init() error { return nil }

func (f *fakeEventsRepo) List(_ context.Context, filter *sports.ListEventsRequestFilter, orderBy string) ([]*sports.Event, error) {
	f.gotFilter = filter
	f.gotOrderBy = orderBy
	return f.events, f.err
}

func TestSportsService_ListEvents(t *testing.T) {
	now := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)
	repo := &fakeEventsRepo{events: []*sports.Event{
		{Id: 1, AdvertisedStartTime: timestamppb.New(now.Add(-time.Hour))},
		{Id: 2, AdvertisedStartTime: timestamppb.New(now.Add(time.Hour))},
	}}
	svc := &sportsService{eventsRepo: repo, now: func() time.Time { return now }}

	req := &sports.ListEventsRequest{
		Filter:  &sports.ListEventsRequestFilter{Sports: []string{"AFL"}, Visible: proto.Bool(true)},
		OrderBy: "name desc",
	}

	resp, err := svc.ListEvents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}

	if repo.gotFilter != req.Filter {
		t.Errorf("repo received filter %v, want %v", repo.gotFilter, req.Filter)
	}
	if repo.gotOrderBy != req.OrderBy {
		t.Errorf("repo received orderBy %q, want %q", repo.gotOrderBy, req.OrderBy)
	}

	want := map[int64]sports.Event_Status{1: sports.Event_CLOSED, 2: sports.Event_OPEN}
	if len(resp.Events) != len(want) {
		t.Fatalf("ListEvents() returned %d events, want %d", len(resp.Events), len(want))
	}
	for _, event := range resp.Events {
		if event.Status != want[event.Id] {
			t.Errorf("event %d status = %v, want %v", event.Id, event.Status, want[event.Id])
		}
	}
}

func TestSportsService_ListEvents_Errors(t *testing.T) {
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
			svc := NewSportsService(&fakeEventsRepo{err: tt.repoErr})

			_, err := svc.ListEvents(context.Background(), &sports.ListEventsRequest{})
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("ListEvents() code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}
		})
	}
}

func TestEventStatus(t *testing.T) {
	now := time.Date(2021, 3, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		start *timestamppb.Timestamp
		want  sports.Event_Status
	}{
		{name: "start in the future is open", start: timestamppb.New(now.Add(time.Second)), want: sports.Event_OPEN},
		{name: "start in the past is closed", start: timestamppb.New(now.Add(-time.Second)), want: sports.Event_CLOSED},
		{name: "start exactly now is closed", start: timestamppb.New(now), want: sports.Event_CLOSED},
		{name: "missing start time is unspecified", start: nil, want: sports.Event_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventStatus(&sports.Event{AdvertisedStartTime: tt.start}, now); got != tt.want {
				t.Errorf("eventStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

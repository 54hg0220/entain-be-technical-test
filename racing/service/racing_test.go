package service

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

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
}

func (f *fakeRacesRepo) Init() error { return nil }

func (f *fakeRacesRepo) List(filter *racing.ListRacesRequestFilter, orderBy string) ([]*racing.Race, error) {
	f.gotFilter = filter
	f.gotOrderBy = orderBy
	return f.races, f.err
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

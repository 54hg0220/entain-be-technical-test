package service

import (
	"errors"
	"time"

	"git.neds.sh/matty/entain/racing/db"
	"git.neds.sh/matty/entain/racing/proto/racing"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Racing serves the racing gRPC API.
type Racing interface {
	// ListRaces returns the races matching the request.
	ListRaces(ctx context.Context, in *racing.ListRacesRequest) (*racing.ListRacesResponse, error)

	// GetRace returns a single race by its ID.
	GetRace(ctx context.Context, in *racing.GetRaceRequest) (*racing.Race, error)
}

// racingService implements the Racing interface.
type racingService struct {
	racesRepo db.RacesRepo
	// now returns the current time. It is a field so tests can fix the clock.
	now func() time.Time
}

// NewRacingService instantiates and returns a new racingService.
func NewRacingService(racesRepo db.RacesRepo) Racing {
	return &racingService{racesRepo: racesRepo, now: time.Now}
}

func (s *racingService) ListRaces(ctx context.Context, in *racing.ListRacesRequest) (*racing.ListRacesResponse, error) {
	races, err := s.racesRepo.List(in.Filter, in.OrderBy)
	if err != nil {
		// A bad order_by is the caller's fault, so surface it as
		// InvalidArgument (HTTP 400 via the gateway) rather than Unknown.
		if errors.Is(err, db.ErrInvalidOrderBy) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, err
	}

	// Status depends on the current time, so it is derived on every read
	// rather than stored alongside the race.
	now := s.now()
	for _, race := range races {
		race.Status = raceStatus(race, now)
	}

	return &racing.ListRacesResponse{Races: races}, nil
}

func (s *racingService) GetRace(ctx context.Context, in *racing.GetRaceRequest) (*racing.Race, error) {
	// Race IDs are SQLite integer primary keys, which start at 1.
	if in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive integer")
	}

	race, err := s.racesRepo.Get(in.Id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "race %d not found", in.Id)
		}
		return nil, err
	}

	race.Status = raceStatus(race, s.now())

	return race, nil
}

// raceStatus reports whether a race is OPEN or CLOSED at the given time. A
// race closes as soon as its advertised start time is reached.
func raceStatus(race *racing.Race, now time.Time) racing.Race_Status {
	if race.AdvertisedStartTime == nil {
		return racing.Race_STATUS_UNSPECIFIED
	}

	if now.Before(race.AdvertisedStartTime.AsTime()) {
		return racing.Race_OPEN
	}

	return racing.Race_CLOSED
}

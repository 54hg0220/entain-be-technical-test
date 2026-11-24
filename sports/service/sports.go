// Package service implements the sports gRPC API.
package service

import (
	"errors"
	"time"

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Sports serves the sports gRPC API.
type Sports interface {
	// ListEvents returns the sports events matching the request.
	ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error)
}

// sportsService implements the Sports interface.
type sportsService struct {
	eventsRepo db.EventsRepo
	// now returns the current time. It is a field so tests can fix the clock.
	now func() time.Time
}

// NewSportsService instantiates and returns a new sportsService.
func NewSportsService(eventsRepo db.EventsRepo) Sports {
	return &sportsService{eventsRepo: eventsRepo, now: time.Now}
}

func (s *sportsService) ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error) {
	// Pass the request context down so a cancelled or timed-out RPC also
	// stops its database query.
	events, err := s.eventsRepo.List(ctx, in.Filter, in.OrderBy)
	if err != nil {
		// A bad order_by is the caller's fault, so surface it as
		// InvalidArgument (HTTP 400 via the gateway) rather than Unknown.
		if errors.Is(err, db.ErrInvalidOrderBy) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, err
	}

	// Status depends on the current time, so it is derived on every read
	// rather than stored alongside the event.
	now := s.now()
	for _, event := range events {
		event.Status = eventStatus(event, now)
	}

	return &sports.ListEventsResponse{Events: events}, nil
}

// eventStatus reports whether an event is OPEN or CLOSED at the given time.
// An event closes as soon as its advertised start time is reached.
func eventStatus(event *sports.Event, now time.Time) sports.Event_Status {
	if event.AdvertisedStartTime == nil {
		return sports.Event_STATUS_UNSPECIFIED
	}

	if now.Before(event.AdvertisedStartTime.AsTime()) {
		return sports.Event_OPEN
	}

	return sports.Event_CLOSED
}

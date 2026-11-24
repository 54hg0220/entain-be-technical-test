package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"git.neds.sh/matty/entain/api/proto/racing"
	"git.neds.sh/matty/entain/api/proto/sports"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

var (
	apiEndpoint = flag.String("api-endpoint", "localhost:8000", "API endpoint")
	// grpcEndpoint keeps its original name for backwards compatibility; it
	// is the racing service's endpoint.
	grpcEndpoint       = flag.String("grpc-endpoint", "localhost:9000", "racing gRPC server endpoint")
	sportsGRPCEndpoint = flag.String("sports-grpc-endpoint", "localhost:9001", "sports gRPC server endpoint")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Printf("failed running api server: %s\n", err)
	}
}

func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Each backend service is registered on the same mux; the gateway routes
	// each HTTP request to the service that owns its path.
	mux := runtime.NewServeMux()
	if err := racing.RegisterRacingHandlerFromEndpoint(
		ctx,
		mux,
		*grpcEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return err
	}

	if err := sports.RegisterSportsHandlerFromEndpoint(
		ctx,
		mux,
		*sportsGRPCEndpoint,
		[]grpc.DialOption{grpc.WithInsecure()},
	); err != nil {
		return err
	}

	log.Printf("API server listening on: %s\n", *apiEndpoint)

	return http.ListenAndServe(*apiEndpoint, mux)
}

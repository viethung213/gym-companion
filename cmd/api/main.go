package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"google.golang.org/grpc"
)

func main() {
	httpPort := os.Getenv("APP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9090"
	}

	// 1. Start gRPC Server in a goroutine
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen for gRPC on port %s: %v", grpcPort, err)
	}
	grpcServer := grpc.NewServer()
	fmt.Printf("Starting gRPC server on port %s...\n", grpcPort)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// 2. Start HTTP Server
	fmt.Printf("Starting HTTP API server on port %s...\n", httpPort)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
		log.Fatalf("HTTP server failed to start: %v", err)
	}
}

package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
)

const (
	apiPort = ":8080"
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")

	issueToken = os.Getenv("PKIISSUETOKEN")

	trainStartID = 4000
	trainCount   = 0
)

func main() {
	log.Printf("Starting Train API server (version: %s)\n", Version)
	if Environment == "" {
		Environment = "DEV"
	}

	// check if the issue token is set
	if issueToken == "" {
		log.Fatal("ISSUETOKEN environment variable is not set.")
	}

	// start the HTTP server
	startHTTPServer()
}

func startHTTPServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		versionInfo := fmt.Sprintf("Train API version: %s (%s environment)", Version, Environment)
		w.WriteHeader(http.StatusOK)
		if _, e := w.Write([]byte(versionInfo)); e != nil {
			log.Printf("Failed to write health check response: %v\n", e)
		}
	})

	http.HandleFunc("/spawn", handleSpawnTrain)

	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, e := w.Write([]byte(fmt.Sprintf("Train count: %d", trainCount))); e != nil {
			log.Printf("Failed to write health check response: %v\n", e)
		}
	})

	if e := http.ListenAndServe(apiPort, nil); e != nil {
		log.Fatalf("Failed to start HTTP server: %v", e)
	}
}

// handleSpawnTrain will launch a new train goroutine.
func handleSpawnTrain(w http.ResponseWriter, r *http.Request) {
	// For simplicity, we'll just spawn a train with a random cargo weight.
	trainStartID += 1
	trainID := trainStartID
	cargoWeight := rand.Float64() * 1000.0 // up to 1000 tons of cargo
	go trainRoutine(trainID, cargoWeight)
	trainCount += 1
	if _, e := w.Write([]byte("Train spawned with ID: " + fmt.Sprintf("%d", trainID))); e != nil {
		log.Printf("Failed to write response: %v\n", e)
	}
}

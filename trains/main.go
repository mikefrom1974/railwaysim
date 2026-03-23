package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
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
	validTrains  = make(map[string]bool)
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
		if _, err := w.Write([]byte(versionInfo)); err != nil {
			log.Printf("Failed to write health check response: %v\n", err)
		}
	})

	http.HandleFunc("/spawn", handleSpawnTrain)

	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(fmt.Sprintf("Train count: %d", trainCount))); err != nil {
			log.Printf("Failed to write health check response: %v\n", err)
		}
	})

	if err := http.ListenAndServe(apiPort, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

// handleSpawnTrain will launch a new train goroutine.
func handleSpawnTrain(w http.ResponseWriter, r *http.Request) {
	// read count parameter from query string (optional)
	countParam := r.URL.Query().Get("count")
	if countParam == "" {
		countParam = "1"
	}

	numTrains, err := strconv.Atoi(countParam)
	if err != nil {
		http.Error(w, "Invalid count parameter", http.StatusBadRequest)
		return
	}

	for range numTrains {
		// For simplicity, we'll just spawn a train with a random cargo weight.
		trainStartID += 1
		trainID := trainStartID
		cargoWeight := rand.Float64() * 1000.0 // up to 1000 tons of cargo
		go trainRoutine(trainID, cargoWeight)
		trainCount += 1
		validTrains[fmt.Sprintf("train-%d", trainID)] = true
		if _, err := w.Write([]byte("Train spawned with ID: " + fmt.Sprintf("%d\n", trainID))); err != nil {
			log.Printf("Failed to write response: %v\n", err)
		}
	}
}

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	trainInitialWeight = 1000.0 // tons
	trainMaxSpeed      = 120.0  // km/h
	accelerationRate   = 0.5    // km/h per second
	decelerationRate   = 0.8    // km/h per second

	// normally these would be set via helm values or env vars.
	// remember the docker containers talk via service name and local port (not exposed port).
	pkiLocalEndpoint   = "http://localhost:8080"
	pkiStagingEndpoint = "http://pki-staging:8080"
	pkiProdEndpoint    = "http://pki-prod:8080"
)

type Train struct {
	ID          int
	Weight      float64 // tons, affects acceleration and braking
	Speed       float64 // km/h
	TargetSpeed float64 // km/h
	Position    float64 // km along the track (assuming a single track for simplicity)
	Status      string  // "cruising", "accelerating", "decelerating", "stopped"

	PrivateKey string // PEM-encoded private key for secure communication
	Cert       string // PEM-encoded certificate for secure communication
	CertSerial string // serial number of the cert, for easy reference
}

type Telemetry struct {
	TrainID   int     `json:"train_id"`
	Timestamp int64   `json:"timestamp"`
	Speed     float64 `json:"speed"`
	Position  float64 `json:"position"`
	Status    string  `json:"status"`
}

// trainRoutine simulates the behavior of a train.
// Each train is contained in its own goroutine.
// For this sim, the trains will only go forward and will not interact with each other.
func trainRoutine(id int, cargoWeight float64) {
	train := new(Train)
	train.ID = id
	train.Weight = trainInitialWeight + cargoWeight
	train.Speed = 0.0
	train.Position = 0.0
	train.Status = "stopped"

	// register with the PKI to get a cert for secure communication
	if e := train.registerWithPKI(); e != nil {
		log.Printf("Train %d failed to register with PKI: %v\n", train.ID, e)
	}

	// Simulate train behavior.
	lastTick := time.Now()
	for {
		delta := time.Since(lastTick).Seconds()
		lastTick = time.Now()
		// Update train status based on current speed and target speed.
		if train.Speed < train.TargetSpeed {
			train.Status = "accelerating"
			train.Speed += accelerationRate * (trainInitialWeight / train.Weight) * delta // Heavier trains accelerate slower
			if train.Speed > train.TargetSpeed {
				train.Speed = train.TargetSpeed
			}
			if train.Speed > trainMaxSpeed {
				train.Speed = trainMaxSpeed
			}
		} else if train.Speed > train.TargetSpeed {
			train.Status = "decelerating"
			train.Speed -= decelerationRate * (trainInitialWeight / train.Weight) * delta // Heavier trains decelerate slower
			if train.Speed < train.TargetSpeed {
				train.Speed = train.TargetSpeed
			}
			if train.Speed < 0.0 {
				train.Speed = 0.0
			}
		} else if train.Speed > 0.0 {
			train.Status = "cruising"
		} else {
			train.Status = "stopped"
		}

		// Update position based on speed and direction.
		train.Position += (train.Speed / 3600.0) // Convert km/h to km/s

		// Simulate telemetry reporting.
		if e := train.sendTelemetry(); e != nil {
			log.Printf("Train %d failed to send telemetry: %v\n", train.ID, e)
		}

		// Receive commands from the control system.
		// placeholder for command reception

		// Keep our metrics to ~1 update per second to avoid overwhelming the system.
		time.Sleep(1 * time.Second)
	}
}

func (train *Train) sendTelemetry() error {
	telemetry := Telemetry{
		TrainID:   train.ID,
		Timestamp: time.Now().Unix(),
		Speed:     train.Speed,
		Position:  train.Position,
		Status:    train.Status,
	}
	var telmString string
	if b, e := json.MarshalIndent(telemetry, "", " "); e != nil {
		return fmt.Errorf("failed to marshal telemetry: %v", e)
	} else {
		telmString = string(b)
	}

	// Send the telemetry to the control system.
	// This could be an HTTP POST request, a message to a message queue, etc.
	// For this example, we'll just print it to the console.
	if false {
		log.Printf("Train %d telemetry: \n%s\n", train.ID, telmString)
	}

	return nil
}

func (train *Train) registerWithPKI() error {
	// assign endpoint based on environment variable
	var endpoint string
	switch Environment {
	case "STAGING":
		endpoint = pkiStagingEndpoint
	case "PROD":
		endpoint = pkiProdEndpoint
	default:
		log.Printf("Train %d: '%s' environment, train will not have a valid cert.\n", train.ID, Environment)
		return nil
	}

	// generate the train's private key and CSR
	var csrBytes []byte
	if pk, e := rsa.GenerateKey(rand.Reader, 4096); e != nil {
		return fmt.Errorf("failed to generate private key: %v", e)
	} else {
		template := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName: fmt.Sprintf("train-%d", train.ID),
			},
		}
		if cb, e := x509.CreateCertificateRequest(rand.Reader, template, pk); e != nil {
			return fmt.Errorf("failed to create CSR: %v", e)
		} else {
			csrBytes = cb
			train.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)}))
		}
	}

	// call the PKI API to request a certificate for this train
	var req *http.Request
	if r, e := http.NewRequest(http.MethodPost, endpoint+"/issue", bytes.NewReader(csrBytes)); e != nil {
		return fmt.Errorf("failed to create CSR HTTP request: %v", e)
	} else {
		req = r
	}
	req.Header.Set("X-Auth-Token", issueToken)
	req.Header.Set("X-Cert-Type", "client")
	req.Header.Set("Content-Type", "application/application/octet-stream")

	client := &http.Client{Timeout: 10 * time.Second}
	var resp *http.Response
	if r, e := client.Do(req); e != nil {
		return fmt.Errorf("failed to send CSR HTTP request: %v", e)
	} else {
		resp = r
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respString := ""
		if b, e := io.ReadAll(resp.Body); e == nil {
			respString = string(b)
		}
		return fmt.Errorf("PKI server returned non-OK status: %s\nResponse: %s", resp.Status, respString)
	}

	var certBytes []byte
	if cb, e := io.ReadAll(resp.Body); e != nil {
		return fmt.Errorf("failed to read certificate from PKI response: %v", e)
	} else {
		certBytes = cb
		// Extract the serial number from the certificate
		block, _ := pem.Decode(certBytes)
		if block == nil {
			return fmt.Errorf("failed to decode certificate PEM")
		} else {
			cert, e := x509.ParseCertificate(block.Bytes)
			if e != nil {
				return fmt.Errorf("failed to parse certificate: %v", e)
			}
			train.CertSerial = cert.SerialNumber.String()
		}
	}

	train.Cert = string(certBytes)
	log.Printf("Train %d obtained cert %s.\n", train.ID, train.CertSerial)

	return nil
}

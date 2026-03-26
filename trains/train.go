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
	"os"
	"time"
)

const (
	trainInitialWeight = 1000.0 // tons
	trainMaxSpeed      = 120.0  // km/h
	accelerationRate   = 0.5    // km/h per second
	decelerationRate   = 0.6    // km/h per second
	emergencyBrakeRate = 1.0    // km/h per second

	// normally these would be set via helm values or env vars.
	// remember the docker containers talk via service name and local port (not exposed port).
	pkiLocalEndpoint   = "http://localhost:8080"
	pkiStagingEndpoint = "http://pki-staging:8080"
	pkiProdEndpoint    = "http://pki-prod:8080"
)

var (
	// we are using cert as both authentication AND authorization.
	// this should be secure enough for a demo as the train has to have a valid cert.
	// in a real prod environment we'd use an external auth source
	rabbitMQStagingEndpoint = "rabbit-server-staging:5671"
	rabbitMQProdEndpoint    = "rabbit-server-prod:5671"
)

type (
	Train struct {
		ID          int
		Weight      float64 // tons, affects acceleration and braking
		Speed       float64 // km/h
		TargetSpeed float64 // km/h
		Position    float64 // km along the track (assuming a single track for simplicity)
		Status      string  // "cruising", "accelerating", "decelerating", "emergency_braking", "stopped"

		IsRegistered    bool   // set true when we successfully register with our PKI
		PrivateKey      string // PEM-encoded private key
		CACert          string // PEM-encoded CA certificate
		Cert            string // PEM-encoded certificate
		CertSerial      string // serial number of the cert, for easy reference
		EnableTelemetry bool   // set true to send telemetry data, false to skip
	}

	Telemetry struct {
		TrainID    int     `json:"train_id"`
		Timestamp  int64   `json:"timestamp"`
		Speed      float64 `json:"speed"`
		Position   float64 `json:"position"`
		Status     string  `json:"status"`
		CertSerial string  `json:"cert_serial"`
	}

	MQCommand struct {
		Command      string    `json:"command"`
		ParamsBool   []bool    `json:"params_bool"`
		ParamsInt    []int     `json:"params_int"`
		ParamsFloat  []float64 `json:"params_float"`
		ParamsString []string  `json:"params_string"`
	}
)

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
	train.IsRegistered = false
	train.EnableTelemetry = false

	// wait until we successfully register with the PKI
	for !train.IsRegistered {
		if err := train.registerWithPKI(); err != nil {
			log.Printf("Train %d failed to register with PKI: %v\n", train.ID, err)
		} else {
			train.IsRegistered = true
		}
		time.Sleep(5 * time.Second)
	}

	// connect with rabbit MQ control
	mqURL := "amqps://"
	endpointUser := os.Getenv("RABBITMQ_TRAIN_USER")
	endpointPass := os.Getenv("RABBITMQ_TRAIN_PASS")
	if endpointUser != "" && endpointPass != "" {
		mqURL += fmt.Sprintf("%s:%s@", endpointUser, endpointPass)
	}
	switch Environment {
	case "STAGING":
		mqURL += rabbitMQStagingEndpoint
	case "PROD":
		mqURL += rabbitMQProdEndpoint
	default:
		log.Printf("Train %d: '%s' environment, train will not have a valid rabbit queue .\n", train.ID, Environment)
	}

	rabbit := new(Rabbit)
	cmdChan, err := rabbit.Connect(train.ID, mqURL, train.PrivateKey, train.Cert, train.CACert)
	if err != nil {
		log.Printf("Train %d failed RabbitMQ connection: %s\n", train.ID, err.Error())
	}
	defer rabbit.Close()

	// Simulate train
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	lastTick := time.Now()
	// continuous loop to keep train going
	for {
		select {
		// placeholder for command reception
		case mqd, ok := <-cmdChan:
			if !ok {
				log.Printf("Train %d mq conn lost.\n", train.ID)
				cmdChan = nil
				continue
			}
			train.handleCommand(mqd.Body)

		// process physics and send telemetry when our ticker fires
		case <-ticker.C:
			delta := time.Since(lastTick).Seconds()
			lastTick = time.Now()
			// Update train status based on current speed and target speed.
			train.processPhysics(delta)

			// Update position based on speed and direction.
			train.Position += (train.Speed / 3600.0) // Convert km/h to km/s

			// Simulate telemetry reporting.
			if err := train.sendTelemetry(); err != nil {
				log.Printf("Train %d failed to send telemetry: %v\n", train.ID, err)
			}

			// if our command channel is nil, try to reconnect
			if cmdChan == nil {
				var ccErr error
				cmdChan, ccErr = rabbit.Connect(train.ID, mqURL, train.PrivateKey, train.Cert, train.CACert)
				if ccErr != nil {
					log.Printf("Train %d failed RabbitMQ connection: %s\n", train.ID, ccErr.Error())
				}
			}
		}
	}
}

func (train *Train) handleCommand(body []byte) {
	var cmd MQCommand
	err := json.Unmarshal(body, &cmd)
	if err != nil {
		log.Printf("train %d failure unmarshalling command payload \n%s\n%s\n", train.ID, string(body), err.Error())
		return
	}
	switch cmd.Command {
	case "SET_SPEED":
		if len(cmd.ParamsFloat) > 0 {
			train.TargetSpeed = float64(cmd.ParamsFloat[0])
		} else {
			log.Printf("train %d received SET_SPEED with no params_float", train.ID)
		}
	case "EMERGENCY_STOP":
		train.Status = "emergency_braking"
		train.TargetSpeed = 0.0
	case "ENABLE_TELEMETRY":
		train.EnableTelemetry = true
	case "DISABLE_TELEMETRY":
		train.EnableTelemetry = false
	default:
		log.Printf("Train %d received unknown command: %s\n", train.ID, cmd.Command)
	}
}

func (train *Train) processPhysics(delta float64) {
	if train.Status == "emergency_braking" {
		train.Speed -= emergencyBrakeRate * (trainInitialWeight / train.Weight) * delta
		if train.Speed < 0.0 {
			train.Speed = 0.0
		}
		if train.Speed == 0.0 {
			train.Status = "stopped"
		}
	} else if train.Speed < train.TargetSpeed {
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
}

func (train *Train) sendTelemetry() error {
	if !train.EnableTelemetry {
		return nil
	}
	telemetry := Telemetry{
		TrainID:    train.ID,
		Timestamp:  time.Now().Unix(),
		Speed:      train.Speed,
		Position:   train.Position,
		Status:     train.Status,
		CertSerial: train.CertSerial,
	}
	var telmString string
	if b, err := json.MarshalIndent(telemetry, "", " "); err != nil {
		return fmt.Errorf("failed to marshal telemetry: %v", err)
	} else {
		telmString = string(b)
	}

	// Send the telemetry to the control system.
	// This could be an HTTP POST request, a message to a message queue, etc.
	// For this example, we'll just print it to the console.
	if false {
		log.Printf("Train %d telemetry: \n%s\n", train.ID, telmString)
	}
	log.Printf("%d '%s' s: %.2f / %.2f", train.ID, train.Status, train.Speed, train.TargetSpeed)

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
	if pk, err := rsa.GenerateKey(rand.Reader, 4096); err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	} else {
		template := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName: fmt.Sprintf("train-%d", train.ID),
			},
			DNSNames: []string{fmt.Sprintf("%d", train.ID)},
		}
		if cb, err := x509.CreateCertificateRequest(rand.Reader, template, pk); err != nil {
			return fmt.Errorf("failed to create CSR: %v", err)
		} else {
			csrBytes = cb
			train.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)}))
		}
	}

	// call the PKI API to request a certificate for this train
	req, err := http.NewRequest(http.MethodPost, endpoint+"/issue", bytes.NewReader(csrBytes))
	if err != nil {
		return fmt.Errorf("failed to create CSR HTTP request: %v", err)
	}
	req.Header.Set("X-Auth-Token", issueToken)
	req.Header.Set("X-Cert-Type", "client")
	req.Header.Set("Content-Type", "application/application/octet-stream")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send CSR HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respString := ""
		if b, err := io.ReadAll(resp.Body); err == nil {
			respString = string(b)
		}
		return fmt.Errorf("PKI server returned non-OK status: %s\nResponse: %s", resp.Status, respString)
	}

	certBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read certificate from PKI response: %v", err)
	} else {
		// Extract the serial number from the certificate
		block, _ := pem.Decode(certBytes)
		if block == nil {
			return fmt.Errorf("failed to decode certificate PEM")
		} else {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %v", err)
			}
			train.CertSerial = cert.SerialNumber.String()
		}
	}

	train.Cert = string(certBytes)

	// fetch the CA certificate
	if resp, err := http.Get(endpoint + "/ca"); err != nil {
		return fmt.Errorf("failed to fetch CA certificate: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respString := ""
			if b, err := io.ReadAll(resp.Body); err == nil {
				respString = string(b)
			}
			return fmt.Errorf("PKI server returned non-OK status when fetching CA cert: %s\nResponse: %s", resp.Status, respString)
		}
		caBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate from response: %v", err)
		}
		train.CACert = string(caBytes)
	}

	log.Printf("Train %d obtained cert %s.\n", train.ID, train.CertSerial)

	return nil
}

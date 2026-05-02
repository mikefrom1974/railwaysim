package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	redis "github.com/redis/go-redis/v9"
	kafka "github.com/segmentio/kafka-go"
)

const (
	metricsPort = ":8080"
	serverName  = "exporter"

	pkiLocalEndpoint   = "http://localhost:8080"
	pkiStagingEndpoint = "http://pki-staging:8080"
	pkiProdEndpoint    = "http://pki-prod:8080"

	kafkaServer         = "kafka-server"
	kafkaPort           = "9092"
	kafkaTelemetryTopic = "train-telemetry"

	redisServer = "redis-server"
	redisPort   = "6379"
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")

	issueToken  = os.Getenv("PKIISSUETOKEN")
	pkiEndpoint string
	sanName     string
	cert        *Cert

	kafkaEndpoint string

	redisEndpoint string
	redisPW       string

	// Prometheus metrics formatting
	trainSpeed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "train_speed_kph",
		Help: "Current train speed in KPH",
	}, []string{"train_id"})

	trainPosition = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "train_position_offset",
		Help: "Current train position / offset along track",
	}, []string{"train_id"})

	trainStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "train_status_code",
		Help: "Current train status",
	}, []string{"train_id", "status"})
)

// Telemetry should match the train telemetry payload from train > kafka
type Telemetry struct {
	TrainID     int     `json:"train_id"`
	Timestamp   int64   `json:"timestamp"`
	Speed       float64 `json:"speed"`
	TargetSpeed float64 `json:"target_speed"`
	Position    float64 `json:"position"`
	Status      string  `json:"status"`
	CertSerial  string  `json:"cert_serial"`
}

type Cert struct {
	keyBytes  []byte
	certBytes []byte
	caBytes   []byte
	TLSConfig *tls.Config
}

func main() {
	log.Printf("Starting Exporter (version: %s)\n", Version)
	if Environment == "" {
		Environment = "DEV"
	}

	// assign variables based on environment
	switch Environment {
	case "STAGING":
		pkiEndpoint = pkiStagingEndpoint
		sanName = fmt.Sprintf("%s-staging", serverName)
		kafkaEndpoint = fmt.Sprintf("%s-staging:%s", kafkaServer, kafkaPort)
		redisEndpoint = fmt.Sprintf("%s-staging:%s", redisServer, redisPort)
	case "PROD":
		pkiEndpoint = pkiProdEndpoint
		sanName = fmt.Sprintf("%s-staging", serverName)
		kafkaEndpoint = fmt.Sprintf("%s-prod:%s", kafkaServer, kafkaPort)
		redisEndpoint = fmt.Sprintf("%s-prod:%s", redisServer, redisPort)
	default:
		log.Fatalf("'%s' environment, exporter will not have a valid cert.\n", Environment)
	}

	// read redis pw from env var and unset
	if rpw, ok := os.LookupEnv("REDIS_PASS"); !ok {
		log.Fatalf("'REDIS_PASS' environment variable is not set")
	} else {
		redisPW = rpw
		os.Unsetenv("REDIS_PASS") // unset after reading
	}

	// wait on PKI, then fetch certs
	cert = new(Cert)
	for {
		if err := cert.RegisterWithPKI(); err != nil {
			log.Printf("failed to register with PKI server: %s", err.Error())
		} else {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// start Prometheus scrape endpoint in a goroutine
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Starting Prometheus scrape endpoint on %s\n", metricsPort)
		log.Fatal(http.ListenAndServe(metricsPort, nil))
	}()

	// set up kafka reader
	kDialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
		TLS:     cert.TLSConfig,
	}
	kReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaEndpoint},
		Topic:   kafkaTelemetryTopic,
		GroupID: "train-telemetry-redis",
		Dialer:  kDialer,
	})

	// set up redis client
	rClient := redis.NewClient(&redis.Options{
		Addr:     redisEndpoint,
		Username: os.Getenv("REDIS_USER"),
		Password: redisPW,
		DB:       0,
	})

	log.Println("Exporter connecting to kafka and reading telemetry to redis and prometheus metrics.")
	for {
		// read next available message from kafka
		msg, err := kReader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("failed to read message from kafka: %s\n", err.Error())
			continue
		}

		var telemetry Telemetry
		if err = json.Unmarshal(msg.Value, &telemetry); err != nil {
			log.Printf("failed to unmarshal telemetry message: %s\n", err.Error())
			continue
		}

		// convert train ID to string for Redis keys and Prometheus labels
		idStr := strconv.Itoa(telemetry.TrainID)

		// update Prometheus metrics
		trainSpeed.WithLabelValues(idStr).Set(telemetry.Speed)
		trainPosition.WithLabelValues(idStr).Set(telemetry.Position)
		statuses := []string{"stopped", "accelerating", "cruising", "decelerating", "emergency_braking"}
		for _, s := range statuses {
			if s == telemetry.Status {
				trainStatus.WithLabelValues(idStr, s).Set(1)
			} else {
				trainStatus.WithLabelValues(idStr, s).Set(1)
			}
		}

		// update redis
		rKey := fmt.Sprintf("train:%s", idStr)
		err = rClient.HSet(context.Background(), rKey, map[string]any{
			"speed":        telemetry.Speed,
			"target_speed": telemetry.TargetSpeed,
			"position":     telemetry.Position,
			"status":       telemetry.Status,
			"cert_serial":  telemetry.CertSerial,
			"timestamp":    telemetry.Timestamp,
		}).Err()
		if err != nil {
			log.Printf("failed to update redis with train telemetry: %s\n", err.Error())
		}
	}
}

func (c *Cert) RegisterWithPKI() error {
	// check if the issue token is set
	if issueToken == "" {
		return fmt.Errorf("ISSUETOKEN environment variable is not set")
	}
	// generate private key and CSR
	var csrBytes []byte
	if pk, err := rsa.GenerateKey(rand.Reader, 4096); err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	} else {
		template := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName: serverName,
			},
			DNSNames: []string{sanName},
		}
		if cb, err := x509.CreateCertificateRequest(rand.Reader, template, pk); err != nil {
			return fmt.Errorf("failed to create CSR: %v", err)
		} else {
			csrBytes = cb
			c.keyBytes = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
		}
	}

	// call the PKI API to request a certificate for this service
	req, err := http.NewRequest(http.MethodPost, pkiEndpoint+"/issue", bytes.NewReader(csrBytes))
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

	if c.certBytes, err = io.ReadAll(resp.Body); err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// fetch the CA
	resp, err = http.Get(pkiEndpoint + "/ca")
	if err != nil {
		return fmt.Errorf("failed to fetch CA certificate: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respString := ""
		if b, err := io.ReadAll(resp.Body); err == nil {
			respString = string(b)
		}
		return fmt.Errorf("PKI server returned non-OK status when fetching CA cert: %s\nResponse: %s", resp.Status, respString)
	}
	if c.caBytes, err = io.ReadAll(resp.Body); err != nil {
		return fmt.Errorf("failed to read CA certificate from response: %v", err)
	}

	// create keystore
	keystore, err := tls.X509KeyPair(c.certBytes, c.keyBytes)
	if err != nil {
		return fmt.Errorf("failed to combine key and cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(c.caBytes)
	c.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{keystore},
		RootCAs:      caCertPool,
	}

	return nil
}

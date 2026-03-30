package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

const (
	apiPort    = ":8080"
	serverName = "ingester"

	pkiLocalEndpoint   = "http://localhost:8080"
	pkiStagingEndpoint = "http://pki-staging:8080"
	pkiProdEndpoint    = "http://pki-prod:8080"

	kafkaServer         = "kafka-server"
	kafkaPort           = "9092"
	kafkaTelemetryTopic = "train-telemetry"
)

type (
	Ingester struct {
		ServerName string
		SANName    string

		PKIEndpoint string
		TLSConfig   *tls.Config

		KafkaEndpoint string
		KafkaWriter   *kafka.Writer
	}
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")

	issueToken = os.Getenv("PKIISSUETOKEN")

	ingester *Ingester
)

func main() {
	log.Printf("Starting Ingester (version: %s)\n", Version)
	if Environment == "" {
		Environment = "DEV"
	}

	// check if the issue token is set
	if issueToken == "" {
		log.Fatal("ISSUETOKEN environment variable is not set.")
	}

	ingester = newIngester()
	ingester.KafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(ingester.KafkaEndpoint),
		Topic:        kafkaTelemetryTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Transport: &kafka.Transport{
			TLS: ingester.TLSConfig,
		},
		Async:        false, // don't need concurrency since we're in an http routine
		BatchSize:    1,     // troubleshooting test latency
		BatchTimeout: 10 * time.Second,
	}
	defer ingester.KafkaWriter.Close()

	http.HandleFunc("/ingest", ingester.handleIngest)

	if err := http.ListenAndServe(apiPort, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func newIngester() *Ingester {
	i := new(Ingester)
	// assign endpoints based on environment
	switch Environment {
	case "STAGING":
		i.ServerName = serverName
		i.SANName = fmt.Sprintf("%s-staging", serverName)
		i.PKIEndpoint = pkiStagingEndpoint
		i.KafkaEndpoint = fmt.Sprintf("%s-staging:%s", kafkaServer, kafkaPort)
	case "PROD":
		i.ServerName = serverName
		i.SANName = fmt.Sprintf("%s-prod", serverName)
		i.PKIEndpoint = pkiProdEndpoint
		i.KafkaEndpoint = fmt.Sprintf("%s-prod:%s", kafkaServer, kafkaPort)
	default:
		log.Fatalf("'%s' environment, ingester will not have a valid cert.\n", Environment)
	}

	// fetch certs
	if err := i.registerWithPKI(); err != nil {
		log.Fatalf("error fetching certificates: %s", err.Error())
	}

	return i
}

func (i *Ingester) registerWithPKI() error {
	// generate private key and CSR
	var csrBytes []byte
	var pkBytes []byte
	if pk, err := rsa.GenerateKey(rand.Reader, 4096); err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	} else {
		template := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName: i.ServerName,
			},
			DNSNames: []string{i.SANName},
		}
		if cb, err := x509.CreateCertificateRequest(rand.Reader, template, pk); err != nil {
			return fmt.Errorf("failed to create CSR: %v", err)
		} else {
			csrBytes = cb
			pkBytes = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
		}
	}

	// call the PKI API to request a certificate for this service
	req, err := http.NewRequest(http.MethodPost, i.PKIEndpoint+"/issue", bytes.NewReader(csrBytes))
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
	}

	// fetch the CA
	resp, err = http.Get(i.PKIEndpoint + "/ca")
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
	caBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate from response: %v", err)
	}

	// create keystore
	keystore, err := tls.X509KeyPair(certBytes, pkBytes)
	if err != nil {
		return fmt.Errorf("failed to combine key and cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caBytes)
	i.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{keystore},
		RootCAs:      caCertPool,
	}

	return nil
}

func (i *Ingester) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = i.KafkaWriter.WriteMessages(ctx, kafka.Message{
		Value: payload,
	})
	if err != nil {
		log.Printf("Error producing messages to kafka: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

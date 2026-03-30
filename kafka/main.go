package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// normally these would be set via helm values or env vars.
	// remember the docker containers talk via service name and local port (not exposed port).
	pkiLocalEndpoint   = "http://localhost:8080"
	pkiStagingEndpoint = "http://pki-staging:8080"
	pkiProdEndpoint    = "http://pki-prod:8080"

	// store the key, certificate, and ca in this folder
	serverName = "kafka-server"
	certFolder = "/etc/kafka/certs"
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")

	issueToken = os.Getenv("PKIISSUETOKEN")
)

func main() {
	// this program will generate a private key and CSR,
	// send the CSR to the PKI service to get a signed certificate,
	// and store the private key and certificate in /etc/<server-name>/certs

	// assign endpoint based on environment variable
	var endpoint string
	var saName string
	switch Environment {
	case "STAGING":
		endpoint = pkiStagingEndpoint
		saName = fmt.Sprintf("%s-staging", serverName)
	case "PROD":
		endpoint = pkiProdEndpoint
		saName = fmt.Sprintf("%s-prod", serverName)
	default:
		log.Fatalf("'%s' environment, container will not have a valid cert.\n", Environment)
	}

	// generate the private key and CSR
	var csrBytes []byte
	var pkString string
	if pk, err := rsa.GenerateKey(rand.Reader, 4096); err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	} else {
		template := &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName: serverName,
			},
			DNSNames: []string{saName},
		}
		if cb, err := x509.CreateCertificateRequest(rand.Reader, template, pk); err != nil {
			log.Fatalf("failed to create CSR: %v", err)
		} else {
			csrBytes = cb
			pkString = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)}))
		}
	}

	// call the PKI API to request a certificate (insert X-Cert-Type header for server)
	req, err := http.NewRequest(http.MethodPost, endpoint+"/issue", bytes.NewReader(csrBytes))
	if err != nil {
		log.Fatalf("failed to create CSR HTTP request: %v", err)
	}
	req.Header.Set("X-Auth-Token", issueToken)
	req.Header.Set("X-Cert-Type", "server")
	req.Header.Set("Content-Type", "application/application/octet-stream")

	client := &http.Client{Timeout: 10 * time.Second}
	var resp *http.Response
	for {
		r, err := client.Do(req)
		if err != nil {
			log.Printf("failed to send CSR HTTP request: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		defer r.Body.Close()

		if r.StatusCode != http.StatusOK {
			respString := ""
			if b, err := io.ReadAll(r.Body); err == nil {
				respString = string(b)
			}
			log.Printf("PKI server returned non-OK status: %s\nResponse: %s", r.Status, respString)
			_ = r.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		resp = r
		break
	}

	var certSerial string
	var certString string
	certBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read certificate from PKI response: %v", err)
	} else {
		// Extract the serial number from the certificate
		block, _ := pem.Decode(certBytes)
		if block == nil {
			log.Fatalf("failed to decode certificate PEM")
		} else {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				log.Fatalf("failed to parse certificate: %v", err)
			}
			certSerial = cert.SerialNumber.String()
			certString = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
		}
	}

	// fetch the CA certificate
	reqCACert, err := http.NewRequest(http.MethodGet, endpoint+"/ca", nil)
	if err != nil {
		log.Fatalf("failed to create CA cert HTTP request: %v", err)
	}

	respCACert, err := client.Do(reqCACert)
	if err != nil {
		log.Fatalf("failed to send CA cert HTTP request: %v", err)
	}
	defer respCACert.Body.Close()

	if respCACert.StatusCode != http.StatusOK {
		respString := ""
		if b, err := io.ReadAll(respCACert.Body); err == nil {
			respString = string(b)
		}
		log.Fatalf("PKI server returned non-OK status for CA cert request: %s\nResponse: %s", respCACert.Status, respString)
	}

	caCertBytes, err := io.ReadAll(respCACert.Body)
	if err != nil {
		log.Fatalf("failed to read CA certificate from PKI response: %v", err)
	}

	// store the private key, certificate, and cs in <certFolder>
	if err := os.MkdirAll(certFolder, 0755); err != nil {
		log.Fatalf("failed to create certs directory: %v", err)
	}

	keyPath := certFolder + "/key.pem"
	certPath := certFolder + "/cert.pem"
	caPath := certFolder + "/ca.pem"

	if err := os.WriteFile(keyPath, []byte(pkString), 0644); err != nil {
		log.Fatalf("failed to write private key file: %v", err)
	}
	if err := os.WriteFile(certPath, []byte(certString), 0644); err != nil {
		log.Fatalf("failed to write certificate file: %v", err)
	}
	if err := os.WriteFile(caPath, caCertBytes, 0644); err != nil {
		log.Fatalf("failed to write CA file: %v", err)
	}

	log.Printf("Successfully obtained certificate with serial %s\n", certSerial)
	log.Printf("Private key, certificate, and CA stored at %s\n", certPath)
	if fileList, err := os.ReadDir(certFolder); err != nil {
		log.Fatalf("failed to read certs directory: %s\n", err.Error())
	} else {
		for _, f := range fileList {
			log.Println(f.Name())
		}
	}
}

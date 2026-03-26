package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type (
	RevokeRequest struct {
		Serial     string `json:"serial"`
		ReasonCode int    `json:"reason_code"`
	}
)

const (
	apiPort       = ":8080"
	wipeCAOnStart = true
)

var (
	Version     = "development" // set at build time with -ldflags "-X main.Version=1.0.0"
	Environment = os.Getenv("ENVIRONMENT")

	certAuth   *CA
	issueToken = os.Getenv("PKIISSUETOKEN")
	adminToken = os.Getenv("PKIADMINTOKEN")
)

func main() {
	log.Printf("Starting CA REST server (version: %s)\n", Version)
	if Environment == "" {
		Environment = "DEV"
	}

	// check if the issue token is set
	if issueToken == "" {
		log.Fatal("ISSUETOKEN environment variable is not set.")
	}

	// check if the admin token is set	// (not used in this example, but you can implement admin endpoints that require this token)
	if adminToken == "" {
		log.Fatal("PKIADMINTOKEN environment variable is not set.")
	}

	// load CA from disk or initialize a new one
	if ca, err := initCA(wipeCAOnStart); err != nil {
		log.Fatalf("Failed to initialize CA: %v", err)
	} else {
		certAuth = ca
	}

	// load revoked certs from disk (or initialize empty map if file doesn't exist)
	if err := certAuth.loadRevokedCerts(); err != nil {
		log.Fatalf("Failed to load revoked certs: %v", err)
	}

	// start the HTTP server
	startHTTPServer()
}

func startHTTPServer() {
	// handler for health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		versionInfo := fmt.Sprintf("PKI version: %s (%s environment)", Version, Environment)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(versionInfo)); err != nil {
			log.Printf("Failed to write health check response: %v\n", err)
		}
	})

	// handler for issuing certificates
	http.HandleFunc("/issue", handleIssueCert)

	// handler for retrieving CA certificate
	http.HandleFunc("/ca", handleGetCACert)

	// handler for revoking certificates
	http.HandleFunc("/revoke", handleRevokeCert)

	// handler for getting CRL
	http.HandleFunc("/crl", handleGetCRL)

	// handler for wiping CA and all certs (for testing purposes)
	http.HandleFunc("/wipe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed (DELETE required)", http.StatusMethodNotAllowed)
			return
		}
		token := r.Header.Get("X-Auth-Token")
		if token == "" || token != adminToken {
			log.Printf("Unauthorized access attempt to wipe CA: missing/invalid token from: %s\n", r.RemoteAddr)
			http.Error(w, "Unauthorized: missing or invalid token", http.StatusUnauthorized)
			return
		}
		if ca, err := initCA(true); err != nil {
			http.Error(w, "Failed to wipe CA: "+err.Error(), http.StatusInternalServerError)
			return
		} else {
			certAuth = ca
			log.Printf("Successfully wiped CA from %s\n", r.RemoteAddr)
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("CA wiped successfully")); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	// start the server
	if err := http.ListenAndServe(apiPort, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func handleIssueCert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed (POST required)", http.StatusMethodNotAllowed)
		return
	}

	// authenticate the request (for simplicity, we just check for a header)
	token := r.Header.Get("X-Auth-Token")
	if token == "" || token != issueToken {
		log.Printf("Unauthorized access attempt to issue cert: missing/invalid token from: %s\n", r.RemoteAddr)
		http.Error(w, "Unauthorized: missing or invalid token", http.StatusUnauthorized)
		return
	}

	// read the CSR from the request body
	csrBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read CSR: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// get the signed certificate from the CA
	isServer := r.Header.Get("X-Cert-Type") == "server"
	certBytes, err := certAuth.SignCSR(csrBytes, isServer)
	if err != nil {
		http.Error(w, "Failed to sign CSR: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// get the common name and serial from the cert
	block, _ := pem.Decode(certBytes)
	if block == nil {
		log.Println("failed to decode certificate PEM")
	} else {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			http.Error(w, "failed to parse certificate: "+err.Error(), http.StatusInternalServerError)
			return
		}
		cn := cert.Subject.CommonName
		// retrieve SAN list from certificate
		san := ""
		if len(cert.DNSNames) > 0 {
			san = fmt.Sprintf(" (%s)", cert.DNSNames[0])
		}
		serial := cert.SerialNumber.String()
		log.Printf("Issued cert for %s%s with serial number %s\n", cn, san, serial)
	}

	// return the signed certificate in the response
	w.Header().Set("Content-Type", "application/x-pem-file")
	if _, err := w.Write(certBytes); err != nil {
		log.Printf("Failed to write response: %v\n", err)
		return
	}

}

func handleGetCACert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed (GET required)", http.StatusMethodNotAllowed)
		return
	}

	// return the CA certificate in PEM format
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certAuth.RootCert.Raw})
	w.Header().Set("Content-Type", "application/x-pem-file")
	if _, err := w.Write(caCertPEM); err != nil {
		log.Printf("Failed to write response: %v\n", err)
		return
	}
}

func handleRevokeCert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed (POST required)", http.StatusMethodNotAllowed)
		return
	}

	// authenticate the request (for simplicity, we just check for a header)
	token := r.Header.Get("X-Auth-Token")
	if token == "" || token != adminToken {
		log.Printf("Unauthorized access attempt to revoke cert: missing/invalid token from: %s\n", r.RemoteAddr)
		http.Error(w, "Unauthorized: missing or invalid token", http.StatusUnauthorized)
		return
	}

	// parse json from request body
	var revokeReq RevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&revokeReq); err != nil {
		http.Error(w, "Failed to parse request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// read serial number from request body
	serial := revokeReq.Serial
	if serial == "" {
		http.Error(w, "Missing serial number in query parameters", http.StatusBadRequest)
		return
	}

	// revoke the cert
	if err := certAuth.RevokeCert(serial); err != nil {
		log.Printf("Failed to revoke cert: %s\n", err.Error())
		return
	}

	// return success
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Certificate revoked successfully")); err != nil {
		log.Printf("Failed to write response: %v\n", err)
	}
}

func handleGetCRL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed (GET required)", http.StatusMethodNotAllowed)
		return
	}

	if crlPEM, err := certAuth.GenerateCRL(); err != nil {
		http.Error(w, "Failed to generate CRL: "+err.Error(), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "application/x-pem-file")
		if _, err := w.Write(crlPEM); err != nil {
			log.Printf("Failed to write response: %v\n", err)
			return
		}
	}
}

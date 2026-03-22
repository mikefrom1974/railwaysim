package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCA(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "ca_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change the storage location to temp dir
	originalStorage := storageLocation
	storageLocation = filepath.Join(tempDir, "ca_data")
	defer func() { storageLocation = originalStorage }()

	// Test initializing a new CA
	ca, err := initCA(false)
	if err != nil {
		t.Fatalf("Failed to init CA: %v", err)
	}

	if ca.RootCert == nil {
		t.Error("RootCert is nil")
	}
	if ca.RootKey == nil {
		t.Error("RootKey is nil")
	}

	// Check if files were created
	if !fileExists(filepath.Join(storageLocation, keyFile)) {
		t.Error("Key file not created")
	}
	if !fileExists(filepath.Join(storageLocation, certFile)) {
		t.Error("Cert file not created")
	}

	// Test loading existing CA
	ca2, err := initCA(false)
	if err != nil {
		t.Fatalf("Failed to load existing CA: %v", err)
	}

	if ca2.RootCert.SerialNumber.Cmp(ca.RootCert.SerialNumber) != 0 {
		t.Error("Loaded CA has different serial number")
	}
}

func TestSignCSR(t *testing.T) {
	// Create a temporary CA
	tempDir, err := os.MkdirTemp("", "ca_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalStorage := storageLocation
	storageLocation = filepath.Join(tempDir, "ca_data")
	defer func() { storageLocation = originalStorage }()

	ca, err := initCA(false)
	if err != nil {
		t.Fatalf("Failed to init CA: %v", err)
	}

	// Generate a test CSR
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, template, privKey)
	if err != nil {
		t.Fatalf("Failed to create CSR: %v", err)
	}

	// Sign the CSR
	certPEM, err := ca.SignCSR(csrBytes, false)
	if err != nil {
		t.Fatalf("Failed to sign CSR: %v", err)
	}

	// Parse the certificate
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Verify the certificate
	if cert.Subject.CommonName != "test.example.com" {
		t.Errorf("Expected CN 'test.example.com', got '%s'", cert.Subject.CommonName)
	}

	// Verify signature
	err = cert.CheckSignatureFrom(ca.RootCert)
	if err != nil {
		t.Errorf("Certificate signature verification failed: %v", err)
	}
}

func TestRevokeCert(t *testing.T) {
	// Create a temporary CA
	tempDir, err := os.MkdirTemp("", "ca_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalStorage := storageLocation
	storageLocation = filepath.Join(tempDir, "ca_data")
	defer func() { storageLocation = originalStorage }()

	ca, err := initCA(false)
	if err != nil {
		t.Fatalf("Failed to init CA: %v", err)
	}

	// Initialize revoked certs
	err = ca.loadRevokedCerts()
	if err != nil {
		t.Fatalf("Failed to load revoked certs: %v", err)
	}

	// Revoke a cert
	serial := "12345"
	err = ca.RevokeCert(serial)
	if err != nil {
		t.Fatalf("Failed to revoke cert: %v", err)
	}

	if !ca.RevokedCerts[serial] {
		t.Error("Cert not marked as revoked")
	}

	// Check if saved to disk
	ca2 := &CA{}
	ca2.RevokedCerts = make(map[string]bool)
	err = ca2.loadRevokedCerts()
	if err != nil {
		t.Fatalf("Failed to load revoked certs: %v", err)
	}

	if !ca2.RevokedCerts[serial] {
		t.Error("Revoked cert not persisted")
	}
}

func TestGenerateCRL(t *testing.T) {
	// Create a temporary CA
	tempDir, err := os.MkdirTemp("", "ca_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalStorage := storageLocation
	storageLocation = filepath.Join(tempDir, "ca_data")
	defer func() { storageLocation = originalStorage }()

	ca, err := initCA(false)
	if err != nil {
		t.Fatalf("Failed to init CA: %v", err)
	}

	// Initialize revoked certs
	err = ca.loadRevokedCerts()
	if err != nil {
		t.Fatalf("Failed to load revoked certs: %v", err)
	}

	// Revoke a cert
	serial := "12345"
	err = ca.RevokeCert(serial)
	if err != nil {
		t.Fatalf("Failed to revoke cert: %v", err)
	}

	// Generate CRL
	crlPEM, err := ca.GenerateCRL()
	if err != nil {
		t.Fatalf("Failed to generate CRL: %v", err)
	}

	// Parse the CRL
	block, _ := pem.Decode(crlPEM)
	if block == nil {
		t.Fatal("Failed to decode CRL PEM")
	}

	crl, err := x509.ParseRevocationList(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse CRL: %v", err)
	}

	// Check if our revoked cert is in the CRL
	found := false
	for _, rc := range crl.RevokedCertificateEntries {
		if rc.SerialNumber.String() == serial {
			found = true
			break
		}
	}
	if !found {
		t.Error("Revoked cert not found in CRL")
	}

	// Verify CRL signature
	err = crl.CheckSignatureFrom(ca.RootCert)
	if err != nil {
		t.Errorf("CRL signature verification failed: %v", err)
	}
}

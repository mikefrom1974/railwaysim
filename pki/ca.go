package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

var (
	storageLocation  = "./ca_data"
	keyFile          = "rootCA.key"
	certFile         = "rootCA.crt"
	RevokedCertsFile = "revoked_certs.json"
)

type CA struct {
	RootCert     *x509.Certificate
	RootKey      *rsa.PrivateKey
	RevokedCerts map[string]bool // revoked cert serial numbers
}

// SignCSR takes a raw CSR and returns a signed cert
func (c *CA) SignCSR(csrBytes []byte, isServer bool) ([]byte, error) {
	// parse the CSR
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return nil, err
	}

	// validate the CSR
	if err := csr.CheckSignature(); err != nil {
		return nil, err
	}

	// create a cert template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      csr.Subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // valid for 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// add server auth if needed
	if isServer {
		template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	}

	// sign the cert and return it in PEM format ([]byte)
	certBytes, err := x509.CreateCertificate(rand.Reader, template, c.RootCert, csr.PublicKey, c.RootKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes}), nil
}

// initCA loads the CA from disk if it exists, or creates a new CA and saves it to disk.
//
//	If wipe is true, it will delete any existing CA files and create a new CA
func initCA(wipe bool) (*CA, error) {
	keyPath := filepath.Join(storageLocation, keyFile)
	certPath := filepath.Join(storageLocation, certFile)

	// if wipe is true, delete existing CA files
	if wipe {
		if fileExists(keyPath) {
			if err := os.Remove(keyPath); err != nil {
				return nil, err
			}
		}
		if fileExists(certPath) {
			if err := os.Remove(certPath); err != nil {
				return nil, err
			}
		}
	}

	// check if CA key and cert already exist
	if fileExists(keyPath) && fileExists(certPath) {
		// load existing CA
		return loadCA(keyPath, certPath)
	}

	// create new CA
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Organization: []string{"Railway Sim"},
			Country:      []string{"US"},
			Province:     []string{"TX"},
			Locality:     []string{"Fort Worth"},
			CommonName:   "My CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// self-sign the CA cert
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, err
	}

	// save the CA key and cert to disk
	if err := saveCA(keyPath, certPath, privKey, certBytes); err != nil {
		return nil, err
	}

	// return the CA struct
	rootCert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	return &CA{RootCert: rootCert, RootKey: privKey}, nil
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// loadCA loads the CA key and cert from disk
func loadCA(keyPath, certPath string) (*CA, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	privKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return &CA{RootCert: cert, RootKey: privKey}, nil
}

// saveCA saves the CA key and cert to disk
func saveCA(keyPath, certPath string, key *rsa.PrivateKey, cert []byte) error {
	// ensure storage directory exists
	if err := os.MkdirAll(storageLocation, 0700); err != nil {
		return err
	}

	// save key
	if keyOut, err := os.Create(keyPath); err != nil {
		return err
	} else {
		defer keyOut.Close()
		if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
			return err
		}
		if err := os.Chmod(keyPath, 0600); err != nil {
			return err
		}
	}

	// save cert
	if certOut, err := os.Create(certPath); err != nil {
		return err
	} else {
		defer certOut.Close()
		if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
			return err
		}
		if err := os.Chmod(certPath, 0644); err != nil {
			return err
		}
	}

	return nil
}

// saveRevokedCerts saves the revoked certs map to disk as JSON
func (c *CA) saveRevokedCerts() error {
	path := filepath.Join(storageLocation, RevokedCertsFile)
	if d, err := json.Marshal(c.RevokedCerts); err != nil {
		return err
	} else {
		return os.WriteFile(path, d, 0644)
	}
}

// loadRevokedCerts loads the revoked certs map from disk
func (c *CA) loadRevokedCerts() error {
	path := filepath.Join(storageLocation, RevokedCertsFile)
	// if the file doesn't exist, initialize an empty map
	if !fileExists(path) {
		c.RevokedCerts = make(map[string]bool)
		return nil
	}

	// load our json file into the map
	if d, err := os.ReadFile(path); err != nil {
		return err
	} else {
		return json.Unmarshal(d, &c.RevokedCerts)
	}
}

// RevokeCert adds a cert serial number to the revoked certs map and saves it to disk
func (c *CA) RevokeCert(serialNumber string) error {
	c.RevokedCerts[serialNumber] = true
	return c.saveRevokedCerts()
}

// GenerateCRL creates a signed CRL containing all revoked certs and returns it in PEM format
func (c *CA) GenerateCRL() ([]byte, error) {
	var revokedCerts []pkix.RevokedCertificate
	for serialStr := range c.RevokedCerts {
		serialNum := new(big.Int)
		serialNum.SetString(serialStr, 10)
		revokedCerts = append(revokedCerts, pkix.RevokedCertificate{
			SerialNumber:   serialNum,
			RevocationTime: time.Now(),
		})
	}

	template := &x509.RevocationList{
		Number:              big.NewInt(time.Now().UnixNano()),
		ThisUpdate:          time.Now(),
		NextUpdate:          time.Now().Add(24 * time.Hour), // next update in 1 days
		RevokedCertificates: revokedCerts,
	}

	crlBytes, err := x509.CreateRevocationList(rand.Reader, template, c.RootCert, c.RootKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crlBytes}), nil
}

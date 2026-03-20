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

const (
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
	var csr *x509.CertificateRequest
	if r, e := x509.ParseCertificateRequest(csrBytes); e != nil {
		return nil, e
	} else {
		csr = r
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
	var certBytes []byte
	if cb, e := x509.CreateCertificate(rand.Reader, template, c.RootCert, csr.PublicKey, c.RootKey); e != nil {
		return nil, e
	} else {
		certBytes = cb
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes}), nil
}

// initCA loads the CA from disk if it exists, or creates a new CA and saves it to disk
func initCA() (*CA, error) {
	keyPath := filepath.Join(storageLocation, keyFile)
	certPath := filepath.Join(storageLocation, certFile)

	// check if CA key and cert already exist
	if fileExists(keyPath) && fileExists(certPath) {
		// load existing CA
		return loadCA(keyPath, certPath)
	}

	// create new CA
	var privKey *rsa.PrivateKey
	if pk, e := rsa.GenerateKey(rand.Reader, 4096); e != nil {
		return nil, e
	} else {
		privKey = pk
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
	var certBytes []byte
	if cb, e := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey); e != nil {
		return nil, e
	} else {
		certBytes = cb
	}

	// save the CA key and cert to disk
	if e := saveCA(keyPath, certPath, privKey, certBytes); e != nil {
		return nil, e
	}

	// return the CA struct
	var rootCert *x509.Certificate
	if rc, e := x509.ParseCertificate(certBytes); e != nil {
		return nil, e
	} else {
		rootCert = rc
	}

	return &CA{RootCert: rootCert, RootKey: privKey}, nil
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	if _, e := os.Stat(path); e == nil {
		return true
	}
	return false
}

// loadCA loads the CA key and cert from disk
func loadCA(keyPath, certPath string) (*CA, error) {
	var keyPEM, certPEM []byte
	if kp, e := os.ReadFile(keyPath); e != nil {
		return nil, e
	} else {
		keyPEM = kp
	}
	if cp, e := os.ReadFile(certPath); e != nil {
		return nil, e
	} else {
		certPEM = cp
	}

	keyBlock, _ := pem.Decode(keyPEM)
	var privKey *rsa.PrivateKey
	if pk, e := x509.ParsePKCS1PrivateKey(keyBlock.Bytes); e != nil {
		return nil, e
	} else {
		privKey = pk
	}

	certBlock, _ := pem.Decode(certPEM)
	var cert *x509.Certificate
	if c, e := x509.ParseCertificate(certBlock.Bytes); e != nil {
		return nil, e
	} else {
		cert = c
	}

	return &CA{RootCert: cert, RootKey: privKey}, nil
}

// saveCA saves the CA key and cert to disk
func saveCA(keyPath, certPath string, key *rsa.PrivateKey, cert []byte) error {
	// ensure storage directory exists
	if e := os.MkdirAll(storageLocation, 0700); e != nil {
		return e
	}

	// save key
	if keyOut, e := os.Create(keyPath); e != nil {
		return e
	} else {
		defer keyOut.Close()
		if e = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); e != nil {
			return e
		}
		if e = os.Chmod(keyPath, 0600); e != nil {
			return e
		}
	}

	// save cert
	if certOut, e := os.Create(certPath); e != nil {
		return e
	} else {
		defer certOut.Close()
		if e = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); e != nil {
			return e
		}
		if e = os.Chmod(certPath, 0644); e != nil {
			return e
		}
	}

	return nil
}

// saveRevokedCerts saves the revoked certs map to disk as JSON
func (c *CA) saveRevokedCerts() error {
	path := filepath.Join(storageLocation, RevokedCertsFile)
	if d, e := json.Marshal(c.RevokedCerts); e != nil {
		return e
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
	if d, e := os.ReadFile(path); e != nil {
		return e
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

	var crlBytes []byte
	if cb, e := x509.CreateRevocationList(rand.Reader, template, c.RootCert, c.RootKey); e != nil {
		return nil, e
	} else {
		crlBytes = cb
	}

	return pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crlBytes}), nil
}

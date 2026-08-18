// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/devproje/mininaru/util"
)

type ServerIdentity struct {
	Certificate tls.Certificate
	CAPool      *x509.CertPool
	CAPEM       []byte
	Fingerprint string
}

const pkiDirectory = "pki"

const (
	caCertificateFile     = "ca.crt"
	caPrivateKeyFile      = "ca.key"
	serverCertificateFile = "server.crt"
	serverPrivateKeyFile  = "server.key"
)

const (
	caLifetime     = 10 * 365 * 24 * time.Hour
	serverLifetime = 365 * 24 * time.Hour
	clientLifetime = 365 * 24 * time.Hour
)

func publicKeyFingerprint(publicKey any) (string, error) {
	var encoded []byte
	var sum [sha256.Size]byte

	var err error

	encoded, err = x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	sum = sha256.Sum256(encoded)

	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

func certificateFingerprint(certificate *x509.Certificate) (string, error) {
	if certificate == nil {
		return "", fmt.Errorf("certificate is required")
	}

	return publicKeyFingerprint(certificate.PublicKey)
}

func randomSerial() (*big.Int, error) {
	var limit *big.Int
	var serial *big.Int

	var err error

	limit = new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err = rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}

	return serial, nil
}

func writePEM(path, kind string, bytes []byte, permission os.FileMode) error {
	var encoded []byte

	encoded = pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: bytes})
	if len(encoded) == 0 {
		return fmt.Errorf("encode %s", kind)
	}

	return util.WriteFileAtomic(path, encoded, permission)
}

func createCA(directory string) error {
	var publicKey ed25519.PublicKey
	var privateKey ed25519.PrivateKey
	var serial *big.Int
	var template x509.Certificate
	var certificate []byte
	var encodedKey []byte
	var now time.Time

	var err error

	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	serial, err = randomSerial()
	if err != nil {
		return err
	}

	now = time.Now()
	template = x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "mininaru local ca"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(caLifetime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certificate, err = x509.CreateCertificate(rand.Reader, &template, &template, publicKey, privateKey)
	if err != nil {
		return err
	}

	encodedKey, err = x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	err = writePEM(filepath.Join(directory, caCertificateFile), "CERTIFICATE", certificate, 0600)
	if err != nil {
		return err
	}

	return writePEM(filepath.Join(directory, caPrivateKeyFile), "PRIVATE KEY", encodedKey, 0600)
}

func loadCA(directory string) (*x509.Certificate, ed25519.PrivateKey, []byte, error) {
	var certificatePEM []byte
	var keyPEM []byte
	var certificateBlock *pem.Block
	var keyBlock *pem.Block
	var certificate *x509.Certificate
	var parsedKey any
	var privateKey ed25519.PrivateKey
	var ok bool

	var err error

	certificatePEM, err = os.ReadFile(filepath.Join(directory, caCertificateFile))
	if err != nil {
		return nil, nil, nil, err
	}

	keyPEM, err = os.ReadFile(filepath.Join(directory, caPrivateKeyFile))
	if err != nil {
		return nil, nil, nil, err
	}

	certificateBlock, _ = pem.Decode(certificatePEM)
	keyBlock, _ = pem.Decode(keyPEM)
	if certificateBlock == nil || keyBlock == nil {
		return nil, nil, nil, fmt.Errorf("invalid ca pem")
	}

	certificate, err = x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return nil, nil, nil, err
	}

	parsedKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, nil, err
	}

	privateKey, ok = parsedKey.(ed25519.PrivateKey)
	if !ok {
		return nil, nil, nil, fmt.Errorf("ca key is not ed25519")
	}

	return certificate, privateKey, certificatePEM, nil
}

func createServerCertificate(directory string) error {
	var ca *x509.Certificate
	var caKey ed25519.PrivateKey
	var publicKey ed25519.PublicKey
	var privateKey ed25519.PrivateKey
	var serial *big.Int
	var template x509.Certificate
	var certificate []byte
	var encodedKey []byte
	var now time.Time

	var err error

	ca, caKey, _, err = loadCA(directory)
	if err != nil {
		return err
	}

	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	serial, err = randomSerial()
	if err != nil {
		return err
	}

	now = time.Now()
	template = x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "mininaru server"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(serverLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificate, err = x509.CreateCertificate(rand.Reader, &template, ca, publicKey, caKey)
	if err != nil {
		return err
	}

	encodedKey, err = x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	err = writePEM(filepath.Join(directory, serverCertificateFile), "CERTIFICATE", certificate, 0600)
	if err != nil {
		return err
	}

	return writePEM(filepath.Join(directory, serverPrivateKeyFile), "PRIVATE KEY", encodedKey, 0600)
}

func ensurePKIFiles(directory string) error {
	var err error

	err = os.MkdirAll(directory, 0700)
	if err != nil {
		return err
	}

	_, err = os.Stat(filepath.Join(directory, caCertificateFile))
	if os.IsNotExist(err) {
		err = createCA(directory)
	}
	if err != nil {
		return err
	}

	_, err = os.Stat(filepath.Join(directory, serverCertificateFile))
	if os.IsNotExist(err) {
		err = createServerCertificate(directory)
	}

	return err
}

func LoadServerIdentity() (*ServerIdentity, error) {
	var directory string
	var identity ServerIdentity
	var leaf *x509.Certificate

	var err error

	directory = util.Path(pkiDirectory)
	err = ensurePKIFiles(directory)
	if err != nil {
		return nil, err
	}

	identity.Certificate, err = tls.LoadX509KeyPair(filepath.Join(directory, serverCertificateFile), filepath.Join(directory, serverPrivateKeyFile))
	if err != nil {
		return nil, err
	}

	leaf, err = x509.ParseCertificate(identity.Certificate.Certificate[0])
	if err != nil {
		return nil, err
	}
	identity.Certificate.Leaf = leaf

	_, _, identity.CAPEM, err = loadCA(directory)
	if err != nil {
		return nil, err
	}

	identity.CAPool = x509.NewCertPool()
	if !identity.CAPool.AppendCertsFromPEM(identity.CAPEM) {
		return nil, fmt.Errorf("load ca certificate")
	}

	identity.Fingerprint, err = certificateFingerprint(leaf)
	if err != nil {
		return nil, err
	}

	return &identity, nil
}

func IssueClientCertificate(publicKeyDER []byte, clientId string) ([]byte, string, error) {
	var directory string
	var ca *x509.Certificate
	var caKey ed25519.PrivateKey
	var parsedKey any
	var publicKey ed25519.PublicKey
	var ok bool
	var serial *big.Int
	var template x509.Certificate
	var certificate []byte
	var now time.Time

	var err error

	directory = util.Path(pkiDirectory)
	ca, caKey, _, err = loadCA(directory)
	if err != nil {
		return nil, "", err
	}

	parsedKey, err = x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return nil, "", err
	}

	publicKey, ok = parsedKey.(ed25519.PublicKey)
	if !ok {
		return nil, "", fmt.Errorf("client key is not ed25519")
	}

	serial, err = randomSerial()
	if err != nil {
		return nil, "", err
	}

	now = time.Now()
	template = x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: clientId},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(clientLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certificate, err = x509.CreateCertificate(rand.Reader, &template, ca, publicKey, caKey)
	if err != nil {
		return nil, "", err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), serial.String(), nil
}

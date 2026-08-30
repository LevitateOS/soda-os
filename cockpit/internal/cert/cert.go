package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/LevitateOS/soda-os/cockpit/internal/appliance"
)

func Ensure(certPath, keyPath string, identity appliance.Identity) error {
	if existingCertificateMatches(certPath, keyPath, identity) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o750); err != nil {
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate cockpit key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate certificate serial: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: identity.Address, Organization: []string{"Soda OS"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     identity.CertificateNames(),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create cockpit certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal cockpit key: %w", err)
	}
	if err := writePEM(keyPath, 0o600, "PRIVATE KEY", keyDER); err != nil {
		return err
	}
	if err := writePEM(certPath, 0o644, "CERTIFICATE", der); err != nil {
		return err
	}
	return nil
}

func existingCertificateMatches(certPath, keyPath string, identity appliance.Identity) bool {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil || len(pair.Certificate) == 0 {
		return false
	}
	certificate, err := x509.ParseCertificate(pair.Certificate[0])
	return err == nil && certificate.Subject.CommonName == identity.Address && slices.Equal(certificate.DNSNames, identity.CertificateNames())
}

func writePEM(path string, mode os.FileMode, kind string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if err := pem.Encode(file, &pem.Block{Type: kind, Bytes: contents}); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

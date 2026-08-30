package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LevitateOS/soda-os/cockpit/internal/appliance"
)

func TestEnsureCreatesTLSKeyPair(t *testing.T) {
	directory := t.TempDir()
	certificate := filepath.Join(directory, "cockpit.crt")
	key := filepath.Join(directory, "cockpit.key")
	identity, err := appliance.FromTailnet("Atlas", "atlas.example.ts.net")
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(certificate, key, identity); err != nil {
		t.Fatal(err)
	}
	if _, err := tls.LoadX509KeyPair(certificate, key); err != nil {
		t.Fatalf("load generated key pair: %v", err)
	}
	contents := readFile(t, certificate)
	block, _ := pem.Decode(contents)
	parsed := parseCertificate(t, block)
	requireEqual(t, "certificate common name", parsed.Subject.CommonName, "atlas.example.ts.net")
	requireEqual(t, "certificate DNS names", parsed.DNSNames, []string{"atlas.example.ts.net", "atlas.local", "atlas"})
	requireEqual(t, "certificate IP addresses", ipStrings(parsed.IPAddresses), []string{"127.0.0.1", "::1"})
}

func TestEnsureKeepsExistingKeyPair(t *testing.T) {
	directory := t.TempDir()
	certificate := filepath.Join(directory, "cockpit.crt")
	key := filepath.Join(directory, "cockpit.key")
	first, err := appliance.FromHostname("soda")
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(certificate, key, first); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, certificate)
	beforeKey := readFile(t, key)
	second, err := appliance.FromHostname("soda")
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(certificate, key, second); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, "existing certificate", readFile(t, certificate), before)
	requireEqual(t, "existing key", readFile(t, key), beforeKey)
}

func TestEnsureReplacesCertificateWhenTailnetIdentityBecomesAvailable(t *testing.T) {
	directory := t.TempDir()
	certificate := filepath.Join(directory, "cockpit.crt")
	key := filepath.Join(directory, "cockpit.key")
	local, err := appliance.FromHostname("atlas")
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(certificate, key, local); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, certificate)
	tailnet, err := appliance.FromTailnet("atlas", "atlas.example.ts.net")
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(certificate, key, tailnet); err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(readFile(t, certificate))
	parsed := parseCertificate(t, block)
	if reflect.DeepEqual(before, readFile(t, certificate)) || !reflect.DeepEqual(parsed.DNSNames, []string{"atlas.example.ts.net", "atlas.local", "atlas"}) {
		t.Fatalf("certificate was not reissued for Tailnet identity: %#v", parsed.DNSNames)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func parseCertificate(t *testing.T, block *pem.Block) *x509.Certificate {
	t.Helper()
	if block == nil {
		t.Fatal("generated certificate is not PEM")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func requireEqual(t *testing.T, name string, actual, want any) {
	t.Helper()
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("%s = %#v; want %#v", name, actual, want)
	}
}

func ipStrings(addresses []net.IP) []string {
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.String())
	}
	return values
}

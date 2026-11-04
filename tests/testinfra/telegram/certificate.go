package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/psyb0t/ctxerrors"
)

func certificates(directory string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ctxerrors.Wrap(err, "generate fixture key")
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), DNSNames: []string{"headscale"},
		NotBefore: time.Now().
			Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour),
		IsCA:     true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	cert, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		return ctxerrors.Wrap(err, "create fixture certificate")
	}

	encodedKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return ctxerrors.Wrap(err, "encode fixture key")
	}

	for name, block := range map[string]*pem.Block{
		"cert.pem": {Type: "CERTIFICATE", Bytes: cert},
		"key.pem":  {Type: "EC PRIVATE KEY", Bytes: encodedKey},
	} {
		if err := os.WriteFile(
			filepath.Join(directory, name),
			pem.EncodeToMemory(block),
			privateFileMode,
		); err != nil {
			return ctxerrors.Wrap(err, "write private fixture certificate")
		}
	}

	return nil
}

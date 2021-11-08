package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"
)

const (
	CA_CERTIFICATE_FILENAME = "ca.pem"
	CA_KEY_FILENAME         = "ca.key.pem"
	CA_KEY_BITSIZE          = 2048
)

func loadPemFile(path string) (*pem.Block, error) {
	pemFile, err := ioutil.ReadFile(*app_certificate_path + "/" + path)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(pemFile)
	if pemBlock == nil {
		return nil, fmt.Errorf("pem.Decode failed")
	}

	return pemBlock, nil
}

func loadPemCertificate(path string) (*x509.Certificate, error) {
	pemBlock, err := loadPemFile(path)
	if err != nil {
		return nil, err
	}

	certificate, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return certificate, nil
}

func loadPemKey(path string) (*rsa.PrivateKey, error) {
	pemBlock, err := loadPemFile(path)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return key.(*rsa.PrivateKey), nil
}

func savePemKey(key *rsa.PrivateKey, path string) error {
	keyOut, err := os.OpenFile(*app_certificate_path+"/"+path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}
	if err := keyOut.Close(); err != nil {
		return err
	}
	return nil
}

func savePemCertificate(cert []byte, path string) error {
	certOut, err := os.Create(*app_certificate_path + "/" + path)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}
	return nil
}

func generateX509Certificate(privateKey *rsa.PrivateKey, days int, isCA bool, caCert *x509.Certificate, caKey *rsa.PrivateKey) ([]byte, error) {
	keyUsage := x509.KeyUsageDigitalSignature
	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, days)
	hosts := []string{*app_certificate_common_name}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: hosts,
			CommonName:   hosts[0],
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage: keyUsage,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign

	}

	if caCert == nil {
		log.Printf("Using template for ca\n")
		caCert = &template
	}

	if caKey == nil {
		caKey = privateKey
	}

	return x509.CreateCertificate(rand.Reader, &template, caCert, privateKey.Public(), caKey)
}

func generateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, CA_KEY_BITSIZE)
}

func generateCertificates() error {
	CACertificate, err := loadPemCertificate(CA_CERTIFICATE_FILENAME)
	if err != nil && !os.IsNotExist(err) {
		return err

	}

	var CAKey *rsa.PrivateKey

	if CACertificate == nil {
		log.Printf("CA missing, generating\n")
		CAKey, err = loadPemKey(CA_KEY_FILENAME)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if CAKey == nil {
			log.Printf("CA key missing, generating\n")
			CAKey, err = generateKey()
			if err != nil {
				return err
			}

			if err := savePemKey(CAKey, CA_KEY_FILENAME); err != nil {
				return err
			}
		}

		new_cert, err := generateX509Certificate(CAKey, 3650, true, nil, nil)
		if err != nil {
			return err
		}

		if err := savePemCertificate(new_cert, CA_CERTIFICATE_FILENAME); err != nil {
			return err
		}

		CACertificate, err = x509.ParseCertificate(new_cert)
		if err != nil {
			return err
		}

	}

	serverKey, err := generateKey()
	if err != nil {
		return err
	}

	serverCertificate, err := generateX509Certificate(serverKey, 3650, true, CACertificate, CAKey)
	if err != nil {
		return err
	}

	if err := savePemKey(serverKey, "server.key.pem"); err != nil {
		return err
	}

	if err := savePemCertificate(serverCertificate, "server.pem"); err != nil {
		return err
	}

	fmt.Printf("Certificates ready\n")
	os.Exit(0)

	return nil
}

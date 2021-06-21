package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/cmodk/phoenix"
)

func deviceCertificateRequestHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {

	if d.Token != nil {
		app.HttpBadRequest(w, fmt.Errorf("Device already assigned certificate"))
		return
	}

	body, _ := ioutil.ReadAll(r.Body)

	log.Printf("Body: %s\n", body)

	block, _ := pem.Decode(body)
	log.Printf("Bytes: %s\n", block.Bytes)

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	log.Printf("Fisk: %s\n", csr.Subject.CommonName)
	for _, n := range csr.Subject.Names {
		log.Printf("Certs: %s -> %s\n", n.Type, n.Value)
	}

	if err = csr.CheckSignature(); err != nil {
		panic(err)
	}

	// create client certificate template
	clientCRTTemplate := x509.Certificate{
		Signature:          csr.Signature,
		SignatureAlgorithm: csr.SignatureAlgorithm,

		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,
		PublicKey:          csr.PublicKey,

		SerialNumber: big.NewInt(int64(d.Id)),
		Issuer:       app.CACertificate.Subject,
		Subject:      csr.Subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 365),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// create client certificate from template and CA public key
	clientCRTRaw, err := x509.CreateCertificate(rand.Reader, &clientCRTTemplate, app.CACertificate, csr.PublicKey, app.CAPrivateKey)
	if err != nil {
		panic(err)
	}

	if err := pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: clientCRTRaw}); err != nil {
		app.HttpInternalError(w, err)
		return
	}

	h := sha256.New()
	h.Write(clientCRTRaw)

	certificate_hash := fmt.Sprintf("%x", h.Sum(nil))
	fmt.Printf("Cert hash: %s\n", certificate_hash)
	if err := d.Update("token", &certificate_hash); err != nil {
		app.HttpInternalError(w, err)
		return
	}

}

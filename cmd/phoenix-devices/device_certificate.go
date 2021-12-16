package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	"github.com/cmodk/phoenix"
)

func deviceCertificateRequestHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {

	if d.Token != nil {
		//Check if device tries to renew a certificate
		auth_header := r.Header["Authorization"]
		if len(auth_header) == 0 {
			app.HttpBadRequest(w, fmt.Errorf("Device already assigned certificate"))
			return
		}

		bearer := auth_header[0][7:]

		if bearer != *d.Token {
			app.HttpBadRequest(w, fmt.Errorf("Wrong token for certificate renewal"))
			return
		}
	}

	body, _ := ioutil.ReadAll(r.Body)

	block, _ := pem.Decode(body)

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	if err = csr.CheckSignature(); err != nil {
		panic(err)
	}

	now := time.Now()
	not_before := now
	not_after := now.Add(certificate_expiration_time)

	// create client certificate template
	clientCRTTemplate := x509.Certificate{
		Signature:          csr.Signature,
		SignatureAlgorithm: csr.SignatureAlgorithm,

		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,
		PublicKey:          csr.PublicKey,

		SerialNumber: big.NewInt(int64(d.Id)),
		Issuer:       app.CACertificate.Subject,
		Subject:      csr.Subject,
		NotBefore:    not_before,
		NotAfter:     not_after,
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
	if err := d.Update("token", &certificate_hash); err != nil {
		app.HttpInternalError(w, err)
		return
	}

	if err := d.Update("token_expiration", &not_after); err != nil {
		app.HttpInternalError(w, err)
		return
	}

}

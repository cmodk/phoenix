package main

import (
	"crypto/sha256"
	"crypto/x509"
	"fmt"

	"github.com/cmodk/phoenix"
)

func VerifyClient(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {

	cert := rawCerts[0]
	c, err := x509.ParseCertificate(cert)
	if err != nil {
		log.Fatal(err)
	}

	log.Debugf("Name %s\n", c.Subject.CommonName)
	log.Debugf("Serial: %d\n", c.SerialNumber)
	log.Debugf("Not before %s\n", c.NotBefore.String())
	log.Debugf("Not after %s\n", c.NotAfter.String())
	log.Print(c.Subject.Names)

	h := sha256.New()
	h.Write(c.Raw)
	certificate_hash := fmt.Sprintf("%x", h.Sum(nil))
	log.Debugf("Cert hash: %s\n", certificate_hash)

	d, err := app.Devices.Get(phoenix.DeviceCriteria{
		Id:    c.SerialNumber.Uint64(),
		Guid:  c.Subject.CommonName,
		Token: certificate_hash,
	})
	if err != nil || d.Id == 0 {
		lg.WithField("error", err).Error("Error verifying client certificate")
		return fmt.Errorf("Certificate error")
	}

	return nil
}

package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/pki"
)

func loadCert(path string) *x509.Certificate {
	b, err := os.ReadFile(path)
	if err != nil {panic(err)}
	fmt.Printf("%s: %d PEM Blocks\n", path, countBlocks(b))
	block, _ := pem.Decode(b)
	if block == nil {panic("no PEM in" +path)}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {panic(err)}
	fmt.Printf("subject=%s issuer=%s isCA=%v\n", c.Subject, c.Issuer, c.IsCA)
	return c
}


func countBlocks(b []byte) int {
	n:=0
	for len(b) >0 {
		var block *pem.Block
		block, b = pem.Decode(b)
		if block == nil {break}
		n++
	}
	return n
}
func main() {
	ca := loadCert("/home/johnnyash/.ashrix/pki/ca_cert.crt")
	gw := loadCert("/home/johnnyash/.ashrix/pki/gateway.crt")
	conn := loadCert("/home/johnnyash/.ashrix/connector/connector.crt")

	pool := x509.NewCertPool()

	pool.AddCert(ca)

	fmt.Println("\ngateway leaf vs CA")
	_, err := gw.Verify(x509.VerifyOptions{
		Roots: pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSName: "localhost",
	})

	fmt.Println("gateway verify", err)

	fmt.Println("\nconnector leaf vs CA")
	_, err = conn.Verify(x509.VerifyOptions{
		Roots: pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		DNSName: "localhost",
	})

	fmt.Println("connector verify", err)
}


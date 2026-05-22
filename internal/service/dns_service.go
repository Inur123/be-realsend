package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
)

type DNSRecords struct {
	SPFRecord         string `json:"spf_record"`
	DKIMSelector      string `json:"dkim_selector"`
	DKIMHost          string `json:"dkim_host"`
	DKIMRecord        string `json:"dkim_record"`
	DMARCRecord       string `json:"dmarc_record"`
	ReturnPathCNAME    string `json:"return_path_cname"`
	ReturnPathValue   string `json:"return_path_value"`
}

type DNSService interface {
	GenerateDKIMKeyPair() (string, string, error)
	BuildDNSRecords(domainName, dkimPublicKeyBase64 string) *DNSRecords
	VerifyDNS(ctx context.Context, domainName string, expected *DNSRecords) (spfOk, dkimOk, dmarcOk bool, err error)
}

type dnsService struct{}

func NewDNSService() DNSService {
	return &dnsService{}
}

func (s *dnsService) GenerateDKIMKeyPair() (publicKeyStr, privateKeyStr string, err error) {
	// Generate 1024-bit RSA key pair (ideal balance of security and DNS size limits)
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", "", fmt.Errorf("generate rsa key: %w", err)
	}

	// 1. Marshall Private Key to PKCS#8 PEM
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal pkcs8 private key: %w", err)
	}
	privBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	}
	privKeyPEM := pem.EncodeToMemory(privBlock)

	// 2. Marshall Public Key to PKIX DER (DKIM standard)
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal pkix public key: %w", err)
	}
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)

	return pubKeyBase64, string(privKeyPEM), nil
}

func (s *dnsService) BuildDNSRecords(domainName, dkimPublicKeyBase64 string) *DNSRecords {
	selector := "realsend"
	return &DNSRecords{
		SPFRecord:         "v=spf1 include:relay.realsend.id ~all",
		DKIMSelector:      selector,
		DKIMHost:          fmt.Sprintf("%s._domainkey.%s", selector, domainName),
		DKIMRecord:        fmt.Sprintf("v=DKIM1; k=rsa; p=%s", dkimPublicKeyBase64),
		DMARCRecord:       "v=DMARC1; p=none; pct=100; rua=mailto:dmarc-reports@realsend.id",
		ReturnPathCNAME:   fmt.Sprintf("realsend-cname.%s", domainName),
		ReturnPathValue:   "feedback.realsend.id",
	}
}

func (s *dnsService) VerifyDNS(ctx context.Context, domainName string, expected *DNSRecords) (spfOk, dkimOk, dmarcOk bool, err error) {
	// Perform DNS TXT lookup using system resolver
	var r net.Resolver

	// 1. Verify SPF Record on the apex domain
	spfTXT, err := r.LookupTXT(ctx, domainName)
	if err == nil {
		for _, record := range spfTXT {
			// Clean record strings
			cleanRecord := strings.Join(strings.Fields(record), " ")
			if strings.Contains(cleanRecord, "v=spf1") && strings.Contains(cleanRecord, "include:relay.realsend.id") {
				spfOk = true
				break
			}
		}
	}

	// 2. Verify DKIM Record on the selector hostname
	dkimHost := fmt.Sprintf("%s._domainkey.%s", expected.DKIMSelector, domainName)
	dkimTXT, err := r.LookupTXT(ctx, dkimHost)
	if err == nil {
		for _, record := range dkimTXT {
			cleanRecord := strings.ReplaceAll(record, " ", "")
			cleanRecord = strings.ReplaceAll(cleanRecord, "\t", "")
			expectedClean := strings.ReplaceAll(expected.DKIMRecord, " ", "")
			
			if strings.Contains(cleanRecord, "v=DKIM1") && strings.Contains(cleanRecord, expectedClean) {
				dkimOk = true
				break
			}
		}
	}

	// 3. Verify DMARC Record on the _dmarc prefix
	dmarcHost := fmt.Sprintf("_dmarc.%s", domainName)
	dmarcTXT, err := r.LookupTXT(ctx, dmarcHost)
	if err == nil {
		for _, record := range dmarcTXT {
			cleanRecord := strings.Join(strings.Fields(record), " ")
			if strings.Contains(cleanRecord, "v=DMARC1") {
				dmarcOk = true
				break
			}
		}
	}

	// We return any DNS lookup failures as metadata, not system failures
	return spfOk, dkimOk, dmarcOk, nil
}

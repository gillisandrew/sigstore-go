package bundle

import (
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"

	"github.com/github/sigstore-verifier/pkg/root"
)

type VerificationContent interface {
	CompareKey(any, root.TrustedMaterial) bool
	ValidAtTime(time.Time, root.TrustedMaterial) bool
	Verify(SignatureContent, root.TrustedMaterial) error
	GetIssuer() string
	GetSAN() string
}

type CertificateChain struct {
	Certificates []*x509.Certificate
}

type PublicKey struct {
	Hint string
}

func (cc *CertificateChain) CompareKey(key any, _ root.TrustedMaterial) bool {
	x509Key, ok := key.(*x509.Certificate)
	if !ok {
		return false
	}

	return cc.Certificates[0].Equal(x509Key)
}

func (cc *CertificateChain) ValidAtTime(t time.Time, _ root.TrustedMaterial) bool {
	return !(cc.Certificates[0].NotAfter.Before(t) || cc.Certificates[0].NotBefore.After(t))
}

func (cc *CertificateChain) Verify(sigContent SignatureContent, trustedMaterial root.TrustedMaterial) error {
	verifier, err := signature.LoadVerifier(cc.Certificates[0].PublicKey, crypto.SHA256)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	err = sigContent.CheckSignature(verifier)
	if err != nil {
		return err
	}

	leafCert := cc.Certificates[0]

	for _, ca := range trustedMaterial.FulcioCertificateAuthorities() {
		if !ca.ValidityPeriodStart.IsZero() && leafCert.NotBefore.Before(ca.ValidityPeriodStart) {
			continue
		}
		if !ca.ValidityPeriodEnd.IsZero() && leafCert.NotAfter.After(ca.ValidityPeriodEnd) {
			continue
		}

		rootCertPool := x509.NewCertPool()
		rootCertPool.AddCert(ca.Root)
		intermediateCertPool := x509.NewCertPool()
		for _, cert := range ca.Intermediates {
			intermediateCertPool.AddCert(cert)
		}

		opts := x509.VerifyOptions{
			// CurrentTime is intentionally set to the leaf certificate's
			// NotBefore time to ensure that we can continue to verify
			// old bundles after they expire.
			CurrentTime:   leafCert.NotBefore,
			Roots:         rootCertPool,
			Intermediates: intermediateCertPool,
			KeyUsages: []x509.ExtKeyUsage{
				x509.ExtKeyUsageCodeSigning,
			},
		}

		_, err = leafCert.Verify(opts)
		if err == nil {
			return nil
		}
	}

	return errors.New("certificate verification failed")
}

func (cc *CertificateChain) GetIssuer() string {
	for _, extension := range cc.Certificates[0].Extensions {
		if extension.Id.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}) {
			return string(extension.Value)
		}
	}
	return ""
}

func (cc *CertificateChain) GetSAN() string {
	if len(cc.Certificates[0].URIs) == 0 {
		return ""
	}

	return cc.Certificates[0].URIs[0].String()
}

func (pk *PublicKey) CompareKey(key any, tm root.TrustedMaterial) bool {
	verifier, err := tm.PublicKeyVerifier(pk.Hint)
	if err != nil {
		return false
	}
	pubKey, err := verifier.PublicKey()
	if err != nil {
		return false
	}
	if equaler, ok := key.(interface{ Equal(x crypto.PublicKey) bool }); ok {
		return equaler.Equal(pubKey)
	}
	return false
}

func (pk *PublicKey) ValidAtTime(t time.Time, tm root.TrustedMaterial) bool {
	verifier, err := tm.PublicKeyVerifier(pk.Hint)
	if err != nil {
		return false
	}
	return verifier.ValidAtTime(t)
}

func (pk *PublicKey) Verify(sigContent SignatureContent, tm root.TrustedMaterial) error {
	verifier, err := tm.PublicKeyVerifier(pk.Hint)
	if err != nil {
		return err
	}

	err = sigContent.CheckSignature(verifier)
	if err != nil {
		return err
	}

	return nil
}

func (pk *PublicKey) GetIssuer() string {
	return ""
}

func (pk *PublicKey) GetSAN() string {
	return ""
}
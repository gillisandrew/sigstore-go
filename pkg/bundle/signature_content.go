package bundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/github/sigstore-verifier/pkg/verify"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type MessageSignature struct {
	Digest          []byte
	DigestAlgorithm string
	Signature       []byte
}

func (m *MessageSignature) GetDigest() []byte {
	return m.Digest
}

func (m *MessageSignature) GetDigestAlgorithm() string {
	return m.DigestAlgorithm
}

type Envelope struct {
	*dsse.Envelope
}

func (e *Envelope) GetStatement() (*in_toto.Statement, error) {
	if e.PayloadType != IntotoMediaType {
		return nil, ErrIncorrectMediaType
	}

	var statement *in_toto.Statement
	raw, err := e.DecodeB64Payload()
	if err != nil {
		return nil, ErrDecodingB64
	}
	err = json.Unmarshal(raw, &statement)
	if err != nil {
		return nil, ErrDecodingJSON
	}
	return statement, nil
}

func (e *Envelope) HasEnvelope() (verify.EnvelopeContent, bool) {
	return e, true
}

func (e *Envelope) GetRawEnvelope() *dsse.Envelope {
	return e.Envelope
}

func (m *MessageSignature) HasEnvelope() (verify.EnvelopeContent, bool) {
	return nil, false
}

func (e *Envelope) HasMessage() (verify.MessageSignatureContent, bool) {
	return nil, false
}

func (m *MessageSignature) HasMessage() (verify.MessageSignatureContent, bool) {
	return m, true
}

func (m *MessageSignature) EnsureFileMatchesDigest(fileBytes []byte) error {
	if m.DigestAlgorithm != "SHA2_256" {
		return errors.New("Message has unsupported hash algorithm")
	}

	fileDigest := sha256.Sum256(fileBytes)
	if !bytes.Equal(m.Digest, fileDigest[:]) {
		return errors.New("Message signature does not match supplied file")
	}
	return nil
}

func (e *Envelope) EnsureFileMatchesDigest(fileBytes []byte) error {
	if e.Payload != base64.StdEncoding.EncodeToString(fileBytes) {
		return errors.New("Envelope payload does not match supplied file")
	}
	return nil
}

func (m *MessageSignature) GetSignature() []byte {
	return m.Signature
}

func (e *Envelope) GetSignature() []byte {
	if len(e.Envelope.Signatures) == 0 {
		return []byte{}
	}

	sigBytes, err := base64.StdEncoding.DecodeString(e.Envelope.Signatures[0].Sig)
	if err != nil {
		return []byte{}
	}

	return sigBytes
}

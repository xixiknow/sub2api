// Package qqbot implements the QQ official bot (api-v2) webhook protocol:
// ed25519 callback signing/verification, event parsing and the OpenAPI client.
package qqbot

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"strings"
)

// Signer derives the bot's ed25519 key pair from its AppSecret.
type Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

var ErrBadSignature = errors.New("qqbot: invalid signature")

// NewSigner repeats the secret until it reaches the ed25519 seed size and
// truncates it, as specified by the QQ bot docs.
func NewSigner(secret string) (*Signer, error) {
	if secret == "" {
		return nil, errors.New("qqbot: empty secret")
	}
	seed := secret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	priv := ed25519.NewKeyFromSeed([]byte(seed[:ed25519.SeedSize]))
	return &Signer{priv: priv, pub: priv.Public().(ed25519.PublicKey)}, nil
}

// SignValidation answers the op=13 callback URL validation challenge.
func (s *Signer) SignValidation(eventTs, plainToken string) string {
	return hex.EncodeToString(ed25519.Sign(s.priv, []byte(eventTs+plainToken)))
}

// Verify checks X-Signature-Ed25519 over X-Signature-Timestamp + body.
func (s *Signer) Verify(signatureHex, timestamp string, body []byte) error {
	sig, err := hex.DecodeString(signatureHex)
	if err != nil || len(sig) != ed25519.SignatureSize || sig[63]&224 != 0 {
		return ErrBadSignature
	}
	msg := make([]byte, 0, len(timestamp)+len(body))
	msg = append(append(msg, timestamp...), body...)
	if !ed25519.Verify(s.pub, msg, sig) {
		return ErrBadSignature
	}
	return nil
}

// sign is used by tests to produce callback signatures.
func (s *Signer) sign(timestamp string, body []byte) string {
	return hex.EncodeToString(ed25519.Sign(s.priv, append([]byte(timestamp), body...)))
}

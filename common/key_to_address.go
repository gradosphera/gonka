package common

import (
	"encoding/base64"
	"errors"
	"github.com/cometbft/cometbft/crypto/ed25519"
)

func ConsensusKeyToConsensusAddress(keyBase64 string) (string, error) {
	if keyBase64 == "" {
		return "", errors.New("key is empty")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return "", err
	}
	pk := ed25519.PubKey(keyBytes)
	if len(keyBytes) != ed25519.PubKeySize {
		return "", errors.New("invalid public key len")
	}

	return pk.Address().String(), nil
}

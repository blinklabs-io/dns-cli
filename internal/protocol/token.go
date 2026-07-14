package protocol

import (
	"encoding/hex"

	"golang.org/x/crypto/blake2b"
)

// CreateReferenceTokenTN returns blake2b_256("r" + name) as hex.
func CreateReferenceTokenTN(name []byte) string {
	return hex.EncodeToString(tokenHash('r', name))
}

// CreateUserTokenTN returns blake2b_256("u" + name) as hex.
func CreateUserTokenTN(name []byte) string {
	return hex.EncodeToString(tokenHash('u', name))
}

// CreateReferenceTokenTNBytes returns the raw hash bytes.
func CreateReferenceTokenTNBytes(name []byte) []byte {
	return tokenHash('r', name)
}

// CreateUserTokenTNBytes returns the raw hash bytes.
func CreateUserTokenTNBytes(name []byte) []byte {
	return tokenHash('u', name)
}

func tokenHash(prefix byte, name []byte) []byte {
	buf := make([]byte, 0, 1+len(name))
	buf = append(buf, prefix)
	buf = append(buf, name...)
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	h.Write(buf)
	return h.Sum(nil)
}

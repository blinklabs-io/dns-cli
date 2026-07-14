package wallet

import (
	"crypto/ed25519"
	"fmt"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Ed25519Wallet signs with a raw ed25519 private key (32-byte seed or 64-byte key).
type Ed25519Wallet struct {
	address    common.Address
	privateKey ed25519.PrivateKey
}

// NewEd25519Wallet creates a wallet from raw key material.
func NewEd25519Wallet(addr common.Address, key []byte) (*Ed25519Wallet, error) {
	var priv ed25519.PrivateKey
	switch len(key) {
	case ed25519.SeedSize:
		priv = ed25519.NewKeyFromSeed(key)
	case ed25519.PrivateKeySize:
		priv = ed25519.PrivateKey(append([]byte{}, key...))
	default:
		return nil, fmt.Errorf("unsupported ed25519 key length %d", len(key))
	}
	w := &Ed25519Wallet{address: addr, privateKey: priv}
	if w.PubKeyHash() != addr.PaymentKeyHash() {
		return nil, fmt.Errorf("ed25519 key does not control address")
	}
	return w, nil
}

func (w *Ed25519Wallet) Address() common.Address { return w.address }

func (w *Ed25519Wallet) SignTxBody(txBodyHash common.Blake2b256) (common.VkeyWitness, error) {
	sig := ed25519.Sign(w.privateKey, txBodyHash.Bytes())
	return common.VkeyWitness{
		Vkey:      w.privateKey.Public().(ed25519.PublicKey),
		Signature: sig,
	}, nil
}

func (w *Ed25519Wallet) PubKeyHash() common.Blake2b224 {
	pub := w.privateKey.Public().(ed25519.PublicKey)
	return common.Blake2b224Hash(pub)
}

func (w *Ed25519Wallet) StakePubKeyHash() common.Blake2b224 {
	return common.Blake2b224{}
}

func (w Ed25519Wallet) String() string {
	return fmt.Sprintf("Ed25519Wallet{address: %s}", w.address.String())
}

func (w Ed25519Wallet) GoString() string { return w.String() }

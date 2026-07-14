package wallet

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
)

// SignEnvelope adds a witness from the wallet to a decoded Conway transaction
// without rebuilding the body. It verifies the body hash against expectedBodyHashHex
// when provided.
func SignEnvelope(tx *conway.ConwayTransaction, w interface {
	SignTxBody(common.Blake2b256) (common.VkeyWitness, error)
	PubKeyHash() common.Blake2b224
}, expectedBodyHashHex string, requiredSigners []string, allowExtra bool) error {
	bodyCbor, err := cbor.Encode(&tx.Body)
	if err != nil {
		return fmt.Errorf("encode tx body: %w", err)
	}
	hash := common.Blake2b256Hash(bodyCbor)
	if expectedBodyHashHex != "" {
		want, err := hex.DecodeString(expectedBodyHashHex)
		if err != nil || len(want) != 32 {
			return fmt.Errorf("invalid expected body hash")
		}
		var expected common.Blake2b256
		copy(expected[:], want)
		if hash != expected {
			return fmt.Errorf("transaction body hash mismatch with manifest")
		}
	}
	pkhHex := hex.EncodeToString(w.PubKeyHash().Bytes())
	if !allowExtra && len(requiredSigners) > 0 {
		ok := false
		for _, r := range requiredSigners {
			if stringsEqualFold(r, pkhHex) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("signer %s is not in the required signers list (use --allow-extra-signer to override)", pkhHex[:16]+"…")
		}
	}
	wit, err := w.SignTxBody(hash)
	if err != nil {
		return err
	}
	items := tx.WitnessSet.VkeyWitnesses.Items()
	for _, existing := range items {
		if equalBytes(existing.Vkey, wit.Vkey) {
			// duplicate — leave unchanged
			return nil
		}
	}
	items = append(items, wit)
	tx.WitnessSet.VkeyWitnesses = cbor.NewSetType(items, true)
	return nil
}

func stringsEqualFold(a, b string) bool {
	return len(a) == len(b) && (a == b || equalFoldHex(a, b))
}

func equalFoldHex(a, b string) bool {
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'F' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'F' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// WriteAtomic writes data to path via a temporary file.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Ensure ed25519 is used so the import stays when SignEnvelope is used alone.
var _ = ed25519.PrivateKeySize

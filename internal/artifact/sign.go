package artifact

import (
	"encoding/hex"
	"fmt"

	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
	"golang.org/x/crypto/blake2b"
)

// DecodeConwayTx decodes transaction CBOR into a Conway transaction.
func DecodeConwayTx(txCbor []byte) (*conway.ConwayTransaction, error) {
	var tx conway.ConwayTransaction
	if _, err := cbor.Decode(txCbor, &tx); err != nil {
		return nil, fmt.Errorf("decode conway tx: %w", err)
	}
	return &tx, nil
}

// EncodeConwayTx encodes a Conway transaction.
func EncodeConwayTx(tx *conway.ConwayTransaction) ([]byte, error) {
	return cbor.Encode(tx)
}

// BodyHashHex returns the Blake2b-256 hex of the transaction body.
func BodyHashHex(tx *conway.ConwayTransaction) (string, error) {
	bodyCbor, err := cbor.Encode(&tx.Body)
	if err != nil {
		return "", err
	}
	h, err := blake2b.New256(nil)
	if err != nil {
		return "", err
	}
	h.Write(bodyCbor)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CountVKeyWitnesses returns the number of payment witnesses.
func CountVKeyWitnesses(tx *conway.ConwayTransaction) int {
	return len(tx.WitnessSet.VkeyWitnesses.Items())
}

// TxIDHex returns the transaction ID (body hash) once built.
func TxIDHex(tx *conway.ConwayTransaction) (string, error) {
	return BodyHashHex(tx)
}

// SignWithWallet signs an envelope using the wallet and optional manifest constraints.
func SignWithWallet(envelopePath, manifestPath, outPath string, w wallet.Signer, allowExtra bool) error {
	env, err := ReadEnvelope(envelopePath)
	if err != nil {
		return err
	}
	var required []string
	var bodyHash string
	if man, err := ReadManifest(manifestPath); err == nil {
		required = man.RequiredSigners
		bodyHash = man.BodyHash
	}
	raw, err := env.DecodeCBORHex()
	if err != nil {
		return err
	}
	tx, err := DecodeConwayTx(raw)
	if err != nil {
		return err
	}
	if err := wallet.SignEnvelope(tx, w, bodyHash, required, allowExtra); err != nil {
		return err
	}
	out, err := EncodeConwayTx(tx)
	if err != nil {
		return err
	}
	outEnv := Envelope{
		Type:        TypeWitnessedConway,
		Description: env.Description + " (signed)",
		CBORHex:     hex.EncodeToString(out),
	}
	_ = common.Blake2b256{} // keep import stable for future hash checks
	return WriteEnvelope(outPath, outEnv)
}

func blake2b256(b []byte) []byte {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	h.Write(b)
	return h.Sum(nil)
}

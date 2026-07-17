package artifact

import (
	"encoding/hex"
	"fmt"
)

// TxInputRefs returns transaction input references as "txidhex#index".
func TxInputRefs(envelopePath string) ([]string, error) {
	env, err := ReadEnvelope(envelopePath)
	if err != nil {
		return nil, err
	}
	raw, err := env.DecodeCBORHex()
	if err != nil {
		return nil, err
	}
	tx, err := DecodeConwayTx(raw)
	if err != nil {
		return nil, err
	}
	inputs := tx.Body.Inputs()
	out := make([]string, 0, len(inputs))
	for _, in := range inputs {
		id := in.Id()
		out = append(out, fmt.Sprintf("%s#%d", hex.EncodeToString(id[:]), in.Index()))
	}
	return out, nil
}

// UTxORef formats a UTxO id + index as "txidhex#index".
func UTxORef(txID []byte, index uint32) string {
	return fmt.Sprintf("%s#%d", hex.EncodeToString(txID), index)
}

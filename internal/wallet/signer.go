package wallet

import (
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Signer can produce a VKey witness for a transaction body hash.
type Signer interface {
	SignTxBody(txBodyHash common.Blake2b256) (common.VkeyWitness, error)
	PubKeyHash() common.Blake2b224
}

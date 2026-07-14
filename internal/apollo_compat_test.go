package dnscli_test

import (
	"testing"

	apollo "github.com/Salvionied/apollo/v2"
	"github.com/Salvionied/apollo/v2/backend"
	"github.com/Salvionied/apollo/v2/backend/fixed"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Compile-time proof that the pinned Apollo v2 revision exposes required APIs.
func TestApolloV2APICompatibility(t *testing.T) {
	var cc backend.ChainContext = fixed.NewEmptyFixedChainContext()
	a := apollo.New(cc)
	if a == nil {
		t.Fatal("apollo.New returned nil")
	}
	addr := mustTestAddress(t)
	w := apollo.NewExternalWallet(addr)
	a = a.SetWallet(w)
	a.AddLoadedUTxOs()
	unit := apollo.NewUnit("a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", "746f6b656e", 1)
	rd := common.Datum{}
	a.Mint(unit, &rd, nil)
	a.PayToContract(addr, &rd, 2_000_000)
	// CollectFrom requires a valid UTxO id; API presence is verified at compile time.
	// Method presence checks (no Complete — requires funded wallet UTxOs).
	if _, err := a.GetTxCbor(); err == nil {
		t.Fatal("expected GetTxCbor to fail before Complete")
	}
	if _, err := a.SignWithSkey(make([]byte, 96)); err == nil {
		t.Fatal("expected SignWithSkey to fail before Complete")
	}
	if _, err := a.AddReferenceInput("0000000000000000000000000000000000000000000000000000000000000001", 0); err != nil {
		t.Fatalf("AddReferenceInput: %v", err)
	}
}

func mustTestAddress(t *testing.T) common.Address {
	t.Helper()
	var raw [57]byte
	raw[0] = 0x00
	raw[1] = 0xAA
	raw[29] = 0xBB
	addr, err := common.NewAddressFromBytes(raw[:])
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

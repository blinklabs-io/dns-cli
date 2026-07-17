package provider

import (
	"testing"
)

// Sample Blockfrost GET /txs/{hash}/utxos body: outputs omit tx_hash (only top-level hash).
const sampleTxUtxosJSON = `{
  "hash": "702295f7436a1d6be316ae21c9da79c2d403618f8694c253a6f75931d81f9f91",
  "inputs": [],
  "outputs": [
    {"address": "addr_test1qzls", "amount": [{"unit": "lovelace", "quantity": "5000000"}], "output_index": 0},
    {"address": "addr_test1qzls", "amount": [{"unit": "lovelace", "quantity": "25000000"}], "output_index": 1},
    {"address": "addr_test1qp92", "amount": [{"unit": "lovelace", "quantity": "5000000"}], "output_index": 2},
    {"address": "addr_test1qp92", "amount": [{"unit": "lovelace", "quantity": "45000000"}], "output_index": 3},
    {"address": "addr_test1qpxw", "amount": [{"unit": "lovelace", "quantity": "5000000"}], "output_index": 4},
    {"address": "addr_test1qpxw", "amount": [{"unit": "lovelace", "quantity": "25000000"}], "output_index": 5},
    {"address": "addr_test1qrt0", "amount": [{"unit": "lovelace", "quantity": "279620950"}], "output_index": 6}
  ]
}`

func TestTxUtxosContainIndexes(t *testing.T) {
	ok, err := txUtxosContainIndexes([]byte(sampleTxUtxosJSON), []uint32{0, 1, 2, 3, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected indexes 0-5 present")
	}
}

func TestTxUtxosContainIndexesMissing(t *testing.T) {
	ok, err := txUtxosContainIndexes([]byte(sampleTxUtxosJSON), []uint32{0, 7})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("index 7 should be missing")
	}
}

func TestTxUtxosOutputAddress(t *testing.T) {
	addr, err := txUtxosOutputAddress([]byte(sampleTxUtxosJSON), 3)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "addr_test1qp92" {
		t.Fatalf("got %q", addr)
	}
}

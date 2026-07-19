package chainquery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func TestNormalizeRefs(t *testing.T) {
	m := normalizeRefs([]string{" Ab#1 ", "ab#1", ""})
	if len(m) != 1 {
		t.Fatalf("got %d", len(m))
	}
	if _, ok := m["ab#1"]; !ok {
		t.Fatalf("missing key: %#v", m)
	}
}

func TestFundingSyncStateEmpty(t *testing.T) {
	stale, total := fundingSyncState(nil, normalizeRefs([]string{"aa#0"}))
	if len(stale) != 0 || total != 0 {
		t.Fatalf("stale=%v total=%d", stale, total)
	}
}

func TestBlakeFromHex(t *testing.T) {
	hex32 := "e644611f706ed566e7dcc131803d2da89df72991f0a196739290553467b3af47"
	h, err := blakeFromHex(hex32)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 32 {
		t.Fatalf("len=%d", len(h))
	}
	if _, err := blakeFromHex("abcd"); err == nil {
		t.Fatal("expected error")
	}
}

func TestWaitByAssetNilProvider(t *testing.T) {
	err := WaitByAsset(context.Background(), nil, common.Address{}, AssetID{PolicyID: "aa", Name: "bb"}, WaitByAssetOpts{Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "nil provider") {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureFundingVisibleNilProvider(t *testing.T) {
	err := EnsureFundingVisible(context.Background(), nil, common.Address{}, MinActorFundingLovelace)
	if err == nil || !strings.Contains(err.Error(), "nil provider") {
		t.Fatalf("got %v", err)
	}
}

func TestSyncFundingAfterSpendNilProvider(t *testing.T) {
	err := SyncFundingAfterSpend(context.Background(), nil, common.Address{}, []string{"aa#0"})
	if err == nil || !strings.Contains(err.Error(), "nil provider") {
		t.Fatalf("got %v", err)
	}
}

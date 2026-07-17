package ops

import (
	"path/filepath"
	"testing"
)

func TestTxInspectMissingFile(t *testing.T) {
	c := New("test")
	_, err := c.TxInspect(filepath.Join(t.TempDir(), "missing.unsigned.json"))
	if err == nil {
		t.Fatal("expected error for missing envelope")
	}
}

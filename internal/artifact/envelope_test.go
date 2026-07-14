package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteUnsignedAndRead(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "tx")
	cbor := []byte{0x84, 0x00, 0x01, 0x02}
	man := Manifest{Operation: "test", Network: "preview", Provider: "utxorpc"}
	path, err := WriteUnsigned(prefix, cbor, "test tx", man)
	if err != nil {
		t.Fatal(err)
	}
	env, err := ReadEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != TypeUnwitnessedConway {
		t.Fatalf("type %q", env.Type)
	}
	raw, err := env.DecodeCBORHex()
	if err != nil || len(raw) != len(cbor) {
		t.Fatal("cbor round-trip failed")
	}
	manPath := SiblingManifestPath(path)
	if _, err := os.Stat(manPath); err != nil {
		t.Fatal(err)
	}
}

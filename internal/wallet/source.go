// Package wallet loads actor signing credentials without logging secrets.
package wallet

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Salvionied/apollo/v2"
	"github.com/blinklabs-io/bursa"
	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// Source describes how an actor signs.
type Source struct {
	Name           string
	Address        common.Address
	SigningKeyFile string
	MnemonicEnv    string
	AccountID      uint32
	AddressID      uint32
	Network        string
}

// FromActor builds a Source from config.
func FromActor(name string, a config.ActorConfig, network string) (*Source, error) {
	addr, err := common.NewAddress(a.Address)
	if err != nil {
		return nil, fmt.Errorf("actor %s: invalid address: %w", name, err)
	}
	hasKey := strings.TrimSpace(a.SigningKeyFile) != ""
	hasMn := strings.TrimSpace(a.MnemonicEnv) != ""
	if hasKey == hasMn {
		return nil, fmt.Errorf("actor %s: exactly one of signingKeyFile or mnemonicEnv required", name)
	}
	return &Source{
		Name:           name,
		Address:        addr,
		SigningKeyFile: a.SigningKeyFile,
		MnemonicEnv:    a.MnemonicEnv,
		AccountID:      a.AccountID,
		AddressID:      a.AddressID,
		Network:        network,
	}, nil
}

// LoadWallet loads a signing wallet for the actor.
func (s *Source) LoadWallet() (apollo.Wallet, error) {
	slog.Debug("Loading actor wallet", "actor", s.Name, "address", s.Address.String())
	var w apollo.Wallet
	var err error
	if s.MnemonicEnv != "" {
		w, err = s.loadMnemonic()
	} else {
		w, err = s.loadKeyFile()
	}
	if err != nil {
		slog.Error("Failed to load actor wallet", "actor", s.Name, "error", err)
		return nil, err
	}
	cred := "signingKeyFile"
	if s.MnemonicEnv != "" {
		cred = "mnemonicEnv"
	}
	slog.Info("Actor wallet loaded", "actor", s.Name, "credential", cred)
	return w, nil
}

func (s *Source) loadMnemonic() (apollo.Wallet, error) {
	mn := os.Getenv(s.MnemonicEnv)
	if mn == "" {
		return nil, fmt.Errorf("actor %s: environment variable %s is empty", s.Name, s.MnemonicEnv)
	}
	network := s.Network
	if network == "" {
		network = "preprod"
	}
	bursaNet := BursaNetwork(network)
	opts := []bursa.WalletOption{bursa.WithNetwork(bursaNet)}
	if s.AccountID != 0 || s.AddressID != 0 {
		opts = append(opts, bursa.WithAccountID(s.AccountID), bursa.WithAddressID(s.AddressID))
	}
	w, err := apollo.NewBursaWallet(mn, opts...)
	if err != nil {
		return nil, fmt.Errorf("actor %s: create wallet: %w", s.Name, err)
	}
	if w.Address().String() != s.Address.String() {
		return nil, fmt.Errorf("actor %s: derived address %s does not match configured %s", s.Name, w.Address().String(), s.Address.String())
	}
	return w, nil
}

func (s *Source) loadKeyFile() (apollo.Wallet, error) {
	env, err := ReadKeyEnvelope(s.SigningKeyFile)
	if err != nil {
		return nil, fmt.Errorf("actor %s: read key file: %w", s.Name, err)
	}
	cborBytes, err := DecodeKeyEnvelopeCBOR(env)
	if err != nil {
		return nil, fmt.Errorf("actor %s: invalid cborHex", s.Name)
	}
	// Cardano CLI payment signing key CBOR is typically a bytestring of 32, 64, 96, or 128 bytes.
	keyBytes, err := ExtractKeyBytes(cborBytes)
	if err != nil {
		return nil, fmt.Errorf("actor %s: %w", s.Name, err)
	}
	w, err := walletFromKeyBytes(s.Address, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("actor %s: %w", s.Name, err)
	}
	if w.PubKeyHash() != s.Address.PaymentKeyHash() {
		return nil, fmt.Errorf("actor %s: key does not control configured address", s.Name)
	}
	return w, nil
}

// walletFromKeyBytes builds a signing wallet from raw key material extracted from a text envelope.
//
// Supported shapes:
//   - 96-byte BIP32-Ed25519 XPrv
//   - 128-byte cardano-cli extended (priv||pub||chaincode)
//   - 32/64-byte raw ed25519 (non-HD / fixture keys)
func walletFromKeyBytes(addr common.Address, keyBytes []byte) (apollo.Wallet, error) {
	switch len(keyBytes) {
	case 96:
		w, err := apollo.NewKeyPairWallet(addr, keyBytes)
		if err != nil {
			return nil, err
		}
		return w, nil
	case 128:
		// cardano-cli extended: priv(64) || pub(32) || chainCode(32) → XPrv = priv || chainCode
		xprv := make([]byte, 96)
		copy(xprv[0:64], keyBytes[0:64])
		copy(xprv[64:96], keyBytes[96:128])
		w, err := apollo.NewKeyPairWallet(addr, xprv)
		if err != nil {
			return nil, err
		}
		return w, nil
	case 32, 64:
		return NewEd25519Wallet(addr, keyBytes)
	default:
		return nil, fmt.Errorf("unsupported signing key length %d", len(keyBytes))
	}
}

// String redacts secrets.
func (s Source) String() string {
	cred := "keyfile"
	if s.MnemonicEnv != "" {
		cred = "mnemonicEnv:" + s.MnemonicEnv
	}
	return fmt.Sprintf("wallet.Source{name:%s address:%s cred:%s}", s.Name, s.Address.String(), cred)
}

// PubKeyHashHex returns the payment key hash hex for manifests.
func PubKeyHashHex(w apollo.Wallet) string {
	return hex.EncodeToString(w.PubKeyHash().Bytes())
}

package wallet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Salvionied/apollo/v2"
	"github.com/blinklabs-io/bursa"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// WalletFormat selects which artifacts GenerateWallet writes.
type WalletFormat string

const (
	FormatKeyEnvelope WalletFormat = "key-envelope"
	FormatMnemonic    WalletFormat = "mnemonic"
	FormatBoth        WalletFormat = "both"
)

// GenerateOptions configures wallet generation.
type GenerateOptions struct {
	Name      string
	Network   string
	Format    WalletFormat
	OutDir    string
	Force     bool
	AccountID uint32
	AddressID uint32
}

// GeneratedWallet summarizes a created wallet without exposing secret material.
type GeneratedWallet struct {
	Name           string            `json:"name"`
	Network        string            `json:"network"`
	Address        string            `json:"address"`
	PaymentKeyHash string            `json:"paymentKeyHash"`
	StakeKeyHash   string            `json:"stakeKeyHash"`
	Paths          map[string]string `json:"paths"`
}

// MnemonicRecord stores a generated mnemonic and derivation metadata for Bursa reload.
type MnemonicRecord struct {
	Version        int    `json:"version"`
	Network        string `json:"network"`
	BursaNetwork   string `json:"bursaNetwork"`
	Mnemonic       string `json:"mnemonic"`
	AccountID      uint32 `json:"accountId"`
	AddressID      uint32 `json:"addressId"`
	DerivationPath string `json:"derivationPath"`
	PaymentAddress string `json:"paymentAddress"`
	PaymentKeyHash string `json:"paymentKeyHash"`
	StakeKeyHash   string `json:"stakeKeyHash"`
}

// GenerateWallet creates a new preprod wallet and writes the requested artifacts.
func GenerateWallet(opts GenerateOptions) (*GeneratedWallet, error) {
	if err := validateGenerateOptions(opts); err != nil {
		return nil, err
	}

	bursaNet := BursaNetwork(opts.Network)
	walletOpts := []bursa.WalletOption{
		bursa.WithNetwork(bursaNet),
		bursa.WithAccountID(opts.AccountID),
		bursa.WithAddressID(opts.AddressID),
	}

	apolloWallet, err := apollo.NewBursaWalletGenerate(walletOpts...)
	if err != nil {
		return nil, fmt.Errorf("generate wallet: %w", err)
	}

	if err := ensureOutDir(opts); err != nil {
		return nil, err
	}

	material, err := deriveKeyMaterial(apolloWallet.Mnemonic(), bursaNet, opts.AccountID, opts.AddressID)
	if err != nil {
		return nil, err
	}

	if err := material.matches(apolloWallet); err != nil {
		return nil, err
	}

	paths := map[string]string{}
	switch opts.Format {
	case FormatKeyEnvelope:
		if err := writeKeyEnvelopeArtifacts(opts.OutDir, material, paths); err != nil {
			return nil, err
		}
		if err := roundTripSigner(opts.Name, opts.Network, opts.OutDir, apolloWallet.Address()); err != nil {
			return nil, err
		}
	case FormatMnemonic:
		path, err := writeMnemonicArtifact(opts.OutDir, opts.Network, bursaNet, apolloWallet.Mnemonic(), opts.AccountID, opts.AddressID, material)
		if err != nil {
			return nil, err
		}
		paths["mnemonic.json"] = path
		if err := attachMnemonicPhrasePath(opts.OutDir, paths); err != nil {
			return nil, err
		}
		if err := roundTripMnemonic(apolloWallet.Mnemonic(), walletOpts, apolloWallet.Address()); err != nil {
			return nil, err
		}
	case FormatBoth:
		if err := writeKeyEnvelopeArtifacts(opts.OutDir, material, paths); err != nil {
			return nil, err
		}
		path, err := writeMnemonicArtifact(opts.OutDir, opts.Network, bursaNet, apolloWallet.Mnemonic(), opts.AccountID, opts.AddressID, material)
		if err != nil {
			return nil, err
		}
		paths["mnemonic.json"] = path
		if err := attachMnemonicPhrasePath(opts.OutDir, paths); err != nil {
			return nil, err
		}
		if err := verifyMnemonicMatchesKeys(apolloWallet.Mnemonic(), walletOpts, material); err != nil {
			return nil, err
		}
		if err := roundTripSigner(opts.Name, opts.Network, opts.OutDir, apolloWallet.Address()); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported wallet format %q", opts.Format)
	}

	return &GeneratedWallet{
		Name:           opts.Name,
		Network:        opts.Network,
		Address:        apolloWallet.Address().String(),
		PaymentKeyHash: hex.EncodeToString(apolloWallet.PubKeyHash().Bytes()),
		StakeKeyHash:   hex.EncodeToString(apolloWallet.StakePubKeyHash().Bytes()),
		Paths:          paths,
	}, nil
}

// BursaNetwork maps dns-cli network names to bursa/gouroboros network identifiers.
func BursaNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet":
		return "mainnet"
	case "preprod", "testnet":
		return "preprod"
	case "preview":
		return "preview"
	default:
		return strings.ToLower(strings.TrimSpace(network))
	}
}

func validateGenerateOptions(opts GenerateOptions) error {
	if strings.TrimSpace(opts.Name) == "" {
		return fmt.Errorf("wallet name is required")
	}
	if opts.Network != "preprod" {
		return fmt.Errorf("wallet generation supports preprod only (got %q)", opts.Network)
	}
	switch opts.Format {
	case FormatKeyEnvelope, FormatMnemonic, FormatBoth:
	default:
		return fmt.Errorf("invalid wallet format %q (want key-envelope|mnemonic|both)", opts.Format)
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return fmt.Errorf("output directory is required")
	}
	return nil
}

func ensureOutDir(opts GenerateOptions) error {
	if err := os.MkdirAll(opts.OutDir, 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	marker := filepath.Join(opts.OutDir, "payment.skey")
	if _, err := os.Stat(marker); err == nil && !opts.Force {
		return fmt.Errorf("wallet artifacts already exist in %s (use --force to overwrite)", opts.OutDir)
	}
	if opts.Format == FormatMnemonic || opts.Format == FormatBoth {
		mnPath := filepath.Join(opts.OutDir, "mnemonic.json")
		if _, err := os.Stat(mnPath); err == nil && !opts.Force {
			return fmt.Errorf("wallet artifacts already exist in %s (use --force to overwrite)", opts.OutDir)
		}
		phrasePath := filepath.Join(opts.OutDir, "mnemonic.phrase")
		if _, err := os.Stat(phrasePath); err == nil && !opts.Force {
			return fmt.Errorf("wallet artifacts already exist in %s (use --force to overwrite)", opts.OutDir)
		}
	}
	return nil
}

func attachMnemonicPhrasePath(outDir string, paths map[string]string) error {
	phrasePath := filepath.Join(outDir, "mnemonic.phrase")
	if _, err := os.Stat(phrasePath); err != nil {
		return fmt.Errorf("missing mnemonic.phrase after mnemonic write: %w", err)
	}
	paths["mnemonic.phrase"] = phrasePath
	return nil
}

type keyMaterial struct {
	address     common.Address
	paymentSKey bursa.KeyFile
	paymentVKey bursa.KeyFile
	stakeSKey   bursa.KeyFile
	stakeVKey   bursa.KeyFile
}

func deriveKeyMaterial(mnemonic, bursaNet string, accountID, addressID uint32) (*keyMaterial, error) {
	rootKey, err := bursa.GetRootKeyFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("derive root key: %w", err)
	}
	accountKey, err := bursa.GetAccountKey(rootKey, accountID)
	if err != nil {
		return nil, fmt.Errorf("derive account key: %w", err)
	}
	paymentKey, err := bursa.GetPaymentKey(accountKey, addressID)
	if err != nil {
		return nil, fmt.Errorf("derive payment key: %w", err)
	}
	stakeKey, err := bursa.GetStakeKey(accountKey, addressID)
	if err != nil {
		return nil, fmt.Errorf("derive stake key: %w", err)
	}
	addr, err := bursa.GetAddress(accountKey, bursaNet, addressID)
	if err != nil {
		return nil, fmt.Errorf("derive address: %w", err)
	}
	// Prefer extended signing keys so LoadWallet can reconstruct the BIP32 XPrv.
	// Non-extended Shelley skeys store only k_L, which is not a Go ed25519 seed.
	paymentSKey, err := bursa.GetPaymentExtendedSKey(paymentKey)
	if err != nil {
		return nil, fmt.Errorf("create payment signing key: %w", err)
	}
	paymentVKey, err := bursa.GetPaymentVKey(paymentKey)
	if err != nil {
		return nil, fmt.Errorf("create payment verification key: %w", err)
	}
	stakeSKey, err := bursa.GetStakeExtendedSKey(stakeKey)
	if err != nil {
		return nil, fmt.Errorf("create stake signing key: %w", err)
	}
	stakeVKey, err := bursa.GetStakeVKey(stakeKey)
	if err != nil {
		return nil, fmt.Errorf("create stake verification key: %w", err)
	}
	return &keyMaterial{
		address:     *addr,
		paymentSKey: paymentSKey,
		paymentVKey: paymentVKey,
		stakeSKey:   stakeSKey,
		stakeVKey:   stakeVKey,
	}, nil
}

func (m *keyMaterial) matches(w *apollo.BursaWallet) error {
	if m.address.String() != w.Address().String() {
		return fmt.Errorf("derived address %s does not match wallet address %s", m.address.String(), w.Address().String())
	}
	if m.address.PaymentKeyHash() != w.PubKeyHash() {
		return fmt.Errorf("derived payment key hash does not match wallet payment key hash")
	}
	if m.address.StakeKeyHash() != w.StakePubKeyHash() {
		return fmt.Errorf("derived stake key hash does not match wallet stake key hash")
	}
	return nil
}

func writeKeyEnvelopeArtifacts(outDir string, material *keyMaterial, paths map[string]string) error {
	files := []struct {
		name string
		kf   bursa.KeyFile
	}{
		{"payment.skey", material.paymentSKey},
		{"payment.vkey", material.paymentVKey},
		{"stake.skey", material.stakeSKey},
		{"stake.vkey", material.stakeVKey},
	}
	for _, file := range files {
		path := filepath.Join(outDir, file.name)
		if err := WriteBursaKeyFile(path, file.kf); err != nil {
			return fmt.Errorf("write %s: %w", file.name, err)
		}
		paths[file.name] = path
	}

	addrPath := filepath.Join(outDir, "payment.addr")
	if err := os.WriteFile(addrPath, []byte(material.address.String()+"\n"), 0o644); err != nil {
		return fmt.Errorf("write payment.addr: %w", err)
	}
	paths["payment.addr"] = addrPath
	return nil
}

func writeMnemonicArtifact(outDir, network, bursaNet, mnemonic string, accountID, addressID uint32, material *keyMaterial) (string, error) {
	record := MnemonicRecord{
		Version:        1,
		Network:        network,
		BursaNetwork:   bursaNet,
		Mnemonic:       mnemonic,
		AccountID:      accountID,
		AddressID:      addressID,
		DerivationPath: fmt.Sprintf("m/1852'/1815'/%d'/%d/%d", accountID, 0, addressID),
		PaymentAddress: material.address.String(),
		PaymentKeyHash: hex.EncodeToString(material.address.PaymentKeyHash().Bytes()),
		StakeKeyHash:   hex.EncodeToString(material.address.StakeKeyHash().Bytes()),
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal mnemonic record: %w", err)
	}
	raw = append(raw, '\n')
	path := filepath.Join(outDir, "mnemonic.json")
	if err := writeSecretFile(path, raw); err != nil {
		return "", fmt.Errorf("write mnemonic.json: %w", err)
	}
	phrasePath := filepath.Join(outDir, "mnemonic.phrase")
	if err := writeSecretFile(phrasePath, []byte(strings.TrimSpace(mnemonic)+"\n")); err != nil {
		return "", fmt.Errorf("write mnemonic.phrase: %w", err)
	}
	return path, nil
}

func verifyMnemonicMatchesKeys(mnemonic string, walletOpts []bursa.WalletOption, material *keyMaterial) error {
	reloaded, err := apollo.NewBursaWallet(mnemonic, walletOpts...)
	if err != nil {
		return fmt.Errorf("reload mnemonic wallet: %w", err)
	}
	if err := material.matches(reloaded); err != nil {
		return fmt.Errorf("mnemonic and key-envelope mismatch: %w", err)
	}
	return nil
}

func roundTripMnemonic(mnemonic string, walletOpts []bursa.WalletOption, addr common.Address) error {
	w, err := apollo.NewBursaWallet(mnemonic, walletOpts...)
	if err != nil {
		return fmt.Errorf("round-trip mnemonic wallet: %w", err)
	}
	if w.Address().String() != addr.String() {
		return fmt.Errorf("round-trip address mismatch: got %s want %s", w.Address().String(), addr.String())
	}
	if w.PubKeyHash() != addr.PaymentKeyHash() {
		return fmt.Errorf("round-trip payment key hash mismatch")
	}
	return nil
}

func roundTripSigner(name, network, outDir string, addr common.Address) error {
	paymentSKey := filepath.Join(outDir, "payment.skey")
	if _, err := os.Stat(paymentSKey); err != nil {
		return nil
	}
	src := &Source{
		Name:           name,
		Address:        addr,
		SigningKeyFile: paymentSKey,
		Network:        network,
	}
	w, err := src.LoadWallet()
	if err != nil {
		return fmt.Errorf("round-trip load wallet: %w", err)
	}
	if w.Address().String() != addr.String() {
		return fmt.Errorf("round-trip address mismatch: got %s want %s", w.Address().String(), addr.String())
	}
	if w.PubKeyHash() != addr.PaymentKeyHash() {
		return fmt.Errorf("round-trip payment key hash mismatch")
	}
	return nil
}

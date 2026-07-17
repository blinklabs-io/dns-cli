package ops

import "github.com/blinklabs-io/dns-cli/internal/domain"

// ProofGenerate writes registrar/owner HNS keys and a proof bundle.
func (c *Client) ProofGenerate(tld, outDir, registrarKey, ownerKey string) (domain.ProofBundleOutput, error) {
	return domain.GenerateProofBundle(tld, outDir, registrarKey, ownerKey)
}

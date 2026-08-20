package ops

import "github.com/blinklabs-io/dns-cli/internal/domain"

// ProofGenerate writes an owner HNS key and a proof bundle.
func (c *Client) ProofGenerate(tld, outDir, ownerKey string) (domain.ProofBundleOutput, error) {
	return domain.GenerateProofBundle(tld, outDir, ownerKey)
}

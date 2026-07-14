package provider

import (
	"github.com/Salvionied/apollo/v2/backend/blockfrost"
	"github.com/blinklabs-io/dns-cli/internal/config"
)

func newBlockfrost(eff *config.Effective) (Provider, error) {
	baseURL, err := resolveBaseURL(eff.Profile.Provider)
	if err != nil {
		return nil, err
	}
	projectID, err := requireEnv(eff.Profile.Provider.ProjectIDEnv)
	if err != nil {
		return nil, err
	}
	cc := blockfrost.NewBlockFrostChainContext(
		baseURL,
		eff.Profile.Network.ID,
		projectID,
	)
	poll, err := eff.Profile.Transaction.PollIntervalDuration()
	if err != nil {
		return nil, err
	}
	return &wrapped{ChainContext: cc, name: "blockfrost", pollInterval: poll}, nil
}

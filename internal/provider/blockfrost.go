package provider

import (
	"net/http"
	"time"

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
	return &blockfrostProvider{
		wrapped:    &wrapped{ChainContext: cc, name: "blockfrost", pollInterval: poll},
		baseURL:    baseURL,
		projectID:  projectID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

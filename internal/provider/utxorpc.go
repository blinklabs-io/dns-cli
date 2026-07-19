package provider

import (
	"fmt"
	"strings"

	"github.com/Salvionied/apollo/v2/backend/utxorpc"
	"github.com/blinklabs-io/dns-cli/internal/config"
	sdk "github.com/utxorpc/go-sdk"
)

func newUtxoRPC(eff *config.Effective) (Provider, error) {
	baseURL, err := resolveBaseURL(eff.Profile.Provider)
	if err != nil {
		return nil, err
	}
	headers, err := resolveUtxoRPCHeaders(eff.Profile.Provider.HeadersEnv)
	if err != nil {
		return nil, err
	}
	opts := []sdk.ClientOption{sdk.WithBaseUrl(baseURL)}
	if len(headers) > 0 {
		opts = append(opts, sdk.WithHeaders(headers))
	}
	client := sdk.NewClient(opts...)

	cc := utxorpc.NewUtxoRpcChainContext(
		baseURL,
		eff.Profile.Network.ID,
		headers,
	)
	poll, err := eff.Profile.Transaction.PollIntervalDuration()
	if err != nil {
		return nil, err
	}
	return &utxorpcProvider{
		wrapped: &wrapped{ChainContext: cc, name: "utxorpc", pollInterval: poll},
		client:  client,
	}, nil
}

// resolveBaseURL picks exactly one usable URL source from provider config.
func resolveBaseURL(p config.ProviderConfig) (string, error) {
	base := strings.TrimSpace(p.BaseURL)
	envName := strings.TrimSpace(p.BaseURLEnv)
	if base != "" {
		return base, nil
	}
	if envName != "" {
		return requireEnv(envName)
	}
	return "", fmt.Errorf("provider: baseURL or baseUrlEnv is required")
}

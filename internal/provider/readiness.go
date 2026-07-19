package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/blinklabs-io/dns-cli/internal/config"
)

// ErrConfig is returned for malformed provider settings or missing required
// environment variables (CLI maps this to ExitConfig).
var ErrConfig = errors.New("provider config")

// ErrHealth is returned when provider construction or Health fails
// (CLI maps this to ExitProvider).
var ErrHealth = errors.New("provider health")

// CredentialReadiness describes one credential / env-backed secret check.
type CredentialReadiness struct {
	Name     string
	Required bool
	Present  bool
}

// Readiness is a secret-safe summary of provider connectivity readiness.
type Readiness struct {
	Provider       string
	Network        string
	EndpointHost   string
	EndpointSource string
	Credentials    []CredentialReadiness
	Healthy        bool
}

// healthChecker is the subset of Provider needed for readiness.
type healthChecker interface {
	Name() string
	Health(ctx context.Context) error
}

// ReadinessChecker runs credential and health checks with an injectable provider factory.
type ReadinessChecker struct {
	NewProvider func(*config.Effective) (healthChecker, error)
}

// CheckReadiness validates credentials and live provider health for eff.
func CheckReadiness(ctx context.Context, eff *config.Effective) (Readiness, error) {
	return ReadinessChecker{
		NewProvider: func(e *config.Effective) (healthChecker, error) { return New(e) },
	}.Check(ctx, eff)
}

// Check inspects provider configuration, required credentials, and Health.
func (c ReadinessChecker) Check(ctx context.Context, eff *config.Effective) (Readiness, error) {
	if eff == nil {
		return Readiness{}, fmt.Errorf("%w: nil effective config", ErrConfig)
	}
	newProv := c.NewProvider
	if newProv == nil {
		newProv = func(e *config.Effective) (healthChecker, error) { return New(e) }
	}

	pType := strings.ToLower(strings.TrimSpace(eff.Profile.Provider.Type))
	out := Readiness{
		Provider: pType,
		Network:  eff.Profile.Network.Name,
	}

	endpoint, source, err := resolveEndpointMeta(eff.Profile.Provider)
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	host, err := endpointHost(endpoint)
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	out.EndpointHost = host
	out.EndpointSource = source

	creds, missing, err := credentialRows(eff.Profile.Provider, host)
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	out.Credentials = creds
	if missing != "" {
		return out, fmt.Errorf("%w: environment variable %s is required", ErrConfig, missing)
	}

	prov, err := newProv(eff)
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrHealth, err)
	}
	if err := prov.Health(ctx); err != nil {
		return out, fmt.Errorf("%w (%s): %v", ErrHealth, pType, err)
	}
	out.Healthy = true
	return out, nil
}

func resolveEndpointMeta(p config.ProviderConfig) (endpoint, source string, err error) {
	base := strings.TrimSpace(p.BaseURL)
	envName := strings.TrimSpace(p.BaseURLEnv)
	switch {
	case base != "" && envName != "":
		return "", "", fmt.Errorf("exactly one of baseURL or baseUrlEnv is required")
	case base != "":
		return base, "baseURL", nil
	case envName != "":
		val := strings.TrimSpace(os.Getenv(envName))
		if val == "" {
			return "", envName, fmt.Errorf("environment variable %s is required", envName)
		}
		return val, envName, nil
	default:
		return "", "", fmt.Errorf("exactly one of baseURL or baseUrlEnv is required")
	}
}

func endpointHost(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		// url.Parse accepts many strings; require a usable host.
		u2, err2 := url.ParseRequestURI(raw)
		if err2 != nil || u2.Hostname() == "" {
			return "", fmt.Errorf("invalid provider endpoint URL")
		}
		u = u2
	}
	host := u.Hostname()
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return host + ":" + port, nil
	}
	return host, nil
}

func credentialRows(p config.ProviderConfig, host string) ([]CredentialReadiness, string, error) {
	switch strings.ToLower(strings.TrimSpace(p.Type)) {
	case "blockfrost":
		name := strings.TrimSpace(p.ProjectIDEnv)
		if name == "" {
			return nil, "", fmt.Errorf("projectIdEnv is required for blockfrost")
		}
		present := strings.TrimSpace(os.Getenv(name)) != ""
		row := CredentialReadiness{Name: name, Required: true, Present: present}
		if !present {
			return []CredentialReadiness{row}, name, nil
		}
		return []CredentialReadiness{row}, "", nil
	case "utxorpc":
		var rows []CredentialReadiness
		if envName := strings.TrimSpace(p.BaseURLEnv); envName != "" {
			// Presence already enforced in resolveEndpointMeta; still report it.
			rows = append(rows, CredentialReadiness{
				Name:     envName,
				Required: true,
				Present:  strings.TrimSpace(os.Getenv(envName)) != "",
			})
		}
		headersEnv := strings.TrimSpace(p.HeadersEnv)
		headersPresent := headersEnv != "" && strings.TrimSpace(os.Getenv(headersEnv)) != ""
		dmtrPresent := strings.TrimSpace(os.Getenv(dmtrAPIKeyEnv)) != ""
		authRequired := headersEnv != "" || isDemeterHost(host)
		if headersEnv != "" {
			rows = append(rows, CredentialReadiness{
				Name:     headersEnv,
				Required: authRequired && !dmtrPresent,
				Present:  headersPresent,
			})
		}
		rows = append(rows, CredentialReadiness{
			Name:     dmtrAPIKeyEnv,
			Required: authRequired && !headersPresent,
			Present:  dmtrPresent,
		})
		if authRequired && !headersPresent && !dmtrPresent {
			missing := dmtrAPIKeyEnv
			if headersEnv != "" {
				missing = headersEnv + " or " + dmtrAPIKeyEnv
			}
			return rows, missing, nil
		}
		return rows, "", nil
	default:
		return nil, "", fmt.Errorf("unsupported provider type %q", p.Type)
	}
}

func isDemeterHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return strings.Contains(h, "demeter") || strings.Contains(h, "dmtr")
}

// IsConfigError reports whether err is a readiness config/credential failure.
func IsConfigError(err error) bool {
	return errors.Is(err, ErrConfig)
}

// IsHealthError reports whether err is a readiness provider health failure.
func IsHealthError(err error) bool {
	return errors.Is(err, ErrHealth)
}

package txbuilder

import (
	"fmt"

	"github.com/blinklabs-io/dns-cli/internal/config"
	"github.com/blinklabs-io/dns-cli/internal/wallet"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// actorEvaluationWitnessProvider supplies Apollo evaluation-only witnesses for
// ExternalWallet builds. Final signing still happens later via dns-cli wallets;
// these witnesses are discarded after ExUnit estimation.
type actorEvaluationWitnessProvider struct {
	eff *config.Effective
}

func (p actorEvaluationWitnessProvider) EvaluationWitnesses(
	txBodyHash common.Blake2b256,
	requiredSigners []common.Blake2b224,
) ([]common.VkeyWitness, error) {
	if p.eff == nil {
		return nil, fmt.Errorf("evaluation witness provider has no config")
	}
	witnesses := make([]common.VkeyWitness, 0, len(requiredSigners))
	for _, requiredHash := range requiredSigners {
		witness, err := p.signAsConfiguredActor(txBodyHash, requiredHash)
		if err != nil {
			return nil, err
		}
		witnesses = append(witnesses, witness)
	}
	return witnesses, nil
}

func (p actorEvaluationWitnessProvider) signAsConfiguredActor(
	txBodyHash common.Blake2b256,
	requiredHash common.Blake2b224,
) (common.VkeyWitness, error) {
	for actorName, actor := range p.eff.Profile.Actors {
		addr, err := common.NewAddress(actor.Address)
		if err != nil || addr.PaymentKeyHash() != requiredHash {
			continue
		}
		source, err := wallet.FromActor(actorName, actor, p.eff.Profile.Network.Name)
		if err != nil {
			return common.VkeyWitness{}, fmt.Errorf("load evaluation signer %s: %w", actorName, err)
		}
		signer, err := source.LoadWallet()
		if err != nil {
			return common.VkeyWitness{}, fmt.Errorf("load evaluation signer %s: %w", actorName, err)
		}
		witness, err := signer.SignTxBody(txBodyHash)
		if err != nil {
			return common.VkeyWitness{}, fmt.Errorf("sign evaluation tx as %s: %w", actorName, err)
		}
		return witness, nil
	}
	return common.VkeyWitness{}, fmt.Errorf("no configured actor matches required signer %s", requiredHash.String())
}

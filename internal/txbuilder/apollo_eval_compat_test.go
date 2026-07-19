package txbuilder

import (
	apollo "github.com/Salvionied/apollo/v2"
)

// Ensures the provider type continues to satisfy Apollo's optional interface
// after the dns-cli EvalTx workarounds were removed.
var _ apollo.EvaluationWitnessProvider = actorEvaluationWitnessProvider{}

package provider

import (
	"encoding/json"
	"testing"
)

func TestMergeCostModelsPreferRaw(t *testing.T) {
	named := map[string][]int64{
		"PlutusV3": {1, 2, 3},
	}
	rawJSON := []byte(`{"PlutusV1":[10],"PlutusV2":[20,21],"PlutusV3":[30,31,32,33]}`)
	got, err := mergeCostModelsPreferRaw(named, rawJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["PlutusV3"]) != 4 {
		t.Fatalf("want raw PlutusV3 len 4, got %d", len(got["PlutusV3"]))
	}
	if got["PlutusV3"][0] != 30 {
		t.Fatalf("want raw values, got %v", got["PlutusV3"])
	}
}

func TestMergeCostModelsFallsBackToNamed(t *testing.T) {
	named := map[string][]int64{"PlutusV3": {1, 2, 3}}
	got, err := mergeCostModelsPreferRaw(named, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["PlutusV3"]) != 3 {
		t.Fatalf("got %v", got["PlutusV3"])
	}
}

func TestParseEpochParamsCostModelsRaw(t *testing.T) {
	body := []byte(`{
		"cost_models": {"PlutusV3": {"addInteger-cpu-arguments-intercept": 1}},
		"cost_models_raw": {"PlutusV3": [100, 200, 300]}
	}`)
	var raw struct {
		CostModels    json.RawMessage `json:"cost_models"`
		CostModelsRaw json.RawMessage `json:"cost_models_raw"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	got, err := mergeCostModelsPreferRaw(nil, raw.CostModelsRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["PlutusV3"]) != 3 || got["PlutusV3"][1] != 200 {
		t.Fatalf("got %#v", got)
	}
}

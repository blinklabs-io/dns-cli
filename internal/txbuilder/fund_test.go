package txbuilder

import (
	"strings"
	"testing"
)

func TestParseFundAllocation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		want    FundAllocation
		wantErr string
	}{
		{
			name: "valid",
			raw:  "registrar=30000000",
			want: FundAllocation{Actor: "registrar", Lovelace: 30_000_000},
		},
		{
			name: "trim spaces",
			raw:  "  tldOwner = 50000000  ",
			want: FundAllocation{Actor: "tldOwner", Lovelace: 50_000_000},
		},
		{
			name:    "missing equals",
			raw:     "registrar30000000",
			wantErr: "want actor=lovelace",
		},
		{
			name:    "empty actor",
			raw:     "=30000000",
			wantErr: "empty actor",
		},
		{
			name:    "empty amount",
			raw:     "registrar=",
			wantErr: "empty lovelace",
		},
		{
			name:    "non integer",
			raw:     "registrar=abc",
			wantErr: "integer",
		},
		{
			name:    "zero",
			raw:     "registrar=0",
			wantErr: "positive",
		},
		{
			name:    "negative",
			raw:     "registrar=-1",
			wantErr: "positive",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseFundAllocation(tc.raw)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestValidateFundAllocations(t *testing.T) {
	t.Parallel()
	actors := map[string]string{
		"registrar": "addr_test1_reg",
		"tldOwner":  "addr_test1_tld",
		"sldOwner":  "addr_test1_sld",
	}
	tests := []struct {
		name        string
		allocations []FundAllocation
		collateral  int64
		actors      map[string]string
		wantErr     string
	}{
		{
			name: "ok three actors",
			allocations: []FundAllocation{
				{Actor: "registrar", Lovelace: 30_000_000},
				{Actor: "tldOwner", Lovelace: 50_000_000},
				{Actor: "sldOwner", Lovelace: 30_000_000},
			},
			collateral: 5_000_000,
			actors:     actors,
		},
		{
			name:        "empty list",
			allocations: nil,
			collateral:  5_000_000,
			actors:      actors,
			wantErr:     "at least one",
		},
		{
			name: "allocation equals collateral",
			allocations: []FundAllocation{
				{Actor: "registrar", Lovelace: 5_000_000},
			},
			collateral: 5_000_000,
			actors:     actors,
			wantErr:    "greater than collateral",
		},
		{
			name: "allocation below collateral",
			allocations: []FundAllocation{
				{Actor: "registrar", Lovelace: 4_000_000},
			},
			collateral: 5_000_000,
			actors:     actors,
			wantErr:    "greater than collateral",
		},
		{
			name: "duplicate actor",
			allocations: []FundAllocation{
				{Actor: "registrar", Lovelace: 30_000_000},
				{Actor: "registrar", Lovelace: 40_000_000},
			},
			collateral: 5_000_000,
			actors:     actors,
			wantErr:    "duplicate allocation actor",
		},
		{
			name: "duplicate address",
			allocations: []FundAllocation{
				{Actor: "registrar", Lovelace: 30_000_000},
				{Actor: "tldOwner", Lovelace: 40_000_000},
			},
			collateral: 5_000_000,
			actors: map[string]string{
				"registrar": "addr_same",
				"tldOwner":  "addr_same",
			},
			wantErr: "duplicate destination address",
		},
		{
			name: "unknown actor",
			allocations: []FundAllocation{
				{Actor: "ghost", Lovelace: 30_000_000},
			},
			collateral: 5_000_000,
			actors:     actors,
			wantErr:    "unknown actor",
		},
		{
			name: "non positive collateral",
			allocations: []FundAllocation{
				{Actor: "registrar", Lovelace: 30_000_000},
			},
			collateral: 0,
			actors:     actors,
			wantErr:    "collateral must be positive",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFundAllocations(tc.allocations, tc.collateral, tc.actors)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFundOutputPlanOrder(t *testing.T) {
	t.Parallel()
	// Document expected collateral/spend pairing without building a chain tx.
	allocations := []FundAllocation{
		{Actor: "registrar", Lovelace: 30_000_000},
		{Actor: "tldOwner", Lovelace: 50_000_000},
		{Actor: "sldOwner", Lovelace: 30_000_000},
	}
	const collateral int64 = 5_000_000
	var total int64
	var roles []string
	for _, alloc := range allocations {
		if alloc.Lovelace <= collateral {
			t.Fatalf("allocation %s invalid", alloc.Actor)
		}
		spend := alloc.Lovelace - collateral
		total += collateral + spend
		roles = append(roles, alloc.Actor+"-collateral", alloc.Actor+"-spend")
	}
	if total != 110_000_000 {
		t.Fatalf("total outputs %d want 110000000", total)
	}
	wantRoles := []string{
		"registrar-collateral", "registrar-spend",
		"tldOwner-collateral", "tldOwner-spend",
		"sldOwner-collateral", "sldOwner-spend",
	}
	if len(roles) != len(wantRoles) {
		t.Fatalf("role count %d want %d", len(roles), len(wantRoles))
	}
	for i := range wantRoles {
		if roles[i] != wantRoles[i] {
			t.Fatalf("role[%d]=%s want %s", i, roles[i], wantRoles[i])
		}
	}
}

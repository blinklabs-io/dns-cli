package demo

import (
	"path/filepath"
	"strings"
	"testing"
)

func writeResumeFixture(t *testing.T, root, tld, sld, runID, provider string, tldConfirmed, sldConfirmed map[string]string, bind bool) {
	t.Helper()
	tldDir := filepath.Join(root, tld)
	mustMkdir(t, tldDir)
	mustWrite(t, filepath.Join(tldDir, "state.json"), `{
		"schemaVersion": 2,
		"mode": "fresh",
		"network": "preprod",
		"provider": "`+provider+`",
		"tld": "`+tld+`",
		"confirmed": {
			"fund": {"txId": "`+tldConfirmed["fund"]+`", "manifest": "m-fund"},
			"deploy": {"txId": "`+tldConfirmed["deploy"]+`", "manifest": "m-deploy"},
			"register": {"txId": "`+tldConfirmed["register"]+`", "manifest": "m-register"},
			"activate": {"txId": "`+tldConfirmed["activate"]+`", "manifest": "m-activate"}
		}
	}`)
	if bind {
		mustMkdir(t, filepath.Join(tldDir, "config"))
		mustWrite(t, filepath.Join(tldDir, "config", provider+".json"), `{"version":1}`)
	}
	runDir := filepath.Join(tldDir, sld, runID)
	mustMkdir(t, runDir)
	mustWrite(t, filepath.Join(runDir, "state.json"), `{
		"schemaVersion": 2,
		"mode": "fresh",
		"network": "preprod",
		"provider": "`+provider+`",
		"tld": "`+tld+`",
		"sld": "`+sld+`",
		"runId": "`+runID+`",
		"confirmed": {
			"mintSld": {"txId": "`+sldConfirmed["mintSld"]+`", "manifest": "m-mint"},
			"updateSld": {"txId": "`+sldConfirmed["updateSld"]+`", "manifest": "m-update"}
		}
	}`)
}

func TestDeriveResumeStages(t *testing.T) {
	cases := []struct {
		name      string
		tld       map[string]string
		sld       map[string]string
		bind      bool
		want      ResumeStage
		resumable bool
	}{
		{"fund", map[string]string{}, map[string]string{}, false, StageFund, true},
		{"deploy", map[string]string{"fund": "aaa"}, map[string]string{}, false, StageDeploy, true},
		{"bind", map[string]string{"fund": "aaa", "deploy": "bbb"}, map[string]string{}, false, StageBind, true},
		{"register", map[string]string{"fund": "aaa", "deploy": "bbb"}, map[string]string{}, true, StageRegister, true},
		{"activate", map[string]string{"fund": "a", "deploy": "b", "register": "c"}, map[string]string{}, true, StageActivate, true},
		{"mint", map[string]string{"fund": "a", "deploy": "b", "register": "c", "activate": "d"}, map[string]string{}, true, StageMintSLD, true},
		{"update", map[string]string{"fund": "a", "deploy": "b", "register": "c", "activate": "d"}, map[string]string{"mintSld": "e"}, true, StageUpdateSLD, true},
		{"complete", map[string]string{"fund": "a", "deploy": "b", "register": "c", "activate": "d"}, map[string]string{"mintSld": "e", "updateSld": "f"}, true, StageComplete, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeResumeFixture(t, root, "alpha", "www", "20260101-120000", "blockfrost", tc.tld, tc.sld, tc.bind)
			cat, err := ReadResumeCatalog(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(cat) != 1 {
				t.Fatalf("want 1 entry, got %d", len(cat))
			}
			if cat[0].Stage != tc.want || cat[0].Resumable != tc.resumable {
				t.Fatalf("got stage=%s resumable=%v want %s/%v", cat[0].Stage, cat[0].Resumable, tc.want, tc.resumable)
			}
		})
	}
}

func TestReadResumeCatalogSortAndSkip(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "shared"))
	mustMkdir(t, filepath.Join(root, "states"))
	writeResumeFixture(t, root, "zeta", "www", "20260102-120000", "blockfrost",
		map[string]string{"fund": "a"}, map[string]string{}, false)
	writeResumeFixture(t, root, "alpha", "app", "20260101-120000", "utxorpc",
		map[string]string{"fund": "a", "deploy": "b"}, map[string]string{}, true)
	writeResumeFixture(t, root, "alpha", "www", "20260103-120000", "utxorpc",
		map[string]string{"fund": "a", "deploy": "b", "register": "c", "activate": "d"},
		map[string]string{"mintSld": "e", "updateSld": "f"}, true)

	cat, err := ReadResumeCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 3 {
		t.Fatalf("want 3, got %d", len(cat))
	}
	wantOrder := []string{"app.alpha", "www.alpha", "www.zeta"}
	for i, want := range wantOrder {
		got := cat[i].SLD + "." + cat[i].TLD
		if got != want {
			t.Fatalf("order[%d]=%s want %s", i, got, want)
		}
	}
	if cat[1].Stage != StageComplete || cat[1].Resumable {
		t.Fatalf("complete entry: %#v", cat[1])
	}
}

func TestReadResumeCatalogConflicts(t *testing.T) {
	root := t.TempDir()
	tldDir := filepath.Join(root, "alpha")
	mustMkdir(t, tldDir)
	mustWrite(t, filepath.Join(tldDir, "state.json"), `{
		"schemaVersion": 2, "mode":"fresh","network":"preprod","provider":"blockfrost","tld":"alpha",
		"confirmed":{"fund":{"txId":"a","manifest":""},"deploy":{"txId":"","manifest":""},"register":{"txId":"","manifest":""},"activate":{"txId":"","manifest":""}}
	}`)
	runDir := filepath.Join(tldDir, "www", "20260101-120000")
	mustMkdir(t, runDir)
	mustWrite(t, filepath.Join(runDir, "state.json"), `{
		"schemaVersion": 2, "mode":"fresh","network":"preprod","provider":"utxorpc","tld":"alpha","sld":"www","runId":"20260101-120000",
		"confirmed":{"mintSld":{"txId":"","manifest":""},"updateSld":{"txId":"","manifest":""}}
	}`)
	_, err := ReadResumeCatalog(root)
	if err == nil || !strings.Contains(err.Error(), "provider conflict") {
		t.Fatalf("want provider conflict, got %v", err)
	}
}

func TestFormatResumeCatalogNoExplorer(t *testing.T) {
	root := t.TempDir()
	writeResumeFixture(t, root, "alpha", "www", "20260101-120000", "blockfrost",
		map[string]string{"fund": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
		map[string]string{}, false)
	cat, err := ReadResumeCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	out := FormatResumeCatalog(cat, false)
	for _, bad := range []string{"explorer", "http", "deadbeef", "manifest", "m-fund"} {
		if strings.Contains(strings.ToLower(out), bad) && bad != "manifest" {
			if strings.Contains(out, bad) {
				t.Fatalf("formatter leaked %q:\n%s", bad, out)
			}
		}
	}
	if strings.Contains(out, "deadbeef") || strings.Contains(out, "explorer") || strings.Contains(out, "http") {
		t.Fatalf("formatter leaked tx/explorer:\n%s", out)
	}
	// Fund is already confirmed in the fixture, so the next resume stage is deploy.
	if !strings.Contains(out, "www.alpha") || !strings.Contains(out, "deploy") {
		t.Fatalf("unexpected:\n%s", out)
	}
}

func TestReadResumeCatalogBadSchema(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "bad"))
	mustWrite(t, filepath.Join(root, "bad", "state.json"), `{"schemaVersion":1,"mode":"fresh","network":"preprod","provider":"blockfrost","tld":"bad","confirmed":{"fund":{"txId":"","manifest":""},"deploy":{"txId":"","manifest":""},"register":{"txId":"","manifest":""},"activate":{"txId":"","manifest":""}}}`)
	_, err := ReadResumeCatalog(root)
	if err == nil {
		t.Fatal("expected schema error")
	}
}

func TestReadResumeCatalogEmpty(t *testing.T) {
	cat, err := ReadResumeCatalog(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 0 {
		t.Fatalf("want empty, got %#v", cat)
	}
}

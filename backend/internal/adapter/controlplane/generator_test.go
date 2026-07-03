package controlplane

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"beacon/internal/domain/monitor"
)

func testGenConfig() GeneratorConfig {
	return GeneratorConfig{BlackboxAddr: "blackbox:9115", DNSResolver: "8.8.8.8:53"}
}

func TestGenerateHTTPSMonitor(t *testing.T) {
	m := monitor.Monitor{
		ID:              uuid.New(),
		OrgID:           uuid.New(),
		ProjectID:       uuid.New(),
		Name:            "Marketing Site",
		Type:            monitor.TypeHTTPS,
		Target:          "https://example.com",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		Settings: monitor.Settings{
			Method:                "GET",
			ValidStatusCodes:      []int{200},
			SSLExpiryWarningDays:  30,
			ResponseTimeWarningMS: 2000,
		},
	}

	arts, err := Generate(testGenConfig(), []monitor.Monitor{m})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Blackbox module.
	var bb blackboxConfig
	mustYAML(t, arts.BlackboxYAML, &bb)
	if len(bb.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(bb.Modules))
	}
	for _, mod := range bb.Modules {
		if mod.Prober != "http" || mod.HTTP == nil {
			t.Fatalf("expected http prober, got %q", mod.Prober)
		}
		if mod.HTTP.Method != "GET" {
			t.Errorf("method = %q, want GET", mod.HTTP.Method)
		}
	}

	// Scrape config (wrapped under scrape_configs:).
	var scrapeFile scrapeConfigFile
	mustYAML(t, arts.ScrapeYAML, &scrapeFile)
	if len(scrapeFile.ScrapeConfigs) != 1 {
		t.Fatalf("expected 1 scrape job, got %d", len(scrapeFile.ScrapeConfigs))
	}
	sc := scrapeFile.ScrapeConfigs[0]
	if sc.MetricsPath != "/probe" {
		t.Errorf("metrics_path = %q", sc.MetricsPath)
	}
	if got := sc.Params["target"]; len(got) != 1 || got[0] != "https://example.com" {
		t.Errorf("target param = %v", got)
	}
	if got := sc.StaticConfigs[0].Targets; len(got) != 1 || got[0] != "blackbox:9115" {
		t.Errorf("scrape target = %v, want blackbox:9115", got)
	}
	if sc.StaticConfigs[0].Labels["monitor_id"] != m.ID.String() {
		t.Errorf("monitor_id label missing")
	}

	// Rules: MonitorDown + SSL + SlowResponse.
	var rf ruleFile
	mustYAML(t, arts.RulesYAML, &rf)
	names := ruleNames(rf)
	for _, want := range []string{"MonitorDown", "SSLCertExpiringSoon", "SlowResponse"} {
		if !names[want] {
			t.Errorf("expected rule %q to be generated; got %v", want, names)
		}
	}
}

func TestGenerateDNSUsesResolverAsTarget(t *testing.T) {
	m := monitor.Monitor{
		ID:              uuid.New(),
		Name:            "DNS check",
		Type:            monitor.TypeDNS,
		Target:          "example.com",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		Settings:        monitor.Settings{DNSQueryType: "A"},
	}
	arts, err := Generate(testGenConfig(), []monitor.Monitor{m})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var bb blackboxConfig
	mustYAML(t, arts.BlackboxYAML, &bb)
	for _, mod := range bb.Modules {
		if mod.Prober != "dns" || mod.DNS == nil {
			t.Fatalf("expected dns prober")
		}
		if mod.DNS.QueryName != "example.com" {
			t.Errorf("query_name = %q, want example.com", mod.DNS.QueryName)
		}
	}

	var scrapeFile scrapeConfigFile
	mustYAML(t, arts.ScrapeYAML, &scrapeFile)
	// For DNS the probe target is the resolver, not the domain.
	if got := scrapeFile.ScrapeConfigs[0].Params["target"]; got[0] != "8.8.8.8:53" {
		t.Errorf("dns probe target = %v, want resolver 8.8.8.8:53", got)
	}
}

func TestAlertForBySensitivity(t *testing.T) {
	cases := []struct {
		sensitivity string
		interval    int
		want        string
	}{
		{monitor.SensitivityImmediate, 60, "0s"},
		{monitor.SensitivityBalanced, 60, "120s"}, // 2 * interval
		{monitor.SensitivityBalanced, 10, "60s"},  // floor 60s
		{monitor.SensitivityRelaxed, 60, "300s"},  // floor 300s
		{monitor.SensitivityRelaxed, 120, "480s"}, // 4 * interval
		{"", 60, "120s"}, // unset falls back to balanced
	}
	for _, c := range cases {
		if got := alertFor(c.sensitivity, c.interval); got != c.want {
			t.Errorf("alertFor(%q, %d) = %q, want %q", c.sensitivity, c.interval, got, c.want)
		}
	}
}

func TestGenerateHonorsImmediateSensitivity(t *testing.T) {
	m := monitor.Monitor{
		ID: uuid.New(), Name: "x", Type: monitor.TypeHTTPS, Target: "https://x.com",
		IntervalSeconds: 60, TimeoutSeconds: 10,
		Settings: monitor.Settings{AlertSensitivity: monitor.SensitivityImmediate},
	}
	arts, err := Generate(testGenConfig(), []monitor.Monitor{m})
	if err != nil {
		t.Fatal(err)
	}
	var rf ruleFile
	mustYAML(t, arts.RulesYAML, &rf)
	for _, g := range rf.Groups {
		for _, r := range g.Rules {
			if r.Alert == "MonitorDown" && r.For != "0s" {
				t.Errorf("immediate sensitivity: MonitorDown for = %q, want 0s", r.For)
			}
		}
	}
}

func TestGenerateEmptyIsValid(t *testing.T) {
	arts, err := Generate(testGenConfig(), nil)
	if err != nil {
		t.Fatalf("Generate empty: %v", err)
	}
	if !strings.Contains(string(arts.BlackboxYAML), "modules") {
		t.Error("expected a modules key even when empty")
	}
}

// ---- helpers ----

func mustYAML(t *testing.T, data []byte, out any) {
	t.Helper()
	if err := yaml.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
}

func ruleNames(rf ruleFile) map[string]bool {
	names := map[string]bool{}
	for _, g := range rf.Groups {
		for _, r := range g.Rules {
			names[r.Alert] = true
		}
	}
	return names
}

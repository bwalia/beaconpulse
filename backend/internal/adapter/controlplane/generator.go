// Package controlplane is the adapter that turns Beacon's monitors into live
// Prometheus and Blackbox configuration. It is the concrete implementation of
// monitor.Syncer: given the current set of enabled monitors it regenerates the
// Blackbox module config, the Prometheus scrape-config file, and the alerting
// rules, writes them atomically, and hot-reloads both services. The whole
// configuration is derived from the database on every sync, so it is always a
// faithful, idempotent projection of the source of truth.
package controlplane

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"beacon/internal/domain/monitor"
)

// GeneratorConfig holds the static inputs the generator needs.
type GeneratorConfig struct {
	// BlackboxAddr is the host:port Prometheus scrapes to reach Blackbox /probe.
	BlackboxAddr string
	// DNSResolver is the resolver (host:port) DNS monitors query.
	DNSResolver string
}

// Artifacts are the three generated config documents, ready to write.
type Artifacts struct {
	BlackboxYAML []byte
	ScrapeYAML   []byte
	RulesYAML    []byte
}

// Generate renders all three artifacts from the given monitors.
func Generate(cfg GeneratorConfig, monitors []monitor.Monitor) (Artifacts, error) {
	bb := blackboxConfig{Modules: map[string]blackboxModule{}}
	var scrapes []scrapeConfig
	var rules []alertRule

	for i := range monitors {
		m := &monitors[i]
		id := sanitizeID(m.ID.String())
		module := "beacon_" + id
		job := "mon_" + id

		mod, probeTarget, err := buildModule(cfg, m)
		if err != nil {
			return Artifacts{}, fmt.Errorf("monitor %s: %w", m.ID, err)
		}
		bb.Modules[module] = mod
		scrapes = append(scrapes, buildScrape(cfg, m, module, job, probeTarget))
		rules = append(rules, buildRules(m)...)
	}

	blackboxYAML, err := yaml.Marshal(bb)
	if err != nil {
		return Artifacts{}, fmt.Errorf("marshal blackbox config: %w", err)
	}
	scrapeYAML, err := yaml.Marshal(scrapeConfigFile{ScrapeConfigs: scrapes})
	if err != nil {
		return Artifacts{}, fmt.Errorf("marshal scrape config: %w", err)
	}
	rulesYAML, err := yaml.Marshal(ruleFile{Groups: []ruleGroup{{Name: "beacon_monitors", Rules: rules}}})
	if err != nil {
		return Artifacts{}, fmt.Errorf("marshal rules: %w", err)
	}

	return Artifacts{
		BlackboxYAML: withHeader(blackboxYAML),
		ScrapeYAML:   withHeader(scrapeYAML),
		RulesYAML:    withHeader(rulesYAML),
	}, nil
}

// buildModule constructs the Blackbox module for a monitor and returns the probe
// target (the value passed as ?target= to Blackbox), which differs for DNS.
func buildModule(cfg GeneratorConfig, m *monitor.Monitor) (blackboxModule, string, error) {
	timeout := fmt.Sprintf("%ds", m.TimeoutSeconds)
	switch m.Type {
	case monitor.TypeHTTP, monitor.TypeHTTPS, monitor.TypeSSL:
		follow := m.Settings.FollowRedirects
		http := &httpProbe{
			Method:              orDefault(m.Settings.Method, "GET"),
			ValidStatusCodes:    m.Settings.ValidStatusCodes,
			FollowRedirects:     &follow,
			PreferredIPProtocol: "ip4",
		}
		if len(m.Settings.Headers) > 0 {
			http.Headers = m.Settings.Headers
		}
		if m.Settings.BodyKeyword != "" {
			http.FailIfBodyNotMatchesRegexp = []string{regexp.QuoteMeta(m.Settings.BodyKeyword)}
		}
		if m.Settings.BodyNotKeyword != "" {
			http.FailIfBodyMatchesRegexp = []string{regexp.QuoteMeta(m.Settings.BodyNotKeyword)}
		}
		if m.Settings.SkipTLSVerify {
			http.TLSConfig = &tlsConfig{InsecureSkipVerify: true}
		}
		return blackboxModule{Prober: "http", Timeout: timeout, HTTP: http}, m.Target, nil

	case monitor.TypeTCP:
		return blackboxModule{Prober: "tcp", Timeout: timeout, TCP: &tcpProbe{PreferredIPProtocol: "ip4"}}, m.Target, nil

	case monitor.TypeICMP:
		return blackboxModule{Prober: "icmp", Timeout: timeout, ICMP: &icmpProbe{PreferredIPProtocol: "ip4"}}, m.Target, nil

	case monitor.TypeDNS:
		dns := &dnsProbe{
			QueryName:           orDefault(m.Settings.DNSQueryName, m.Target),
			QueryType:           orDefault(m.Settings.DNSQueryType, "A"),
			PreferredIPProtocol: "ip4",
		}
		// For DNS the probe target is the resolver; the queried name lives in the
		// module.
		return blackboxModule{Prober: "dns", Timeout: timeout, DNS: dns}, cfg.DNSResolver, nil

	default:
		return blackboxModule{}, "", fmt.Errorf("unsupported monitor type %q", m.Type)
	}
}

func buildScrape(cfg GeneratorConfig, m *monitor.Monitor, module, job, probeTarget string) scrapeConfig {
	return scrapeConfig{
		JobName:        job,
		MetricsPath:    "/probe",
		ScrapeInterval: fmt.Sprintf("%ds", m.IntervalSeconds),
		ScrapeTimeout:  fmt.Sprintf("%ds", m.TimeoutSeconds),
		Params: map[string][]string{
			"module": {module},
			"target": {probeTarget},
		},
		StaticConfigs: []staticConfig{{
			Targets: []string{cfg.BlackboxAddr},
			Labels: map[string]string{
				"monitor_id":   m.ID.String(),
				"monitor_name": m.Name,
				"monitor_type": string(m.Type),
				"project_id":   m.ProjectID.String(),
				"org_id":       m.OrgID.String(),
				"instance":     m.Target,
			},
		}},
	}
}

// ruleLabels builds the label set that EVERY generated rule must carry.
//
// org_id is the tenant boundary, and it is the reason this constructor exists.
// prom-label-proxy filters /api/v1/rules by the tenant label, inspecting the
// rule's STATIC labels — not the labels of the series its expression happens to
// match. A rule without org_id is therefore invisible to the very tenant that
// owns it: Prometheus holds the rule, but the customer's Rules page shows an
// empty list. That was exactly the bug this replaces (20 rules in Prometheus, 0
// visible to the tenant).
//
// Every rule is built through here specifically so that adding a fourth rule
// type cannot silently reintroduce it. Do not hand-roll a label map.
//
// Note these labels are not *new* on a firing alert: the probe_* series already
// carry org_id/project_id/monitor_type (see buildScrape), and an alert inherits
// its expression's labels. Stating them on the rule changes nothing about the
// alert's identity in Alertmanager — it only makes the rule itself attributable
// while it is dormant.
func ruleLabels(m *monitor.Monitor, severity string) map[string]string {
	return map[string]string{
		"severity":     severity,
		"org_id":       m.OrgID.String(),
		"project_id":   m.ProjectID.String(),
		"monitor_id":   m.ID.String(),
		"monitor_type": string(m.Type),
	}
}

// buildRules produces the alerting rules for a monitor: an always-on
// availability rule plus optional SSL-expiry and slow-response rules.
func buildRules(m *monitor.Monitor) []alertRule {
	sel := fmt.Sprintf(`{monitor_id="%s"}`, m.ID.String())
	forDur := alertFor(m.Settings.AlertSensitivity, m.IntervalSeconds)

	rules := []alertRule{{
		Alert:  "MonitorDown",
		Expr:   "probe_success" + sel + " == 0",
		For:    forDur,
		Labels: ruleLabels(m, "critical"),
		Annotations: map[string]string{
			"summary":     fmt.Sprintf("%s is down", m.Name),
			"description": fmt.Sprintf("%s (%s) has failed its health check for more than %s.", m.Name, m.Target, forDur),
		},
	}}

	if (m.Type == monitor.TypeHTTPS || m.Type == monitor.TypeSSL || strings.HasPrefix(m.Target, "https://")) &&
		m.Settings.SSLExpiryWarningDays > 0 {
		rules = append(rules, alertRule{
			Alert:  "SSLCertExpiringSoon",
			Expr:   fmt.Sprintf("probe_ssl_earliest_cert_expiry%s - time() < %d * 86400", sel, m.Settings.SSLExpiryWarningDays),
			For:    "10m",
			Labels: ruleLabels(m, "warning"),
			Annotations: map[string]string{
				"summary":     fmt.Sprintf("TLS certificate for %s expiring soon", m.Name),
				"description": fmt.Sprintf("The certificate for %s expires in less than %d days.", m.Target, m.Settings.SSLExpiryWarningDays),
			},
		})
	}

	if m.Settings.ResponseTimeWarningMS > 0 {
		rules = append(rules, alertRule{
			Alert:  "SlowResponse",
			Expr:   fmt.Sprintf("probe_duration_seconds%s > %.3f", sel, float64(m.Settings.ResponseTimeWarningMS)/1000.0),
			For:    "5m",
			Labels: ruleLabels(m, "warning"),
			Annotations: map[string]string{
				"summary":     fmt.Sprintf("%s is responding slowly", m.Name),
				"description": fmt.Sprintf("%s response time has exceeded %dms for 5 minutes.", m.Target, m.Settings.ResponseTimeWarningMS),
			},
		})
	}
	return rules
}

// alertFor returns the MonitorDown "for" duration for a monitor's sensitivity:
//   - immediate: fire on the first failed check (0s)
//   - balanced:  a sustained failure (>= 60s, ~2 checks) — the default
//   - relaxed:   only prolonged outages (>= 5m, ~4 checks)
func alertFor(sensitivity string, intervalSeconds int) string {
	switch sensitivity {
	case monitor.SensitivityImmediate:
		return "0s"
	case monitor.SensitivityRelaxed:
		d := intervalSeconds * 4
		if d < 300 {
			d = 300
		}
		return fmt.Sprintf("%ds", d)
	default: // balanced (also the fallback for an unset value)
		return availabilityFor(intervalSeconds)
	}
}

// availabilityFor returns the balanced "for" duration: at least 60s, otherwise
// twice the scrape interval so a single missed probe does not alert.
func availabilityFor(intervalSeconds int) string {
	d := intervalSeconds * 2
	if d < 60 {
		d = 60
	}
	return fmt.Sprintf("%ds", d)
}

func sanitizeID(id string) string { return strings.ReplaceAll(id, "-", "") }

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func withHeader(b []byte) []byte {
	const header = "# Generated by Beacon. Do not edit by hand; changes are overwritten on sync.\n"
	return append([]byte(header), b...)
}

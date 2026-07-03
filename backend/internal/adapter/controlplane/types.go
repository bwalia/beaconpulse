package controlplane

// These types mirror the subset of the Prometheus and Blackbox configuration
// schemas that Beacon generates. Keeping them as explicit structs (rather than
// free-form maps) means the generated YAML is type-checked at compile time and
// self-documents exactly what Beacon controls.

// ---- Blackbox ----

type blackboxConfig struct {
	Modules map[string]blackboxModule `yaml:"modules"`
}

type blackboxModule struct {
	Prober  string     `yaml:"prober"`
	Timeout string     `yaml:"timeout,omitempty"`
	HTTP    *httpProbe `yaml:"http,omitempty"`
	TCP     *tcpProbe  `yaml:"tcp,omitempty"`
	ICMP    *icmpProbe `yaml:"icmp,omitempty"`
	DNS     *dnsProbe  `yaml:"dns,omitempty"`
}

type httpProbe struct {
	Method                     string            `yaml:"method,omitempty"`
	ValidStatusCodes           []int             `yaml:"valid_status_codes,omitempty"`
	FollowRedirects            *bool             `yaml:"follow_redirects,omitempty"`
	FailIfBodyNotMatchesRegexp []string          `yaml:"fail_if_body_not_matches_regexp,omitempty"`
	FailIfBodyMatchesRegexp    []string          `yaml:"fail_if_body_matches_regexp,omitempty"`
	Headers                    map[string]string `yaml:"headers,omitempty"`
	TLSConfig                  *tlsConfig        `yaml:"tls_config,omitempty"`
	PreferredIPProtocol        string            `yaml:"preferred_ip_protocol,omitempty"`
}

type tlsConfig struct {
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

type tcpProbe struct {
	PreferredIPProtocol string `yaml:"preferred_ip_protocol,omitempty"`
}

type icmpProbe struct {
	PreferredIPProtocol string `yaml:"preferred_ip_protocol,omitempty"`
}

type dnsProbe struct {
	QueryName           string `yaml:"query_name"`
	QueryType           string `yaml:"query_type,omitempty"`
	PreferredIPProtocol string `yaml:"preferred_ip_protocol,omitempty"`
}

// ---- Prometheus scrape config ----

// scrapeConfigFile is the top-level document written to a file referenced by
// Prometheus's `scrape_config_files`. Prometheus requires a mapping with a
// `scrape_configs:` key (not a bare list).
type scrapeConfigFile struct {
	ScrapeConfigs []scrapeConfig `yaml:"scrape_configs"`
}

type scrapeConfig struct {
	JobName        string              `yaml:"job_name"`
	MetricsPath    string              `yaml:"metrics_path,omitempty"`
	ScrapeInterval string              `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout  string              `yaml:"scrape_timeout,omitempty"`
	Params         map[string][]string `yaml:"params,omitempty"`
	StaticConfigs  []staticConfig      `yaml:"static_configs"`
}

type staticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels,omitempty"`
}

// ---- Prometheus rules ----

type ruleFile struct {
	Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
	Name  string      `yaml:"name"`
	Rules []alertRule `yaml:"rules"`
}

type alertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

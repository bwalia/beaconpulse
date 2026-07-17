package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"context"

	"beacon/internal/domain/diagnose"
)

// diagnoseSystemPrompt is the instrument. It gets the care it does because the model
// is being asked to tell someone why their site is down while it is down, and the
// failure mode of a language model here is not silence — it is a fluent, plausible,
// invented answer that sends an engineer to fix a certificate that never expired.
//
// So the prompt does three jobs. It pins the model to the measurements and forbids
// inventing any others. It hands over the reading order an SRE actually uses, since
// the first broken layer is the cause and everything above it is only a symptom —
// without that, models like to report the last failure they see (an HTTP timeout)
// rather than the first (the name never resolved). And it demands a confidence, so an
// unsupported guess is allowed to look like one.
const diagnoseSystemPrompt = `You are a senior Site Reliability Engineer telling a customer why their monitored endpoint is failing.

You are given the RESULTS OF REAL NETWORK PROBES that Beacon Pulse just ran against the target from its own infrastructure, moments ago.

THE ONE RULE: those measurements are the only facts you have. Reason from them and nothing else.
Never invent or assume DNS records, IP addresses, certificate details, status codes, or timings that are not shown to you.
If the measurements do not determine the cause, say which single check would settle it and set confidence to "low".
A confident wrong answer is worse than an honest "not enough evidence": someone is on call, and they will act on what you say while their site stays down.

HOW TO READ THE EVIDENCE — work down this list and stop at the FIRST failure. That one is the cause; every failure after it is a consequence, not a separate problem.
  1. dns.resolved = false        -> the name does not resolve, so nothing else can possibly work. Think: expired domain registration, a deleted or mistyped record, or a nameserver change still propagating.
  2. dns ok, tcp.connected = false -> the name resolves but nothing accepts a connection on that port. "connection refused" means a host answered and said no, i.e. the service is not running. A timeout usually means a firewall or security group silently dropped the packet.
  3. tcp ok, tls.handshake_ok = false -> the port is open but TLS failed. Check tls.expired, tls.days_remaining and tls.hostname_ok. An expired certificate, or one that does not cover this hostname, is the most common reason a site "just goes down" with no deploy behind it.
  4. tls ok, http.status_code >= 500 -> reachable and broken. 502/503/504 usually mean the proxy is healthy but whatever sits behind it is not.
  5. http.status_code is 4xx     -> reachable and answering, but refusing: 401/403 auth or a WAF, 404 wrong path, 429 rate limited.
  6. everything looks OK         -> the failure may be intermittent or already over. Say exactly that, and set confidence "low". Do not manufacture a cause to fill the space.

Reply with ONLY a JSON object, no prose around it, using exactly these keys:
  "summary":       one short sentence in plain English naming the specific host, port or certificate involved. Write for the person who owns this domain, not for us.
  "likely_cause":  the single most probable root cause, tied to the evidence that shows it. Quote the actual measured value you are relying on.
  "suggested_fix": concrete, ordered next steps the domain's owner can take. Prefer an exact command or setting over advice — "run dig NS example.com and compare the answer with your registrar" is useful, "check your DNS" is not.
  "confidence":    "high", "medium" or "low" — how strongly the evidence supports your cause. Reach for "low" whenever the probes came back clean or ambiguous.`

// diagnosisJSON is the shape the model is told to return.
type diagnosisJSON struct {
	Summary      string `json:"summary"`
	LikelyCause  string `json:"likely_cause"`
	SuggestedFix string `json:"suggested_fix"`
	Confidence   string `json:"confidence"`
}

var _ diagnose.Explainer = (*OllamaAnalyzer)(nil)

// Explain reads probe evidence back as a diagnosis. Any transport, HTTP or parse
// failure is returned as an error; the caller keeps the evidence and drops the prose,
// because a measurement is worth showing on its own and a hallucination is not.
func (a *OllamaAnalyzer) Explain(ctx context.Context, ev diagnose.Evidence) (*diagnose.Analysis, error) {
	// A larger budget than alert enrichment gets: this answer carries ordered
	// remediation steps, and truncating it mid-instruction is its own hazard.
	content, err := a.chat(ctx, diagnoseSystemPrompt, buildDiagnosePrompt(ev), 700)
	if err != nil {
		return nil, err
	}

	var parsed diagnosisJSON
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("ai: model did not return valid JSON: %w", err)
	}
	if strings.TrimSpace(parsed.Summary) == "" {
		return nil, fmt.Errorf("ai: model returned an empty diagnosis")
	}
	return &diagnose.Analysis{
		Summary:      clamp(parsed.Summary),
		LikelyCause:  clamp(parsed.LikelyCause),
		SuggestedFix: clamp(parsed.SuggestedFix),
		Confidence:   normalizeConfidence(parsed.Confidence),
	}, nil
}

// buildDiagnosePrompt renders the evidence as a labelled block. Labelled lines rather
// than raw JSON because the small models this runs against follow prose structure
// more reliably than they parse nested objects — and every field name here matches
// the ones the system prompt reasons about, so the two cannot drift apart.
//
// A probe that did not run is stated as such rather than omitted. Silence reads as
// "fine" to a model, and "we never got far enough to check TLS" is a very different
// fact from "TLS is healthy".
func buildDiagnosePrompt(ev diagnose.Evidence) string {
	var b strings.Builder
	b.WriteString("A monitored endpoint is failing. Diagnose it from these probe results.\n\n")
	writeLine(&b, "Target", ev.Target)
	writeLine(&b, "Monitor type", ev.MonitorType)
	writeLine(&b, "Probed at", ev.CheckedAt.UTC().Format(time.RFC3339))

	b.WriteString("\nDNS\n")
	writeLine(&b, "  resolved", fmt.Sprintf("%t", ev.DNS.Resolved))
	writeLine(&b, "  addresses", strings.Join(ev.DNS.Addresses, ", "))
	writeLine(&b, "  cname", ev.DNS.CNAME)
	writeLine(&b, "  nameservers", strings.Join(ev.DNS.Nameservers, ", "))
	writeLine(&b, "  lookup_ms", fmt.Sprintf("%d", ev.DNS.LookupMS))
	writeLine(&b, "  error", ev.DNS.Error)

	b.WriteString("\nTCP\n")
	if !ev.TCP.Attempted {
		b.WriteString("  not attempted: an earlier step failed first\n")
	} else {
		writeLine(&b, "  connected", fmt.Sprintf("%t", ev.TCP.Connected))
		writeLine(&b, "  address", ev.TCP.Address)
		writeLine(&b, "  connect_ms", fmt.Sprintf("%d", ev.TCP.ConnectMS))
		writeLine(&b, "  error", ev.TCP.Error)
	}

	b.WriteString("\nTLS\n")
	if !ev.TLS.Attempted {
		b.WriteString("  not attempted: not a TLS target, or an earlier step failed first\n")
	} else {
		writeLine(&b, "  handshake_ok", fmt.Sprintf("%t", ev.TLS.HandshakeOK))
		writeLine(&b, "  issuer", ev.TLS.Issuer)
		writeLine(&b, "  subject", ev.TLS.Subject)
		if !ev.TLS.NotAfter.IsZero() {
			writeLine(&b, "  not_after", ev.TLS.NotAfter.UTC().Format(time.RFC3339))
			writeLine(&b, "  days_remaining", fmt.Sprintf("%d", ev.TLS.DaysRemaining))
		}
		writeLine(&b, "  expired", fmt.Sprintf("%t", ev.TLS.Expired))
		writeLine(&b, "  hostname_ok", fmt.Sprintf("%t", ev.TLS.HostnameOK))
		writeLine(&b, "  error", ev.TLS.Error)
	}

	b.WriteString("\nHTTP\n")
	if !ev.HTTP.Attempted {
		b.WriteString("  not attempted: not an HTTP target, or an earlier step failed first\n")
	} else {
		if ev.HTTP.StatusCode > 0 {
			writeLine(&b, "  status_code", fmt.Sprintf("%d", ev.HTTP.StatusCode))
		}
		writeLine(&b, "  response_ms", fmt.Sprintf("%d", ev.HTTP.ResponseMS))
		writeLine(&b, "  redirect_chain", strings.Join(ev.HTTP.RedirectChain, " -> "))
		writeLine(&b, "  server", ev.HTTP.Server)
		writeLine(&b, "  error", ev.HTTP.Error)
	}

	return strings.TrimRight(b.String(), "\n")
}

// normalizeConfidence coerces the model's hedge into our three buckets. Anything
// unrecognised becomes "low": an answer we cannot even read the confidence of has
// not earned a high one.
func normalizeConfidence(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "certain", "very high":
		return "high"
	case "medium", "moderate", "med":
		return "medium"
	default:
		return "low"
	}
}

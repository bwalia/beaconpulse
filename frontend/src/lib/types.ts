// Shared types mirroring the Beacon REST API contract.

export interface User {
  id: string;
  org_id: string;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  twofa_enabled: boolean;
  last_login_at?: string;
  created_at: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}

export interface Project {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  environment: "production" | "staging" | "development";
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export type MonitorType = "http" | "https" | "ssl" | "tcp" | "icmp" | "dns" | "heartbeat";
export type MonitorStatus = "up" | "down" | "degraded" | "unknown" | "paused";

export interface MonitorSettings {
  method?: string;
  valid_status_codes?: number[];
  body_keyword?: string;
  body_not_keyword?: string;
  follow_redirects?: boolean;
  skip_tls_verify?: boolean;
  headers?: Record<string, string>;
  ssl_expiry_warning_days?: number;
  response_time_warning_ms?: number;
  alert_sensitivity?: string;
  dns_query_name?: string;
  dns_query_type?: string;
}

export interface Monitor {
  id: string;
  org_id: string;
  project_id: string;
  name: string;
  type: MonitorType;
  target: string;
  enabled: boolean;
  /** Published on the org's public status page. Defaults to false. */
  public: boolean;
  interval_seconds: number;
  timeout_seconds: number;
  settings: MonitorSettings;
  last_status: MonitorStatus;
  last_checked_at?: string;
  /** Heartbeat-only: the capability URL the customer's job pings (owner-only). */
  ping_url?: string;
  grace_seconds?: number;
  last_ping_at?: string;
  created_at: string;
  updated_at: string;
}

export type ChannelType = "telegram" | "slack" | "discord" | "email" | "webhook" | "teams";

export interface NotificationChannel {
  id: string;
  org_id: string;
  name: string;
  type: ChannelType;
  enabled: boolean;
  config: Record<string, string>;
  has_secret: boolean;
  created_at: string;
  updated_at: string;
}

export interface ActiveAlert {
  name: string;
  severity: string;
  monitor_id: string;
  monitor_name: string;
  monitor_type: string;
  target: string;
  since?: string;
}

export interface MetricPoint {
  t: string;
  v: number;
}

export interface MonitorMetrics {
  monitor_id: string;
  window_hours: number;
  uptime_percent: number;
  response_ms_current: number;
  response_ms_avg: number;
  up: MetricPoint[];
  response_ms: MetricPoint[];
}

export interface Usage {
  plan: string;
  monitors_used: number;
  monitors_limit: number;
  min_interval_seconds: number;
}

export interface PlanInfo {
  id: string;
  name: string;
  price_monthly: number;
  max_monitors: number;
  min_interval_seconds: number;
  features: string[];
}

export interface BillingInfo {
  current_plan: string;
  plans: PlanInfo[];
}

export interface MonitorUptime {
  monitor_id: string;
  monitor_name: string;
  target: string;
  avg_response_ms: number;
  points: MetricPoint[];
}

export interface Overview {
  window_hours: number;
  uptime_percent: number;
  avg_response_ms: number;
  uptime_series: MetricPoint[];
  response_series: MetricPoint[];
  monitors: MonitorUptime[];
}

export interface Pagination {
  total: number;
  limit: number;
  offset: number;
}

export interface ListResponse<T> {
  data: T[];
  pagination: Pagination;
}

export interface ApiFieldError {
  field: string;
  message: string;
}

export interface ApiError {
  code: string;
  message: string;
  fields?: ApiFieldError[];
  request_id?: string;
}

// ---- Public status page ----
// Mirrors the backend's deliberately narrow public projection
// (internal/domain/statuspage). There is no `target` here, and there must never
// be: this data is served to anyone with the URL.

export type StatusOverall = "operational" | "degraded" | "outage" | "unknown";

export interface PublicStatusMonitor {
  name: string;
  status: "up" | "down" | "degraded" | "unknown" | "paused";
  last_checked_at: string | null;
}

export interface PublicStatusGroup {
  name: string;
  environment: string;
  monitors: PublicStatusMonitor[];
}

export interface PublicStatusPage {
  org_name: string;
  title: string;
  overall: StatusOverall;
  groups: PublicStatusGroup[];
  updated_at: string;
}

export interface StatusPageSettings {
  slug: string;
  org_name: string;
  enabled: boolean;
  title: string;
  published_count: number;
  /** Server-provided public path, so the UI never reconstructs the route. */
  url: string;
}

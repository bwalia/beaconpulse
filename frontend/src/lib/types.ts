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
  /** True when an active maintenance window covers this monitor (list responses). */
  in_maintenance?: boolean;
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
  /** The alert's monitor is under an active window, so its notification was suppressed. */
  in_maintenance?: boolean;
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
  /** The tier the org subscribed to (free by default). */
  subscribed_plan: string;
  /** The tier whose limits actually apply right now (may be payg/free). */
  effective_plan: string;
  subscription_status: string;
  period_end?: string;
  /** Remaining pay-as-you-go balance, in monitor-seconds. */
  credit_seconds: number;
  max_monitors: number;
  /** Pay-as-you-go rate: $1 buys this many monitor-hours. */
  monitor_hours_per_dollar: number;
  /** Whether Stripe is configured on this deployment. */
  billing_enabled: boolean;
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

export type StatusOverall =
  | "operational"
  | "degraded"
  | "outage"
  | "under_maintenance"
  | "unknown";

export interface PublicStatusMonitor {
  name: string;
  status: "up" | "down" | "degraded" | "unknown" | "paused";
  // True when an active maintenance window covers this monitor. The real probe
  // `status` is still present so the row can show the true state underneath.
  in_maintenance: boolean;
  last_checked_at: string | null;
}

export interface PublicStatusGroup {
  name: string;
  environment: string;
  monitors: PublicStatusMonitor[];
}

// PublicStatusMaintenance is one active planned-maintenance window, surfaced as a
// banner. Only human-facing fields — never scope ids or internal identifiers.
export interface PublicStatusMaintenance {
  title: string;
  starts_at: string;
  ends_at: string;
}

export interface PublicStatusPage {
  org_name: string;
  title: string;
  overall: StatusOverall;
  groups: PublicStatusGroup[];
  maintenances: PublicStatusMaintenance[];
  updated_at: string;
}

export interface StatusPageSettings {
  /** Effective public slug (custom if set, else the org slug). */
  slug: string;
  org_name: string;
  enabled: boolean;
  title: string;
  published_count: number;
  /** The org's default slug — what the page falls back to with no custom slug. */
  org_slug: string;
  /** The owner-chosen slug, empty when using the default. */
  custom_slug: string;
  /** Server-provided public path, so the UI never reconstructs the route. */
  url: string;
}

// ---- Maintenance windows ----
// Planned downtime that suppresses alerts and relabels the status page. Mirrors
// backend/internal/domain/maintenance.

export type MaintenanceScope = "org" | "project" | "monitor";

export interface MaintenanceWindow {
  id: string;
  org_id: string;
  title: string;
  description: string;
  starts_at: string;
  ends_at: string;
  scope: MaintenanceScope;
  /** Project ids (scope=project) or monitor ids (scope=monitor); empty for org. */
  scope_ids: string[];
  /** Whether the window covers "now" — the server's clock, not the browser's. */
  active: boolean;
  created_at: string;
  updated_at: string;
}

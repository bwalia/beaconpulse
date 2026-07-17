"use client";

// TanStack Query hooks wrapping the Beacon API. Queries and mutations invalidate
// the relevant caches so the UI updates optimistically after writes.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  ActiveAlert,
  BillingInfo,
  Diagnosis,
  ListResponse,
  MaintenanceScope,
  MaintenanceWindow,
  Monitor,
  MonitorMetrics,
  NotificationChannel,
  Overview,
  Project,
  StatusPageSettings,
  Usage,
} from "./types";

// live is the refetch policy for anything that answers "what is happening RIGHT
// NOW?" — monitor status, firing alerts, live rollups. The whole product's promise
// is that the screen matches reality, so these queries are never treated as fresh
// (staleTime 0) and re-ask the API on every focus rather than trusting a value that
// could predate an outage. Returning to the tab is exactly when someone is checking
// after trouble, and it must not answer with a cached "up".
//
// Polling still pauses while the tab is hidden (TanStack's default), so a
// backgrounded dashboard costs nothing — the focus refetch is what makes it correct
// again on return, which is why "always" matters more than a shorter interval.
const live = (intervalMs: number) =>
  ({
    refetchInterval: intervalMs,
    staleTime: 0,
    refetchOnWindowFocus: "always",
  }) as const;

// ---- Projects ----

// useProjects fetches up to 200 projects at once — for the project SELECTORS
// (monitor form, maintenance scope). The Projects PAGE uses useProjectsPage.
export function useProjects() {
  return useQuery({
    queryKey: ["projects"],
    queryFn: () => api.get<ListResponse<Project>>("/api/v1/projects?limit=200"),
  });
}

export interface ProjectPageParams {
  page: number;
  pageSize: number;
  search?: string;
  environment?: string;
}

export function useProjectsPage(p: ProjectPageParams) {
  const qs = new URLSearchParams({ limit: String(p.pageSize), offset: String(p.page * p.pageSize) });
  if (p.search) qs.set("search", p.search);
  if (p.environment) qs.set("environment", p.environment);
  return useQuery({
    queryKey: ["projects", "page", p],
    queryFn: () => api.get<ListResponse<Project>>(`/api/v1/projects?${qs.toString()}`),
    placeholderData: (previous) => previous,
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      name: string;
      description?: string;
      environment?: string;
    }) => api.post<Project>("/api/v1/projects", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["projects"] }),
  });
}

// ---- Monitors ----

// useMonitors fetches up to 200 monitors in one shot. It backs the consumers that
// need the whole set at once — the dashboard rollup, the status-page publish list,
// and the maintenance scope pickers. The paginated Monitors PAGE uses
// useMonitorsPage instead. (Keys share the "monitors" prefix so a create/delete
// invalidation covers both.)
export function useMonitors(projectId?: string) {
  const query = projectId ? `&project_id=${projectId}` : "";
  return useQuery({
    queryKey: ["monitors", projectId ?? "all"],
    queryFn: () => api.get<ListResponse<Monitor>>(`/api/v1/monitors?limit=200${query}`),
    ...live(15_000), // near-real-time status without websockets in this slice
  });
}

export interface MonitorPageParams {
  /** Zero-based page index. */
  page: number;
  pageSize: number;
  search?: string;
  status?: string;
  type?: string;
  projectId?: string;
}

// useMonitorsPage is the server-side-paginated fetch for the Monitors page: one
// page of results plus the total count, so the table scales to thousands of
// monitors without shipping them all to the browser. placeholderData keeps the
// current page on screen while the next loads, so paging doesn't flash a skeleton.
export function useMonitorsPage(p: MonitorPageParams) {
  const qs = new URLSearchParams({
    limit: String(p.pageSize),
    offset: String(p.page * p.pageSize),
  });
  if (p.search) qs.set("search", p.search);
  if (p.status) qs.set("status", p.status);
  if (p.type) qs.set("type", p.type);
  if (p.projectId) qs.set("project_id", p.projectId);
  return useQuery({
    queryKey: ["monitors", "page", p],
    queryFn: () => api.get<ListResponse<Monitor>>(`/api/v1/monitors?${qs.toString()}`),
    ...live(15_000),
    placeholderData: (previous) => previous,
  });
}

export interface CreateMonitorInput {
  project_id: string;
  name: string;
  type: string;
  /** Optional for heartbeat monitors, which have no probe target. */
  target?: string;
  interval_seconds?: number;
  timeout_seconds?: number;
  /** Heartbeat only: slack beyond the interval before a missed ping alerts. */
  grace_seconds?: number;
  settings?: Record<string, unknown>;
}

export function useCreateMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateMonitorInput) => api.post<Monitor>("/api/v1/monitors", input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["usage"] });
    },
  });
}

export function useUsage() {
  return useQuery({
    queryKey: ["usage"],
    queryFn: () => api.get<Usage>("/api/v1/monitors/usage"),
  });
}

export interface UpdateMonitorInput {
  name?: string;
  target?: string;
  interval_seconds?: number;
  settings?: Record<string, unknown>;
  /** Publish (or unpublish) this monitor on the org's public status page. */
  public?: boolean;
}

export function useUpdateMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateMonitorInput }) =>
      api.patch<Monitor>(`/api/v1/monitors/${id}`, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["monitors"] }),
  });
}

export function useSetMonitorEnabled() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.post<Monitor>(`/api/v1/monitors/${id}/${enabled ? "resume" : "pause"}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["monitors"] }),
  });
}

export function useDeleteMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/api/v1/monitors/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["usage"] });
    },
  });
}

// ---- Notification channels ----

export interface ChannelPageParams {
  page: number;
  pageSize: number;
  search?: string;
}

export function useChannels(p: ChannelPageParams) {
  const qs = new URLSearchParams({ limit: String(p.pageSize), offset: String(p.page * p.pageSize) });
  if (p.search) qs.set("search", p.search);
  return useQuery({
    queryKey: ["channels", p],
    queryFn: () => api.get<ListResponse<NotificationChannel>>(`/api/v1/notification-channels?${qs.toString()}`),
    placeholderData: (previous) => previous,
  });
}

export interface CreateChannelInput {
  name: string;
  type: string;
  config: Record<string, string>;
  secret?: string;
}

export function useCreateChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateChannelInput) =>
      api.post<NotificationChannel>("/api/v1/notification-channels", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["channels"] }),
  });
}

export function useSetChannelEnabled() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.patch<NotificationChannel>(`/api/v1/notification-channels/${id}`, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["channels"] }),
  });
}

export function useDeleteChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/api/v1/notification-channels/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["channels"] }),
  });
}

export function useTestChannel() {
  return useMutation({
    mutationFn: (id: string) => api.post<{ status: string }>(`/api/v1/notification-channels/${id}/test`),
  });
}

// ---- Insight (tenant-scoped, from Prometheus) ----

export interface AlertPageParams {
  page: number;
  pageSize: number;
  severity?: string;
}

// useActiveAlerts returns firing alerts. Params paginate/filter server-side; called
// with none (the dashboard), it fetches the default page and still carries the true
// total in pagination.total for the count tile.
export function useActiveAlerts(p?: AlertPageParams) {
  const qs = new URLSearchParams();
  if (p) {
    qs.set("limit", String(p.pageSize));
    qs.set("offset", String(p.page * p.pageSize));
    if (p.severity) qs.set("severity", p.severity);
  }
  const query = qs.toString();
  return useQuery({
    queryKey: ["alerts", p ?? "default"],
    queryFn: () => api.get<ListResponse<ActiveAlert>>(`/api/v1/alerts${query ? `?${query}` : ""}`),
    ...live(15_000),
    placeholderData: (previous) => previous,
  });
}

// useOverview fetches the org dashboard rollup for one window. `hours` must be a
// value the API allowlists (1, 6, 24, 168, 720) and is part of the query key, so
// switching range refetches instead of serving the previous window's data.
export function useOverview(hours: number = 24) {
  return useQuery({
    queryKey: ["overview", hours],
    queryFn: () => api.get<Overview>(`/api/v1/overview?hours=${hours}`),
    ...live(30_000),
    // Keep the previous window on screen while the new one loads, so switching
    // range dissolves rather than flashing a skeleton.
    placeholderData: (previous) => previous,
  });
}

// ---- Billing ----

export function useBilling() {
  return useQuery({ queryKey: ["billing"], queryFn: () => api.get<BillingInfo>("/api/v1/billing") });
}

// useStartSubscription / useStartTopUp create a Stripe Checkout session and return
// its URL; the page redirects the browser there. Nothing changes locally until the
// signed webhook confirms payment, so there is no cache to invalidate here.
export function useStartSubscription() {
  return useMutation({
    mutationFn: (plan: string) =>
      api.post<{ checkout_url: string }>("/api/v1/billing/checkout/subscription", { plan }),
  });
}

export function useStartTopUp() {
  return useMutation({
    mutationFn: (amountCents: number) =>
      api.post<{ checkout_url: string }>("/api/v1/billing/checkout/topup", { amount_cents: amountCents }),
  });
}

export function useMonitorMetrics(id: string | null) {
  return useQuery({
    queryKey: ["monitor-metrics", id],
    queryFn: () => api.get<MonitorMetrics>(`/api/v1/monitors/${id}/metrics`),
    enabled: !!id,
    ...live(30_000),
  });
}

// ---- Public status page settings ----

export function useStatusPageSettings() {
  return useQuery({
    queryKey: ["status-page"],
    queryFn: () => api.get<StatusPageSettings>("/api/v1/status-page"),
  });
}

export function useUpdateStatusPageSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { enabled?: boolean; title?: string; slug?: string }) =>
      api.patch<StatusPageSettings>("/api/v1/status-page", input),
    // Publishing changes the monitor list's meaning too (the "Public" toggles),
    // so refresh both rather than leaving a stale count on screen.
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["status-page"] });
      qc.invalidateQueries({ queryKey: ["monitors"] });
    },
  });
}

/** Publish or unpublish one monitor on the status page. */
export function useSetMonitorPublic() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, isPublic }: { id: string; isPublic: boolean }) =>
      api.patch<Monitor>(`/api/v1/monitors/${id}`, { public: isPublic }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["status-page"] });
    },
  });
}

// ---- Maintenance windows ----

export interface MaintenancePageParams {
  page: number;
  pageSize: number;
}

export function useMaintenanceWindows(p?: MaintenancePageParams) {
  const qs = new URLSearchParams(
    p
      ? { limit: String(p.pageSize), offset: String(p.page * p.pageSize) }
      : { limit: "200" },
  );
  return useQuery({
    queryKey: ["maintenance-windows", p ?? "all"],
    queryFn: () => api.get<ListResponse<MaintenanceWindow>>(`/api/v1/maintenance-windows?${qs.toString()}`),
    // A window flips active/ended on the server clock; refresh so the badge and
    // the "now under maintenance" state do not go stale on an open tab.
    ...live(30_000),
    placeholderData: (previous) => previous,
  });
}

export interface MaintenanceWindowInput {
  title: string;
  description?: string;
  starts_at: string;
  ends_at: string;
  scope: MaintenanceScope;
  scope_ids?: string[];
}

export function useCreateMaintenanceWindow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: MaintenanceWindowInput) =>
      api.post<MaintenanceWindow>("/api/v1/maintenance-windows", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["maintenance-windows"] }),
  });
}

export function useUpdateMaintenanceWindow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: Partial<MaintenanceWindowInput> }) =>
      api.patch<MaintenanceWindow>(`/api/v1/maintenance-windows/${id}`, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["maintenance-windows"] }),
  });
}

export function useDeleteMaintenanceWindow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<void>(`/api/v1/maintenance-windows/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["maintenance-windows"] }),
  });
}

// ---- AI diagnosis ----

// useDiagnose runs an on-demand diagnosis of a failing monitor. A mutation, not a
// query: it makes the server open sockets to a third party and spend a model's time,
// so it must happen only when someone asks — never on mount, focus, or a retry.
export function useDiagnose() {
  return useMutation({
    mutationFn: (monitorId: string) => api.post<Diagnosis>(`/api/v1/monitors/${monitorId}/diagnose`),
    retry: false, // a failed diagnosis is re-run by the user, not silently re-billed
  });
}

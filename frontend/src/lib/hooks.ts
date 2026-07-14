"use client";

// TanStack Query hooks wrapping the Beacon API. Queries and mutations invalidate
// the relevant caches so the UI updates optimistically after writes.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  ActiveAlert,
  BillingInfo,
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

// ---- Projects ----

export function useProjects() {
  return useQuery({
    queryKey: ["projects"],
    queryFn: () => api.get<ListResponse<Project>>("/api/v1/projects?limit=200"),
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

export function useMonitors(projectId?: string) {
  const query = projectId ? `&project_id=${projectId}` : "";
  return useQuery({
    queryKey: ["monitors", projectId ?? "all"],
    queryFn: () => api.get<ListResponse<Monitor>>(`/api/v1/monitors?limit=200${query}`),
    refetchInterval: 15000, // near-real-time status without websockets in this slice
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

export function useChannels() {
  return useQuery({
    queryKey: ["channels"],
    queryFn: () => api.get<{ data: NotificationChannel[] }>("/api/v1/notification-channels"),
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

export function useActiveAlerts() {
  return useQuery({
    queryKey: ["alerts"],
    queryFn: () => api.get<{ data: ActiveAlert[] }>("/api/v1/alerts"),
    refetchInterval: 15000,
  });
}

// useOverview fetches the org dashboard rollup for one window. `hours` must be a
// value the API allowlists (1, 6, 24, 168, 720) and is part of the query key, so
// switching range refetches instead of serving the previous window's data.
export function useOverview(hours: number = 24) {
  return useQuery({
    queryKey: ["overview", hours],
    queryFn: () => api.get<Overview>(`/api/v1/overview?hours=${hours}`),
    refetchInterval: 30000,
    // Keep the previous window on screen while the new one loads, so switching
    // range dissolves rather than flashing a skeleton.
    placeholderData: (previous) => previous,
  });
}

// ---- Billing ----

export function useBilling() {
  return useQuery({ queryKey: ["billing"], queryFn: () => api.get<BillingInfo>("/api/v1/billing") });
}

export function useChangePlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (plan: string) =>
      api.post<{ current_plan: string }>("/api/v1/billing/plan", { plan }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["billing"] });
      qc.invalidateQueries({ queryKey: ["usage"] });
    },
  });
}

export function useMonitorMetrics(id: string | null) {
  return useQuery({
    queryKey: ["monitor-metrics", id],
    queryFn: () => api.get<MonitorMetrics>(`/api/v1/monitors/${id}/metrics`),
    enabled: !!id,
    refetchInterval: 30000,
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
    mutationFn: (input: { enabled?: boolean; title?: string }) =>
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

export function useMaintenanceWindows() {
  return useQuery({
    queryKey: ["maintenance-windows"],
    queryFn: () =>
      api.get<ListResponse<MaintenanceWindow>>("/api/v1/maintenance-windows?limit=200"),
    // A window flips active/ended on the server clock; refresh so the badge and
    // the "now under maintenance" state do not go stale on an open tab.
    refetchInterval: 30000,
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

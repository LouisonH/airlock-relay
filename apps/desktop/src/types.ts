export type RouteKind = "HTTP" | "SSH" | "LLM";
export type RouteStatus = "enabled" | "disabled" | "blocked";
export type RouteHealth = "healthy" | "degraded" | "unknown";

// IPC-facing summaries intentionally have no target or secret fields.
export interface RouteSummary {
  id: string;
  name: string;
  alias: string;
  kind: RouteKind;
  status: RouteStatus;
  localEndpoint: string;
  permissionSummary: string;
  egress: "Direct" | "Proxy" | "Auto";
  health: RouteHealth;
  lastUsed: string;
  currentConnections: number;
}

export interface ActivityEvent {
  id: string;
  time: string;
  routeName: string;
  caller: string;
  action: string;
  result: "allowed" | "blocked" | "failed";
  latency: string;
  egress: "Direct" | "Proxy" | "Auto";
}

export interface ControlState {
  connected: boolean;
  running: boolean;
  routes: RouteSummary[];
  message?: string;
}

export interface ControlUpdate {
  routes: RouteSummary[];
  message?: string;
}

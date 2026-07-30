export type RouteKind = "HTTP" | "SSH" | "LLM";
export type RouteStatus = "enabled" | "disabled" | "blocked";
export type RouteHealth = "healthy" | "degraded" | "unknown";
export type NetworkScope = "loopback" | "lan";
export type SecretStoreMode = "keychain" | "local_file";

export interface SecuritySettings {
  version: number;
  networkScope: NetworkScope;
  secretStore: SecretStoreMode;
}

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
  allowAllCommands: boolean;
  recordCommands: boolean;
  allowedCommand?: string;
  provider?: "openai" | "anthropic";
  allowedModels?: string[];
  maxOutputTokens?: number;
  requestsPerMinute?: number;
  maxConcurrent?: number;
  trackUsage?: boolean;
  totalRequests?: number;
  inputTokens?: number;
  outputTokens?: number;
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
  proxyConfigured: boolean;
  sshReady: boolean;
  activity: ActivityEvent[];
  securitySettings: SecuritySettings;
}

export interface ControlUpdate {
  routes: RouteSummary[];
  message?: string;
}

export interface SecurityUpdate {
  securitySettings: SecuritySettings;
  message?: string;
}

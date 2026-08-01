export type RouteKind = "HTTP" | "SSH" | "LLM";
export type RouteStatus = "enabled" | "disabled" | "blocked";
export type RouteHealth = "healthy" | "degraded" | "unknown";
export type NetworkScope = "loopback" | "lan";
export type SecretStoreMode = "keychain" | "local_file";

export interface SecuritySettings {
  version: number;
  networkScope: NetworkScope;
  secretStore: SecretStoreMode;
  httpPort: number;
  sshPort: number;
}

export interface PlatformInfo {
  os: "macos" | "windows" | "linux" | "other";
  arch: string;
  controlTransport: "unix-socket" | "named-pipe";
  secretStore: "keychain" | "credential-manager" | "secret-service";
  desktopRelease: boolean;
}

export interface PortOwner {
  port: number;
  pid: number;
  command: string;
}

// Persistent route summaries intentionally have no target or secret fields.
export interface RouteSummary {
  id: string;
  name: string;
  alias: string;
  localUsername?: string;
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
  allowSftp: boolean;
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
  authenticationTimeoutSeconds?: number;
  keywordReplacementCount?: number;
}

export interface SSHHostKeyProbe {
  hostKey: string;
  fingerprint: string;
}

export interface KeywordReplacement {
  from: string;
  to: string;
  enabled: boolean;
}

// The generated local credential is returned once for the Airlock completion
// screen and is never included in later control-state responses.
export interface SSHRouteCreationResult {
  route: RouteSummary;
  localCredential: string;
  generatedCredential: boolean;
}

export interface ActivityEvent {
  id: string;
  time: string;
  routeName: string;
  caller: string;
  action: string;
  detail?: string;
  result: "allowed" | "blocked" | "failed";
  latency: string;
  egress: "Direct" | "Proxy" | "Auto";
  category: RouteKind | "System";
  eventType: "request" | "command" | "health";
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

export type RouteStatusFilter = "All" | RouteStatus;
export type RouteHealthFilter = "All" | RouteHealth;
export type RouteEgressFilter = "All" | RouteSummary["egress"];

import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { useEffect, useId, useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  BookOpen,
  Cable,
  Check,
  Copy,
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  CircleMinus,
  CircleStop,
  Eye,
  EyeOff,
  Filter,
  FilterX,
  Gauge,
  GitBranch,
  Github,
  Globe2,
  HardDrive,
  HeartPulse,
  KeyRound,
  Languages,
  LoaderCircle,
  LockKeyhole,
  Monitor,
  Moon,
  Minus,
  Maximize2,
  Network,
  PencilLine,
  Plus,
  RefreshCw,
  RotateCcw,
  Route,
  Search,
  Server,
  Settings,
  Settings2,
  SlidersHorizontal,
  ShieldCheck,
  ShieldAlert,
  Sparkles,
  SquareTerminal,
  Sun,
  Trash2,
  TriangleAlert,
  Wifi,
  X,
} from "lucide-react";
import { applyTheme, getAccentTheme, getDensityPreference, getMotionPreference, getRefreshInterval, getThemePreference, saveAccentTheme, saveDensityPreference, saveMotionPreference, saveRefreshInterval, saveThemePreference, watchSystemTheme, type AccentTheme, type DensityPreference, type MotionPreference, type RefreshInterval, type ThemePreference } from "./theme";
import { getLocalePreference, getResolvedLocale, saveLocalePreference, translate, watchSystemLocale, type LocalePreference } from "./i18n";
import { APP_VERSION, checkForUpdates, type UpdateCheckResult } from "./version";
import type { ActivityEvent, ControlState, ControlUpdate, KeywordReplacement, NetworkScope, PlatformInfo, PortOwner, RouteEgressFilter, RouteHealthFilter, RouteKind, RouteStatusFilter, RouteSummary, SecretStoreMode, SecuritySettings, SecurityUpdate, SSHHostKeyProbe, SSHRouteCreationResult } from "./types";

type Page = "overview" | "routes" | "activity" | "guide" | "settings";
type RouteFilter = "All" | RouteKind;
type ActivityFilter = "All" | RouteKind | "Health" | "Problems";
type SSHHostUpdateInput = { address: string; username: string; password: string; expectedHostKey: string; egress: RouteSummary["egress"] };

function validSSHHost(value: string): boolean {
  const host = value.trim();
  return host.length > 0
    && host.length <= 505
    && !/[\s\0\[\]]/.test(host)
    && (!host.includes(":") || /^[0-9a-fA-F:.%]+$/.test(host));
}

function validSSHPort(value: string): boolean {
  return /^\d{1,5}$/.test(value) && Number(value) >= 1 && Number(value) <= 65_535;
}

function composeSSHAddress(host: string, port: string): string {
  const normalizedHost = host.trim();
  return `${normalizedHost.includes(":") ? `[${normalizedHost}]` : normalizedHost}:${port}`;
}

const isTauri = "__TAURI_INTERNALS__" in window;
const previewRoutes: RouteSummary[] = isTauri ? [] : [
  { id: "release", name: "Release downloads", alias: "release", kind: "HTTP", status: "enabled", localEndpoint: "127.0.0.1:4768/r/release", permissionSummary: "GET, HEAD · Range", egress: "Direct", health: "healthy", lastUsed: "2 分钟前", currentConnections: 1, allowAllCommands: false, recordCommands: false, allowSftp: false },
  { id: "models", name: "Model gateway", alias: "models", kind: "LLM", status: "disabled", localEndpoint: "http://127.0.0.1:4768/r/models", permissionSummary: "OpenAI · 3 models · output ≤ 8192 · 60/min · 4 concurrent", egress: "Proxy", health: "unknown", lastUsed: "从未", currentConnections: 0, allowAllCommands: false, recordCommands: false, allowSftp: false, provider: "openai", allowedModels: ["gpt-5.2", "gpt-5.2-codex", "gpt-5.1"], maxOutputTokens: 8192, requestsPerMinute: 60, maxConcurrent: 4, trackUsage: true, totalRequests: 128, inputTokens: 184320, outputTokens: 42670 },
  { id: "build", name: "Release builder", alias: "build", localUsername: "builder", kind: "SSH", status: "enabled", localEndpoint: "builder@127.0.0.1:4770", permissionSummary: "all exec commands · high risk · recorded", egress: "Auto", health: "healthy", lastUsed: "刚刚", currentConnections: 0, allowAllCommands: true, recordCommands: true, allowSftp: false, allowedCommand: "" },
];

const previewActivity: ActivityEvent[] = isTauri ? [] : [
  { id: "ssh-preview", time: "07-29 23:40:12", routeName: "Release builder", caller: "build@loopback", action: "printf airlock-ok", result: "allowed", latency: "182 ms", egress: "Auto", category: "SSH", eventType: "command" },
];

const navItems: Array<{ id: Page; label: string; icon: typeof Gauge }> = [
  { id: "overview", label: "概览", icon: Gauge },
  { id: "routes", label: "路由", icon: Route },
  { id: "activity", label: "活动", icon: Activity },
  { id: "guide", label: "指南", icon: BookOpen },
  { id: "settings", label: "设置", icon: Settings },
];

const emptyControl: ControlState = {
  connected: !isTauri,
  running: !isTauri,
  routes: previewRoutes,
  proxyConfigured: false,
  sshReady: !isTauri,
  activity: previewActivity,
  securitySettings: { version: 1, networkScope: "loopback", secretStore: "local_file", httpPort: 4768, sshPort: 4770 },
  message: isTauri ? "正在连接 airlockd" : undefined,
};

export default function App() {
  const [page, setPage] = useState<Page>("overview");
  const [control, setControl] = useState<ControlState>(emptyControl);
  const [platform, setPlatform] = useState<PlatformInfo>(() => ({
    os: "other",
    arch: "",
    controlTransport: "unix-socket",
    secretStore: "keychain",
    desktopRelease: false,
  }));
  const [emergencyOpen, setEmergencyOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorKind, setEditorKind] = useState<RouteKind>("HTTP");
  const [theme, setTheme] = useState<ThemePreference>(getThemePreference);
  const [accent, setAccent] = useState<AccentTheme>(getAccentTheme);
  const [density, setDensity] = useState<DensityPreference>(getDensityPreference);
  const [motion, setMotion] = useState<MotionPreference>(getMotionPreference);
  const [refreshInterval, setRefreshInterval] = useState<RefreshInterval>(getRefreshInterval);
  const [language, setLanguage] = useState<LocalePreference>(getLocalePreference);
  const [notice, setNotice] = useState<string>();
  const [pendingDelete, setPendingDelete] = useState<RouteSummary>();
  const [policyRoute, setPolicyRoute] = useState<RouteSummary>();
  const [testingAliases, setTestingAliases] = useState<Set<string>>(() => new Set());
  const [proxyTesting, setProxyTesting] = useState(false);
  const [updateCheck, setUpdateCheck] = useState<UpdateCheckResult>();
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [coreRetrying, setCoreRetrying] = useState(false);
  const [portManagerOpen, setPortManagerOpen] = useState(false);

  const refresh = async () => {
    if (!isTauri) return;
    const next = await invoke<ControlState>("get_control_state");
    setControl(next);
  };

  useEffect(() => {
    saveThemePreference(theme);
    if (theme !== "system") return;
    return watchSystemTheme(() => applyTheme("system"));
  }, [theme]);

  useEffect(() => {
    saveAccentTheme(accent);
  }, [accent]);

  useEffect(() => saveDensityPreference(density), [density]);
  useEffect(() => saveMotionPreference(motion), [motion]);
  useEffect(() => saveRefreshInterval(refreshInterval), [refreshInterval]);
  useEffect(() => {
    const resolved = saveLocalePreference(language);
    if (isTauri) void invoke("set_ui_locale", { locale: resolved });
    if (language !== "system") return;
    return watchSystemLocale(() => {
      const next = saveLocalePreference("system");
      if (isTauri) void invoke("set_ui_locale", { locale: next });
    });
  }, [language]);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), refreshInterval);
    return () => window.clearInterval(timer);
  }, [refreshInterval]);

  useEffect(() => {
    if (!isTauri) return;
    void invoke<PlatformInfo>("get_platform_info")
      .then(setPlatform)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(undefined), 3600);
    return () => window.clearTimeout(timer);
  }, [notice]);

  const enabledCount = control.routes.filter((route) => route.status === "enabled").length;
  const openEditor = (kind: RouteKind = "HTTP") => {
    setEditorKind(kind);
    setEditorOpen(true);
  };

  const restartLocalCore = async () => {
    if (!isTauri || coreRetrying) return;
    setCoreRetrying(true);
    try {
      const message = await invoke<string>("restart_local_core");
      setNotice(message);
      await refresh();
    } catch (error) {
      setNotice(String(error));
      await refresh();
    } finally {
      setCoreRetrying(false);
    }
  };

  const stopAll = async () => {
    try {
      const routes = isTauri
        ? await invoke<RouteSummary[]>("stop_all_routes")
        : control.routes.map((route) => ({ ...route, status: "disabled" as const, currentConnections: 0 }));
      setControl((current) => ({ ...current, routes }));
      setEmergencyOpen(false);
      setNotice("全部路由已停止");
    } catch (error) {
      setNotice(String(error));
    }
  };

  const toggleRoute = async (alias: string, enabled: boolean) => {
    try {
      const routes = isTauri
        ? await invoke<RouteSummary[]>("set_route_enabled", { alias, enabled })
        : control.routes.map((route) => route.alias === alias ? { ...route, status: enabled ? "enabled" as const : "disabled" as const } : route);
      setControl((current) => ({ ...current, routes }));
    } catch (error) {
      setNotice(String(error));
    }
  };

  const deleteRoute = async () => {
    if (!pendingDelete) return;
    try {
      const update: ControlUpdate = isTauri
        ? await invoke<ControlUpdate>("delete_route", { alias: pendingDelete.alias })
        : { routes: control.routes.filter((route) => route.alias !== pendingDelete.alias) };
      setControl((current) => ({ ...current, routes: update.routes }));
      setNotice(update.message ?? `已删除 ${pendingDelete.name} 并清理凭据`);
      setPendingDelete(undefined);
    } catch (error) {
      setNotice(String(error));
    }
  };

  const configureProxy = async () => {
    try {
      const configured = isTauri ? await invoke<boolean>("configure_proxy") : true;
      setControl((current) => ({ ...current, proxyConfigured: configured }));
      setNotice("代理出口已安全保存");
    } catch (error) {
      setNotice(String(error));
    }
  };

  const clearProxy = async () => {
    try {
      const configured = isTauri ? await invoke<boolean>("clear_proxy") : false;
      setControl((current) => ({ ...current, proxyConfigured: configured }));
      setNotice("代理出口已清除");
    } catch (error) {
      setNotice(String(error));
    }
  };

  const updateSSHPolicy = async (name: string, localUsername: string, allowedCommand: string, allowAllCommands: boolean, recordCommands: boolean, allowSftp: boolean, authenticationTimeoutSeconds: number, egress: RouteSummary["egress"], keywordReplacements: KeywordReplacement[]) => {
    if (!policyRoute) return;
    try {
      const routes = isTauri
        ? await invoke<RouteSummary[]>("set_ssh_policy", { alias: policyRoute.alias, name, localUsername, allowedCommand, allowAllCommands, recordCommands, allowSftp, authenticationTimeoutSeconds, egress }).then(() => invoke<RouteSummary[]>("set_ssh_keyword_replacements", { alias: policyRoute.alias, replacements: keywordReplacements }))
        : control.routes.map((route) => route.alias === policyRoute.alias ? {
          ...route,
          name,
          localUsername,
          localEndpoint: `${localUsername}@127.0.0.1:4770`,
          allowedCommand: allowAllCommands ? "" : allowedCommand,
          allowAllCommands,
          recordCommands,
          allowSftp,
          authenticationTimeoutSeconds,
          keywordReplacementCount: keywordReplacements.length,
          egress,
          permissionSummary: `${allowAllCommands ? "all exec commands · high risk" : "1 exact command · stdin denied"}${recordCommands ? " · recorded" : ""}${allowSftp ? " · SFTP high risk" : ""}${keywordReplacements.length ? ` · ${keywordReplacements.length} rewrite rules` : ""}`,
        } : route);
      setControl((current) => ({ ...current, routes }));
      setPolicyRoute(undefined);
      setNotice("SSH 映射与出口替换规则已更新");
    } catch (error) {
      setNotice(String(error));
      throw error;
    }
  };

  const updateSSHHost = async (input: SSHHostUpdateInput) => {
    if (!policyRoute) return;
    const routes = isTauri
      ? await invoke<RouteSummary[]>("update_ssh_target", { input: { alias: policyRoute.alias, ...input } })
      : control.routes.map((route) => route.alias === policyRoute.alias ? { ...route, health: "unknown" as const } : route);
    const updated = routes.find((route) => route.alias === policyRoute.alias);
    setControl((current) => ({ ...current, routes }));
    if (updated) setPolicyRoute(updated);
    setNotice("SSH 宿主机与受保护凭据已更新");
  };

  const rotateSSHCredential = async (localPassword: string) => {
    if (!policyRoute) throw new Error("SSH 映射不可用");
    const result = isTauri
      ? await invoke<SSHRouteCreationResult>("rotate_ssh_credential", { alias: policyRoute.alias, localPassword })
      : { route: policyRoute, localCredential: localPassword ? "" : "airlock_preview_rotated_credential", generatedCredential: !localPassword };
    setControl((current) => ({ ...current, routes: current.routes.map((route) => route.alias === result.route.alias ? result.route : route) }));
    setPolicyRoute(result.route);
    setNotice("SSH 本地凭据已轮换，旧凭据立即失效");
    return result;
  };

  const updateLLMPolicy = async (models: string[], maxOutputTokens: number, requestsPerMinute: number, maxConcurrent: number, trackUsage: boolean) => {
    if (!policyRoute) return;
    try {
      const routes = isTauri
        ? await invoke<RouteSummary[]>("set_llm_policy", { alias: policyRoute.alias, models, maxOutputTokens, requestsPerMinute, maxConcurrent, trackUsage })
        : control.routes.map((route) => route.alias === policyRoute.alias ? {
          ...route,
          allowedModels: models,
          maxOutputTokens,
          requestsPerMinute,
          maxConcurrent,
          trackUsage,
          permissionSummary: `${route.provider === "anthropic" ? "Anthropic" : "OpenAI"} · ${models.length} models · output ≤ ${maxOutputTokens} · ${requestsPerMinute}/min · ${maxConcurrent} concurrent`,
        } : route);
      setControl((current) => ({ ...current, routes }));
      setPolicyRoute(undefined);
      setNotice("LLM 访问边界已更新");
    } catch (error) {
      setNotice(String(error));
    }
  };

  const rotateLLMKey = async () => {
    if (!policyRoute) return;
    try {
      const updated = isTauri
        ? await invoke<RouteSummary>("rotate_llm_api_key", { alias: policyRoute.alias })
        : policyRoute;
      setControl((current) => ({ ...current, routes: current.routes.map((route) => route.alias === updated.alias ? updated : route) }));
      setPolicyRoute(updated);
      setNotice("二次 API Key 已轮换，旧 Key 已失效");
    } catch (error) {
      setNotice(String(error));
    }
  };

  const resetLLMUsage = async () => {
    if (!policyRoute) return;
    try {
      const routes = isTauri
        ? await invoke<RouteSummary[]>("reset_llm_usage", { alias: policyRoute.alias })
        : control.routes.map((route) => route.alias === policyRoute.alias ? { ...route, totalRequests: 0, inputTokens: 0, outputTokens: 0 } : route);
      const updated = routes.find((route) => route.alias === policyRoute.alias);
      setControl((current) => ({ ...current, routes }));
      if (updated) setPolicyRoute(updated);
      setNotice("LLM 使用量统计已清零");
    } catch (error) {
      setNotice(String(error));
    }
  };

  const runHealthCheck = async (alias: string, announce = true) => {
    setTestingAliases((current) => new Set(current).add(alias));
    try {
      const update: ControlUpdate = isTauri
        ? await invoke<ControlUpdate>("test_route_health", { alias, authenticationTimeoutSeconds: control.routes.find((route) => route.alias === alias)?.authenticationTimeoutSeconds })
        : {
          routes: control.routes.map((route) => route.alias === alias ? { ...route, health: "healthy" as const } : route),
          message: "健康 · 本地预览连通性检查通过",
        };
      setControl((current) => ({ ...current, routes: update.routes }));
      if (announce) setNotice(update.message ?? "健康检查完成");
      return update.routes.find((route) => route.alias === alias)?.health === "healthy";
    } catch (error) {
      if (announce) setNotice(String(error));
      return false;
    } finally {
      setTestingAliases((current) => {
        const next = new Set(current);
        next.delete(alias);
        return next;
      });
    }
  };

  const testAllRoutes = async (aliases = control.routes.map((route) => route.alias)) => {
    let healthy = 0;
    for (const alias of aliases) {
      if (await runHealthCheck(alias, false)) healthy += 1;
    }
    setNotice(`健康检查完成 · ${healthy}/${aliases.length} 条通过`);
  };

  const checkUpdates = async () => {
    setCheckingUpdate(true);
    const result = await checkForUpdates();
    setUpdateCheck(result);
    setCheckingUpdate(false);
    if (result.status === "available") setNotice(`发现 Airlock v${result.latest}，请手动下载并核验。`);
    else if (result.status === "current") setNotice("已是最新稳定版本");
    else setNotice("暂时无法读取公开发布信息");
  };

  const testProxyHealth = async () => {
    setProxyTesting(true);
    try {
      const update = isTauri
        ? await invoke<ControlUpdate>("test_proxy_health")
        : { routes: control.routes, message: "本地代理 TCP 端口可达" };
      setControl((current) => ({ ...current, routes: update.routes }));
      setNotice(update.message ?? "代理健康检查完成");
    } catch (error) {
      setNotice(String(error));
    } finally {
      setProxyTesting(false);
    }
  };

	const applySecuritySettings = async (settings: SecuritySettings) => {
		try {
			const update = isTauri
				? await invoke<SecurityUpdate>("apply_security_settings", { networkScope: settings.networkScope, secretStore: settings.secretStore, httpPort: settings.httpPort, sshPort: settings.sshPort })
				: { securitySettings: settings, message: "安全设置已更新" };
			setControl((current) => ({ ...current, securitySettings: update.securitySettings }));
			setNotice(update.message ?? "安全设置已更新");
			await refresh();
		} catch (error) {
			setNotice(String(error));
			throw error;
		}
	};

  const terminatePortOwner = async (owner: PortOwner) => {
    try {
      const message = await invoke<string>("terminate_listener_port_owner", { port: owner.port, pid: owner.pid });
      setNotice(message);
      await restartLocalCore();
    } catch (error) {
      setNotice(String(error));
      throw error;
    }
  };

  return (
    <div className="app-shell">
      <WindowChrome />
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark"><LockKeyhole size={17} /></span><span><strong>Airlock</strong><small>Local security relay</small></span></div>
        <nav aria-label="主导航">
          {navItems.map((item) => {
            const Icon = item.icon;
            return <button key={item.id} className={`nav-item ${page === item.id ? "active" : ""}`} onClick={() => setPage(item.id)}><Icon size={17} /><span>{item.label}</span></button>;
          })}
        </nav>
        <div className="daemon-summary">
          <span className={`status-dot ${control.connected ? "online" : "offline"}`} />
          <div><strong>{translate(control.connected ? "本地核心已连接" : "等待本地核心")}</strong><span>airlockd · {control.securitySettings.networkScope === "lan" ? "LAN relay" : "loopback only"}</span></div>
        </div>
      </aside>

      <main className="workspace">
        <header className="toolbar">
          <div className="service-state"><ShieldCheck size={16} /><span>{control.connected ? translate(`${enabledCount} 条路由已开放`) : translate("控制通道未连接")}</span></div>
          <div className="toolbar-actions">
            <button className="icon-button" onClick={() => void refresh()} title="刷新状态" aria-label="刷新状态"><RefreshCw size={16} /></button>
            <button className="danger-button" onClick={() => setEmergencyOpen(true)} disabled={!control.connected || enabledCount === 0}><CircleStop size={16} />停止全部</button>
          </div>
        </header>

        <div className="page-content" key={page}>
          {page === "overview" && <Overview control={control} platform={platform} onRoutes={() => setPage("routes")} onAdd={() => openEditor()} onRetry={restartLocalCore} onManagePorts={() => setPortManagerOpen(true)} retrying={coreRetrying} />}
          {page === "routes" && <Routes routes={control.routes} connected={control.connected} testingAliases={testingAliases} onToggle={toggleRoute} onDelete={setPendingDelete} onPolicy={setPolicyRoute} onTest={runHealthCheck} onTestAll={testAllRoutes} onAdd={() => openEditor()} />}
          {page === "activity" && <ActivityPage events={control.activity} />}
          {page === "guide" && <GuidePage onRoutes={() => setPage("routes")} />}
          {page === "settings" && <SettingsPage platform={platform} language={language} onLanguage={setLanguage} theme={theme} onTheme={setTheme} accent={accent} onAccent={setAccent} density={density} onDensity={setDensity} motion={motion} onMotion={setMotion} refreshInterval={refreshInterval} onRefreshInterval={setRefreshInterval} connected={control.connected} proxyConfigured={control.proxyConfigured} proxyTesting={proxyTesting} sshReady={control.sshReady} securitySettings={control.securitySettings} onSecuritySettings={applySecuritySettings} onConfigureProxy={configureProxy} onClearProxy={clearProxy} onTestProxy={testProxyHealth} updateCheck={updateCheck} checkingUpdate={checkingUpdate} onCheckUpdates={checkUpdates} onOpenGuide={() => setPage("guide")} onManagePorts={() => setPortManagerOpen(true)} />}
        </div>
      </main>

      {notice && <div className="toast" role="status">{notice}</div>}
      {portManagerOpen && <PortManager securitySettings={control.securitySettings} onClose={() => setPortManagerOpen(false)} onSave={applySecuritySettings} onTerminate={terminatePortOwner} />}
      {emergencyOpen && <Modal title="停止全部路由" onClose={() => setEmergencyOpen(false)}><div className="warning-panel"><AlertTriangle size={19} /><p>新请求将立即被拒绝，已建立的连接会进入关闭流程。</p></div><div className="modal-actions"><button className="secondary-button" onClick={() => setEmergencyOpen(false)}>取消</button><button className="danger-button" onClick={() => void stopAll()}><CircleStop size={16} />确认停止</button></div></Modal>}
	  {pendingDelete && <Modal title="删除路由" onClose={() => setPendingDelete(undefined)}><div className="danger-panel"><Trash2 size={19} /><div><strong>{pendingDelete.name}</strong><p>本地入口、Capability 和当前 SecretStore 中的受保护目标都会被永久删除。</p></div></div><div className="modal-actions"><button className="secondary-button" onClick={() => setPendingDelete(undefined)}>取消</button><button className="danger-button" onClick={() => void deleteRoute()}><Trash2 size={16} />删除路由</button></div></Modal>}
      {policyRoute?.kind === "SSH" && <SSHPolicyEditor route={policyRoute} platform={platform} testing={testingAliases.has(policyRoute.alias)} onClose={() => setPolicyRoute(undefined)} onSave={updateSSHPolicy} onUpdateHost={updateSSHHost} onRotateCredential={rotateSSHCredential} onTest={() => runHealthCheck(policyRoute.alias)} onAddHost={() => { setPolicyRoute(undefined); openEditor("SSH"); }} onDelete={() => { setPendingDelete(policyRoute); setPolicyRoute(undefined); }} />}
      {policyRoute?.kind === "LLM" && <LLMPolicyEditor route={policyRoute} onClose={() => setPolicyRoute(undefined)} onSave={updateLLMPolicy} onRotate={rotateLLMKey} onResetUsage={resetLLMUsage} />}
      {editorOpen && <RouteEditor initialKind={editorKind} routes={control.routes} connected={control.connected} proxyConfigured={control.proxyConfigured} sshReady={control.sshReady} onClose={() => setEditorOpen(false)} onCreated={(route) => setControl((current) => ({ ...current, routes: [...current.routes.filter((item) => item.id !== route.id), route] }))} onError={setNotice} />}
    </div>
  );
}

function platformSecretStoreName(platform: PlatformInfo): string {
  return platform.secretStore === "credential-manager" ? "凭据管理器" : platform.secretStore === "secret-service" ? "Secret Service" : "Keychain";
}

function platformFileStoreName(platform: PlatformInfo): string {
  return platform.os === "windows" ? "ACL 文件" : "0600 文件";
}

function platformControlTransportName(platform: PlatformInfo): string {
  return platform.controlTransport === "named-pipe" ? "命名管道 · 仅当前用户" : "Unix Socket · 仅当前用户";
}

function platformSecretStoreExplain(platform: PlatformInfo): string {
  return platform.os === "windows"
    ? "Windows 决定何时显示凭据输入框，Airlock 不能绕过该系统授权。"
    : platform.os === "linux"
      ? "系统决定何时要求解锁 Secret Service 钥匙串，Airlock 不能绕过该授权。"
      : "macOS 决定何时显示密码框，Airlock 不能绕过该系统授权。";
}

function platformFileStoreExplain(platform: PlatformInfo): string {
  return platform.os === "windows"
    ? "Secret 仅由当前 Windows 账户与 ACL 文件权限隔离；同账户的其他进程可能读取。"
    : "Secret 仅由当前账户与 0600 文件权限隔离；同账户的其他进程可能读取。";
}

function platformNativeConfirmText(platform: PlatformInfo): string {
  return platform.os === "windows"
    ? "该模式接近远程命令执行权限。保存时还会出现一次 Windows 原生风险确认。"
    : platform.os === "linux"
      ? "该模式接近远程命令执行权限。保存时还会出现一次系统原生风险确认。"
      : "该模式接近远程命令执行权限。保存时还会出现一次 macOS 原生风险确认。";
}

function platformOsLabel(platform: PlatformInfo): string {
  const arch = platform.arch || "unknown";
  return platform.os === "windows"
    ? `Windows · ${arch}`
    : platform.os === "linux"
      ? `Linux · ${arch}`
      : platform.os === "macos"
        ? `macOS · ${arch}`
        : `未知系统 · ${arch}`;
}

function platformStoreLabel(platform: PlatformInfo): string {
  return platform.secretStore === "credential-manager"
    ? "Windows 凭据管理器"
    : platform.secretStore === "secret-service"
      ? "Secret Service"
      : "macOS Keychain";
}

function Overview({ control, platform, onRoutes, onAdd, onRetry, onManagePorts, retrying }: { control: ControlState; platform: PlatformInfo; onRoutes: () => void; onAdd: () => void; onRetry: () => void; onManagePorts: () => void; retrying: boolean }) {
  const enabled = control.routes.filter((route) => route.status === "enabled").length;
  const connections = control.routes.reduce((sum, route) => sum + route.currentConnections, 0);
  return <>
    <PageHeader title="概览" subtitle="本机开放能力与安全状态" action={<button className="primary-button" onClick={onAdd} disabled={!control.connected}><Plus size={16} />新增路由</button>} />
    <section className={`service-band ${control.connected ? "running" : "stopped"}`}>
      <span className="service-icon"><ShieldCheck size={20} /></span>
      <div className="service-copy"><strong>{translate(control.connected ? "受保护控制通道已连接" : "airlockd 尚未连接")}</strong><span>{control.connected ? translate(platformControlTransportName(platform)) : translate(control.message ?? "启动本地核心后将自动重连")}</span></div>
      <div className="listener-status"><span><Server size={14} />HTTP <b>{control.connected ? "ON" : "OFF"}</b></span><span><SquareTerminal size={14} />SSH <b>{control.sshReady ? "ON" : "OFF"}</b></span></div>
      {!control.connected && <div className="core-recovery-actions"><button className="secondary-button compact" onClick={onManagePorts} disabled={retrying}><Cable size={14} />{translate("管理端口")}</button><button className="secondary-button compact core-retry" onClick={onRetry} disabled={retrying}>{retrying ? <LoaderCircle className="spin" size={14} /> : <RefreshCw size={14} />}{translate(retrying ? "正在重启本地核心" : "重试本地核心")}</button></div>}
    </section>
    <section className="metric-strip" aria-label="运行指标">
      <Metric label="开放路由" value={String(enabled)} detail={translate(`共 ${control.routes.length} 条`)} />
      <Metric label="当前连接" value={String(connections)} detail="仅统计本地入口" />
	  <Metric label="凭据存储" value={control.securitySettings.secretStore === "local_file" ? "本机文件" : platformSecretStoreName(platform)} detail={control.securitySettings.secretStore === "local_file" ? "账户与文件权限隔离" : "系统加密保护"} tone={control.securitySettings.secretStore === "local_file" ? "warning" : "success"} />
    </section>
    <section className="section-block">
      <div className="section-heading"><div><h2>路由</h2><p>界面只显示安全别名和本地入口。</p></div><button className="text-button" onClick={onRoutes}>查看全部<ChevronRight size={15} /></button></div>
      <RouteTable routes={control.routes.slice(0, 5)} compact />
    </section>
  </>;
}

function Routes({ routes, connected, testingAliases, onToggle, onDelete, onPolicy, onTest, onTestAll, onAdd }: { routes: RouteSummary[]; connected: boolean; testingAliases: ReadonlySet<string>; onToggle: (alias: string, enabled: boolean) => void; onDelete: (route: RouteSummary) => void; onPolicy: (route: RouteSummary) => void; onTest: (alias: string) => Promise<boolean>; onTestAll: (aliases: string[]) => Promise<void>; onAdd: () => void }) {
  const [kind, setKind] = useState<RouteFilter>("All");
  const [status, setStatus] = useState<RouteStatusFilter>("All");
  const [health, setHealth] = useState<RouteHealthFilter>("All");
  const [egress, setEgress] = useState<RouteEgressFilter>("All");
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return routes.filter((route) => {
      const searchable = `${route.name} ${route.alias} ${route.localUsername ?? ""} ${route.localEndpoint} ${route.kind}`.toLocaleLowerCase();
      return (kind === "All" || route.kind === kind)
        && (status === "All" || route.status === status)
        && (health === "All" || route.health === health)
        && (egress === "All" || route.egress === egress)
        && (!normalizedQuery || searchable.includes(normalizedQuery));
    });
  }, [routes, kind, status, health, egress, query]);
  const filterCount = Number(kind !== "All") + Number(status !== "All") + Number(health !== "All") + Number(egress !== "All") + Number(Boolean(query.trim()));
  const clearFilters = () => { setKind("All"); setStatus("All"); setHealth("All"); setEgress("All"); setQuery(""); };
  return <>
    <PageHeader title="路由" subtitle={translate(`${routes.length} 条 · ${routes.filter((route) => route.status === "enabled").length} 条已开放`)} action={<div className="page-actions"><button className="secondary-button" onClick={() => void onTestAll(filtered.map((route) => route.alias))} disabled={!connected || filtered.length === 0 || testingAliases.size > 0}>{testingAliases.size > 0 ? <LoaderCircle className="spin" size={15} /> : <HeartPulse size={15} />}{testingAliases.size > 0 ? "检测中" : "检测筛选结果"}</button><button className="primary-button" onClick={onAdd} disabled={!connected}><Plus size={16} />新增路由</button></div>} />
    <div className="route-filter-panel">
      <div className="filter-bar"><label className="search-field"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索名称、别名或本地入口" /></label><span className="filter-result"><Filter size={14} />{translate(`${filtered.length} 条匹配`)}</span><button className="text-button clear-filter" onClick={clearFilters} disabled={filterCount === 0}><FilterX size={15} />清空筛选</button></div>
      <div className="route-filter-controls">
        <RouteFilterSelect label="类型" value={kind} onChange={setKind} options={[{ value: "All", label: "全部类型" }, { value: "HTTP", label: "HTTP" }, { value: "SSH", label: "SSH" }, { value: "LLM", label: "LLM" }]} />
        <RouteFilterSelect label="状态" value={status} onChange={setStatus} options={[{ value: "All", label: "全部状态" }, { value: "enabled", label: "已启用" }, { value: "disabled", label: "已停用" }, { value: "blocked", label: "已阻止" }]} />
        <RouteFilterSelect label="健康" value={health} onChange={setHealth} options={[{ value: "All", label: "全部健康状态" }, { value: "healthy", label: "健康" }, { value: "degraded", label: "异常" }, { value: "unknown", label: "未测试" }]} />
        <RouteFilterSelect label="出口" value={egress} onChange={setEgress} options={[{ value: "All", label: "全部出口" }, { value: "Direct", label: "Direct" }, { value: "Proxy", label: "Proxy" }, { value: "Auto", label: "Auto" }]} />
      </div>
    </div>
    <RouteTable routes={filtered} testingAliases={testingAliases} onToggle={(route) => onToggle(route.alias, route.status !== "enabled")} onDelete={onDelete} onPolicy={onPolicy} onTest={(route) => void onTest(route.alias)} />
  </>;
}

function RouteFilterSelect<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: Array<{ value: T; label: string }>; onChange: (value: T) => void }) {
  const id = useId();
  return <label className="route-filter-select" htmlFor={id}><span>{label}</span><select id={id} value={value} onChange={(event) => onChange(event.target.value as T)}>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>;
}

function RouteTable({ routes, compact = false, testingAliases = new Set<string>(), onToggle, onDelete, onPolicy, onTest }: { routes: RouteSummary[]; compact?: boolean; testingAliases?: ReadonlySet<string>; onToggle?: (route: RouteSummary) => void; onDelete?: (route: RouteSummary) => void; onPolicy?: (route: RouteSummary) => void; onTest?: (route: RouteSummary) => void }) {
  if (routes.length === 0) return <EmptyState icon={Route} title="暂无路由" detail="创建后，本地入口会显示在这里。" />;
  return <div className={`route-list ${compact ? "route-list-compact" : ""}`} role="list" aria-label="路由列表">{routes.map((route, index) => {
    const testing = testingAliases.has(route.alias);
    const hasPolicyAction = onPolicy && (route.kind === "SSH" || route.kind === "LLM");
    return <article key={route.id} className={`route-item ${route.status === "enabled" ? "" : "route-muted"}`} role="listitem" style={{ animationDelay: `${index * 32}ms` }}>
      <div className="route-state-cell"><StatusBadge status={route.status} /></div>
      <div className="route-identity" data-i18n="off"><div><strong>{route.name}</strong><span className="route-alias">{route.alias}</span></div><KindBadge kind={route.kind} /></div>
      <div className="route-endpoint" data-i18n="off"><span>本地入口</span><code title={route.localEndpoint}>{route.localEndpoint}</code></div>
      <div className="route-policy"><span>访问边界</span><div className="route-policy-copy"><PermissionSummary route={route} /></div></div>
      <div className="route-network"><span className="route-egress"><Network size={13} />{route.egress}</span><HealthBadge health={route.health} checking={testing} />{!compact && <small>{route.lastUsed}</small>}</div>
      {onToggle && <div className="route-actions" aria-label={`${route.name} 操作`}>
        {hasPolicyAction && <button className="row-icon-button visible" title={route.kind === "SSH" ? (route.health === "unknown" || route.health === "degraded" ? "编辑或修复 SSH 映射" : "SSH 命令权限") : "LLM 访问边界"} aria-label={translate(`设置 ${route.name} 的访问边界`)} onClick={() => onPolicy(route)}>{route.kind === "LLM" ? <SlidersHorizontal size={14} /> : route.health === "unknown" || route.health === "degraded" ? <PencilLine size={14} /> : <Settings2 size={14} />}</button>}
        {onTest && <button className={`row-icon-button visible health-action ${testing ? "checking" : ""}`} title="测试上游连通性与身份校验" aria-label={translate(`测试 ${route.name} 的连通性与健康状态`)} onClick={() => onTest(route)} disabled={testing}>{testing ? <LoaderCircle className="spin" size={14} /> : <HeartPulse size={14} />}</button>}
        <button className="route-switch" role="switch" aria-checked={route.status === "enabled"} title={route.status === "enabled" ? "停用路由" : "启用路由"} aria-label={route.status === "enabled" ? "停用路由" : "启用路由"} onClick={() => onToggle(route)}><span /></button>
        {onDelete && <button className="row-icon-button danger visible" title="删除路由" aria-label={translate(`删除 ${route.name}`)} onClick={() => onDelete(route)}><Trash2 size={14} /></button>}
      </div>}
    </article>;
  })}</div>;
}

function ActivityPage({ events }: { events: ActivityEvent[] }) {
  const [filter, setFilter] = useState<ActivityFilter>("All");
  const filtered = useMemo(() => events.filter((event) => {
    if (filter === "All") return true;
    if (filter === "Health") return event.eventType === "health";
    if (filter === "Problems") return event.result !== "allowed";
    return event.category === filter;
  }), [events, filter]);
  const options: Array<{ value: ActivityFilter; label: string }> = [
    { value: "All", label: `全部 ${events.length}` },
    { value: "HTTP", label: `HTTP ${events.filter((event) => event.category === "HTTP").length}` },
    { value: "SSH", label: `SSH ${events.filter((event) => event.category === "SSH").length}` },
    { value: "LLM", label: `LLM ${events.filter((event) => event.category === "LLM").length}` },
    { value: "Health", label: translate(`健康检查分类 ${events.filter((event) => event.eventType === "health").length}`) },
    { value: "Problems", label: `异常 ${events.filter((event) => event.result !== "allowed").length}` },
  ];
  return <><PageHeader title="活动" subtitle={translate("默认仅保留脱敏类别、结果与耗时；开启命令审计后，可在详情中查看完整 SSH 命令")} />
    <div className="activity-summary" aria-label="活动摘要"><span><Activity size={15} /><b>{events.length}</b> 最近事件</span><span><HeartPulse size={15} /><b>{events.filter((event) => event.eventType === "health").length}</b> 健康检查</span><span className={events.some((event) => event.result !== "allowed") ? "has-problems" : ""}><ShieldAlert size={15} /><b>{events.filter((event) => event.result !== "allowed").length}</b> 异常或阻止</span></div>
    <div className="activity-filter-bar"><div className="segmented activity-segmented" aria-label="活动分类">{options.map((option) => <button key={option.value} className={filter === option.value ? "selected" : ""} onClick={() => setFilter(option.value)}>{option.label}</button>)}</div><span>{translate(`${filtered.length} 条匹配`)}</span></div>
    <ActivityTable events={filtered} /></>;
}

function ActivityTable({ events }: { events: ActivityEvent[] }) {
  const [selected, setSelected] = useState<ActivityEvent>();
  if (events.length === 0) return <EmptyState icon={Activity} title="暂无活动" detail="新的脱敏事件会显示在这里。" />;
  const open = (event: ActivityEvent) => setSelected(event);
  return <>
    <div className="table-wrap activity-table-wrap"><table className="activity-table"><thead><tr><th>时间</th><th>类别</th><th>路由</th><th>动作</th><th>结果</th><th>延迟</th><th><span className="sr-only">查看详情</span></th></tr></thead><tbody>{events.map((event, index) => <tr key={event.id} className="activity-row" tabIndex={0} role="button" aria-label={`${translate("查看活动详情")} · ${event.routeName}`} style={{ animationDelay: `${index * 26}ms` }} onClick={() => open(event)} onKeyDown={(keyEvent) => { if (keyEvent.key === "Enter" || keyEvent.key === " ") { keyEvent.preventDefault(); open(event); } }}><td>{event.time}</td><td><ActivityKindBadge event={event} /></td><td data-i18n="off"><strong>{event.routeName}</strong></td><td><code className="command-cell" data-i18n={event.eventType === "command" ? "off" : undefined}>{formatActivityAction(event)}</code></td><td><StatusBadge status={event.result} /></td><td>{event.latency}</td><td><ChevronRight className="activity-detail-arrow" size={16} aria-hidden="true" /></td></tr>)}</tbody></table></div>
    {selected && <ActivityDetail event={selected} onClose={() => setSelected(undefined)} />}
  </>;
}

function ActivityDetail({ event, onClose }: { event: ActivityEvent; onClose: () => void }) {
  const action = event.detail || formatActivityAction(event);
  const copy = (value: string) => { if (navigator.clipboard) void navigator.clipboard.writeText(value); };
  const eventType = event.eventType === "health" ? "健康检查" : event.eventType === "command" ? "SSH 动作" : "请求";
  return <Modal title={translate("活动详情")} className="modal-wide activity-detail-modal" onClose={onClose}>
    <div className="activity-detail">
      <div className="activity-detail-lead"><ActivityKindBadge event={event} /><StatusBadge status={event.result} /><span>{event.time}</span></div>
      <dl className="activity-detail-grid">
        <div><dt>路由</dt><dd data-i18n="off">{event.routeName}</dd></div>
        <div><dt>调用者</dt><dd data-i18n="off">{event.caller}</dd></div>
        <div><dt>{translate("事件类型")}</dt><dd>{translate(eventType)}</dd></div>
        <div><dt>延迟</dt><dd>{event.latency}</dd></div>
        <div><dt>出口</dt><dd>{event.egress}</dd></div>
        <div className="activity-event-id"><dt>事件 ID</dt><dd data-i18n="off"><code>{event.id}</code><button className="icon-button small" type="button" onClick={() => copy(event.id)} title={translate("复制事件 ID")} aria-label={translate("复制事件 ID")}><Copy size={14} /></button></dd></div>
      </dl>
      <section className="activity-action-detail"><header><div><strong>{translate("完整动作")}</strong><span>{translate(event.eventType === "command" ? "仅在路由启用命令记录时保存完整命令" : "已脱敏，不包含目标、凭据或请求正文")}</span></div><button className="secondary-button compact" type="button" onClick={() => copy(action)}><Copy size={14} />{translate("复制内容")}</button></header><pre data-i18n="off">{action}</pre></section>
    </div>
  </Modal>;
}

function GuidePage({ onRoutes }: { onRoutes: () => void }) {
  return <>
    <PageHeader title="使用指南" subtitle="配置固定能力、观察本地状态，并始终把真实凭据留在 Airlock 内" action={<button className="primary-button" onClick={onRoutes}><Route size={16} />管理路由</button>} />
    <section className="guide-lead"><span className="guide-lead-icon"><BookOpen size={22} /></span><div><strong>Airlock 是固定路由转发器，不是通用代理。</strong><p>调用方只获得单条路由的本地凭据；上游 URL、SSH 账户密码与真实 API Key 始终保留在受保护的本机存储中。</p></div></section>
    <section className="guide-section"><div className="guide-section-heading"><span>01</span><div><h2>创建最小权限路由</h2><p>先给每个调用方一条独立入口，再决定它能做什么。</p></div></div><div className="guide-grid"><div><Server size={17} /><strong>HTTP / Wget</strong><p>固定目标、方法、路径与查询边界，适合下载和 API 中转。</p></div><div><SquareTerminal size={17} /><strong>SSH</strong><p>本地用户名和密码与上游身份分离，默认只允许明确的非交互命令。</p></div><div><Sparkles size={17} /><strong>LLM API</strong><p>使用可轮换的二次 Key，按模型、输出、频率和并发限制调用。</p></div></div></section>
    <section className="guide-section"><div className="guide-section-heading"><span>02</span><div><h2>搜索、筛选与健康检查</h2><p>路由页可组合类型、状态、健康和出口筛选，并只在已筛选的路由上批量运行健康检查。</p></div></div><div className="guide-detail"><Filter size={17} /><p>搜索仅匹配路由名称、别名、本地用户名与本地入口。不会读取、显示或发送受保护的上游地址、密码、请求内容或 API Key。</p></div></section>
    <section className="guide-section"><div className="guide-section-heading"><span>03</span><div><h2>版本、CLI 与平台状态</h2><p>检查更新只读取官方公开发布元数据；不会自动下载或安装。</p></div></div><div className="guide-command-list"><code>airlock status</code><span>显示当前安装包、目标平台与发布状态。</span><code>airlock platform --json</code><span>供脚本读取已发布和计划中的平台契约。</span><code>airlock doctor</code><span>验证本地安装镜像的 SHA-256，不启动应用。</span></div><div className="guide-detail"><ShieldCheck size={17} /><p>只有同时具备对应平台产物和固定 SHA-256 的目标才会被标记为已发布。Windows 与 Linux 契约已准备，但当前不会被误报为可下载安装。</p></div></section>
  </>;
}

function SettingsPage({ platform, language, onLanguage, theme, onTheme, accent, onAccent, density, onDensity, motion, onMotion, refreshInterval, onRefreshInterval, connected, proxyConfigured, proxyTesting, sshReady, securitySettings, onSecuritySettings, onConfigureProxy, onClearProxy, onTestProxy, updateCheck, checkingUpdate, onCheckUpdates, onOpenGuide, onManagePorts }: { platform: PlatformInfo; language: LocalePreference; onLanguage: (language: LocalePreference) => void; theme: ThemePreference; onTheme: (theme: ThemePreference) => void; accent: AccentTheme; onAccent: (accent: AccentTheme) => void; density: DensityPreference; onDensity: (density: DensityPreference) => void; motion: MotionPreference; onMotion: (motion: MotionPreference) => void; refreshInterval: RefreshInterval; onRefreshInterval: (interval: RefreshInterval) => void; connected: boolean; proxyConfigured: boolean; proxyTesting: boolean; sshReady: boolean; securitySettings: SecuritySettings; onSecuritySettings: (settings: SecuritySettings) => Promise<void>; onConfigureProxy: () => void; onClearProxy: () => void; onTestProxy: () => void; updateCheck?: UpdateCheckResult; checkingUpdate: boolean; onCheckUpdates: () => Promise<void>; onOpenGuide: () => void; onManagePorts: () => void }) {
  const [draft, setDraft] = useState(securitySettings);
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    setDraft((current) => {
      const hasUnsavedChanges = current.secretStore !== securitySettings.secretStore || current.networkScope !== securitySettings.networkScope || current.httpPort !== securitySettings.httpPort || current.sshPort !== securitySettings.sshPort;
      return saving || hasUnsavedChanges ? current : securitySettings;
    });
  }, [securitySettings, saving]);
  const preset = draft.secretStore === "keychain" && draft.networkScope === "loopback" ? "strict" : draft.secretStore === "local_file" && draft.networkScope === "loopback" ? "standard" : draft.secretStore === "local_file" && draft.networkScope === "lan" ? "convenient" : "custom";
  const dirty = draft.secretStore !== securitySettings.secretStore || draft.networkScope !== securitySettings.networkScope || draft.httpPort !== securitySettings.httpPort || draft.sshPort !== securitySettings.sshPort;
  const choosePreset = (value: "strict" | "standard" | "convenient") => setDraft((current) => ({ ...current, secretStore: value === "strict" ? "keychain" : "local_file", networkScope: value === "convenient" ? "lan" : "loopback" }));
  const apply = async () => { setSaving(true); try { await onSecuritySettings(draft); } catch {} finally { setSaving(false); } };
  const activeNetwork = securitySettings.networkScope;
  const presetLabel = preset === "strict" ? "严格" : preset === "standard" ? "标准" : preset === "convenient" ? "便捷" : "自定义";
  const levelLabel = preset === "strict" ? "高保护" : preset === "convenient" ? "局域网暴露" : preset === "standard" ? "推荐默认" : "自定义边界";
  const secretStoreName = platformSecretStoreName(platform);
  const fileStoreName = platformFileStoreName(platform);
  const strictDescription = platform.os === "windows"
    ? "适合长期保存高价值凭据。Secret 由 Windows 凭据管理器加密并控制访问，系统可能要求验证登录凭据。"
    : platform.os === "linux"
      ? "适合长期保存高价值凭据。Secret 由桌面 Secret Service 加密并控制访问，系统可能要求解锁钥匙串。"
      : "适合长期保存高价值凭据。Secret 由 macOS 加密并控制访问，系统可能要求验证登录密码。";
  const strictStore = platform.secretStore === "credential-manager" ? "Windows 凭据管理器" : platform.secretStore === "secret-service" ? "Secret Service" : "系统钥匙串";
  const profiles = [
    { id: "strict" as const, title: "严格", subtitle: `${secretStoreName} · 仅本机`, icon: ShieldCheck, description: strictDescription, store: strictStore, ingress: "仅本机" },
    { id: "standard" as const, title: "标准", subtitle: `${fileStoreName} · 仅本机`, icon: HardDrive, description: "默认方案。启动时不读取系统钥匙串，不弹授权框；Secret 未加密，由当前账户和文件权限隔离。", store: "当前用户文件", ingress: "仅本机" },
    { id: "convenient" as const, title: "便捷", subtitle: `${fileStoreName} · 局域网`, icon: Wifi, description: "用于受信任私网中转。持有路由凭据的局域网设备可访问入口，控制面仍保持本机专用。", store: "当前用户文件", ingress: "私有局域网" },
  ];
  const updateSummary = updateCheck?.status === "available" ? `v${updateCheck.latest} 可手动下载` : updateCheck?.status === "current" ? "已是最新稳定版本" : updateCheck?.status === "unavailable" ? "公开发布信息暂不可用" : "尚未检查";
  return <><PageHeader title="设置" subtitle="本地外观、网络与安全边界" />
    <section className="settings-section"><div><h2>外观</h2><p>主题偏好保存在本机</p></div><div className="settings-controls"><div className="setting-row"><span>显示模式</span><ThemeControl value={theme} onChange={onTheme} /></div><div className="setting-row"><span>配色风格</span><AccentControl value={accent} onChange={onAccent} /></div></div></section>
    <section className="settings-section"><div><h2>界面行为</h2><p>调整刷新节奏、密度与动画</p></div><div className="settings-controls"><PreferenceRow label="界面语言" detail="跟随系统语言，或固定一种语言"><div className="language-control"><Languages size={14} /><PreferenceSegment value={language} options={[{ value: "system", label: "跟随系统" }, { value: "zh-CN", label: "简体中文" }, { value: "en", label: "English" }, { value: "ja", label: "日本語" }]} onChange={onLanguage} /></div></PreferenceRow><PreferenceRow label="自动刷新" detail="控制状态轮询频率"><PreferenceSegment value={refreshInterval} options={[{ value: 2000, label: "2 秒" }, { value: 5000, label: "5 秒" }, { value: 15000, label: "15 秒" }]} onChange={onRefreshInterval} /></PreferenceRow><PreferenceRow label="信息密度" detail="影响表格行高与页面间距"><PreferenceSegment value={density} options={[{ value: "comfortable", label: "舒适" }, { value: "compact", label: "紧凑" }]} onChange={onDensity} /></PreferenceRow><PreferenceRow label="界面动效" detail="精简模式会关闭循环和位移动画"><PreferenceSegment value={motion} options={[{ value: "system", label: "跟随系统" }, { value: "standard", label: "标准" }, { value: "reduced", label: "精简" }]} onChange={onMotion} /></PreferenceRow></div></section>
    <section className="settings-section"><div><h2>版本与文档</h2><p>只检查公开发布信息，不上传本机配置</p></div><div className="settings-controls update-controls"><div className="update-summary"><span className={`update-symbol ${updateCheck?.status ?? "idle"}`}><Sparkles size={16} /></span><div><strong>Airlock v{APP_VERSION}</strong><p>{updateSummary}</p></div><div className="inline-actions"><button className="secondary-button compact" onClick={() => void onCheckUpdates()} disabled={checkingUpdate}>{checkingUpdate ? <LoaderCircle className="spin" size={14} /> : <RefreshCw size={14} />}{checkingUpdate ? "检查中" : "检查更新"}</button><button className="secondary-button compact" onClick={onOpenGuide}><BookOpen size={14} />打开指南</button></div></div><p className="update-note">版本检查不会下载、安装或自动打开外部页面；更新始终由你手动下载并核验。</p></div></section>
    <section className="settings-section"><div><h2>平台状态</h2><p>按当前操作系统同步的本地能力</p></div><div className="settings-controls"><ReadOnlyField label="操作系统" value={platformOsLabel(platform)} tone="success" /><ReadOnlyField label="控制通道" value={platformControlTransportName(platform)} tone="success" /><ReadOnlyField label="凭据存储" value={platformStoreLabel(platform)} tone="success" /><ReadOnlyField label="桌面发行版" value={platform.desktopRelease ? "已发布" : "本分支 · 未发布"} tone={platform.desktopRelease ? "success" : "warning"} /></div></section>
    <section className="settings-section security-settings"><div><h2>安全方案</h2><p>新安装默认标准；迁移会先验证再切换</p></div><div className="settings-controls security-controls">
      <div className="security-heading"><div><span>{dirty ? "待应用方案" : "当前方案"}</span><strong>{presetLabel}</strong></div><span className={`security-level level-${preset}`}>{levelLabel}</span></div>
      <div className="security-profile-grid" role="radiogroup" aria-label="安全等级">{profiles.map((profile) => <SecurityProfile key={profile.id} {...profile} selected={preset === profile.id} recommended={profile.id === "standard"} risk={profile.id === "convenient"} onSelect={() => choosePreset(profile.id)} />)}</div>
      <details className="security-advanced"><summary><Settings2 size={14} />高级组合<span>分别调整凭据保护与入口范围</span></summary><div className="security-advanced-body"><SecurityChoice label="凭据保护" detail="上游地址、账号、密码与代理认证" value={draft.secretStore} options={[{ value: "keychain", label: secretStoreName, icon: KeyRound }, { value: "local_file", label: fileStoreName, icon: HardDrive }]} onChange={(secretStore) => setDraft((current) => ({ ...current, secretStore }))} /><SecurityChoice label="网络范围" detail="只影响数据入口，控制面始终仅当前用户" value={draft.networkScope} options={[{ value: "loopback", label: "仅本机", icon: Monitor }, { value: "lan", label: "局域网", icon: Wifi }]} onChange={(networkScope) => setDraft((current) => ({ ...current, networkScope }))} /></div></details>
      <div className={`security-explainer ${draft.secretStore === "local_file" || draft.networkScope === "lan" ? "warning" : "safe"}`}>{draft.secretStore === "keychain" ? <ShieldCheck size={17} /> : <TriangleAlert size={17} />}<div><strong>{draft.secretStore === "keychain" ? "系统加密保护，会按需授权" : draft.networkScope === "lan" ? "免钥匙串提示，但入口对私网开放" : "免钥匙串提示，但 Secret 不加密"}</strong><p>{draft.secretStore === "keychain" ? platformSecretStoreExplain(platform) : platformFileStoreExplain(platform)}{draft.networkScope === "lan" ? " 请只在受信任局域网使用，绝不要映射到公网。" : ""}</p></div></div>
      {draft.secretStore === "keychain" && (platform.os === "windows"
        ? <div className="keychain-explanation"><KeyRound size={17} /><div><strong>{translate("为什么调试包更容易弹出系统凭据框？")}</strong><p>{translate("本地开发包为未签名或自签名构建；每次重建后，Windows 可能把新的 airlockd 视为不同程序并重新验证凭据管理器访问。正式稳定签名可减少询问，但系统仍保留最终授权决定。")}</p></div></div>
        : platform.os === "linux"
          ? <div className="keychain-explanation"><KeyRound size={17} /><div><strong>{translate("为什么调试包更容易弹出系统钥匙串？")}</strong><p>{translate("Airlock 使用桌面 Secret Service（如 GNOME Keyring / KWallet）。系统要求解锁时，Airlock 不能绕过；正式签名不影响该授权决定。")}</p></div></div>
          : <div className="keychain-explanation"><KeyRound size={17} /><div><strong>{translate("为什么调试包更容易弹出系统密码框？")}</strong><p>{translate("本地开发包采用 ad-hoc 签名；每次重建后，macOS 可能把新的 airlockd 视为不同程序并重新验证钥匙串访问。选择“始终允许”只对当前构建有效。正式稳定签名可减少询问，但 Keychain 仍保留最终授权决定。")}</p></div></div>)}
      <div className="security-actions"><span>{dirty ? "应用后会校验迁移结果并短暂重启 airlockd" : preset === "standard" ? "标准模式启动时不会读取系统钥匙串" : "已与当前运行设置一致"}</span><button className="primary-button" disabled={!dirty || saving || (!connected && draft.secretStore !== securitySettings.secretStore)} onClick={() => void apply()}>{saving ? <RefreshCw className="spin" size={15} /> : <ShieldCheck size={15} />}{saving ? "正在迁移并重启" : "应用设置"}</button></div>
    </div></section>
    <section className="settings-section"><div><h2>网络与出口</h2><p>{activeNetwork === "lan" ? "数据入口已对局域网开放" : "数据入口仅本机可访问"}</p></div><div className="settings-controls"><ReadOnlyField label="HTTP 入口" value={`${activeNetwork === "lan" ? "0.0.0.0" : "127.0.0.1"}:${securitySettings.httpPort}${activeNetwork === "lan" ? " · 请使用本机局域网 IP" : ""}`} tone={activeNetwork === "lan" ? "warning" : undefined} /><ReadOnlyField label="SSH 入口" value={sshReady ? `${activeNetwork === "lan" ? "0.0.0.0" : "127.0.0.1"}:${securitySettings.sshPort}${activeNetwork === "lan" ? " · 局域网" : " · 已就绪"}` : "等待 airlockd"} tone={sshReady && activeNetwork !== "lan" ? "success" : "warning"} /><div className="listener-port-setting"><div><span>{translate("本地监听端口")}</span><strong className="setting-value">HTTP {securitySettings.httpPort} · SSH {securitySettings.sshPort}</strong><small>{translate("端口被占用时，可切换到其他非特权端口，或结束当前用户的占用进程。")}</small></div><button className="secondary-button compact" onClick={onManagePorts}><Cable size={14} />{translate("管理端口")}</button></div><ReadOnlyField label="控制通道" value={connected ? platformControlTransportName(platform) : "等待 airlockd"} tone={connected ? "success" : "warning"} /><div className="proxy-setting"><div><span>Clash / SOCKS5 出口</span><strong className={proxyConfigured ? "setting-value success" : "setting-value"}>{proxyConfigured ? `${securitySettings.secretStore === "local_file" ? fileStoreName : secretStoreName} · 已配置` : "未配置"}</strong></div><div className="inline-actions">{proxyConfigured && <button className="secondary-button compact" onClick={onTestProxy} disabled={!connected || proxyTesting}>{proxyTesting ? <LoaderCircle className="spin" size={14} /> : <HeartPulse size={14} />}{proxyTesting ? "检测中" : "检测连接"}</button>}<button className="secondary-button compact" onClick={onConfigureProxy} disabled={!connected}><Network size={14} />{proxyConfigured ? "更换" : "配置"}</button>{proxyConfigured && <button className="row-icon-button danger visible" onClick={onClearProxy} aria-label="清除代理出口" title="清除代理出口"><Trash2 size={14} /></button>}</div></div></div></section>
    <section className="settings-section"><div><h2>不变的安全边界</h2><p>便捷模式也不会放开控制面</p></div><div className="settings-controls"><ReadOnlyField label="路由元数据" value={platform.os === "windows" ? "ACL · 不包含明文本地密码" : "0600 · 不包含明文本地密码"} tone="success" /><ReadOnlyField label="SSH 安全核心" value={sshReady ? "双会话隔离 · Shell/PTY 默认拒绝" : "等待本地核心"} tone={sshReady ? "success" : "warning"} /><ReadOnlyField label="敏感录入" value="SSH 内嵌录入 · 仅发送到本机核心" tone="success" /></div></section>
    <DeveloperCard />
  </>;
}

function PortManager({ securitySettings, onClose, onSave, onTerminate }: { securitySettings: SecuritySettings; onClose: () => void; onSave: (settings: SecuritySettings) => Promise<void>; onTerminate: (owner: PortOwner) => Promise<void> }) {
  const [httpPort, setHttpPort] = useState(String(securitySettings.httpPort));
  const [sshPort, setSshPort] = useState(String(securitySettings.sshPort));
  const [owners, setOwners] = useState<PortOwner[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [terminating, setTerminating] = useState(false);
  const [pendingOwner, setPendingOwner] = useState<PortOwner>();
  const [error, setError] = useState<string>();
  const parsedHTTPPort = Number(httpPort);
  const parsedSSHPort = Number(sshPort);
  const validPorts = Number.isInteger(parsedHTTPPort) && Number.isInteger(parsedSSHPort) && parsedHTTPPort >= 1024 && parsedHTTPPort <= 65535 && parsedSSHPort >= 1024 && parsedSSHPort <= 65535 && parsedHTTPPort !== parsedSSHPort;
  const changed = parsedHTTPPort !== securitySettings.httpPort || parsedSSHPort !== securitySettings.sshPort;

  const inspect = async () => {
    setLoading(true);
    setError(undefined);
    try {
      if (!isTauri) {
        setOwners([]);
        return;
      }
      const [httpOwners, sshOwners] = await Promise.all([
        invoke<PortOwner[]>("list_listener_port_owners", { port: securitySettings.httpPort }),
        invoke<PortOwner[]>("list_listener_port_owners", { port: securitySettings.sshPort }),
      ]);
      setOwners([...httpOwners, ...sshOwners]);
    } catch (reason) {
      setOwners([]);
      setError(String(reason));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setHttpPort(String(securitySettings.httpPort));
    setSshPort(String(securitySettings.sshPort));
    void inspect();
  }, [securitySettings.httpPort, securitySettings.sshPort]);

  const save = async () => {
    if (!validPorts || saving) return;
    setSaving(true);
    setError(undefined);
    try {
      await onSave({ ...securitySettings, httpPort: parsedHTTPPort, sshPort: parsedSSHPort });
    } catch (reason) {
      setError(String(reason));
    } finally {
      setSaving(false);
    }
  };

  const terminate = async () => {
    if (!pendingOwner || terminating) return;
    setTerminating(true);
    setError(undefined);
    try {
      await onTerminate(pendingOwner);
      setPendingOwner(undefined);
      await inspect();
    } catch (reason) {
      setError(String(reason));
    } finally {
      setTerminating(false);
    }
  };

  return <Modal title={translate("本地端口管理")} className="modal-wide port-manager-modal" onClose={saving || terminating ? () => undefined : onClose}>
    <div className="port-manager-lead"><Cable size={19} /><div><strong>{translate("端口冲突处置")}</strong><p>{translate("只管理 Airlock 的 HTTP 与 SSH 监听端口。结束进程前会再次核对它仍在监听对应端口，且仅允许结束当前系统用户的进程。")}</p></div></div>
    <section className="port-manager-section"><div className="port-manager-heading"><div><strong>{translate("监听端口")}</strong><span>{translate("使用 1024-65535 的不同端口；保存后本地核心会重启，失败时自动恢复原设置。")}</span></div><button className="icon-button small" onClick={() => void inspect()} disabled={loading || saving || terminating} aria-label={translate("检查端口占用")} title={translate("检查端口占用")}>{loading ? <LoaderCircle className="spin" size={15} /> : <RefreshCw size={15} />}</button></div>
      <div className="port-input-grid"><label className={`form-field ${httpPort && (!Number.isInteger(parsedHTTPPort) || parsedHTTPPort < 1024 || parsedHTTPPort > 65535) ? "invalid" : ""}`}><span>HTTP</span><input type="text" inputMode="numeric" value={httpPort} onChange={(event) => setHttpPort(event.target.value.replace(/\D/g, "").slice(0, 5))} maxLength={5} aria-invalid={!validPorts} /><small>{translate("固定 URL 与 LLM 路由")}</small></label><label className={`form-field ${sshPort && (!Number.isInteger(parsedSSHPort) || parsedSSHPort < 1024 || parsedSSHPort > 65535) ? "invalid" : ""}`}><span>SSH</span><input type="text" inputMode="numeric" value={sshPort} onChange={(event) => setSshPort(event.target.value.replace(/\D/g, "").slice(0, 5))} maxLength={5} aria-invalid={!validPorts} /><small>{translate("SSH 用户名映射")}</small></label></div>
      {!validPorts && <div className="inline-error"><TriangleAlert size={16} /><span>{translate("请输入两个不同的 1024-65535 端口。")}</span></div>}
      <div className="port-save-row"><span>{changed ? translate("保存会重启本地核心；已有路由和受保护凭据不会改变。") : translate("当前端口设置已生效。")}</span><button className="primary-button" onClick={() => void save()} disabled={!changed || !validPorts || saving || terminating}>{saving ? <LoaderCircle className="spin" size={15} /> : <Check size={15} />}{saving ? translate("正在应用") : translate("保存端口")}</button></div>
    </section>
    <section className="port-manager-section port-owner-section"><div className="port-manager-heading"><div><strong>{translate("占用当前端口的进程")}</strong><span>{translate("只显示当前用户的监听进程；Airlock 自己的本地核心不会出现在列表中。")}</span></div></div>
      {loading ? <div className="port-owner-empty"><LoaderCircle className="spin" size={16} />{translate("正在检查端口占用")}</div> : owners.length === 0 ? <div className="port-owner-empty"><CircleCheck size={16} />{translate("没有发现可结束的占用进程")}</div> : <div className="port-owner-list">{owners.map((owner) => <div className="port-owner" key={`${owner.port}-${owner.pid}`}><span className="port-owner-port">:{owner.port}</span><div><strong>{owner.command}</strong><small>PID {owner.pid}</small></div><button className="secondary-button compact danger-outline" onClick={() => setPendingOwner(owner)} disabled={saving || terminating}><CircleStop size={14} />{translate("结束")}</button></div>)}</div>}
      {pendingOwner && <div className="port-termination-confirm"><TriangleAlert size={17} /><div><strong>{translate("确认结束该进程？")}</strong><p>{translate("将向")} <code>{pendingOwner.command} · PID {pendingOwner.pid}</code> {translate("发送正常结束请求。Airlock 不会使用强制终止。")}</p></div><div className="inline-actions"><button className="secondary-button compact" onClick={() => setPendingOwner(undefined)} disabled={terminating}>{translate("取消")}</button><button className="danger-button compact" onClick={() => void terminate()} disabled={terminating}>{terminating ? <LoaderCircle className="spin" size={14} /> : <CircleStop size={14} />}{terminating ? translate("正在结束") : translate("确认结束")}</button></div></div>}
    </section>
    {error && <div className="inline-error port-manager-error"><TriangleAlert size={16} /><span>{error}</span></div>}
  </Modal>;
}

function SecurityProfile({ title, subtitle, icon: Icon, description, store, ingress, selected, recommended, risk, onSelect }: { title: string; subtitle: string; icon: typeof Monitor; description: string; store: string; ingress: string; selected: boolean; recommended: boolean; risk: boolean; onSelect: () => void }) {
  return <button type="button" className={`security-profile ${selected ? "selected" : ""} ${risk ? "risk" : ""}`} role="radio" aria-checked={selected} onClick={onSelect}><span className="security-profile-icon"><Icon size={18} /></span><span className="security-profile-title"><strong>{title}</strong>{recommended && <b>推荐</b>}</span><small>{subtitle}</small><p>{description}</p><span className="security-profile-facts"><span><b>存储</b>{store}</span><span><b>入口</b>{ingress}</span></span>{selected && <Check className="security-profile-check" size={15} />}</button>;
}

function SecurityChoice<T extends NetworkScope | SecretStoreMode>({ label, detail, value, options, onChange }: { label: string; detail: string; value: T; options: Array<{ value: T; label: string; icon: typeof Monitor }>; onChange: (value: T) => void }) {
  return <div className="security-choice"><div><strong>{label}</strong><small>{detail}</small></div><div className="choice-control" role="group" aria-label={label}>{options.map((option) => { const Icon = option.icon; return <button key={option.value} className={value === option.value ? "selected" : ""} aria-pressed={value === option.value} onClick={() => onChange(option.value)}><Icon size={14} />{option.label}{value === option.value && <Check size={12} />}</button>; })}</div></div>;
}

function PreferenceRow({ label, detail, children }: { label: string; detail: string; children: React.ReactNode }) { return <div className="preference-row"><div><strong>{label}</strong><small>{detail}</small></div>{children}</div>; }
function PreferenceSegment<T extends string | number>({ value, options, onChange }: { value: T; options: Array<{ value: T; label: string }>; onChange: (value: T) => void }) { return <div className="preference-segment" role="group">{options.map((option) => <button key={String(option.value)} className={value === option.value ? "selected" : ""} aria-pressed={value === option.value} onClick={() => onChange(option.value)}>{option.label}</button>)}</div>; }

function SSHPolicyEditor({ route, platform, testing, onClose, onSave, onUpdateHost, onRotateCredential, onTest, onAddHost, onDelete }: { route: RouteSummary; platform: PlatformInfo; testing: boolean; onClose: () => void; onSave: (name: string, localUsername: string, allowedCommand: string, allowAllCommands: boolean, recordCommands: boolean, allowSftp: boolean, authenticationTimeoutSeconds: number, egress: RouteSummary["egress"], keywordReplacements: KeywordReplacement[]) => Promise<void>; onUpdateHost: (input: SSHHostUpdateInput) => Promise<void>; onRotateCredential: (localPassword: string) => Promise<SSHRouteCreationResult>; onTest: () => Promise<boolean>; onAddHost: () => void; onDelete: () => void }) {
  const [name, setName] = useState(route.name);
  const [localUsername, setLocalUsername] = useState(route.localUsername || route.alias);
  const [egress, setEgress] = useState(route.egress);
  const [allowAll, setAllowAll] = useState(route.allowAllCommands);
  const [allowedCommand, setAllowedCommand] = useState(route.allowedCommand || "printf airlock-ok");
  const [recordCommands, setRecordCommands] = useState(route.recordCommands);
  const [allowSftp, setAllowSftp] = useState(route.allowSftp);
  const [authenticationTimeoutSeconds, setAuthenticationTimeoutSeconds] = useState(route.authenticationTimeoutSeconds ?? 20);
  const [keywordReplacements, setKeywordReplacements] = useState<KeywordReplacement[]>([]);
  const [replacementsLoading, setReplacementsLoading] = useState(isTauri);
  const [busy, setBusy] = useState<"save" | "probe" | "host" | "rotate">();
  const [error, setError] = useState<string>();
  const [hostEditorOpen, setHostEditorOpen] = useState(false);
  const [hostAddress, setHostAddress] = useState("");
  const [hostPort, setHostPort] = useState("22");
  const [hostUsername, setHostUsername] = useState("");
  const [hostPassword, setHostPassword] = useState("");
  const [hostPasswordVisible, setHostPasswordVisible] = useState(false);
  const [hostProbe, setHostProbe] = useState<SSHHostKeyProbe>();
  const [hostKeyAccepted, setHostKeyAccepted] = useState(false);
  const [credentialMode, setCredentialMode] = useState<"generated" | "custom">("generated");
  const [localPassword, setLocalPassword] = useState("");
  const [localPasswordConfirmation, setLocalPasswordConfirmation] = useState("");
  const [localPasswordVisible, setLocalPasswordVisible] = useState(false);
  const [rotatedCredential, setRotatedCredential] = useState("");
  const localPasswordBytes = new TextEncoder().encode(localPassword).length;
  const nameValid = name.trim().length > 0 && name.length <= 80;
  const usernameValid = /^[a-z0-9][a-z0-9._-]{0,63}$/.test(localUsername);
  const commandValid = allowAll || (allowedCommand.trim().length > 0 && allowedCommand.length <= 4096 && !/[\r\n\0]/.test(allowedCommand));
  const hostEndpoint = composeSSHAddress(hostAddress, hostPort);
  const hostValid = validSSHHost(hostAddress) && validSSHPort(hostPort) && hostUsername.length > 0 && hostPassword.length > 0;
  const localPasswordValid = credentialMode === "generated" || (localPasswordBytes >= 12 && localPasswordBytes <= 1024 && localPassword === localPasswordConfirmation && !/[\r\n\0]/.test(localPassword));
  const authenticationTimeoutValid = Number.isInteger(authenticationTimeoutSeconds) && authenticationTimeoutSeconds >= 3 && authenticationTimeoutSeconds <= 120;
  const replacementsValid = keywordReplacements.length <= 64 && keywordReplacements.every((replacement) => replacement.from.length > 0 && replacement.from.length <= 256 && replacement.to.length <= 1024 && !/[\r\n\0]/.test(`${replacement.from}${replacement.to}`));
  useEffect(() => {
    if (!isTauri) return;
    let active = true;
    void invoke<KeywordReplacement[]>("get_ssh_keyword_replacements", { alias: route.alias })
      .then((rules) => { if (active) setKeywordReplacements(rules); })
      .catch((caught) => { if (active) setError(String(caught)); })
      .finally(() => { if (active) setReplacementsLoading(false); });
    return () => { active = false; };
  }, [route.alias]);
  const save = async () => {
    setBusy("save");
    setError(undefined);
    try { await onSave(name.trim(), localUsername, allowedCommand, allowAll, recordCommands, allowSftp, authenticationTimeoutSeconds, egress, keywordReplacements); }
    catch (caught) { setError(translate(String(caught))); }
    finally { setBusy(undefined); }
  };
  const probeHost = async () => {
    setBusy("probe");
    setError(undefined);
    try {
      const probe = isTauri ? await invoke<SSHHostKeyProbe>("probe_ssh_host_key", { address: hostEndpoint, egress }) : { hostKey: "preview-host-key", fingerprint: "SHA256:AirlockPreviewFingerprint" };
      setHostProbe(probe);
      setHostKeyAccepted(false);
    } catch (caught) { setError(translate(String(caught))); }
    finally { setBusy(undefined); }
  };
  const replaceHost = async () => {
    if (!hostProbe) return;
    setBusy("host");
    setError(undefined);
    try {
      await onUpdateHost({ address: hostEndpoint, username: hostUsername, password: hostPassword, expectedHostKey: hostProbe.hostKey, egress });
      setHostAddress(""); setHostPort("22"); setHostUsername(""); setHostPassword(""); setHostProbe(undefined); setHostKeyAccepted(false); setHostEditorOpen(false);
    } catch (caught) { setError(translate(String(caught))); }
    finally { setBusy(undefined); }
  };
  const rotateCredential = async () => {
    setBusy("rotate");
    setError(undefined);
    setRotatedCredential("");
    try {
      const result = await onRotateCredential(credentialMode === "custom" ? localPassword : "");
      setRotatedCredential(result.localCredential);
      setLocalPassword(""); setLocalPasswordConfirmation("");
    } catch (caught) { setError(translate(String(caught))); }
    finally { setBusy(undefined); }
  };
  const updateHostAddress = (value: string) => { setHostAddress(value); setHostProbe(undefined); setHostKeyAccepted(false); };
  const updateHostPort = (value: string) => { setHostPort(value.replace(/\D/g, "").slice(0, 5)); setHostProbe(undefined); setHostKeyAccepted(false); };
  return <Modal title={translate(`SSH 映射管理 · ${route.name}`)} className="modal-wide ssh-manager-modal" onClose={busy ? () => undefined : onClose}>
    <div className="ssh-manager">
      <div className="policy-identity"><span className="protected-icon"><SquareTerminal size={19} /></span><div><strong>{route.localEndpoint}</strong><code>{route.alias}</code></div><span className={`status-badge status-${route.status}`}>{route.status === "enabled" ? "已启用" : "已停用"}</span></div>
      <div className="ssh-manager-actions"><button className="secondary-button compact" onClick={() => void onTest()} disabled={testing || Boolean(busy)}>{testing ? <LoaderCircle className="spin" size={14} /> : <HeartPulse size={14} />}{testing ? "检测中" : "立即检查连接"}</button><button className="secondary-button compact" onClick={onAddHost} disabled={Boolean(busy)}><Plus size={14} />新增宿主映射</button><button className="text-button danger-text" onClick={onDelete} disabled={Boolean(busy)}><Trash2 size={14} />删除宿主映射</button></div>
      <section className="ssh-manager-section"><div className="ssh-manager-heading"><div><strong>映射身份与出口</strong><span>一个本地用户名对应一个受保护 SSH 宿主关系</span></div></div><div className="identity-grid"><label className="form-field"><span>映射名称</span><input value={name} onChange={(event) => setName(event.target.value)} maxLength={80} /></label><label className="form-field"><span>本地 SSH 用户名</span><input value={localUsername} onChange={(event) => setLocalUsername(event.target.value.toLowerCase().replace(/[^a-z0-9._-]/g, ""))} maxLength={64} spellCheck={false} placeholder="builder" /><small>修改后旧用户名立即失效。</small></label></div><div className="identity-grid ssh-budget-grid"><label className={`form-field ${authenticationTimeoutValid ? "" : "invalid"}`}><span>认证预算（秒）</span><input type="number" min={3} max={120} step={1} value={authenticationTimeoutSeconds} onChange={(event) => setAuthenticationTimeoutSeconds(Number(event.target.value))} /><small>默认 20 秒，仅用于手动上游认证检查。</small></label></div><div className="egress-field"><span>出口策略</span><div className="egress-control" role="group">{(["Direct", "Proxy", "Auto"] as const).map((value) => <button key={value} className={egress === value ? "selected" : ""} onClick={() => { setEgress(value); setHostProbe(undefined); setHostKeyAccepted(false); }}>{value}</button>)}</div></div></section>
      <section className="ssh-manager-section"><div className="ssh-manager-heading"><div><strong>命令权限与记录</strong><span>停用路由的连接尝试始终进入脱敏活动；命令正文记录由下方开关控制</span></div></div>
      <div className="policy-options" role="radiogroup" aria-label="SSH 命令范围">
        <button className={!allowAll ? "selected" : ""} role="radio" aria-checked={!allowAll} onClick={() => setAllowAll(false)}><ShieldCheck size={17} /><span><strong>指定命令</strong><small>只开放一个完整 exec 命令</small></span></button>
        <button className={allowAll ? "selected risk" : "risk"} role="radio" aria-checked={allowAll} onClick={() => setAllowAll(true)}><AlertTriangle size={17} /><span><strong>所有命令</strong><small>任意 exec，仍拒绝 Shell 与 PTY</small></span></button>
      </div>
      {!allowAll && <label className="form-field ssh-command-field"><span>唯一允许命令 <small>{allowedCommand.length}/4096</small></span><input value={allowedCommand} onChange={(event) => setAllowedCommand(event.target.value)} maxLength={4096} spellCheck={false} placeholder="例如：uptime" /><small>按完整字符串匹配，不要在命令参数中填写密码或 Token。</small></label>}
      {allowAll && <div className="inline-warning policy-warning"><TriangleAlert size={16} /><span>{translate(platformNativeConfirmText(platform))}</span></div>}
      <label className="audit-toggle"><input type="checkbox" checked={recordCommands} onChange={(event) => setRecordCommands(event.target.checked)} /><span><strong>{translate("记录执行命令")}</strong><small>{translate("完整命令保存在本机受保护审计文件，参数可能包含敏感内容。")}</small></span></label>
      <label className={`audit-toggle sftp-toggle ${allowSftp ? "enabled" : ""}`}><input type="checkbox" checked={allowSftp} onChange={(event) => setAllowSftp(event.target.checked)} /><span><strong>{translate("允许 SFTP 文件传输（高风险）")}</strong><small>{translate("允许列出、读取、写入、重命名和删除上游账号可访问的文件。请使用专用低权限账号；Shell、PTY、端口转发及其他子系统仍拒绝。")}</small></span></label>
      </section>
      <section className="ssh-manager-section keyword-replacement-section"><div className="ssh-manager-heading"><div><strong>出口关键词替换</strong><span>按顺序替换命令文本；原始命令仍用于权限匹配和本地审计</span></div><button className="secondary-button compact" onClick={() => setKeywordReplacements((rules) => [...rules, { from: "", to: "", enabled: true }])} disabled={Boolean(busy) || replacementsLoading || keywordReplacements.length >= 64}><Plus size={14} />添加规则</button></div>
        {replacementsLoading ? <div className="keyword-replacement-loading"><LoaderCircle className="spin" size={15} />正在读取受保护规则</div> : keywordReplacements.length === 0 ? <div className="keyword-replacement-empty">没有出口替换。调用方的命令将按原样发送给上游。</div> : <div className="keyword-replacement-list">{keywordReplacements.map((replacement, index) => <div className="keyword-replacement-row" key={`${index}-${replacement.from}`}><label className="form-field"><span>匹配关键词</span><input value={replacement.from} onChange={(event) => setKeywordReplacements((rules) => rules.map((rule, ruleIndex) => ruleIndex === index ? { ...rule, from: event.target.value } : rule))} maxLength={256} autoComplete="off" spellCheck={false} placeholder="例如 input.user.passwd" /></label><label className="form-field"><span>替换为</span><input value={replacement.to} onChange={(event) => setKeywordReplacements((rules) => rules.map((rule, ruleIndex) => ruleIndex === index ? { ...rule, to: event.target.value } : rule))} maxLength={1024} autoComplete="off" spellCheck={false} placeholder="仅发送给上游" /></label><label className="keyword-replacement-toggle"><input type="checkbox" checked={replacement.enabled} onChange={(event) => setKeywordReplacements((rules) => rules.map((rule, ruleIndex) => ruleIndex === index ? { ...rule, enabled: event.target.checked } : rule))} /><span>{replacement.enabled ? "启用" : "停用"}</span></label><button className="row-icon-button danger visible" type="button" title="删除替换规则" aria-label="删除替换规则" onClick={() => setKeywordReplacements((rules) => rules.filter((_, ruleIndex) => ruleIndex !== index))} disabled={Boolean(busy)}><Trash2 size={14} /></button></div>)}</div>}
        <div className="keyword-replacement-note"><ShieldCheck size={15} /><span>替换后的内容只会进入上游 SSH 会话，不会写入活动记录或普通路由列表。</span></div>
      </section>
      <section className="ssh-manager-section"><div className="ssh-manager-heading"><div><strong>受保护宿主机</strong><span>真实地址、上游账号和密码默认不回显；替换时重新输入</span></div><button className="secondary-button compact" onClick={() => { setHostEditorOpen((current) => !current); setError(undefined); }} disabled={Boolean(busy)}>{hostEditorOpen ? <ChevronRight size={14} /> : <Settings2 size={14} />}{hostEditorOpen ? "收起" : "替换宿主机"}</button></div>{hostEditorOpen && <div className="ssh-host-editor"><div className="ssh-upstream-grid"><label className="form-field"><span>新的 SSH 主机</span><input value={hostAddress} onChange={(event) => updateHostAddress(event.target.value)} placeholder="192.168.1.20" maxLength={505} /></label><label className="form-field ssh-port-field"><span>端口</span><input type="text" inputMode="numeric" value={hostPort} onChange={(event) => updateHostPort(event.target.value)} maxLength={5} placeholder="22" /></label><label className="form-field"><span>新的上游用户名</span><input value={hostUsername} onChange={(event) => setHostUsername(event.target.value)} maxLength={255} /></label></div><SecretField label="新的上游密码" value={hostPassword} visible={hostPasswordVisible} onVisible={() => setHostPasswordVisible((current) => !current)} onChange={setHostPassword} placeholder="输入新的上游密码" detail="只保存到当前受保护凭据存储" />{hostProbe ? <div className="host-key-check ready"><div className="host-key-copy"><strong>新的 SSH Host Key</strong><span>通过可信渠道核对</span></div><code data-i18n="off">{hostProbe.fingerprint}</code><label className="compact-check host-key-accept"><input type="checkbox" checked={hostKeyAccepted} onChange={(event) => setHostKeyAccepted(event.target.checked)} /><span>我已核对并信任此 Host Key</span></label></div> : <button className="secondary-button host-probe-button" onClick={() => void probeHost()} disabled={!hostValid || Boolean(busy)}>{busy === "probe" ? <LoaderCircle className="spin" size={14} /> : <HeartPulse size={14} />}{busy === "probe" ? "检测中" : "检测新宿主 Host Key"}</button>}<div className="secure-entry-actions"><span>替换成功后，原宿主凭据会被覆盖</span><button className="primary-button" onClick={() => void replaceHost()} disabled={!hostValid || !hostProbe || !hostKeyAccepted || Boolean(busy)}>{busy === "host" ? <LoaderCircle className="spin" size={14} /> : <ShieldCheck size={14} />}{busy === "host" ? "正在替换" : "确认替换宿主机"}</button></div></div>}</section>
      <section className="ssh-manager-section"><div className="ssh-manager-heading"><div><strong>本地登录凭据</strong><span>轮换后旧密码或随机凭据立即失效</span></div><div className="policy-segmented"><button className={credentialMode === "generated" ? "selected" : ""} onClick={() => { setCredentialMode("generated"); setLocalPassword(""); setLocalPasswordConfirmation(""); }}><KeyRound size={13} />随机生成</button><button className={credentialMode === "custom" ? "selected" : ""} onClick={() => setCredentialMode("custom")}><ShieldCheck size={13} />自定义密码</button></div></div>{credentialMode === "custom" && <div className="local-password-grid"><SecretField label="新的本地密码" value={localPassword} visible={localPasswordVisible} onVisible={() => setLocalPasswordVisible((current) => !current)} onChange={setLocalPassword} placeholder="至少 12 个字节" detail={`${localPasswordBytes}/1024 bytes`} /><SecretField label="确认新密码" value={localPasswordConfirmation} visible={localPasswordVisible} onVisible={() => setLocalPasswordVisible((current) => !current)} onChange={setLocalPasswordConfirmation} placeholder="再次输入" detail={localPasswordConfirmation && localPassword !== localPasswordConfirmation ? "两次输入不一致" : "只保存摘要"} invalid={Boolean(localPasswordConfirmation && localPassword !== localPasswordConfirmation)} /></div>}<button className="secondary-button rotate-ssh-credential" onClick={() => void rotateCredential()} disabled={!localPasswordValid || Boolean(busy)}>{busy === "rotate" ? <LoaderCircle className="spin" size={14} /> : <RotateCcw size={14} />}{busy === "rotate" ? "正在轮换" : "轮换本地凭据"}</button>{rotatedCredential && <div className="one-time-credential"><span>新的本地凭据，仅显示一次</span><code data-i18n="off">{rotatedCredential}</code></div>}</section>
      {error && <div className="inline-error"><TriangleAlert size={15} />{error}</div>}
    </div>
    <div className="modal-actions"><button className="secondary-button" onClick={onClose} disabled={Boolean(busy)}>关闭</button><button className={allowAll ? "danger-button" : "primary-button"} onClick={() => void save()} disabled={Boolean(busy) || replacementsLoading || !nameValid || !usernameValid || !commandValid || !authenticationTimeoutValid || !replacementsValid}>{busy === "save" ? <RefreshCw className="spin" size={16} /> : <ShieldCheck size={16} />}{busy === "save" ? "正在保存" : "保存映射设置"}</button></div>
  </Modal>;
}

function LLMPolicyEditor({ route, onClose, onSave, onRotate, onResetUsage }: { route: RouteSummary; onClose: () => void; onSave: (models: string[], maxOutputTokens: number, requestsPerMinute: number, maxConcurrent: number, trackUsage: boolean) => Promise<void>; onRotate: () => Promise<void>; onResetUsage: () => Promise<void> }) {
  const [modelText, setModelText] = useState((route.allowedModels ?? []).join(", "));
  const [maxOutputTokens, setMaxOutputTokens] = useState(route.maxOutputTokens ?? 8192);
  const [requestsPerMinute, setRequestsPerMinute] = useState(route.requestsPerMinute ?? 60);
  const [maxConcurrent, setMaxConcurrent] = useState(route.maxConcurrent ?? 4);
  const [trackUsage, setTrackUsage] = useState(route.trackUsage ?? false);
  const [busy, setBusy] = useState<"save" | "rotate" | "reset">();
  const models = useMemo(() => [...new Set(modelText.split(",").map((model) => model.trim()).filter(Boolean))], [modelText]);
  const valid = models.length > 0 && models.length <= 32 && models.every((model) => model.length <= 200) && maxOutputTokens >= 1 && maxOutputTokens <= 1_000_000 && requestsPerMinute >= 1 && requestsPerMinute <= 60_000 && maxConcurrent >= 1 && maxConcurrent <= 1_024;
  const save = async () => { setBusy("save"); try { await onSave(models, maxOutputTokens, requestsPerMinute, maxConcurrent, trackUsage); } finally { setBusy(undefined); } };
  const rotate = async () => { setBusy("rotate"); try { await onRotate(); } finally { setBusy(undefined); } };
  const reset = async () => { setBusy("reset"); try { await onResetUsage(); } finally { setBusy(undefined); } };

  return <Modal title={translate(`LLM 访问边界 · ${route.name}`)} className="modal-wide" onClose={busy ? () => undefined : onClose}>
    <div className="llm-policy-editor">
      <div className="policy-identity"><span className="protected-icon"><Sparkles size={19} /></span><div><strong>{route.provider === "anthropic" ? "Anthropic-compatible" : "OpenAI-compatible"}</strong><code>{route.localEndpoint}</code></div><span className="policy-lock"><ShieldCheck size={14} />固定上游</span></div>
      <label className="form-field model-allowlist"><span>允许模型 <small>{models.length}/32</small></span><textarea value={modelText} onChange={(event) => setModelText(event.target.value)} maxLength={6400} rows={3} placeholder="model-a, model-b" /></label>
      <div className="llm-limit-editor">
        <label className="form-field"><span>最大输出 Token</span><input type="number" min={1} max={1_000_000} step={256} value={maxOutputTokens} onChange={(event) => setMaxOutputTokens(Number(event.target.value))} /></label>
        <label className="form-field"><span>每分钟请求</span><input type="number" min={1} max={60_000} value={requestsPerMinute} onChange={(event) => setRequestsPerMinute(Number(event.target.value))} /></label>
        <label className="form-field"><span>并发请求</span><input type="number" min={1} max={1_024} value={maxConcurrent} onChange={(event) => setMaxConcurrent(Number(event.target.value))} /></label>
      </div>
      <div className={`usage-control ${trackUsage ? "enabled" : ""}`}>
        <label className="audit-toggle"><input type="checkbox" checked={trackUsage} onChange={(event) => setTrackUsage(event.target.checked)} /><span><strong>统计调用与 Token</strong><small>只读取上游 usage 数字，不记录提示词或响应正文；统计随 airlockd 重启归零。</small></span></label>
        <div className="usage-metrics" aria-label="LLM 使用量统计"><UsageMetric label="调用" value={route.totalRequests ?? 0} /><UsageMetric label="输入 Token" value={route.inputTokens ?? 0} /><UsageMetric label="输出 Token" value={route.outputTokens ?? 0} /></div>
        <button className="text-button usage-reset" onClick={() => void reset()} disabled={Boolean(busy) || !((route.totalRequests ?? 0) + (route.inputTokens ?? 0) + (route.outputTokens ?? 0))}>{busy === "reset" ? <RefreshCw className="spin" size={13} /> : <RotateCcw size={13} />}清零统计</button>
      </div>
      <div className="capability-rotation"><div><span className="rotation-icon"><KeyRound size={16} /></span><div><strong>二次 API Key</strong><p>轮换后旧 Key 立即失效；上游 API Key 不会改变。</p></div></div><button className="secondary-button" onClick={() => void rotate()} disabled={Boolean(busy)}>{busy === "rotate" ? <RefreshCw className="spin" size={15} /> : <RotateCcw size={15} />}{busy === "rotate" ? "等待原生窗口" : "轮换 Key"}</button></div>
    </div>
    <div className="modal-actions"><button className="secondary-button" onClick={onClose} disabled={Boolean(busy)}>取消</button><button className="primary-button" onClick={() => void save()} disabled={!valid || Boolean(busy)}>{busy === "save" ? <RefreshCw className="spin" size={16} /> : <ShieldCheck size={16} />}{busy === "save" ? "正在保存" : "保存访问边界"}</button></div>
  </Modal>;
}

function UsageMetric({ label, value }: { label: string; value: number }) { return <div><span>{label}</span><strong>{formatUsage(value)}</strong></div>; }

function RouteEditor({ initialKind, routes, connected, proxyConfigured, sshReady, onClose, onCreated, onError }: { initialKind: RouteKind; routes: RouteSummary[]; connected: boolean; proxyConfigured: boolean; sshReady: boolean; onClose: () => void; onCreated: (route: RouteSummary) => void; onError: (message: string) => void }) {
  const [step, setStep] = useState(1);
  const [kind, setKind] = useState<RouteKind>(initialKind);
  const [name, setName] = useState("");
  const [alias, setAlias] = useState("");
  const [localUsername, setLocalUsername] = useState("");
  const [sshHost, setSSHHost] = useState("");
  const [sshPort, setSSHPort] = useState("22");
  const [sshUsername, setSSHUsername] = useState("");
  const [sshPassword, setSSHPassword] = useState("");
  const [localCredentialMode, setLocalCredentialMode] = useState<"generated" | "custom">("generated");
  const [localPassword, setLocalPassword] = useState("");
  const [localPasswordConfirmation, setLocalPasswordConfirmation] = useState("");
  const [showSSHPassword, setShowSSHPassword] = useState(false);
  const [showLocalPassword, setShowLocalPassword] = useState(false);
  const [sshProbe, setSSHProbe] = useState<SSHHostKeyProbe>();
  const [hostKeyAccepted, setHostKeyAccepted] = useState(false);
  const [allowAllConfirmed, setAllowAllConfirmed] = useState(false);
  const [sshError, setSSHError] = useState<string>();
  const [sshCreation, setSSHCreation] = useState<SSHRouteCreationResult>();
  const [sshBusy, setSSHBusy] = useState<"probe" | "create">();
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState<RouteSummary>();
  const [egress, setEgress] = useState<RouteSummary["egress"]>("Direct");
  const [allowAllCommands, setAllowAllCommands] = useState(false);
  const [allowedCommand, setAllowedCommand] = useState("printf airlock-ok");
  const [recordCommands, setRecordCommands] = useState(true);
  const [allowSftp, setAllowSftp] = useState(false);
  const [sshAuthenticationTimeoutSeconds, setSSHAuthenticationTimeoutSeconds] = useState(20);
  const [llmProvider, setLLMProvider] = useState<"openai" | "anthropic">("openai");
  const [llmModels, setLLMModels] = useState("");
  const [maxOutputTokens, setMaxOutputTokens] = useState(8192);
  const [requestsPerMinute, setRequestsPerMinute] = useState(60);
  const [maxConcurrent, setMaxConcurrent] = useState(4);
  const [trackUsage, setTrackUsage] = useState(false);
  const models = useMemo(() => [...new Set(llmModels.split(",").map((model) => model.trim()).filter(Boolean))], [llmModels]);
  const validSSHCommand = allowAllCommands || (allowedCommand.trim().length > 0 && allowedCommand.length <= 4096 && !/[\r\n\0]/.test(allowedCommand));
  const validLocalUsername = /^[a-z0-9][a-z0-9._-]{0,63}$/.test(localUsername);
  const aliasInUse = routes.some((route) => route.alias === alias);
  const localUsernameInUse = kind === "SSH" && routes.some((route) => route.kind === "SSH" && (route.localUsername || route.alias) === localUsername);
  const validIdentity = name.trim().length > 0 && /^[a-z0-9][a-z0-9-]{0,62}$/.test(alias) && !aliasInUse && (kind !== "SSH" || (validSSHCommand && validLocalUsername && !localUsernameInUse)) && (kind !== "LLM" || (models.length > 0 && models.length <= 32 && models.every((model) => model.length <= 200) && maxOutputTokens >= 1 && maxOutputTokens <= 1_000_000 && requestsPerMinute >= 1 && requestsPerMinute <= 60_000 && maxConcurrent >= 1 && maxConcurrent <= 1_024));
  const localPasswordBytes = new TextEncoder().encode(localPassword).length;
  const sshEndpoint = composeSSHAddress(sshHost, sshPort);
  const validSSHAddress = validSSHHost(sshHost) && validSSHPort(sshPort);
  const validSSHUsername = sshUsername.length > 0 && sshUsername.length <= 255 && !/[\r\n\0]/.test(sshUsername);
  const validSSHPassword = sshPassword.length > 0 && new TextEncoder().encode(sshPassword).length <= 8192 && !sshPassword.includes("\0");
  const validCustomLocalPassword = localCredentialMode === "generated" || (localPasswordBytes >= 12 && localPasswordBytes <= 1024 && !/[\r\n\0]/.test(localPassword) && localPassword === localPasswordConfirmation);
  const canProbeSSH = connected && validSSHAddress && !saving;
  const canCreateSSH = connected && validSSHAddress && validSSHUsername && validSSHPassword && validCustomLocalPassword && sshAuthenticationTimeoutSeconds >= 3 && sshAuthenticationTimeoutSeconds <= 120 && Boolean(sshProbe) && hostKeyAccepted && (!allowAllCommands || allowAllConfirmed) && !saving;

  const updateAlias = (value: string) => {
    const next = value.toLowerCase().replace(/[^a-z0-9-]/g, "");
    setAlias(next);
    setLocalUsername((current) => current === "" || current === alias ? next : current);
  };

  const clearSSHSecrets = () => {
    setSSHPassword("");
    setLocalPassword("");
    setLocalPasswordConfirmation("");
  };

  const selectKind = (next: RouteKind) => {
    if (kind === "SSH" && next !== "SSH") {
      clearSSHSecrets();
      setSSHProbe(undefined);
      setHostKeyAccepted(false);
      setSSHError(undefined);
    }
    setKind(next);
    if (next === "SSH" && !localUsername) setLocalUsername(alias);
  };

  const updateSSHHost = (value: string) => {
    setSSHHost(value);
    setSSHProbe(undefined);
    setHostKeyAccepted(false);
    setSSHError(undefined);
  };

  const updateSSHPort = (value: string) => {
    setSSHPort(value.replace(/\D/g, "").slice(0, 5));
    setSSHProbe(undefined);
    setHostKeyAccepted(false);
    setSSHError(undefined);
  };

  const updateEgress = (value: RouteSummary["egress"]) => {
    setEgress(value);
    if (kind === "SSH") {
      setSSHProbe(undefined);
      setHostKeyAccepted(false);
      setSSHError(undefined);
    }
  };

  const probeSSHHostKey = async () => {
    setSaving(true);
    setSSHBusy("probe");
    setSSHError(undefined);
    try {
      const probe = isTauri
        ? await invoke<SSHHostKeyProbe>("probe_ssh_host_key", { address: sshEndpoint, egress })
        : { hostKey: "preview-host-key", fingerprint: "SHA256:AirlockPreviewFingerprint" };
      setSSHProbe(probe);
      setHostKeyAccepted(false);
    } catch (error) {
      const message = translate(String(error));
      setSSHError(message);
      onError(message);
    } finally {
      setSaving(false);
      setSSHBusy(undefined);
    }
  };

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape" && !saving) onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose, saving]);

  const secureCreate = async () => {
    setSaving(true);
    if (kind === "SSH") setSSHBusy("create");
    setSSHError(undefined);
    try {
      let route: RouteSummary;
      if (isTauri) {
        if (kind === "SSH") {
          if (!sshProbe) throw new Error("请先检测并确认上游 SSH Host Key");
          const result = await invoke<SSHRouteCreationResult>("create_ssh_route", { input: { name: name.trim(), alias, localUsername, address: sshEndpoint, username: sshUsername, password: sshPassword, localPassword: localCredentialMode === "custom" ? localPassword : "", expectedHostKey: sshProbe.hostKey, egress, allowedCommand, allowAllCommands, allowAllConfirmed, recordCommands, allowSftp, authenticationTimeoutSeconds: sshAuthenticationTimeoutSeconds } });
          route = result.route;
          setSSHCreation(result);
        }
        else if (kind === "LLM") route = await invoke<RouteSummary>("create_llm_route", { name: name.trim(), alias, egress, provider: llmProvider, models, maxOutputTokens, requestsPerMinute, maxConcurrent, trackUsage });
        else route = await invoke<RouteSummary>("create_http_route", { name: name.trim(), alias, egress });
      } else {
        route = { id: alias, name: name.trim(), alias, localUsername: kind === "SSH" ? localUsername : undefined, kind, status: kind === "SSH" ? "disabled" : "enabled", localEndpoint: kind === "SSH" ? `${localUsername}@127.0.0.1:4770` : `${kind === "LLM" ? "http://" : ""}127.0.0.1:4768/r/${alias}`, permissionSummary: kind === "SSH" ? `${allowAllCommands ? "all exec commands · high risk" : "1 exact command · stdin denied"}${recordCommands ? " · recorded" : ""}${allowSftp ? " · SFTP high risk" : ""}` : kind === "LLM" ? `${llmProvider === "openai" ? "OpenAI" : "Anthropic"} · ${models.length} models · output ≤ ${maxOutputTokens} · ${requestsPerMinute}/min · ${maxConcurrent} concurrent` : "GET, HEAD · Range", egress, health: kind === "SSH" ? "unknown" : "healthy", lastUsed: "从未", currentConnections: 0, allowAllCommands: kind === "SSH" && allowAllCommands, recordCommands: kind === "SSH" && recordCommands, allowSftp: kind === "SSH" && allowSftp, allowedCommand: kind === "SSH" && !allowAllCommands ? allowedCommand : undefined, ...(kind === "SSH" ? { authenticationTimeoutSeconds: sshAuthenticationTimeoutSeconds } : {}), ...(kind === "LLM" ? { provider: llmProvider, allowedModels: models, maxOutputTokens, requestsPerMinute, maxConcurrent, trackUsage, totalRequests: 0, inputTokens: 0, outputTokens: 0 } : {}) };
        if (kind === "SSH") setSSHCreation({ route, localCredential: localCredentialMode === "generated" ? "airlock_preview_credential" : "", generatedCredential: localCredentialMode === "generated" });
      }
      if (kind === "SSH") clearSSHSecrets();
      setCreated(route);
      onCreated(route);
      setStep(3);
    } catch (error) {
      const message = translate(String(error));
      if (kind === "SSH") setSSHError(message);
      onError(message);
    } finally {
      setSaving(false);
      setSSHBusy(undefined);
    }
  };

  return <div className="editor-overlay" role="dialog" aria-modal="true" aria-label="新增路由"><div className="editor-panel">
    <header className="editor-header"><div><span className="editor-kicker">SECURE ROUTE</span><h2>{translate(`新增 ${kind} 路由`)}</h2><p>{kind === "SSH" ? "目标与认证在 Airlock 内完成，仅发送到本机核心" : "敏感信息仅发送到本机核心并受隔离存储保护"}</p></div><button className="icon-button" onClick={onClose} disabled={saving} aria-label="关闭"><X size={18} /></button></header>
    <ol className="step-list">{["本地身份", "安全录入", "完成"].map((label, index) => <li key={label} className={step === index + 1 ? "current" : step > index + 1 ? "done" : ""}><span>{step > index + 1 ? <Check size={14} /> : index + 1}</span>{label}</li>)}</ol>
    <div className="editor-body" key={step}>
      {step === 1 && <>
        <h3>选择入口类型</h3>
        <div className="type-grid"><button className={kind === "HTTP" ? "selected" : ""} onClick={() => selectKind("HTTP")}><Route size={19} /><strong>HTTP</strong><span>固定 URL · GET / HEAD</span></button><button className={kind === "SSH" ? "selected" : ""} onClick={() => selectKind("SSH")} disabled={!sshReady}><SquareTerminal size={19} /><strong>SSH</strong><span>双会话隔离 · 可控 exec</span></button><button className={kind === "LLM" ? "selected" : ""} onClick={() => selectKind("LLM")}><Sparkles size={19} /><strong>LLM</strong><span>模型白名单 · Key 隔离</span></button></div>
        <div className="identity-grid"><label className="form-field"><span>名称</span><input value={name} onChange={(event) => setName(event.target.value)} maxLength={80} placeholder={kind === "SSH" ? "Release builder" : kind === "LLM" ? "Coding assistant" : "Release downloads"} autoFocus /></label><label className={`form-field ${aliasInUse ? "invalid" : ""}`}><span>本地别名</span><input value={alias} onChange={(event) => updateAlias(event.target.value)} maxLength={63} placeholder={kind === "SSH" ? "build" : kind === "LLM" ? "coding" : "release-downloads"} aria-invalid={aliasInUse} />{aliasInUse && <small>这个本地别名已被使用，请换一个。</small>}</label></div>
        {kind === "SSH" && <label className={`form-field ssh-username-field ${localUsernameInUse ? "invalid" : ""}`}><span>本地 SSH 用户名</span><input value={localUsername} onChange={(event) => setLocalUsername(event.target.value.toLowerCase().replace(/[^a-z0-9._-]/g, ""))} maxLength={64} spellCheck={false} placeholder="builder" aria-invalid={localUsernameInUse} /><small>{localUsernameInUse ? "这个本地 SSH 用户名已映射到其他对象，请换一个。" : "同一目标 IP 可以重复使用；请用不同的本地用户名选择不同映射，本地密码也可以相同。"}</small></label>}
        {kind === "LLM" && <div className="llm-create-policy"><div className="llm-policy-grid"><div className="llm-provider"><span>协议预设</span><div className="policy-segmented"><button className={llmProvider === "openai" ? "selected" : ""} onClick={() => setLLMProvider("openai")}><Sparkles size={13} />OpenAI</button><button className={llmProvider === "anthropic" ? "selected" : ""} onClick={() => setLLMProvider("anthropic")}><ShieldCheck size={13} />Anthropic</button></div></div><label className="form-field"><span>允许模型 <small>逗号分隔 · {models.length}/32</small></span><input value={llmModels} onChange={(event) => setLLMModels(event.target.value)} maxLength={2048} placeholder={llmProvider === "openai" ? "model-a, model-b" : "claude-model-a"} /></label></div><div className="llm-limit-grid"><label className="form-field"><span>最大输出 Token</span><input type="number" min={1} max={1_000_000} step={256} value={maxOutputTokens} onChange={(event) => setMaxOutputTokens(Number(event.target.value))} /></label><label className="form-field"><span>每分钟请求</span><input type="number" min={1} max={60_000} value={requestsPerMinute} onChange={(event) => setRequestsPerMinute(Number(event.target.value))} /></label><label className="form-field"><span>最大并发</span><input type="number" min={1} max={1_024} value={maxConcurrent} onChange={(event) => setMaxConcurrent(Number(event.target.value))} /></label></div><label className="compact-check usage-opt-in"><input type="checkbox" checked={trackUsage} onChange={(event) => setTrackUsage(event.target.checked)} /><span><strong>统计调用与 Token</strong><small>默认关闭，不记录提示词或响应正文</small></span></label></div>}
        {kind === "SSH" && <div className="route-policy-strip"><span>命令权限</span><div className="policy-segmented"><button className={!allowAllCommands ? "selected" : ""} onClick={() => { setAllowAllCommands(false); setAllowAllConfirmed(false); }}><ShieldCheck size={14} />指定命令</button><button className={allowAllCommands ? "selected risk" : "risk"} onClick={() => { setAllowAllCommands(true); setAllowAllConfirmed(false); }}><AlertTriangle size={14} />所有命令</button></div><label className="compact-check"><input type="checkbox" checked={recordCommands} onChange={(event) => setRecordCommands(event.target.checked)} />记录命令</label></div>}
        {kind === "SSH" && !allowAllCommands && <label className="form-field ssh-command-field"><span>唯一允许命令 <small>{allowedCommand.length}/4096</small></span><input value={allowedCommand} onChange={(event) => setAllowedCommand(event.target.value)} maxLength={4096} spellCheck={false} placeholder="例如：uptime" /><small>按完整字符串匹配，不要在命令参数中填写密码或 Token。</small></label>}
        {kind === "SSH" && allowAllCommands && <div className="inline-warning"><TriangleAlert size={15} />所有命令是高风险能力，创建时需要在 Airlock 内明确确认。</div>}
        {kind === "SSH" && <label className={`audit-toggle sftp-toggle ${allowSftp ? "enabled" : ""}`}><input type="checkbox" checked={allowSftp} onChange={(event) => setAllowSftp(event.target.checked)} /><span><strong>{translate("允许 SFTP 文件传输（高风险）")}</strong><small>{translate("现代 OpenSSH 的 scp 默认使用 SFTP。启用后可列出、读取、写入、重命名和删除上游账号可访问的文件；请使用专用低权限账号。")}</small></span></label>}
        <div className="egress-field"><span>出口策略</span><div className="egress-control" role="group" aria-label="出口策略">{([{ value: "Direct", label: "直连", icon: Cable }, { value: "Proxy", label: "代理", icon: Network }, { value: "Auto", label: "自动", icon: GitBranch }] as const).map((option) => { const Icon = option.icon; return <button key={option.value} className={egress === option.value ? "selected" : ""} onClick={() => updateEgress(option.value)} aria-pressed={egress === option.value}><Icon size={14} />{option.label}</button>; })}</div></div>
        {egress !== "Direct" && !proxyConfigured && <div className="inline-warning"><TriangleAlert size={15} />代理出口尚未在设置中安全配置。</div>}
      </>}
	  {step === 2 && kind === "SSH" && <div className="ssh-secure-entry">
          <div className="secure-entry-heading"><span className="protected-icon"><SquareTerminal size={20} /></span><div><h3>上游连接与本地凭据</h3><p>同一上游地址可以创建多个映射；本地用户名负责选择不同的上游账号。</p></div><span className="local-only-badge"><ShieldCheck size={13} />仅本机处理</span></div>
          <div className="ssh-upstream-grid">
            <label className="form-field ssh-address-field"><span>上游 SSH 主机</span><input value={sshHost} onChange={(event) => updateSSHHost(event.target.value)} maxLength={505} spellCheck={false} placeholder="192.168.1.20" autoFocus /><small>支持主机名、IP 与 IPv6；不能填写 Airlock 自己的监听地址。</small></label>
            <label className="form-field ssh-port-field"><span>端口</span><input type="text" inputMode="numeric" value={sshPort} onChange={(event) => updateSSHPort(event.target.value)} maxLength={5} placeholder="22" /><small>默认 22</small></label>
            <label className="form-field"><span>上游用户名</span><input value={sshUsername} onChange={(event) => setSSHUsername(event.target.value)} maxLength={255} spellCheck={false} autoComplete="off" placeholder="deploy" /></label>
          </div>
          <SecretField label="上游密码" value={sshPassword} visible={showSSHPassword} onVisible={() => setShowSSHPassword((current) => !current)} onChange={setSSHPassword} placeholder="输入真实上游密码" detail="只发送到本机 airlockd，并按当前凭据保护方式保存。" />
          <label className={`form-field ssh-authentication-budget ${sshAuthenticationTimeoutSeconds >= 3 && sshAuthenticationTimeoutSeconds <= 120 ? "" : "invalid"}`}><span>上游认证预算（秒）</span><input type="number" min={3} max={120} step={1} value={sshAuthenticationTimeoutSeconds} onChange={(event) => setSSHAuthenticationTimeoutSeconds(Number(event.target.value))} /><small>默认 20 秒。创建只保存，之后点击健康检查时才会尝试认证。</small></label>
          <div className="local-auth-section">
            <div className="local-auth-heading"><div><strong>本地登录凭据</strong><span>调用方只使用这组凭据，不会接触上游密码</span></div><div className="policy-segmented"><button className={localCredentialMode === "generated" ? "selected" : ""} onClick={() => { setLocalCredentialMode("generated"); setLocalPassword(""); setLocalPasswordConfirmation(""); }}><KeyRound size={13} />随机生成</button><button className={localCredentialMode === "custom" ? "selected" : ""} onClick={() => setLocalCredentialMode("custom")}><ShieldCheck size={13} />自定义密码</button></div></div>
            {localCredentialMode === "generated" ? <p className="generated-credential-note">Airlock 将生成高强度本地凭据，并只在完成页显示一次。</p> : <div className="local-password-grid"><SecretField label="本地登录密码" value={localPassword} visible={showLocalPassword} onVisible={() => setShowLocalPassword((current) => !current)} onChange={setLocalPassword} placeholder="至少 12 个字节" detail={`${localPasswordBytes}/1024 bytes`} /><SecretField label="确认本地密码" value={localPasswordConfirmation} visible={showLocalPassword} onVisible={() => setShowLocalPassword((current) => !current)} onChange={setLocalPasswordConfirmation} placeholder="再次输入" detail={localPasswordConfirmation && localPassword !== localPasswordConfirmation ? "两次输入不一致" : "Airlock 只保存摘要"} invalid={Boolean(localPasswordConfirmation && localPassword !== localPasswordConfirmation)} /></div>}
          </div>
          <div className={`host-key-check ${sshProbe ? "ready" : ""}`}><div className="host-key-copy"><strong>SSH Host Key</strong><span>{sshProbe ? "通过可信渠道核对后再确认" : "先连接上游并读取公开指纹，不会尝试登录"}</span></div>{sshProbe ? <><code data-i18n="off">{sshProbe.fingerprint}</code><label className="compact-check host-key-accept"><input type="checkbox" checked={hostKeyAccepted} onChange={(event) => setHostKeyAccepted(event.target.checked)} /><span>我已核对并信任此 Host Key</span></label></> : <button className="secondary-button" onClick={() => void probeSSHHostKey()} disabled={!canProbeSSH}>{sshBusy === "probe" ? <RefreshCw className="spin" size={15} /> : <HeartPulse size={15} />}{sshBusy === "probe" ? "正在检测" : "检测 Host Key"}</button>}</div>
          {allowAllCommands && <label className="risk-consent"><input type="checkbox" checked={allowAllConfirmed} onChange={(event) => setAllowAllConfirmed(event.target.checked)} /><TriangleAlert size={16} /><span><strong>确认开放所有非交互 exec 命令</strong><small>Shell、PTY 与端口转发仍拒绝；SFTP 由独立开关控制。命令拥有上游账号可访问的数据与操作权限。</small></span></label>}
          {sshError && <div className="inline-error ssh-entry-error"><TriangleAlert size={16} /><span>{sshError}</span></div>}
          {!connected && <div className="inline-error"><TriangleAlert size={16} />airlockd 未连接，暂时无法保存。</div>}
          <div className="secure-entry-actions"><span>{sshProbe ? "Host Key 已读取；确认后安全保存，稍后可在路由页检查连接" : "先检测 Host Key，再保存凭据和路由"}</span><button className="primary-button" onClick={() => void secureCreate()} disabled={!canCreateSSH}>{sshBusy === "create" ? <RefreshCw className="spin" size={16} /> : <ShieldCheck size={16} />}{sshBusy === "create" ? "正在安全保存" : "信任并保存路由"}</button></div>
        </div>}
	  {step === 2 && kind !== "SSH" && <><h3>完成受保护配置</h3><div className="protected-box"><span className="protected-icon">{kind === "LLM" ? <Sparkles size={21} /> : <KeyRound size={21} />}</span><div><strong>系统安全引导</strong><p>{kind === "LLM" ? "录入上游 Base URL 与 API Key，然后自定义或随机生成完全隔离的本地 API Key。" : "完整 URL 与 Authorization 按当前凭据保护方式存储。"}</p></div><button className="primary-button" onClick={() => void secureCreate()} disabled={!connected || saving}>{saving ? <><RefreshCw className="spin" size={16} />等待系统窗口</> : <><KeyRound size={16} />开始安全设置</>}</button></div>{!connected && <div className="inline-error"><TriangleAlert size={16} />airlockd 未连接，暂时无法保存。</div>}</>}
	  {step === 3 && created && <div className="success-state"><CircleCheck size={32} /><h3>{kind === "SSH" ? "路由已安全保存" : "路由已启用"}</h3><p>{kind === "SSH" ? sshCreation?.generatedCredential ? "本地凭据只显示这一次。请先在路由页检查上游认证，再手动启用该路由。" : "请在路由页检查上游认证，确认后再手动启用；Airlock 不会回显本地密码。" : kind === "LLM" ? "Base URL 与本地 API Key 已在安全窗口中确认。" : "本地访问入口已创建。"}</p><code data-i18n="off">{created.localEndpoint}</code>{kind === "SSH" && sshCreation?.generatedCredential && <div className="one-time-credential"><span>一次性本地凭据</span><code data-i18n="off">{sshCreation.localCredential}</code><small>关闭此页面后无法再次查看。</small></div>}</div>}
    </div>
    <footer className="editor-footer"><button className="secondary-button" onClick={step === 1 || step === 3 ? onClose : () => setStep(1)} disabled={saving}>{step === 2 && <ChevronLeft size={16} />}{step === 3 ? "完成" : step === 1 ? "取消" : "上一步"}</button>{step === 1 && <button className="primary-button" onClick={() => setStep(2)} disabled={!validIdentity}>继续<ChevronRight size={16} /></button>}</footer>
  </div></div>;
}

function SecretField({ label, value, visible, onVisible, onChange, placeholder, detail, invalid = false }: { label: string; value: string; visible: boolean; onVisible: () => void; onChange: (value: string) => void; placeholder: string; detail: string; invalid?: boolean }) {
  const inputId = useId();
  return <div className={`form-field secret-field ${invalid ? "invalid" : ""}`}><label htmlFor={inputId}>{label}</label><span className="secret-input-wrap"><input id={inputId} type={visible ? "text" : "password"} value={value} onChange={(event) => onChange(event.target.value)} autoComplete="new-password" spellCheck={false} placeholder={placeholder} aria-invalid={invalid} /><button type="button" className="secret-visibility" onClick={onVisible} aria-label={visible ? "隐藏密码" : "显示密码"} title={visible ? "隐藏密码" : "显示密码"}>{visible ? <EyeOff size={15} /> : <Eye size={15} />}</button></span><small>{detail}</small></div>;
}

function WindowChrome() {
  if (!isTauri) return null;
  const window = getCurrentWindow();
  const minimize = () => void window.minimize().catch(() => undefined);
  const toggleMaximize = () => void window.toggleMaximize().catch(() => undefined);
  const close = () => void window.close().catch(() => undefined);
  return <div className="window-chrome" data-tauri-drag-region onDoubleClick={toggleMaximize}>
    <div className="window-chrome-brand" data-tauri-drag-region><span className="window-chrome-mark"><LockKeyhole size={12} /></span><strong>Airlock</strong></div>
    <div className="window-chrome-actions" aria-label={translate("窗口控制")}>
      <button type="button" className="window-control" onClick={minimize} aria-label={translate("最小化窗口")} title={translate("最小化窗口")}><Minus size={15} /></button>
      <button type="button" className="window-control" onClick={toggleMaximize} aria-label={translate("最大化窗口")} title={translate("最大化窗口")}><Maximize2 size={13} /></button>
      <button type="button" className="window-control window-control-close" onClick={close} aria-label={translate("关闭窗口")} title={translate("关闭窗口")}><X size={14} /></button>
    </div>
  </div>;
}

function DeveloperCard() {
  const [avatarFailed, setAvatarFailed] = useState(false);
  const openExternal = async (url: "https://0o0.site" | "https://github.com/LouisonH") => {
    if (isTauri) {
      await invoke("open_external_url", { url });
      return;
    }
    window.open(url, "_blank", "noopener,noreferrer");
  };
  return <section className="settings-section about-section"><div><h2>关于</h2><p>为不受信任的调用方隔离真实凭据</p></div><div className="developer-card">
    <button type="button" className={`developer-avatar ${avatarFailed ? "fallback" : ""}`} onClick={() => void openExternal("https://0o0.site")} title="访问 LouisonH 的网站" aria-label="访问 LouisonH 的网站">{avatarFailed ? <span>LH</span> : <img src="/louisonh.png" alt="" onError={() => setAvatarFailed(true)} />}</button>
    <div className="developer-copy"><span className="developer-label">{translate("Developer")}</span><strong>LouisonH</strong><p>{translate("产品设计与核心开发")}</p><p>{translate("华南理工大学（SCUT）相关开发者 · 独立个人项目")}</p><p className="developer-purpose">{translate("Airlock 面向不受信任的 LLM、自动化工具与第三方 API 中转环境，让调用方只接触本地二次凭据，隔离真实 URL、账号、密码与 API Key。")}</p><p className="developer-purpose">{translate("Airlock 由 LouisonH 独立开发，不代表华南理工大学的官方项目、立场或背书。")}</p></div>
    <div className="developer-meta"><button type="button" className="developer-link" onClick={() => void openExternal("https://github.com/LouisonH")}><Github size={14} />github.com/LouisonH</button><button type="button" className="developer-link" onClick={() => void openExternal("https://0o0.site")}><Globe2 size={14} />0o0.site</button><span><Sparkles size={14} />{translate("AI 协作 · GPT-5.6 Sol")}</span><span>v{APP_VERSION} · {translate("Technical Preview")}</span></div>
  </div></section>;
}

function EmptyState({ icon: Icon, title, detail }: { icon: typeof Route; title: string; detail: string }) { return <div className="empty-state"><Icon size={22} /><strong>{title}</strong><span>{detail}</span></div>; }
function PageHeader({ title, subtitle, action }: { title: string; subtitle: string; action?: React.ReactNode }) { return <div className="page-header"><div><h1>{title}</h1><p>{subtitle}</p></div>{action}</div>; }
function Metric({ label, value, detail, tone }: { label: string; value: string; detail: string; tone?: string }) { return <div className="metric"><span>{label}</span><strong className={tone}>{value}</strong><small>{detail}</small></div>; }
function StatusBadge({ status }: { status: string }) { const labels: Record<string, string> = { enabled: "已启用", disabled: "已停用", blocked: "已阻止", allowed: "已允许", failed: "失败" }; const Icon = status === "enabled" || status === "allowed" ? CircleCheck : status === "disabled" ? CircleMinus : TriangleAlert; return <span className={`status-badge status-${status}`}><Icon size={13} />{labels[status] ?? status}</span>; }
function HealthBadge({ health, checking = false }: { health: RouteSummary["health"]; checking?: boolean }) { const labels = { healthy: "健康", degraded: "异常", unknown: "未测试" }; const Icon = checking ? LoaderCircle : health === "healthy" ? CircleCheck : health === "degraded" ? TriangleAlert : CircleMinus; return <span className={`health health-${checking ? "checking" : health}`}><Icon className={checking ? "spin" : undefined} size={13} />{checking ? "检测中" : labels[health]}</span>; }
function KindBadge({ kind }: { kind: RouteKind }) { const Icon = kind === "HTTP" ? Server : kind === "SSH" ? SquareTerminal : Sparkles; return <span className={`kind kind-${kind.toLowerCase()}`}><Icon size={13} />{kind}</span>; }
function ActivityKindBadge({ event }: { event: ActivityEvent }) { const Icon = event.eventType === "health" ? HeartPulse : event.category === "HTTP" ? Server : event.category === "SSH" ? SquareTerminal : event.category === "LLM" ? Sparkles : Activity; const label = event.eventType === "health" ? translate("健康检查") : event.category; return <span className={`activity-kind activity-kind-${event.eventType === "health" ? "health" : event.category.toLowerCase()}`}><Icon size={13} />{label}</span>; }
function formatActivityAction(event: ActivityEvent) { if (event.eventType === "health") return translate("手动健康检查"); if (event.eventType === "request") return translate(event.category === "LLM" ? "LLM API 请求" : event.action.replace(" request", " 请求")); return event.action; }
function PermissionSummary({ route }: { route: RouteSummary }) { if (route.kind !== "LLM") return <span className={route.allowAllCommands ? "permission-risk" : "permission-copy"}>{route.permissionSummary}</span>; return <div className="llm-policy-summary"><strong>{route.provider === "anthropic" ? "Anthropic" : "OpenAI"} · output ≤ {route.maxOutputTokens ?? "-"}</strong><span><b>{route.allowedModels?.length ?? 0}</b> models</span><span><b>{route.requestsPerMinute ?? 0}</b>/min</span><span><b>{route.maxConcurrent ?? 0}</b> concurrent</span>{route.trackUsage && <span className="usage-chip"><b>{formatUsage(route.totalRequests ?? 0)}</b> calls</span>}</div>; }
function formatUsage(value: number) { return new Intl.NumberFormat(getResolvedLocale(), { notation: value >= 100_000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(value); }
function ReadOnlyField({ label, value, tone }: { label: string; value: string; tone?: "success" | "warning" }) { return <div className="readonly-field"><span>{label}</span><strong className={tone ? `setting-value ${tone}` : "setting-value"}>{value}</strong></div>; }
function ThemeControl({ value, onChange }: { value: ThemePreference; onChange: (value: ThemePreference) => void }) { const options: Array<{ value: ThemePreference; label: string; icon: typeof Monitor }> = [{ value: "system", label: "系统", icon: Monitor }, { value: "light", label: "浅色", icon: Sun }, { value: "dark", label: "深色", icon: Moon }]; return <div className="theme-control" role="group" aria-label="界面主题">{options.map((option) => { const Icon = option.icon; return <button key={option.value} className={value === option.value ? "selected" : ""} onClick={() => onChange(option.value)} aria-pressed={value === option.value}><Icon size={14} />{option.label}</button>; })}</div>; }
function AccentControl({ value, onChange }: { value: AccentTheme; onChange: (value: AccentTheme) => void }) { const options: Array<{ value: AccentTheme; label: string }> = [{ value: "forest", label: "Miku" }, { value: "ocean", label: "天依" }, { value: "amber", label: "镜音双子" }, { value: "coral", label: "阿绫" }]; return <div className="accent-control" role="group" aria-label="配色风格">{options.map((option) => <button key={option.value} className={value === option.value ? "selected" : ""} onClick={() => onChange(option.value)} aria-pressed={value === option.value}><span className={`accent-swatch ${option.value}`} />{option.label}</button>)}</div>; }
function Modal({ title, onClose, children, className = "" }: { title: string; onClose: () => void; children: React.ReactNode; className?: string }) { return <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><div className={`modal ${className}`} role="dialog" aria-modal="true"><header className="modal-header"><h2>{title}</h2><button className="icon-button small" onClick={onClose} aria-label="关闭"><X size={16} /></button></header>{children}</div></div>; }

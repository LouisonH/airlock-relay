import { invoke } from "@tauri-apps/api/core";
import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  Cable,
  Check,
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  CircleMinus,
  CircleStop,
  Gauge,
  GitBranch,
  Github,
  Globe2,
  HardDrive,
  KeyRound,
  LockKeyhole,
  Monitor,
  Moon,
  Network,
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
  Sparkles,
  SquareTerminal,
  Sun,
  Trash2,
  TriangleAlert,
  Wifi,
  X,
} from "lucide-react";
import { applyTheme, getAccentTheme, getDensityPreference, getMotionPreference, getRefreshInterval, getThemePreference, saveAccentTheme, saveDensityPreference, saveMotionPreference, saveRefreshInterval, saveThemePreference, watchSystemTheme, type AccentTheme, type DensityPreference, type MotionPreference, type RefreshInterval, type ThemePreference } from "./theme";
import type { ActivityEvent, ControlState, ControlUpdate, NetworkScope, RouteKind, RouteSummary, SecretStoreMode, SecuritySettings, SecurityUpdate } from "./types";

type Page = "overview" | "routes" | "activity" | "settings";
type RouteFilter = "All" | RouteKind;

const isTauri = "__TAURI_INTERNALS__" in window;
const previewRoutes: RouteSummary[] = isTauri ? [] : [
  { id: "release", name: "Release downloads", alias: "release", kind: "HTTP", status: "enabled", localEndpoint: "127.0.0.1:4768/r/release", permissionSummary: "GET, HEAD · Range", egress: "Direct", health: "healthy", lastUsed: "2 分钟前", currentConnections: 1, allowAllCommands: false, recordCommands: false },
  { id: "models", name: "Model gateway", alias: "models", kind: "LLM", status: "disabled", localEndpoint: "http://127.0.0.1:4768/r/models", permissionSummary: "OpenAI · 3 models · output ≤ 8192 · 60/min · 4 concurrent", egress: "Proxy", health: "unknown", lastUsed: "从未", currentConnections: 0, allowAllCommands: false, recordCommands: false, provider: "openai", allowedModels: ["gpt-5.2", "gpt-5.2-codex", "gpt-5.1"], maxOutputTokens: 8192, requestsPerMinute: 60, maxConcurrent: 4, trackUsage: true, totalRequests: 128, inputTokens: 184320, outputTokens: 42670 },
  { id: "build", name: "Release builder", alias: "build", kind: "SSH", status: "enabled", localEndpoint: "build@127.0.0.1:4770", permissionSummary: "all exec commands · high risk · recorded", egress: "Auto", health: "healthy", lastUsed: "刚刚", currentConnections: 0, allowAllCommands: true, recordCommands: true, allowedCommand: "" },
];

const previewActivity: ActivityEvent[] = isTauri ? [] : [
  { id: "ssh-preview", time: "07-29 23:40:12", routeName: "Release builder", caller: "build@loopback", action: "printf airlock-ok", result: "allowed", latency: "182 ms", egress: "Auto" },
];

const navItems: Array<{ id: Page; label: string; icon: typeof Gauge }> = [
  { id: "overview", label: "概览", icon: Gauge },
  { id: "routes", label: "路由", icon: Route },
  { id: "activity", label: "活动", icon: Activity },
  { id: "settings", label: "设置", icon: Settings },
];

const emptyControl: ControlState = {
  connected: !isTauri,
  running: !isTauri,
  routes: previewRoutes,
  proxyConfigured: false,
  sshReady: !isTauri,
  activity: previewActivity,
  securitySettings: { version: 1, networkScope: "loopback", secretStore: "local_file" },
  message: isTauri ? "正在连接 airlockd" : undefined,
};

export default function App() {
  const [page, setPage] = useState<Page>("overview");
  const [control, setControl] = useState<ControlState>(emptyControl);
  const [emergencyOpen, setEmergencyOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [theme, setTheme] = useState<ThemePreference>(getThemePreference);
  const [accent, setAccent] = useState<AccentTheme>(getAccentTheme);
  const [density, setDensity] = useState<DensityPreference>(getDensityPreference);
  const [motion, setMotion] = useState<MotionPreference>(getMotionPreference);
  const [refreshInterval, setRefreshInterval] = useState<RefreshInterval>(getRefreshInterval);
  const [notice, setNotice] = useState<string>();
  const [pendingDelete, setPendingDelete] = useState<RouteSummary>();
  const [policyRoute, setPolicyRoute] = useState<RouteSummary>();

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
    void refresh();
    const timer = window.setInterval(() => void refresh(), refreshInterval);
    return () => window.clearInterval(timer);
  }, [refreshInterval]);

  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(undefined), 3600);
    return () => window.clearTimeout(timer);
  }, [notice]);

  const enabledCount = control.routes.filter((route) => route.status === "enabled").length;

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

  const updateSSHPolicy = async (allowedCommand: string, allowAllCommands: boolean, recordCommands: boolean) => {
    if (!policyRoute) return;
    try {
      const routes = isTauri
        ? await invoke<RouteSummary[]>("set_ssh_policy", { alias: policyRoute.alias, allowedCommand, allowAllCommands, recordCommands })
        : control.routes.map((route) => route.alias === policyRoute.alias ? {
          ...route,
          allowedCommand: allowAllCommands ? "" : allowedCommand,
          allowAllCommands,
          recordCommands,
          permissionSummary: `${allowAllCommands ? "all exec commands · high risk" : "1 exact command · stdin denied"}${recordCommands ? " · recorded" : ""}`,
        } : route);
      setControl((current) => ({ ...current, routes }));
      setPolicyRoute(undefined);
      setNotice("SSH 命令权限已更新");
    } catch (error) {
      setNotice(String(error));
    }
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

	const applySecuritySettings = async (settings: SecuritySettings) => {
		try {
			const update = isTauri
				? await invoke<SecurityUpdate>("apply_security_settings", { networkScope: settings.networkScope, secretStore: settings.secretStore })
				: { securitySettings: settings, message: "安全设置已更新" };
			setControl((current) => ({ ...current, securitySettings: update.securitySettings }));
			setNotice(update.message ?? "安全设置已更新");
			await refresh();
		} catch (error) {
			setNotice(String(error));
		}
	};

  return (
    <div className="app-shell">
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
		  <div><strong>{control.connected ? "本地核心已连接" : "等待本地核心"}</strong><span>airlockd · {control.securitySettings.networkScope === "lan" ? "LAN relay" : "loopback only"}</span></div>
        </div>
      </aside>

      <main className="workspace">
        <header className="toolbar">
          <div className="service-state"><ShieldCheck size={16} /><span>{control.connected ? `${enabledCount} 条路由已开放` : "控制通道未连接"}</span></div>
          <div className="toolbar-actions">
            <button className="icon-button" onClick={() => void refresh()} title="刷新状态" aria-label="刷新状态"><RefreshCw size={16} /></button>
            <button className="danger-button" onClick={() => setEmergencyOpen(true)} disabled={!control.connected || enabledCount === 0}><CircleStop size={16} />停止全部</button>
          </div>
        </header>

        <div className="page-content" key={page}>
          {page === "overview" && <Overview control={control} onRoutes={() => setPage("routes")} onAdd={() => setEditorOpen(true)} />}
          {page === "routes" && <Routes routes={control.routes} connected={control.connected} onToggle={toggleRoute} onDelete={setPendingDelete} onPolicy={setPolicyRoute} onAdd={() => setEditorOpen(true)} />}
          {page === "activity" && <ActivityPage events={control.activity} />}
          {page === "settings" && <SettingsPage theme={theme} onTheme={setTheme} accent={accent} onAccent={setAccent} density={density} onDensity={setDensity} motion={motion} onMotion={setMotion} refreshInterval={refreshInterval} onRefreshInterval={setRefreshInterval} connected={control.connected} proxyConfigured={control.proxyConfigured} sshReady={control.sshReady} securitySettings={control.securitySettings} onSecuritySettings={applySecuritySettings} onConfigureProxy={configureProxy} onClearProxy={clearProxy} />}
        </div>
      </main>

      {notice && <div className="toast" role="status">{notice}</div>}
      {emergencyOpen && <Modal title="停止全部路由" onClose={() => setEmergencyOpen(false)}><div className="warning-panel"><AlertTriangle size={19} /><p>新请求将立即被拒绝，已建立的连接会进入关闭流程。</p></div><div className="modal-actions"><button className="secondary-button" onClick={() => setEmergencyOpen(false)}>取消</button><button className="danger-button" onClick={() => void stopAll()}><CircleStop size={16} />确认停止</button></div></Modal>}
	  {pendingDelete && <Modal title="删除路由" onClose={() => setPendingDelete(undefined)}><div className="danger-panel"><Trash2 size={19} /><div><strong>{pendingDelete.name}</strong><p>本地入口、Capability 和当前 SecretStore 中的受保护目标都会被永久删除。</p></div></div><div className="modal-actions"><button className="secondary-button" onClick={() => setPendingDelete(undefined)}>取消</button><button className="danger-button" onClick={() => void deleteRoute()}><Trash2 size={16} />删除路由</button></div></Modal>}
      {policyRoute?.kind === "SSH" && <SSHPolicyEditor route={policyRoute} onClose={() => setPolicyRoute(undefined)} onSave={updateSSHPolicy} />}
      {policyRoute?.kind === "LLM" && <LLMPolicyEditor route={policyRoute} onClose={() => setPolicyRoute(undefined)} onSave={updateLLMPolicy} onRotate={rotateLLMKey} onResetUsage={resetLLMUsage} />}
      {editorOpen && <RouteEditor connected={control.connected} proxyConfigured={control.proxyConfigured} sshReady={control.sshReady} onClose={() => setEditorOpen(false)} onCreated={(route) => setControl((current) => ({ ...current, routes: [...current.routes.filter((item) => item.id !== route.id), route] }))} onError={setNotice} />}
    </div>
  );
}

function Overview({ control, onRoutes, onAdd }: { control: ControlState; onRoutes: () => void; onAdd: () => void }) {
  const enabled = control.routes.filter((route) => route.status === "enabled").length;
  const connections = control.routes.reduce((sum, route) => sum + route.currentConnections, 0);
  return <>
    <PageHeader title="概览" subtitle="本机开放能力与安全状态" action={<button className="primary-button" onClick={onAdd} disabled={!control.connected}><Plus size={16} />新增路由</button>} />
    <section className={`service-band ${control.connected ? "running" : "stopped"}`}>
      <span className="service-icon"><ShieldCheck size={20} /></span>
      <div className="service-copy"><strong>{control.connected ? "受保护控制通道已连接" : "airlockd 尚未连接"}</strong><span>{control.connected ? "Unix Socket · 当前用户专用" : control.message ?? "启动本地核心后将自动重连"}</span></div>
      <div className="listener-status"><span><Server size={14} />HTTP <b>{control.connected ? "ON" : "OFF"}</b></span><span><SquareTerminal size={14} />SSH <b>{control.sshReady ? "ON" : "OFF"}</b></span></div>
    </section>
    <section className="metric-strip" aria-label="运行指标">
      <Metric label="开放路由" value={String(enabled)} detail={`共 ${control.routes.length} 条`} />
      <Metric label="当前连接" value={String(connections)} detail="仅统计本地入口" />
	  <Metric label="凭据存储" value={control.securitySettings.secretStore === "keychain" ? "Keychain" : "本机文件"} detail={control.securitySettings.secretStore === "keychain" ? "系统加密保护" : "0600 权限隔离"} tone={control.securitySettings.secretStore === "keychain" ? "success" : "warning"} />
    </section>
    <section className="section-block">
      <div className="section-heading"><div><h2>路由</h2><p>界面只显示安全别名和本地入口。</p></div><button className="text-button" onClick={onRoutes}>查看全部<ChevronRight size={15} /></button></div>
      <RouteTable routes={control.routes.slice(0, 5)} compact />
    </section>
  </>;
}

function Routes({ routes, connected, onToggle, onDelete, onPolicy, onAdd }: { routes: RouteSummary[]; connected: boolean; onToggle: (alias: string, enabled: boolean) => void; onDelete: (route: RouteSummary) => void; onPolicy: (route: RouteSummary) => void; onAdd: () => void }) {
  const [filter, setFilter] = useState<RouteFilter>("All");
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => routes.filter((route) => (filter === "All" || route.kind === filter) && `${route.name} ${route.alias}`.toLowerCase().includes(query.toLowerCase())), [routes, filter, query]);
  return <>
    <PageHeader title="路由" subtitle={`${routes.length} 条 · ${routes.filter((route) => route.status === "enabled").length} 条已开放`} action={<button className="primary-button" onClick={onAdd} disabled={!connected}><Plus size={16} />新增路由</button>} />
    <div className="filter-bar">
      <label className="search-field"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索名称或别名" /></label>
      <div className="segmented" aria-label="路由类型">{(["All", "HTTP", "SSH", "LLM"] as RouteFilter[]).map((value) => <button key={value} className={filter === value ? "selected" : ""} onClick={() => setFilter(value)}>{value === "All" ? "全部" : value}</button>)}</div>
    </div>
    <RouteTable routes={filtered} onToggle={(route) => onToggle(route.alias, route.status !== "enabled")} onDelete={onDelete} onPolicy={onPolicy} />
  </>;
}

function RouteTable({ routes, compact = false, onToggle, onDelete, onPolicy }: { routes: RouteSummary[]; compact?: boolean; onToggle?: (route: RouteSummary) => void; onDelete?: (route: RouteSummary) => void; onPolicy?: (route: RouteSummary) => void }) {
  if (routes.length === 0) return <EmptyState icon={Route} title="暂无路由" detail="创建后，本地入口会显示在这里。" />;
  return <div className="table-wrap"><table className="route-table"><thead><tr><th>状态</th><th>名称</th><th>类型</th><th>本地入口</th><th>权限</th><th>出口</th><th>健康</th>{!compact && <th>最近使用</th>}<th aria-label="操作" /></tr></thead><tbody>{routes.map((route, index) => <tr key={route.id} className={route.status === "enabled" ? "" : "route-muted"} style={{ animationDelay: `${index * 32}ms` }}><td><StatusBadge status={route.status} /></td><td><strong>{route.name}</strong><span className="cell-subtext">{route.alias}</span></td><td><KindBadge kind={route.kind} /></td><td><code>{route.localEndpoint}</code></td><td><PermissionSummary route={route} /></td><td>{route.egress}</td><td><HealthBadge health={route.health} /></td>{!compact && <td>{route.lastUsed}</td>}<td>{onToggle && <div className="route-actions"><button className="route-switch" role="switch" aria-checked={route.status === "enabled"} title={route.status === "enabled" ? "停用路由" : "启用路由"} aria-label={route.status === "enabled" ? "停用路由" : "启用路由"} onClick={() => onToggle(route)}><span /></button>{onPolicy && (route.kind === "SSH" || route.kind === "LLM") && <button className="row-icon-button visible" title={route.kind === "SSH" ? "SSH 命令权限" : "LLM 访问边界"} aria-label={`设置 ${route.name} 的访问边界`} onClick={() => onPolicy(route)}>{route.kind === "LLM" ? <SlidersHorizontal size={14} /> : <Settings2 size={14} />}</button>}{onDelete && <button className="row-icon-button danger" title="删除路由" aria-label={`删除 ${route.name}`} onClick={() => onDelete(route)}><Trash2 size={14} /></button>}</div>}</td></tr>)}</tbody></table></div>;
}

function ActivityPage({ events }: { events: ActivityEvent[] }) {
  return <><PageHeader title="活动" subtitle="SSH 命令记录仅保存在本机，不包含上游地址或凭据" /><ActivityTable events={events} /></>;
}

function ActivityTable({ events }: { events: ActivityEvent[] }) {
  if (events.length === 0) return <EmptyState icon={Activity} title="暂无活动" detail="新的脱敏事件会显示在这里。" />;
  return <div className="table-wrap"><table className="activity-table"><thead><tr><th>时间</th><th>路由</th><th>调用者</th><th>命令</th><th>结果</th><th>延迟</th><th>出口</th><th>事件 ID</th></tr></thead><tbody>{events.map((event) => <tr key={event.id}><td>{event.time}</td><td><strong>{event.routeName}</strong></td><td>{event.caller}</td><td><code className="command-cell">{event.action}</code></td><td><StatusBadge status={event.result} /></td><td>{event.latency}</td><td>{event.egress}</td><td><code>{event.id}</code></td></tr>)}</tbody></table></div>;
}

function SettingsPage({ theme, onTheme, accent, onAccent, density, onDensity, motion, onMotion, refreshInterval, onRefreshInterval, connected, proxyConfigured, sshReady, securitySettings, onSecuritySettings, onConfigureProxy, onClearProxy }: { theme: ThemePreference; onTheme: (theme: ThemePreference) => void; accent: AccentTheme; onAccent: (accent: AccentTheme) => void; density: DensityPreference; onDensity: (density: DensityPreference) => void; motion: MotionPreference; onMotion: (motion: MotionPreference) => void; refreshInterval: RefreshInterval; onRefreshInterval: (interval: RefreshInterval) => void; connected: boolean; proxyConfigured: boolean; sshReady: boolean; securitySettings: SecuritySettings; onSecuritySettings: (settings: SecuritySettings) => Promise<void>; onConfigureProxy: () => void; onClearProxy: () => void }) {
  const [draft, setDraft] = useState(securitySettings);
  const [saving, setSaving] = useState(false);
  useEffect(() => setDraft(securitySettings), [securitySettings]);
  const preset = draft.secretStore === "keychain" && draft.networkScope === "loopback" ? "strict" : draft.secretStore === "local_file" && draft.networkScope === "loopback" ? "standard" : draft.secretStore === "local_file" && draft.networkScope === "lan" ? "convenient" : "custom";
  const dirty = draft.secretStore !== securitySettings.secretStore || draft.networkScope !== securitySettings.networkScope;
  const choosePreset = (value: "strict" | "standard" | "convenient") => setDraft((current) => ({ ...current, secretStore: value === "strict" ? "keychain" : "local_file", networkScope: value === "convenient" ? "lan" : "loopback" }));
  const apply = async () => { setSaving(true); try { await onSecuritySettings(draft); } finally { setSaving(false); } };
  const activeNetwork = securitySettings.networkScope;
  return <><PageHeader title="设置" subtitle="本地外观、网络与安全边界" />
    <section className="settings-section"><div><h2>外观</h2><p>主题偏好保存在本机</p></div><div className="settings-controls"><div className="setting-row"><span>显示模式</span><ThemeControl value={theme} onChange={onTheme} /></div><div className="setting-row"><span>配色风格</span><AccentControl value={accent} onChange={onAccent} /></div></div></section>
    <section className="settings-section"><div><h2>界面行为</h2><p>调整刷新节奏、密度与动画</p></div><div className="settings-controls"><PreferenceRow label="自动刷新" detail="控制状态轮询频率"><PreferenceSegment value={refreshInterval} options={[{ value: 2000, label: "2 秒" }, { value: 5000, label: "5 秒" }, { value: 15000, label: "15 秒" }]} onChange={onRefreshInterval} /></PreferenceRow><PreferenceRow label="信息密度" detail="影响表格行高与页面间距"><PreferenceSegment value={density} options={[{ value: "comfortable", label: "舒适" }, { value: "compact", label: "紧凑" }]} onChange={onDensity} /></PreferenceRow><PreferenceRow label="界面动效" detail="精简模式会关闭循环和位移动画"><PreferenceSegment value={motion} options={[{ value: "system", label: "跟随系统" }, { value: "standard", label: "标准" }, { value: "reduced", label: "精简" }]} onChange={onMotion} /></PreferenceRow></div></section>
    <section className="settings-section security-settings"><div><h2>安全方案</h2><p>新安装默认标准，切换前会原生确认</p></div><div className="settings-controls security-controls">
      <div className="security-heading"><div><span>当前组合</span><strong>{preset === "strict" ? "严格" : preset === "standard" ? "标准" : preset === "convenient" ? "便捷" : "自定义"}</strong></div><span className={`security-level level-${preset}`}>{preset === "strict" ? "高保护" : preset === "convenient" ? "高暴露" : "受控"}</span></div>
      <div className="security-presets" role="group" aria-label="安全等级"><button className={preset === "strict" ? "selected" : ""} onClick={() => choosePreset("strict")}><ShieldCheck size={16} /><span><strong>严格</strong><small>Keychain · 仅本机</small></span></button><button className={preset === "standard" ? "selected" : ""} onClick={() => choosePreset("standard")}><HardDrive size={16} /><span><strong>标准</strong><small>0600 文件 · 仅本机</small></span></button><button className={preset === "convenient" ? "selected risk" : "risk"} onClick={() => choosePreset("convenient")}><Wifi size={16} /><span><strong>便捷</strong><small>0600 文件 · 局域网</small></span></button></div>
      <SecurityChoice label="凭据保护" detail="上游地址、账号、密码与代理认证" value={draft.secretStore} options={[{ value: "keychain", label: "Keychain", icon: KeyRound }, { value: "local_file", label: "0600 文件", icon: HardDrive }]} onChange={(secretStore) => setDraft((current) => ({ ...current, secretStore }))} />
      <SecurityChoice label="网络范围" detail="只影响 HTTP/LLM/SSH 数据入口，控制面始终仅本机" value={draft.networkScope} options={[{ value: "loopback", label: "仅本机", icon: Monitor }, { value: "lan", label: "局域网", icon: Wifi }]} onChange={(networkScope) => setDraft((current) => ({ ...current, networkScope }))} />
      <div className={`security-explainer ${draft.secretStore === "local_file" || draft.networkScope === "lan" ? "warning" : "safe"}`}><TriangleAlert size={16} /><div><strong>{draft.secretStore === "keychain" ? "系统加密保护" : "免钥匙串提示，但不加密"}</strong><p>{draft.secretStore === "keychain" ? "首次访问、调试包重建或系统锁定后，macOS 可能要求验证登录密码。" : "Secret 仅由当前 macOS 账户和 0600 文件权限隔离；同账户的其他进程可能读取。"}{draft.networkScope === "lan" ? " 局域网设备持有路由密码时可访问转发入口，不要映射到公网。" : ""}</p></div></div>
      <div className="security-actions"><span>{dirty ? "应用后会短暂重启 airlockd" : "已与当前运行设置一致"}</span><button className="primary-button" disabled={!connected || !dirty || saving} onClick={() => void apply()}>{saving ? <RefreshCw className="spin" size={15} /> : <ShieldCheck size={15} />}{saving ? "正在迁移并重启" : "应用设置"}</button></div>
    </div></section>
    <section className="settings-section"><div><h2>网络与出口</h2><p>{activeNetwork === "lan" ? "数据入口已对局域网开放" : "数据入口仅本机可访问"}</p></div><div className="settings-controls"><ReadOnlyField label="HTTP 入口" value={activeNetwork === "lan" ? "0.0.0.0:4768 · 请使用本机局域网 IP" : "127.0.0.1:4768"} tone={activeNetwork === "lan" ? "warning" : undefined} /><ReadOnlyField label="SSH 入口" value={sshReady ? activeNetwork === "lan" ? "0.0.0.0:4770 · 局域网" : "127.0.0.1:4770 · 已就绪" : "等待 airlockd"} tone={sshReady && activeNetwork !== "lan" ? "success" : "warning"} /><ReadOnlyField label="控制通道" value={connected ? "Unix Socket · 仅当前用户" : "等待 airlockd"} tone={connected ? "success" : "warning"} /><div className="proxy-setting"><div><span>Clash / SOCKS5 出口</span><strong className={proxyConfigured ? "setting-value success" : "setting-value"}>{proxyConfigured ? `${securitySettings.secretStore === "keychain" ? "Keychain" : "0600 文件"} · 已配置` : "未配置"}</strong></div><div className="inline-actions"><button className="secondary-button compact" onClick={onConfigureProxy} disabled={!connected}><Network size={14} />{proxyConfigured ? "更换" : "配置"}</button>{proxyConfigured && <button className="row-icon-button danger visible" onClick={onClearProxy} aria-label="清除代理出口" title="清除代理出口"><Trash2 size={14} /></button>}</div></div></div></section>
    <section className="settings-section"><div><h2>不变的安全边界</h2><p>便捷模式也不会放开控制面</p></div><div className="settings-controls"><ReadOnlyField label="路由元数据" value="0600 · 不包含明文本地密码" tone="success" /><ReadOnlyField label="SSH 安全核心" value={sshReady ? "双会话隔离 · Shell/PTY 默认拒绝" : "等待本地核心"} tone={sshReady ? "success" : "warning"} /><ReadOnlyField label="敏感录入" value="macOS 原生窗口 · 不进入 WebView" tone="success" /></div></section>
    <DeveloperCard />
  </>;
}

function SecurityChoice<T extends NetworkScope | SecretStoreMode>({ label, detail, value, options, onChange }: { label: string; detail: string; value: T; options: Array<{ value: T; label: string; icon: typeof Monitor }>; onChange: (value: T) => void }) {
  return <div className="security-choice"><div><strong>{label}</strong><small>{detail}</small></div><div className="choice-control" role="group" aria-label={label}>{options.map((option) => { const Icon = option.icon; return <button key={option.value} className={value === option.value ? "selected" : ""} aria-pressed={value === option.value} onClick={() => onChange(option.value)}><Icon size={14} />{option.label}{value === option.value && <Check size={12} />}</button>; })}</div></div>;
}

function PreferenceRow({ label, detail, children }: { label: string; detail: string; children: React.ReactNode }) { return <div className="preference-row"><div><strong>{label}</strong><small>{detail}</small></div>{children}</div>; }
function PreferenceSegment<T extends string | number>({ value, options, onChange }: { value: T; options: Array<{ value: T; label: string }>; onChange: (value: T) => void }) { return <div className="preference-segment" role="group">{options.map((option) => <button key={String(option.value)} className={value === option.value ? "selected" : ""} aria-pressed={value === option.value} onClick={() => onChange(option.value)}>{option.label}</button>)}</div>; }

function SSHPolicyEditor({ route, onClose, onSave }: { route: RouteSummary; onClose: () => void; onSave: (allowedCommand: string, allowAllCommands: boolean, recordCommands: boolean) => Promise<void> }) {
  const [allowAll, setAllowAll] = useState(route.allowAllCommands);
  const [allowedCommand, setAllowedCommand] = useState(route.allowedCommand || "printf airlock-ok");
  const [recordCommands, setRecordCommands] = useState(route.recordCommands);
  const [saving, setSaving] = useState(false);
  const commandValid = allowAll || (allowedCommand.trim().length > 0 && allowedCommand.length <= 4096 && !/[\r\n\0]/.test(allowedCommand));
  const save = async () => {
    setSaving(true);
    await onSave(allowedCommand, allowAll, recordCommands);
    setSaving(false);
  };
  return <Modal title={`SSH 权限 · ${route.name}`} onClose={saving ? () => undefined : onClose}>
    <div className="policy-editor">
      <div className="policy-options" role="radiogroup" aria-label="SSH 命令范围">
        <button className={!allowAll ? "selected" : ""} role="radio" aria-checked={!allowAll} onClick={() => setAllowAll(false)}><ShieldCheck size={17} /><span><strong>指定命令</strong><small>只开放一个完整 exec 命令</small></span></button>
        <button className={allowAll ? "selected risk" : "risk"} role="radio" aria-checked={allowAll} onClick={() => setAllowAll(true)}><AlertTriangle size={17} /><span><strong>所有命令</strong><small>任意 exec，仍拒绝 Shell 与 PTY</small></span></button>
      </div>
      {!allowAll && <label className="form-field ssh-command-field"><span>唯一允许命令 <small>{allowedCommand.length}/4096</small></span><input value={allowedCommand} onChange={(event) => setAllowedCommand(event.target.value)} maxLength={4096} spellCheck={false} placeholder="例如：uptime" /><small>按完整字符串匹配，不要在命令参数中填写密码或 Token。</small></label>}
      {allowAll && <div className="inline-warning policy-warning"><TriangleAlert size={16} /><span>该模式接近远程命令执行权限。保存时还会出现一次 macOS 原生风险确认。</span></div>}
      <label className="audit-toggle"><input type="checkbox" checked={recordCommands} onChange={(event) => setRecordCommands(event.target.checked)} /><span><strong>记录执行命令</strong><small>完整命令保存在本机 0600 审计文件，参数可能包含敏感内容。</small></span></label>
    </div>
    <div className="modal-actions"><button className="secondary-button" onClick={onClose} disabled={saving}>取消</button><button className={allowAll ? "danger-button" : "primary-button"} onClick={() => void save()} disabled={saving || !commandValid}>{saving ? <RefreshCw className="spin" size={16} /> : <ShieldCheck size={16} />}{saving ? "等待原生确认" : "保存权限"}</button></div>
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

  return <Modal title={`LLM 访问边界 · ${route.name}`} className="modal-wide" onClose={busy ? () => undefined : onClose}>
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

function RouteEditor({ connected, proxyConfigured, sshReady, onClose, onCreated, onError }: { connected: boolean; proxyConfigured: boolean; sshReady: boolean; onClose: () => void; onCreated: (route: RouteSummary) => void; onError: (message: string) => void }) {
  const [step, setStep] = useState(1);
  const [kind, setKind] = useState<RouteKind>("HTTP");
  const [name, setName] = useState("");
  const [alias, setAlias] = useState("");
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState<RouteSummary>();
  const [egress, setEgress] = useState<RouteSummary["egress"]>("Direct");
  const [allowAllCommands, setAllowAllCommands] = useState(false);
  const [allowedCommand, setAllowedCommand] = useState("printf airlock-ok");
  const [recordCommands, setRecordCommands] = useState(true);
  const [llmProvider, setLLMProvider] = useState<"openai" | "anthropic">("openai");
  const [llmModels, setLLMModels] = useState("");
  const [maxOutputTokens, setMaxOutputTokens] = useState(8192);
  const [requestsPerMinute, setRequestsPerMinute] = useState(60);
  const [maxConcurrent, setMaxConcurrent] = useState(4);
  const [trackUsage, setTrackUsage] = useState(false);
  const models = useMemo(() => [...new Set(llmModels.split(",").map((model) => model.trim()).filter(Boolean))], [llmModels]);
  const validSSHCommand = allowAllCommands || (allowedCommand.trim().length > 0 && allowedCommand.length <= 4096 && !/[\r\n\0]/.test(allowedCommand));
  const validIdentity = name.trim().length > 0 && /^[a-z0-9][a-z0-9-]{0,62}$/.test(alias) && (kind !== "SSH" || validSSHCommand) && (kind !== "LLM" || (models.length > 0 && models.length <= 32 && models.every((model) => model.length <= 200) && maxOutputTokens >= 1 && maxOutputTokens <= 1_000_000 && requestsPerMinute >= 1 && requestsPerMinute <= 60_000 && maxConcurrent >= 1 && maxConcurrent <= 1_024));

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape" && !saving) onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose, saving]);

  const secureCreate = async () => {
    setSaving(true);
    try {
      let route: RouteSummary;
      if (isTauri) {
        if (kind === "SSH") route = await invoke<RouteSummary>("create_ssh_route", { name: name.trim(), alias, egress, allowedCommand, allowAllCommands, recordCommands });
        else if (kind === "LLM") route = await invoke<RouteSummary>("create_llm_route", { name: name.trim(), alias, egress, provider: llmProvider, models, maxOutputTokens, requestsPerMinute, maxConcurrent, trackUsage });
        else route = await invoke<RouteSummary>("create_http_route", { name: name.trim(), alias, egress });
      } else {
        route = { id: alias, name: name.trim(), alias, kind, status: "enabled", localEndpoint: kind === "SSH" ? `${alias}@127.0.0.1:4770` : `${kind === "LLM" ? "http://" : ""}127.0.0.1:4768/r/${alias}`, permissionSummary: kind === "SSH" ? `${allowAllCommands ? "all exec commands · high risk" : "1 exact command · stdin denied"}${recordCommands ? " · recorded" : ""}` : kind === "LLM" ? `${llmProvider === "openai" ? "OpenAI" : "Anthropic"} · ${models.length} models · output ≤ ${maxOutputTokens} · ${requestsPerMinute}/min · ${maxConcurrent} concurrent` : "GET, HEAD · Range", egress, health: "healthy", lastUsed: "从未", currentConnections: 0, allowAllCommands: kind === "SSH" && allowAllCommands, recordCommands: kind === "SSH" && recordCommands, allowedCommand: kind === "SSH" && !allowAllCommands ? allowedCommand : undefined, ...(kind === "LLM" ? { provider: llmProvider, allowedModels: models, maxOutputTokens, requestsPerMinute, maxConcurrent, trackUsage, totalRequests: 0, inputTokens: 0, outputTokens: 0 } : {}) };
      }
      setCreated(route);
      onCreated(route);
      setStep(3);
    } catch (error) {
      onError(String(error));
    } finally {
      setSaving(false);
    }
  };

  return <div className="editor-overlay" role="dialog" aria-modal="true" aria-label="新增路由"><div className="editor-panel">
    <header className="editor-header"><div><span className="editor-kicker">SECURE ROUTE</span><h2>新增 {kind} 路由</h2><p>目标、身份与认证由系统安全窗口处理</p></div><button className="icon-button" onClick={onClose} disabled={saving} aria-label="关闭"><X size={18} /></button></header>
    <ol className="step-list">{["本地身份", "安全录入", "完成"].map((label, index) => <li key={label} className={step === index + 1 ? "current" : step > index + 1 ? "done" : ""}><span>{step > index + 1 ? <Check size={14} /> : index + 1}</span>{label}</li>)}</ol>
    <div className="editor-body" key={step}>
      {step === 1 && <>
        <h3>选择入口类型</h3>
        <div className="type-grid"><button className={kind === "HTTP" ? "selected" : ""} onClick={() => setKind("HTTP")}><Route size={19} /><strong>HTTP</strong><span>固定 URL · GET / HEAD</span></button><button className={kind === "SSH" ? "selected" : ""} onClick={() => setKind("SSH")} disabled={!sshReady}><SquareTerminal size={19} /><strong>SSH</strong><span>双会话隔离 · 可控 exec</span></button><button className={kind === "LLM" ? "selected" : ""} onClick={() => setKind("LLM")}><Sparkles size={19} /><strong>LLM</strong><span>模型白名单 · Key 隔离</span></button></div>
        <div className="identity-grid"><label className="form-field"><span>名称</span><input value={name} onChange={(event) => setName(event.target.value)} maxLength={80} placeholder={kind === "SSH" ? "Release builder" : kind === "LLM" ? "Coding assistant" : "Release downloads"} autoFocus /></label><label className="form-field"><span>本地别名</span><input value={alias} onChange={(event) => setAlias(event.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))} maxLength={63} placeholder={kind === "SSH" ? "build" : kind === "LLM" ? "coding" : "release-downloads"} /></label></div>
        {kind === "LLM" && <div className="llm-create-policy"><div className="llm-policy-grid"><div className="llm-provider"><span>协议预设</span><div className="policy-segmented"><button className={llmProvider === "openai" ? "selected" : ""} onClick={() => setLLMProvider("openai")}><Sparkles size={13} />OpenAI</button><button className={llmProvider === "anthropic" ? "selected" : ""} onClick={() => setLLMProvider("anthropic")}><ShieldCheck size={13} />Anthropic</button></div></div><label className="form-field"><span>允许模型 <small>逗号分隔 · {models.length}/32</small></span><input value={llmModels} onChange={(event) => setLLMModels(event.target.value)} maxLength={2048} placeholder={llmProvider === "openai" ? "model-a, model-b" : "claude-model-a"} /></label></div><div className="llm-limit-grid"><label className="form-field"><span>最大输出 Token</span><input type="number" min={1} max={1_000_000} step={256} value={maxOutputTokens} onChange={(event) => setMaxOutputTokens(Number(event.target.value))} /></label><label className="form-field"><span>每分钟请求</span><input type="number" min={1} max={60_000} value={requestsPerMinute} onChange={(event) => setRequestsPerMinute(Number(event.target.value))} /></label><label className="form-field"><span>最大并发</span><input type="number" min={1} max={1_024} value={maxConcurrent} onChange={(event) => setMaxConcurrent(Number(event.target.value))} /></label></div><label className="compact-check usage-opt-in"><input type="checkbox" checked={trackUsage} onChange={(event) => setTrackUsage(event.target.checked)} /><span><strong>统计调用与 Token</strong><small>默认关闭，不记录提示词或响应正文</small></span></label></div>}
        {kind === "SSH" && <div className="route-policy-strip"><span>命令权限</span><div className="policy-segmented"><button className={!allowAllCommands ? "selected" : ""} onClick={() => setAllowAllCommands(false)}><ShieldCheck size={14} />指定命令</button><button className={allowAllCommands ? "selected risk" : "risk"} onClick={() => setAllowAllCommands(true)}><AlertTriangle size={14} />所有命令</button></div><label className="compact-check"><input type="checkbox" checked={recordCommands} onChange={(event) => setRecordCommands(event.target.checked)} />记录命令</label></div>}
        {kind === "SSH" && !allowAllCommands && <label className="form-field ssh-command-field"><span>唯一允许命令 <small>{allowedCommand.length}/4096</small></span><input value={allowedCommand} onChange={(event) => setAllowedCommand(event.target.value)} maxLength={4096} spellCheck={false} placeholder="例如：uptime" /><small>按完整字符串匹配，不要在命令参数中填写密码或 Token。</small></label>}
        {kind === "SSH" && allowAllCommands && <div className="inline-warning"><TriangleAlert size={15} />所有命令是高风险能力，保存前需要原生二次确认。</div>}
        <div className="egress-field"><span>出口策略</span><div className="egress-control" role="group" aria-label="出口策略">{([{ value: "Direct", label: "直连", icon: Cable }, { value: "Proxy", label: "代理", icon: Network }, { value: "Auto", label: "自动", icon: GitBranch }] as const).map((option) => { const Icon = option.icon; return <button key={option.value} className={egress === option.value ? "selected" : ""} onClick={() => setEgress(option.value)} aria-pressed={egress === option.value}><Icon size={14} />{option.label}</button>; })}</div></div>
        {egress !== "Direct" && !proxyConfigured && <div className="inline-warning"><TriangleAlert size={15} />代理出口尚未在设置中安全配置。</div>}
      </>}
	  {step === 2 && <><h3>完成受保护配置</h3><div className="protected-box"><span className="protected-icon">{kind === "SSH" ? <SquareTerminal size={21} /> : kind === "LLM" ? <Sparkles size={21} /> : <KeyRound size={21} />}</span><div><strong>macOS 原生引导</strong><p>{kind === "SSH" ? "按步骤填写上游账号与密码，可自定义完全隔离的本地登录密码，随后核对 Host Key。" : kind === "LLM" ? "录入上游 Base URL 与 API Key，然后自定义或随机生成完全隔离的本地 API Key。" : "完整 URL 与 Authorization 按当前凭据保护方式存储。"}</p></div><button className="primary-button" onClick={() => void secureCreate()} disabled={!connected || saving}>{saving ? <><RefreshCw className="spin" size={16} />等待系统窗口</> : <><KeyRound size={16} />开始安全设置</>}</button></div>{!connected && <div className="inline-error"><TriangleAlert size={16} />airlockd 未连接，暂时无法保存。</div>}</>}
	  {step === 3 && created && <div className="success-state"><CircleCheck size={32} /><h3>路由已启用</h3><p>{kind === "LLM" ? "Base URL 与本地 API Key 已在原生窗口中确认。" : "本地登录信息已在原生窗口中确认。"}</p><code>{created.localEndpoint}</code></div>}
    </div>
    <footer className="editor-footer"><button className="secondary-button" onClick={step === 1 || step === 3 ? onClose : () => setStep(1)} disabled={saving}>{step === 2 && <ChevronLeft size={16} />}{step === 3 ? "完成" : step === 1 ? "取消" : "上一步"}</button>{step === 1 && <button className="primary-button" onClick={() => setStep(2)} disabled={!validIdentity}>继续<ChevronRight size={16} /></button>}</footer>
  </div></div>;
}

function DeveloperCard() {
  const [avatarFailed, setAvatarFailed] = useState(false);
  return <section className="settings-section about-section"><div><h2>关于</h2><p>Airlock 的开发者信息</p></div><div className="developer-card">
    <div className={`developer-avatar ${avatarFailed ? "fallback" : ""}`}>{avatarFailed ? <span>LH</span> : <img src="/louisonh.png" alt="LouisonH" onError={() => setAvatarFailed(true)} />}</div>
    <div className="developer-copy"><span className="developer-label">Developer</span><strong>LouisonH</strong><p>产品设计与核心开发</p></div>
    <div className="developer-meta"><a href="https://github.com/LouisonH" target="_blank" rel="noreferrer"><Github size={14} />github.com/LouisonH</a><a href="https://0o0.site" target="_blank" rel="noreferrer"><Globe2 size={14} />0o0.site</a><span><Sparkles size={14} />AI 协作 · GPT-5.6 Sol</span><span>v0.1.0 · Technical Preview</span></div>
  </div></section>;
}

function EmptyState({ icon: Icon, title, detail }: { icon: typeof Route; title: string; detail: string }) { return <div className="empty-state"><Icon size={22} /><strong>{title}</strong><span>{detail}</span></div>; }
function PageHeader({ title, subtitle, action }: { title: string; subtitle: string; action?: React.ReactNode }) { return <div className="page-header"><div><h1>{title}</h1><p>{subtitle}</p></div>{action}</div>; }
function Metric({ label, value, detail, tone }: { label: string; value: string; detail: string; tone?: string }) { return <div className="metric"><span>{label}</span><strong className={tone}>{value}</strong><small>{detail}</small></div>; }
function StatusBadge({ status }: { status: string }) { const labels: Record<string, string> = { enabled: "已启用", disabled: "已停用", blocked: "已阻止", allowed: "已允许", failed: "失败" }; const Icon = status === "enabled" || status === "allowed" ? CircleCheck : status === "disabled" ? CircleMinus : TriangleAlert; return <span className={`status-badge status-${status}`}><Icon size={13} />{labels[status] ?? status}</span>; }
function HealthBadge({ health }: { health: RouteSummary["health"] }) { const labels = { healthy: "健康", degraded: "异常", unknown: "未测试" }; const Icon = health === "healthy" ? CircleCheck : health === "degraded" ? TriangleAlert : CircleMinus; return <span className={`health health-${health}`}><Icon size={13} />{labels[health]}</span>; }
function KindBadge({ kind }: { kind: RouteKind }) { const Icon = kind === "HTTP" ? Server : kind === "SSH" ? SquareTerminal : Sparkles; return <span className={`kind kind-${kind.toLowerCase()}`}><Icon size={13} />{kind}</span>; }
function PermissionSummary({ route }: { route: RouteSummary }) { if (route.kind !== "LLM") return <span className={route.allowAllCommands ? "permission-risk" : "permission-copy"}>{route.permissionSummary}</span>; return <div className="llm-policy-summary"><strong>{route.provider === "anthropic" ? "Anthropic" : "OpenAI"} · output ≤ {route.maxOutputTokens ?? "-"}</strong><span><b>{route.allowedModels?.length ?? 0}</b> models</span><span><b>{route.requestsPerMinute ?? 0}</b>/min</span><span><b>{route.maxConcurrent ?? 0}</b> concurrent</span>{route.trackUsage && <span className="usage-chip"><b>{formatUsage(route.totalRequests ?? 0)}</b> calls</span>}</div>; }
function formatUsage(value: number) { return new Intl.NumberFormat("zh-CN", { notation: value >= 100_000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(value); }
function ReadOnlyField({ label, value, tone }: { label: string; value: string; tone?: "success" | "warning" }) { return <div className="readonly-field"><span>{label}</span><strong className={tone ? `setting-value ${tone}` : "setting-value"}>{value}</strong></div>; }
function ThemeControl({ value, onChange }: { value: ThemePreference; onChange: (value: ThemePreference) => void }) { const options: Array<{ value: ThemePreference; label: string; icon: typeof Monitor }> = [{ value: "system", label: "系统", icon: Monitor }, { value: "light", label: "浅色", icon: Sun }, { value: "dark", label: "深色", icon: Moon }]; return <div className="theme-control" role="group" aria-label="界面主题">{options.map((option) => { const Icon = option.icon; return <button key={option.value} className={value === option.value ? "selected" : ""} onClick={() => onChange(option.value)} aria-pressed={value === option.value}><Icon size={14} />{option.label}</button>; })}</div>; }
function AccentControl({ value, onChange }: { value: AccentTheme; onChange: (value: AccentTheme) => void }) { const options: Array<{ value: AccentTheme; label: string }> = [{ value: "forest", label: "青峦" }, { value: "ocean", label: "海岸" }, { value: "amber", label: "暖阳" }]; return <div className="accent-control" role="group" aria-label="配色风格">{options.map((option) => <button key={option.value} className={value === option.value ? "selected" : ""} onClick={() => onChange(option.value)} aria-pressed={value === option.value}><span className={`accent-swatch ${option.value}`} />{option.label}</button>)}</div>; }
function Modal({ title, onClose, children, className = "" }: { title: string; onClose: () => void; children: React.ReactNode; className?: string }) { return <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><div className={`modal ${className}`} role="dialog" aria-modal="true"><header className="modal-header"><h2>{title}</h2><button className="icon-button small" onClick={onClose} aria-label="关闭"><X size={16} /></button></header>{children}</div></div>; }

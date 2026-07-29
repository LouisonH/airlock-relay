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
  KeyRound,
  LockKeyhole,
  Monitor,
  Moon,
  Network,
  Plus,
  RefreshCw,
  Route,
  Search,
  Settings,
  ShieldCheck,
  Sparkles,
  Sun,
  Trash2,
  TriangleAlert,
  X,
} from "lucide-react";
import { applyTheme, getAccentTheme, getThemePreference, saveAccentTheme, saveThemePreference, watchSystemTheme, type AccentTheme, type ThemePreference } from "./theme";
import type { ActivityEvent, ControlState, ControlUpdate, RouteKind, RouteSummary } from "./types";

type Page = "overview" | "routes" | "activity" | "settings";
type RouteFilter = "All" | RouteKind;

const isTauri = "__TAURI_INTERNALS__" in window;
const previewRoutes: RouteSummary[] = isTauri ? [] : [
  { id: "release", name: "Release downloads", alias: "release", kind: "HTTP", status: "enabled", localEndpoint: "127.0.0.1:4768/r/release", permissionSummary: "GET, HEAD · Range", egress: "Direct", health: "healthy", lastUsed: "2 分钟前", currentConnections: 1 },
  { id: "models", name: "Model gateway", alias: "models", kind: "LLM", status: "disabled", localEndpoint: "127.0.0.1:4768/llm/models", permissionSummary: "Responses · 3 models", egress: "Proxy", health: "unknown", lastUsed: "从未", currentConnections: 0 },
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
  message: isTauri ? "正在连接 airlockd" : undefined,
};

export default function App() {
  const [page, setPage] = useState<Page>("overview");
  const [control, setControl] = useState<ControlState>(emptyControl);
  const [emergencyOpen, setEmergencyOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [theme, setTheme] = useState<ThemePreference>(getThemePreference);
  const [accent, setAccent] = useState<AccentTheme>(getAccentTheme);
  const [notice, setNotice] = useState<string>();
  const [pendingDelete, setPendingDelete] = useState<RouteSummary>();

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

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), 5000);
    return () => window.clearInterval(timer);
  }, []);

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

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark"><LockKeyhole size={16} /></span><span>Airlock</span></div>
        <nav aria-label="主导航">
          {navItems.map((item) => {
            const Icon = item.icon;
            return <button key={item.id} className={`nav-item ${page === item.id ? "active" : ""}`} onClick={() => setPage(item.id)}><Icon size={17} /><span>{item.label}</span></button>;
          })}
        </nav>
        <div className="daemon-summary">
          <span className={`status-dot ${control.connected ? "online" : "offline"}`} />
          <div><strong>{control.connected ? "本地核心已连接" : "等待本地核心"}</strong><span>airlockd · loopback only</span></div>
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
          {page === "routes" && <Routes routes={control.routes} connected={control.connected} onToggle={toggleRoute} onDelete={setPendingDelete} onAdd={() => setEditorOpen(true)} />}
          {page === "activity" && <ActivityPage />}
          {page === "settings" && <SettingsPage theme={theme} onTheme={setTheme} accent={accent} onAccent={setAccent} connected={control.connected} proxyConfigured={control.proxyConfigured} onConfigureProxy={configureProxy} onClearProxy={clearProxy} />}
        </div>
      </main>

      {notice && <div className="toast" role="status">{notice}</div>}
      {emergencyOpen && <Modal title="停止全部路由" onClose={() => setEmergencyOpen(false)}><div className="warning-panel"><AlertTriangle size={19} /><p>新请求将立即被拒绝，已建立的连接会进入关闭流程。</p></div><div className="modal-actions"><button className="secondary-button" onClick={() => setEmergencyOpen(false)}>取消</button><button className="danger-button" onClick={() => void stopAll()}><CircleStop size={16} />确认停止</button></div></Modal>}
      {pendingDelete && <Modal title="删除路由" onClose={() => setPendingDelete(undefined)}><div className="danger-panel"><Trash2 size={19} /><div><strong>{pendingDelete.name}</strong><p>本地入口、Capability 和 Keychain 中的受保护目标都会被永久删除。</p></div></div><div className="modal-actions"><button className="secondary-button" onClick={() => setPendingDelete(undefined)}>取消</button><button className="danger-button" onClick={() => void deleteRoute()}><Trash2 size={16} />删除路由</button></div></Modal>}
      {editorOpen && <RouteEditor connected={control.connected} proxyConfigured={control.proxyConfigured} onClose={() => setEditorOpen(false)} onCreated={(route) => setControl((current) => ({ ...current, routes: [...current.routes.filter((item) => item.id !== route.id), route] }))} onError={setNotice} />}
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
    </section>
    <section className="metric-strip" aria-label="运行指标">
      <Metric label="开放路由" value={String(enabled)} detail={`共 ${control.routes.length} 条`} />
      <Metric label="当前连接" value={String(connections)} detail="仅统计本地入口" />
      <Metric label="凭据存储" value="Keychain" detail="目标与认证均受保护" tone="success" />
    </section>
    <section className="section-block">
      <div className="section-heading"><div><h2>路由</h2><p>界面只显示安全别名和本地入口。</p></div><button className="text-button" onClick={onRoutes}>查看全部<ChevronRight size={15} /></button></div>
      <RouteTable routes={control.routes.slice(0, 5)} compact />
    </section>
  </>;
}

function Routes({ routes, connected, onToggle, onDelete, onAdd }: { routes: RouteSummary[]; connected: boolean; onToggle: (alias: string, enabled: boolean) => void; onDelete: (route: RouteSummary) => void; onAdd: () => void }) {
  const [filter, setFilter] = useState<RouteFilter>("All");
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => routes.filter((route) => (filter === "All" || route.kind === filter) && `${route.name} ${route.alias}`.toLowerCase().includes(query.toLowerCase())), [routes, filter, query]);
  return <>
    <PageHeader title="路由" subtitle={`${routes.length} 条 · ${routes.filter((route) => route.status === "enabled").length} 条已开放`} action={<button className="primary-button" onClick={onAdd} disabled={!connected}><Plus size={16} />新增路由</button>} />
    <div className="filter-bar">
      <label className="search-field"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索名称或别名" /></label>
      <div className="segmented" aria-label="路由类型">{(["All", "HTTP", "SSH", "LLM"] as RouteFilter[]).map((value) => <button key={value} className={filter === value ? "selected" : ""} onClick={() => setFilter(value)}>{value === "All" ? "全部" : value}</button>)}</div>
    </div>
    <RouteTable routes={filtered} onToggle={(route) => onToggle(route.alias, route.status !== "enabled")} onDelete={onDelete} />
  </>;
}

function RouteTable({ routes, compact = false, onToggle, onDelete }: { routes: RouteSummary[]; compact?: boolean; onToggle?: (route: RouteSummary) => void; onDelete?: (route: RouteSummary) => void }) {
  if (routes.length === 0) return <EmptyState icon={Route} title="暂无路由" detail="创建后，本地入口会显示在这里。" />;
  return <div className="table-wrap"><table className="route-table"><thead><tr><th>状态</th><th>名称</th><th>类型</th><th>本地入口</th><th>权限</th><th>出口</th><th>健康</th>{!compact && <th>最近使用</th>}<th aria-label="操作" /></tr></thead><tbody>{routes.map((route, index) => <tr key={route.id} style={{ animationDelay: `${index * 24}ms` }}><td><StatusBadge status={route.status} /></td><td><strong>{route.name}</strong><span className="cell-subtext">{route.alias}</span></td><td><span className={`kind kind-${route.kind.toLowerCase()}`}>{route.kind}</span></td><td><code>{route.localEndpoint}</code></td><td>{route.permissionSummary}</td><td>{route.egress}</td><td><HealthBadge health={route.health} /></td>{!compact && <td>{route.lastUsed}</td>}<td>{onToggle && <div className="route-actions"><button className="route-switch" role="switch" aria-checked={route.status === "enabled"} title={route.status === "enabled" ? "停用路由" : "启用路由"} aria-label={route.status === "enabled" ? "停用路由" : "启用路由"} onClick={() => onToggle(route)}><span /></button>{onDelete && <button className="row-icon-button danger" title="删除路由" aria-label={`删除 ${route.name}`} onClick={() => onDelete(route)}><Trash2 size={14} /></button>}</div>}</td></tr>)}</tbody></table></div>;
}

function ActivityPage() {
  const events: ActivityEvent[] = [];
  return <><PageHeader title="活动" subtitle="脱敏审计，不记录正文、命令或真实目标" /><ActivityTable events={events} /></>;
}

function ActivityTable({ events }: { events: ActivityEvent[] }) {
  if (events.length === 0) return <EmptyState icon={Activity} title="暂无活动" detail="新的脱敏事件会显示在这里。" />;
  return <div className="table-wrap"><table className="activity-table"><thead><tr><th>时间</th><th>路由</th><th>调用者</th><th>动作</th><th>结果</th><th>延迟</th><th>出口</th><th>事件 ID</th></tr></thead><tbody>{events.map((event) => <tr key={event.id}><td>{event.time}</td><td><strong>{event.routeName}</strong></td><td>{event.caller}</td><td>{event.action}</td><td><StatusBadge status={event.result} /></td><td>{event.latency}</td><td>{event.egress}</td><td><code>{event.id}</code></td></tr>)}</tbody></table></div>;
}

function SettingsPage({ theme, onTheme, accent, onAccent, connected, proxyConfigured, onConfigureProxy, onClearProxy }: { theme: ThemePreference; onTheme: (theme: ThemePreference) => void; accent: AccentTheme; onAccent: (accent: AccentTheme) => void; connected: boolean; proxyConfigured: boolean; onConfigureProxy: () => void; onClearProxy: () => void }) {
  return <><PageHeader title="设置" subtitle="本地外观、网络与安全状态" />
    <section className="settings-section"><div><h2>外观</h2><p>主题偏好保存在本机</p></div><div className="settings-controls"><div className="setting-row"><span>显示模式</span><ThemeControl value={theme} onChange={onTheme} /></div><div className="setting-row"><span>配色风格</span><AccentControl value={accent} onChange={onAccent} /></div></div></section>
    <section className="settings-section"><div><h2>网络</h2><p>入口固定在 loopback</p></div><div className="settings-controls"><ReadOnlyField label="HTTP 入口" value="127.0.0.1:4768" /><ReadOnlyField label="控制通道" value={connected ? "Unix Socket · 已连接" : "等待 airlockd"} tone={connected ? "success" : "warning"} /><div className="proxy-setting"><div><span>Clash / SOCKS5 出口</span><strong className={proxyConfigured ? "setting-value success" : "setting-value"}>{proxyConfigured ? "Keychain · 已配置" : "未配置"}</strong></div><div className="inline-actions"><button className="secondary-button compact" onClick={onConfigureProxy} disabled={!connected}><Network size={14} />{proxyConfigured ? "更换" : "配置"}</button>{proxyConfigured && <button className="row-icon-button danger visible" onClick={onClearProxy} aria-label="清除代理出口" title="清除代理出口"><Trash2 size={14} /></button>}</div></div></div></section>
    <section className="settings-section"><div><h2>安全</h2><p>Secret 不进入 WebView</p></div><div className="settings-controls"><ReadOnlyField label="SecretStore" value="macOS Keychain" tone="success" /><ReadOnlyField label="路由元数据" value="0600 · 已持久化" tone="success" /><ReadOnlyField label="SSH 安全核心" value="E2E 已验证 · 待接入" tone="warning" /><ReadOnlyField label="安全录入" value="macOS 原生窗口" tone="success" /></div></section>
    <DeveloperCard />
  </>;
}

function RouteEditor({ connected, proxyConfigured, onClose, onCreated, onError }: { connected: boolean; proxyConfigured: boolean; onClose: () => void; onCreated: (route: RouteSummary) => void; onError: (message: string) => void }) {
  const [step, setStep] = useState(1);
  const [name, setName] = useState("");
  const [alias, setAlias] = useState("");
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState<RouteSummary>();
  const [egress, setEgress] = useState<RouteSummary["egress"]>("Direct");
  const validIdentity = name.trim().length > 0 && /^[a-z0-9][a-z0-9-]{0,62}$/.test(alias);

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape" && !saving) onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose, saving]);

  const secureCreate = async () => {
    setSaving(true);
    try {
      const route = isTauri
        ? await invoke<RouteSummary>("create_http_route", { name: name.trim(), alias, egress })
        : { id: alias, name: name.trim(), alias, kind: "HTTP" as const, status: "enabled" as const, localEndpoint: `127.0.0.1:4768/r/${alias}`, permissionSummary: "GET, HEAD · Range", egress, health: "unknown" as const, lastUsed: "从未", currentConnections: 0 };
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
    <header className="editor-header"><div><h2>新增 HTTP 路由</h2><p>目标与认证由系统安全窗口处理</p></div><button className="icon-button" onClick={onClose} disabled={saving} aria-label="关闭"><X size={18} /></button></header>
    <ol className="step-list">{["本地身份", "安全录入", "完成"].map((label, index) => <li key={label} className={step === index + 1 ? "current" : step > index + 1 ? "done" : ""}><span>{step > index + 1 ? <Check size={14} /> : index + 1}</span>{label}</li>)}</ol>
    <div className="editor-body" key={step}>
      {step === 1 && <><h3>路由身份</h3><div className="type-grid"><button className="selected"><Route size={18} /><strong>HTTP</strong><span>固定 URL · GET / HEAD</span></button><button disabled><KeyRound size={18} /><strong>SSH</strong><span>核心已验证 · 暂未开放</span></button><button disabled><ShieldCheck size={18} /><strong>LLM</strong><span>下一阶段</span></button></div><label className="form-field"><span>名称</span><input value={name} onChange={(event) => setName(event.target.value)} maxLength={80} placeholder="Release downloads" autoFocus /></label><label className="form-field"><span>本地别名</span><input value={alias} onChange={(event) => setAlias(event.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))} maxLength={63} placeholder="release-downloads" /></label><div className="egress-field"><span>出口策略</span><div className="egress-control" role="group" aria-label="出口策略">{([{ value: "Direct", label: "直连", icon: Cable }, { value: "Proxy", label: "代理", icon: Network }, { value: "Auto", label: "自动", icon: GitBranch }] as const).map((option) => { const Icon = option.icon; return <button key={option.value} className={egress === option.value ? "selected" : ""} onClick={() => setEgress(option.value)} aria-pressed={egress === option.value}><Icon size={14} />{option.label}</button>; })}</div></div>{egress !== "Direct" && !proxyConfigured && <div className="inline-warning"><TriangleAlert size={15} />代理出口尚未在设置中安全配置。</div>}</>}
      {step === 2 && <><h3>受保护目标</h3><div className="protected-box"><span className="protected-icon"><KeyRound size={20} /></span><div><strong>macOS 安全录入</strong><p>完整 URL 与 Authorization 直接写入 Keychain。</p></div><button className="primary-button" onClick={() => void secureCreate()} disabled={!connected || saving}>{saving ? <><RefreshCw className="spin" size={16} />等待系统窗口</> : <><KeyRound size={16} />打开安全窗口</>}</button></div>{!connected && <div className="inline-error"><TriangleAlert size={16} />airlockd 未连接，暂时无法保存。</div>}</>}
      {step === 3 && created && <div className="success-state"><CircleCheck size={32} /><h3>路由已启用</h3><p>Capability 已在原生窗口中一次性显示。</p><code>{created.localEndpoint}</code></div>}
    </div>
    <footer className="editor-footer"><button className="secondary-button" onClick={step === 1 || step === 3 ? onClose : () => setStep(1)} disabled={saving}>{step === 2 && <ChevronLeft size={16} />}{step === 3 ? "完成" : step === 1 ? "取消" : "上一步"}</button>{step === 1 && <button className="primary-button" onClick={() => setStep(2)} disabled={!validIdentity}>继续<ChevronRight size={16} /></button>}</footer>
  </div></div>;
}

function DeveloperCard() {
  const [avatarFailed, setAvatarFailed] = useState(false);
  return <section className="settings-section about-section"><div><h2>关于</h2><p>Airlock 的开发者信息</p></div><div className="developer-card">
    <div className={`developer-avatar ${avatarFailed ? "fallback" : ""}`}>{avatarFailed ? <span>LH</span> : <img src="/louisonh.png" alt="LouisonH" onError={() => setAvatarFailed(true)} />}</div>
    <div className="developer-copy"><span className="developer-label">Developer</span><strong>LouisonH</strong><p>产品设计与核心开发</p></div>
    <div className="developer-meta"><span><Github size={14} />@LouisonH</span><span><Sparkles size={14} />AI 协作 · GPT-5.6 Sol</span></div>
  </div></section>;
}

function EmptyState({ icon: Icon, title, detail }: { icon: typeof Route; title: string; detail: string }) { return <div className="empty-state"><Icon size={22} /><strong>{title}</strong><span>{detail}</span></div>; }
function PageHeader({ title, subtitle, action }: { title: string; subtitle: string; action?: React.ReactNode }) { return <div className="page-header"><div><h1>{title}</h1><p>{subtitle}</p></div>{action}</div>; }
function Metric({ label, value, detail, tone }: { label: string; value: string; detail: string; tone?: string }) { return <div className="metric"><span>{label}</span><strong className={tone}>{value}</strong><small>{detail}</small></div>; }
function StatusBadge({ status }: { status: string }) { const labels: Record<string, string> = { enabled: "已启用", disabled: "已停用", blocked: "已阻止", allowed: "已允许", failed: "失败" }; const Icon = status === "enabled" || status === "allowed" ? CircleCheck : status === "disabled" ? CircleMinus : TriangleAlert; return <span className={`status-badge status-${status}`}><Icon size={13} />{labels[status] ?? status}</span>; }
function HealthBadge({ health }: { health: RouteSummary["health"] }) { const labels = { healthy: "健康", degraded: "异常", unknown: "未测试" }; const Icon = health === "healthy" ? CircleCheck : health === "degraded" ? TriangleAlert : CircleMinus; return <span className={`health health-${health}`}><Icon size={13} />{labels[health]}</span>; }
function ReadOnlyField({ label, value, tone }: { label: string; value: string; tone?: "success" | "warning" }) { return <div className="readonly-field"><span>{label}</span><strong className={tone ? `setting-value ${tone}` : "setting-value"}>{value}</strong></div>; }
function ThemeControl({ value, onChange }: { value: ThemePreference; onChange: (value: ThemePreference) => void }) { const options: Array<{ value: ThemePreference; label: string; icon: typeof Monitor }> = [{ value: "system", label: "系统", icon: Monitor }, { value: "light", label: "浅色", icon: Sun }, { value: "dark", label: "深色", icon: Moon }]; return <div className="theme-control" role="group" aria-label="界面主题">{options.map((option) => { const Icon = option.icon; return <button key={option.value} className={value === option.value ? "selected" : ""} onClick={() => onChange(option.value)} aria-pressed={value === option.value}><Icon size={14} />{option.label}</button>; })}</div>; }
function AccentControl({ value, onChange }: { value: AccentTheme; onChange: (value: AccentTheme) => void }) { const options: Array<{ value: AccentTheme; label: string }> = [{ value: "forest", label: "青峦" }, { value: "ocean", label: "海岸" }, { value: "amber", label: "暖阳" }]; return <div className="accent-control" role="group" aria-label="配色风格">{options.map((option) => <button key={option.value} className={value === option.value ? "selected" : ""} onClick={() => onChange(option.value)} aria-pressed={value === option.value}><span className={`accent-swatch ${option.value}`} />{option.label}</button>)}</div>; }
function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) { return <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><div className="modal" role="dialog" aria-modal="true"><header className="modal-header"><h2>{title}</h2><button className="icon-button small" onClick={onClose} aria-label="关闭"><X size={16} /></button></header>{children}</div></div>; }

import { useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  Check,
  ChevronLeft,
  ChevronRight,
  CircleStop,
  Gauge,
  KeyRound,
  LockKeyhole,
  Network,
  Plus,
  Power,
  Route,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  X,
} from "lucide-react";
import type { ActivityEvent, RouteKind, RouteSummary } from "./types";

type Page = "overview" | "routes" | "activity" | "settings";
type RouteFilter = "All" | RouteKind;

const initialRoutes: RouteSummary[] = [
  {
    id: "manual",
    name: "Release downloads",
    alias: "manual",
    kind: "HTTP",
    status: "enabled",
    localEndpoint: "127.0.0.1:4768/r/manual",
    permissionSummary: "GET, HEAD · Range",
    egress: "Auto",
    health: "healthy",
    lastUsed: "2 分钟前",
    currentConnections: 1,
  },
  {
    id: "coding",
    name: "Coding model",
    alias: "coding",
    kind: "LLM",
    status: "enabled",
    localEndpoint: "127.0.0.1:4768/llm/coding",
    permissionSummary: "Responses · 3 models",
    egress: "Proxy",
    health: "healthy",
    lastUsed: "8 分钟前",
    currentConnections: 0,
  },
  {
    id: "build",
    name: "Build executor",
    alias: "build",
    kind: "SSH",
    status: "disabled",
    localEndpoint: "127.0.0.1:4769 · build",
    permissionSummary: "Forced command · 10 min",
    egress: "Direct",
    health: "unknown",
    lastUsed: "从未",
    currentConnections: 0,
  },
];

const activityEvents: ActivityEvent[] = [
  { id: "ALK-HTTP-2041", time: "10:42:18", routeName: "Release downloads", caller: "agent:build-7", action: "Range download", result: "allowed", latency: "182 ms", egress: "Direct" },
  { id: "ALK-LLM-2039", time: "10:36:02", routeName: "Coding model", caller: "agent:codex", action: "POST /v1/responses", result: "allowed", latency: "1.8 s", egress: "Proxy" },
  { id: "ALK-SSH-2034", time: "10:28:44", routeName: "Build executor", caller: "agent:runner", action: "Exec request", result: "blocked", latency: "4 ms", egress: "Direct" },
];

const navItems: Array<{ id: Page; label: string; icon: typeof Gauge }> = [
  { id: "overview", label: "概览", icon: Gauge },
  { id: "routes", label: "路由", icon: Route },
  { id: "activity", label: "活动", icon: Activity },
  { id: "settings", label: "设置", icon: Settings },
];

export default function App() {
  const [page, setPage] = useState<Page>("overview");
  const [serviceRunning, setServiceRunning] = useState(true);
  const [routes, setRoutes] = useState(initialRoutes);
  const [emergencyOpen, setEmergencyOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);

  const enabledCount = routes.filter((route) => route.status === "enabled").length;

  const stopAll = () => {
    setRoutes((current) => current.map((route) => ({ ...route, status: "disabled", currentConnections: 0 })));
    setServiceRunning(false);
    setEmergencyOpen(false);
  };

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><div className="brand-mark"><LockKeyhole size={18} /></div><span>Airlock</span></div>
        <nav aria-label="主导航">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <button key={item.id} className={`nav-item ${page === item.id ? "active" : ""}`} onClick={() => setPage(item.id)}>
                <Icon size={17} /><span>{item.label}</span>
              </button>
            );
          })}
        </nav>
        <div className="daemon-summary">
          <span className={`status-dot ${serviceRunning ? "online" : "offline"}`} />
          <div><strong>{serviceRunning ? "服务运行中" : "服务已停止"}</strong><span>airlockd · v0.1.0</span></div>
        </div>
      </aside>

      <main className="workspace">
        <header className="toolbar">
          <div className="service-state"><ShieldCheck size={16} /><span>{enabledCount} 条能力已开放</span></div>
          <div className="toolbar-actions">
            <button className="icon-button" title="网络与出口状态" aria-label="网络与出口状态"><Network size={17} /></button>
            <button className="danger-button" onClick={() => setEmergencyOpen(true)} disabled={!serviceRunning}>
              <CircleStop size={16} />停止全部路由
            </button>
          </div>
        </header>

        <div className="page-content">
          {page === "overview" && <Overview serviceRunning={serviceRunning} routes={routes} onStart={() => setServiceRunning(true)} onRoutes={() => setPage("routes")} />}
          {page === "routes" && <Routes routes={routes} setRoutes={setRoutes} onAdd={() => setEditorOpen(true)} />}
          {page === "activity" && <ActivityPage />}
          {page === "settings" && <SettingsPage />}
        </div>
      </main>

      {emergencyOpen && (
        <Modal title="停止全部路由" onClose={() => setEmergencyOpen(false)}>
          <div className="warning-panel"><AlertTriangle size={20} /><p>所有本地入口会立即停止，新请求将被拒绝，现有连接将被关闭。</p></div>
          <div className="modal-actions"><button className="secondary-button" onClick={() => setEmergencyOpen(false)}>取消</button><button className="danger-button" onClick={stopAll}><CircleStop size={16} />确认停止</button></div>
        </Modal>
      )}
      {editorOpen && <RouteEditor onClose={() => setEditorOpen(false)} />}
    </div>
  );
}

function Overview({ serviceRunning, routes, onStart, onRoutes }: { serviceRunning: boolean; routes: RouteSummary[]; onStart: () => void; onRoutes: () => void }) {
  const enabled = routes.filter((route) => route.status === "enabled").length;
  const connections = routes.reduce((sum, route) => sum + route.currentConnections, 0);
  return (
    <>
      <PageHeader title="概览" subtitle="本地转发核心与开放能力的实时状态" />
      <section className={`service-band ${serviceRunning ? "running" : "stopped"}`}>
        <div className="service-icon"><Power size={20} /></div>
        <div className="service-copy"><strong>{serviceRunning ? "Airlock 正在保护本地入口" : "Airlock 转发服务已停止"}</strong><span>{serviceRunning ? "管理通道受保护，真实目标与凭据未暴露给调用者。" : "所有路由均不可访问。"}</span></div>
        {!serviceRunning && <button className="primary-button" onClick={onStart}><Power size={16} />启动服务</button>}
      </section>
      <section className="metric-strip" aria-label="运行指标">
        <Metric label="已启用路由" value={String(enabled)} detail={`共 ${routes.length} 条`} />
        <Metric label="当前连接" value={String(connections)} detail="并发限制正常" />
        <Metric label="待人工确认" value="0" detail="无高风险请求" />
        <Metric label="代理出口" value="待接入" detail="已设定 127.0.0.1:7890" />
      </section>
      <section className="section-block">
        <div className="section-heading"><div><h2>路由健康</h2><p>这里只显示安全别名和本地入口。</p></div><button className="text-button" onClick={onRoutes}>查看全部<ChevronRight size={15} /></button></div>
        <RouteTable routes={routes} compact />
      </section>
      <section className="section-block">
        <div className="section-heading"><div><h2>最近活动</h2><p>请求正文、命令和真实目标不会被记录。</p></div></div>
        <ActivityTable events={activityEvents.slice(0, 3)} />
      </section>
    </>
  );
}

function Routes({ routes, setRoutes, onAdd }: { routes: RouteSummary[]; setRoutes: React.Dispatch<React.SetStateAction<RouteSummary[]>>; onAdd: () => void }) {
  const [filter, setFilter] = useState<RouteFilter>("All");
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => routes.filter((route) => (filter === "All" || route.kind === filter) && `${route.name} ${route.alias}`.toLowerCase().includes(query.toLowerCase())), [routes, filter, query]);
  const toggle = (id: string) => setRoutes((current) => current.map((route) => route.id === id ? { ...route, status: route.status === "enabled" ? "disabled" : "enabled" } : route));
  return (
    <>
      <PageHeader title="路由" subtitle="为调用者签发受限能力，不公开上游连接信息" action={<button className="primary-button" onClick={onAdd}><Plus size={16} />新增路由</button>} />
      <div className="filter-bar">
        <label className="search-field"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索名称或别名" /></label>
        <div className="segmented" aria-label="路由类型">
          {(["All", "HTTP", "SSH", "LLM"] as RouteFilter[]).map((value) => <button key={value} className={filter === value ? "selected" : ""} onClick={() => setFilter(value)}>{value === "All" ? "全部" : value}</button>)}
        </div>
        <button className="icon-button" title="更多筛选" aria-label="更多筛选"><SlidersHorizontal size={17} /></button>
      </div>
      <RouteTable routes={filtered} onToggle={toggle} />
    </>
  );
}

function RouteTable({ routes, compact = false, onToggle }: { routes: RouteSummary[]; compact?: boolean; onToggle?: (id: string) => void }) {
  return (
    <div className="table-wrap"><table><thead><tr><th>状态</th><th>名称</th><th>类型</th><th>本地入口</th><th>权限摘要</th><th>出口</th><th>健康</th>{!compact && <th>最近使用</th>}<th aria-label="操作" /></tr></thead>
      <tbody>{routes.map((route) => <tr key={route.id}>
        <td><StatusBadge status={route.status} /></td><td><strong>{route.name}</strong><span className="cell-subtext">{route.alias}</span></td><td><span className={`kind kind-${route.kind.toLowerCase()}`}>{route.kind}</span></td><td><code>{route.localEndpoint}</code></td><td>{route.permissionSummary}</td><td>{route.egress}</td><td><HealthBadge health={route.health} /></td>{!compact && <td>{route.lastUsed}</td>}<td>{onToggle && <button className="icon-button small" title={route.status === "enabled" ? "停用路由" : "启用路由"} aria-label={route.status === "enabled" ? "停用路由" : "启用路由"} onClick={() => onToggle(route.id)}><Power size={15} /></button>}</td>
      </tr>)}</tbody></table></div>
  );
}

function ActivityPage() { return <><PageHeader title="活动" subtitle="脱敏审计事件，不记录正文、命令或真实目标" /><div className="filter-bar"><button className="secondary-button"><SlidersHorizontal size={16} />全部结果</button><span className="retention-note">保留 14 天 · 3 条事件</span></div><ActivityTable events={activityEvents} /></>; }

function ActivityTable({ events }: { events: ActivityEvent[] }) { return <div className="table-wrap"><table><thead><tr><th>时间</th><th>路由</th><th>调用者</th><th>动作</th><th>结果</th><th>延迟</th><th>出口</th><th>事件 ID</th></tr></thead><tbody>{events.map((event) => <tr key={event.id}><td>{event.time}</td><td><strong>{event.routeName}</strong></td><td>{event.caller}</td><td>{event.action}</td><td><StatusBadge status={event.result} /></td><td>{event.latency}</td><td>{event.egress}</td><td><code>{event.id}</code></td></tr>)}</tbody></table></div>; }

function SettingsPage() {
  const [startup, setStartup] = useState(true);
  const [notifications, setNotifications] = useState(true);
  return <><PageHeader title="设置" subtitle="本地服务、出口和安全默认值" />
    <section className="settings-section"><div><h2>常规</h2><p>桌面窗口和后台服务行为</p></div><div className="settings-controls"><Toggle label="登录时启动 Airlock" checked={startup} onChange={setStartup} /><Toggle label="高风险请求通知" checked={notifications} onChange={setNotifications} /></div></section>
    <section className="settings-section"><div><h2>网络</h2><p>监听地址固定为 loopback</p></div><div className="settings-controls"><ReadOnlyField label="HTTP 入口" value="127.0.0.1:4768" /><ReadOnlyField label="SSH 入口" value="127.0.0.1:4769" /><ReadOnlyField label="代理配置" value="HTTP · 127.0.0.1:7890" /></div></section>
    <section className="settings-section"><div><h2>安全</h2><p>凭据和活动保留策略</p></div><div className="settings-controls"><ReadOnlyField label="SecretStore" value="P0 内存存储 · Keychain 待接入" /><ReadOnlyField label="活动保留" value="待接入" /></div></section>
  </>;
}

function RouteEditor({ onClose }: { onClose: () => void }) {
  const [step, setStep] = useState(1);
  const [kind, setKind] = useState<RouteKind>("HTTP");
  const steps = ["类型与别名", "受保护目标", "权限与限额", "出口", "检查并启用"];
  return <div className="editor-overlay" role="dialog" aria-modal="true" aria-label="新增路由"><div className="editor-panel">
    <header className="editor-header"><div><h2>新增路由</h2><p>真实目标和凭据不会进入 WebView。</p></div><button className="icon-button" onClick={onClose} aria-label="关闭"><X size={18} /></button></header>
    <ol className="step-list">{steps.map((label, index) => <li key={label} className={step === index + 1 ? "current" : step > index + 1 ? "done" : ""}><span>{step > index + 1 ? <Check size={14} /> : index + 1}</span>{label}</li>)}</ol>
    <div className="editor-body">{step === 1 && <><h3>选择路由类型</h3><div className="type-grid">{(["HTTP", "SSH", "LLM"] as RouteKind[]).map((value) => <button key={value} className={kind === value ? "selected" : ""} onClick={() => setKind(value)}><Route size={18} /><strong>{value}</strong><span>{value === "HTTP" ? "固定 URL / Wget" : value === "SSH" ? "受限 Exec 或 Shell" : "OpenAI / Anthropic 预设"}</span></button>)}</div><label className="form-field"><span>名称</span><input placeholder="例如：Release downloads" /></label><label className="form-field"><span>本地别名</span><input placeholder="release-downloads" /></label></>}
      {step === 2 && <><h3>受保护目标</h3><div className="protected-box"><KeyRound size={22} /><div><strong>通过系统安全窗口录入</strong><p>Target Descriptor、账号和 Secret 直接保存到 Keychain，前端只接收“已配置”状态。</p></div><button className="secondary-button">打开安全录入</button></div></>}
      {step === 3 && <><h3>权限与限额</h3><div className="form-grid"><ReadOnlyField label="默认方法" value={kind === "SSH" ? "Exec only" : kind === "LLM" ? "POST" : "GET, HEAD"} /><ReadOnlyField label="并发上限" value="2" /><ReadOnlyField label="能力有效期" value="24 小时" /><ReadOnlyField label="速率限制" value="30 / 分钟" /></div>{kind === "SSH" && <div className="warning-panel"><AlertTriangle size={20} /><p>交互 Shell、PTY、SFTP 和端口转发默认关闭。</p></div>}</>}
      {step === 4 && <><h3>选择出口</h3><div className="type-grid"><button className="selected"><Network size={18} /><strong>Auto</strong><span>安全条件下直连失败后使用代理</span></button><button><Network size={18} /><strong>Direct</strong><span>仅直连</span></button><button><Network size={18} /><strong>Proxy</strong><span>固定使用本地代理</span></button></div></>}
      {step === 5 && <><h3>检查并启用</h3><div className="review-list"><div><span>类型</span><strong>{kind}</strong></div><div><span>受保护目标</span><strong><ShieldCheck size={15} />等待安全录入</strong></div><div><span>出口</span><strong>Auto</strong></div><div><span>Capability</span><strong>保存时生成一次</strong></div></div></>}
    </div>
    <footer className="editor-footer"><button className="secondary-button" onClick={step === 1 ? onClose : () => setStep(step - 1)}>{step > 1 && <ChevronLeft size={16} />}{step === 1 ? "取消" : "上一步"}</button><button className="primary-button" onClick={step === 5 ? onClose : () => setStep(step + 1)}>{step === 5 ? "保存草稿" : "下一步"}{step < 5 && <ChevronRight size={16} />}</button></footer>
  </div></div>;
}

function PageHeader({ title, subtitle, action }: { title: string; subtitle: string; action?: React.ReactNode }) { return <div className="page-header"><div><h1>{title}</h1><p>{subtitle}</p></div>{action}</div>; }
function Metric({ label, value, detail, tone }: { label: string; value: string; detail: string; tone?: string }) { return <div className="metric"><span>{label}</span><strong className={tone}>{value}</strong><small>{detail}</small></div>; }
function StatusBadge({ status }: { status: string }) { const labels: Record<string, string> = { enabled: "已启用", disabled: "已停用", blocked: "已阻止", allowed: "已允许", failed: "失败" }; return <span className={`status-badge status-${status}`}><span />{labels[status] ?? status}</span>; }
function HealthBadge({ health }: { health: RouteSummary["health"] }) { const labels = { healthy: "健康", degraded: "异常", unknown: "未测试" }; return <span className={`health health-${health}`}><span />{labels[health]}</span>; }
function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) { return <label className="toggle-row"><span>{label}</span><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} /><span className="toggle" /></label>; }
function ReadOnlyField({ label, value }: { label: string; value: string }) { return <div className="readonly-field"><span>{label}</span><strong>{value}</strong></div>; }
function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) { return <div className="modal-backdrop" role="presentation"><div className="modal" role="dialog" aria-modal="true" aria-label={title}><div className="modal-header"><h2>{title}</h2><button className="icon-button" onClick={onClose} aria-label="关闭"><X size={18} /></button></div>{children}</div></div>; }

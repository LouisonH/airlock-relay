export type LocalePreference = "system" | AppLocale;
export type AppLocale = "zh-CN" | "en" | "ja";

const storageKey = "airlock.ui.locale";

const translations: Record<string, Record<Exclude<AppLocale, "zh-CN">, string>> = {
  "概览": { en: "Overview", ja: "概要" },
  "路由": { en: "Routes", ja: "ルート" },
  "活动": { en: "Activity", ja: "アクティビティ" },
  "指南": { en: "Guide", ja: "ガイド" },
  "设置": { en: "Settings", ja: "設定" },
  "使用指南": { en: "Guide", ja: "使い方ガイド" },
  "配置固定能力、观察本地状态，并始终把真实凭据留在 Airlock 内": { en: "Configure fixed capabilities, observe local state, and keep real credentials inside Airlock.", ja: "固定 Capability を設定し、ローカル状態を確認しながら、実際の認証情報を Airlock 内に保持します。" },
  "管理路由": { en: "Manage routes", ja: "ルートを管理" },
  "Airlock 是固定路由转发器，不是通用代理。": { en: "Airlock is a fixed-route relay, not a general proxy.", ja: "Airlock は固定ルートのリレーであり、汎用プロキシではありません。" },
  "调用方只获得单条路由的本地凭据；上游 URL、SSH 账户密码与真实 API Key 始终保留在受保护的本机存储中。": { en: "Callers receive only a local credential for one route; upstream URLs, SSH credentials, and real API keys remain in protected local storage.", ja: "呼び出し元が受け取るのは 1 つのルート用ローカル認証情報だけです。上流 URL、SSH 認証情報、実際の API Key は保護されたローカルストレージに残ります。" },
  "创建最小权限路由": { en: "Create least-privilege routes", ja: "最小権限ルートを作成" },
  "先给每个调用方一条独立入口，再决定它能做什么。": { en: "Give each caller an independent endpoint, then decide exactly what it may do.", ja: "各呼び出し元に独立した入口を与え、実行可能な操作を明確に定めます。" },
  "搜索、筛选与健康检查": { en: "Search, filter, and health checks", ja: "検索、フィルター、ヘルスチェック" },
  "版本、CLI 与平台状态": { en: "Version, CLI, and platform status", ja: "バージョン、CLI、プラットフォーム状態" },
  "版本与文档": { en: "Version and documentation", ja: "バージョンとドキュメント" },
  "只检查公开发布信息，不上传本机配置": { en: "Checks public release metadata only; no local configuration is uploaded.", ja: "公開リリースメタデータだけを確認し、ローカル設定は送信しません。" },
  "检查更新": { en: "Check for updates", ja: "アップデートを確認" },
  "珊瑚": { en: "Coral", ja: "コーラル" },
  "检查中": { en: "Checking", ja: "確認中" },
  "打开指南": { en: "Open guide", ja: "ガイドを開く" },
  "尚未检查": { en: "Not checked yet", ja: "未確認" },
  "已是最新稳定版本": { en: "You are on the latest stable release", ja: "最新の安定版です" },
  "公开发布信息暂不可用": { en: "Public release metadata is unavailable", ja: "公開リリースメタデータを取得できません" },
  "版本检查不会下载、安装或自动打开外部页面；更新始终由你手动下载并核验。": { en: "Version checks never download, install, or open external pages automatically. You always download and verify updates manually.", ja: "バージョン確認で自動ダウンロード、インストール、外部ページの表示は行いません。更新は常に手動でダウンロードして検証します。" },
  "主导航": { en: "Main navigation", ja: "メインナビゲーション" },
  "本地核心已连接": { en: "Local core connected", ja: "ローカルコア接続済み" },
  "等待本地核心": { en: "Waiting for local core", ja: "ローカルコアを待機中" },
  "控制通道未连接": { en: "Control channel disconnected", ja: "制御チャネル未接続" },
  "刷新状态": { en: "Refresh status", ja: "状態を更新" },
  "停止全部": { en: "Stop all", ja: "すべて停止" },
  "全部路由已停止": { en: "All routes stopped", ja: "すべてのルートを停止しました" },
  "新请求将立即被拒绝，已建立的连接会进入关闭流程。": { en: "New requests will be rejected immediately and existing connections will begin closing.", ja: "新規リクエストは直ちに拒否され、既存の接続は終了処理に入ります。" },
  "取消": { en: "Cancel", ja: "キャンセル" },
  "确认停止": { en: "Confirm stop", ja: "停止を確認" },
  "删除路由": { en: "Delete route", ja: "ルートを削除" },
  "本地入口、Capability 和当前 SecretStore 中的受保护目标都会被永久删除。": { en: "The local endpoint, capability, and protected target in the current SecretStore will be permanently deleted.", ja: "ローカルエンドポイント、Capability、現在の SecretStore 内の保護対象が完全に削除されます。" },
  "删除": { en: "Delete", ja: "削除" },
  "本机开放能力与安全状态": { en: "Local capabilities and security status", ja: "ローカル機能とセキュリティ状態" },
  "新增路由": { en: "Add route", ja: "ルートを追加" },
  "受保护控制通道已连接": { en: "Protected control channel connected", ja: "保護された制御チャネルに接続済み" },
  "airlockd 尚未连接": { en: "airlockd is not connected", ja: "airlockd は未接続です" },
  "Unix Socket · 当前用户专用": { en: "Unix socket · current user only", ja: "Unix Socket · 現在のユーザー専用" },
  "启动本地核心后将自动重连": { en: "Reconnects automatically after the local core starts", ja: "ローカルコア起動後に自動再接続します" },
  "运行指标": { en: "Runtime metrics", ja: "稼働指標" },
  "开放路由": { en: "Open routes", ja: "公開ルート" },
  "当前连接": { en: "Current connections", ja: "現在の接続" },
  "仅统计本地入口": { en: "Local endpoints only", ja: "ローカルエンドポイントのみ" },
  "凭据存储": { en: "Credential store", ja: "認証情報ストア" },
  "本机文件": { en: "Local file", ja: "ローカルファイル" },
  "系统加密保护": { en: "System encryption", ja: "システム暗号化" },
  "0600 权限隔离": { en: "0600 permission isolation", ja: "0600 権限分離" },
  "界面只显示安全别名和本地入口。": { en: "Only safe aliases and local endpoints are shown.", ja: "安全な別名とローカルエンドポイントのみ表示します。" },
  "查看全部": { en: "View all", ja: "すべて表示" },
  "检测全部": { en: "Test all", ja: "すべてテスト" },
  "搜索名称或别名": { en: "Search name or alias", ja: "名前または別名を検索" },
  "搜索名称、别名或本地入口": { en: "Search name, alias, or local endpoint", ja: "名前、別名、またはローカル入口を検索" },
  "清空筛选": { en: "Clear filters", ja: "フィルターをクリア" },
  "检测筛选结果": { en: "Test filtered routes", ja: "絞り込みルートをテスト" },
  "全部类型": { en: "All types", ja: "すべての種類" },
  "全部状态": { en: "All states", ja: "すべての状態" },
  "全部健康状态": { en: "All health states", ja: "すべてのヘルス状態" },
  "全部出口": { en: "All egress modes", ja: "すべての出口" },
  "路由类型": { en: "Route type", ja: "ルート種別" },
  "全部": { en: "All", ja: "すべて" },
  "状态": { en: "Status", ja: "状態" },
  "名称": { en: "Name", ja: "名前" },
  "类型": { en: "Type", ja: "種別" },
  "本地入口": { en: "Local endpoint", ja: "ローカル入口" },
  "权限": { en: "Permissions", ja: "権限" },
  "出口": { en: "Egress", ja: "出口" },
  "健康": { en: "Healthy", ja: "正常" },
  "最近使用": { en: "Last used", ja: "最終使用" },
  "暂无路由": { en: "No routes", ja: "ルートはありません" },
  "创建后，本地入口会显示在这里。": { en: "Local endpoints appear here after creation.", ja: "作成したローカルエンドポイントがここに表示されます。" },
  "启用路由": { en: "Enable route", ja: "ルートを有効化" },
  "停用路由": { en: "Disable route", ja: "ルートを無効化" },
  "测试上游连通性与身份校验": { en: "Test upstream connectivity and identity", ja: "上流の接続性とアイデンティティをテスト" },
  "SSH 命令权限": { en: "SSH command permissions", ja: "SSH コマンド権限" },
  "LLM 访问边界": { en: "LLM access boundary", ja: "LLM アクセス境界" },
  "活动记录只保留脱敏的类别、结果与耗时；不记录目标、凭据、请求正文或模型内容": { en: "Activity keeps only sanitized category, result, and duration; targets, credentials, bodies, and model content are never recorded", ja: "アクティビティにはサニタイズ済みの種別、結果、所要時間のみ保存し、対象、認証情報、本文、モデル内容は記録しません" },
  "最近事件": { en: "recent events", ja: "最近のイベント" },
  "健康检查": { en: "health checks", ja: "ヘルスチェック" },
  "健康检查分类": { en: "Health", ja: "ヘルスチェック" },
  "异常或阻止": { en: "problems or blocks", ja: "異常または拒否" },
  "异常": { en: "Problems", ja: "異常" },
  "活动分类": { en: "Activity category", ja: "アクティビティ分類" },
  "暂无活动": { en: "No activity", ja: "アクティビティはありません" },
  "新的脱敏事件会显示在这里。": { en: "New sanitized events appear here.", ja: "新しい脱敏済みイベントがここに表示されます。" },
  "时间": { en: "Time", ja: "時刻" },
  "类别": { en: "Category", ja: "分類" },
  "调用者": { en: "Caller", ja: "呼び出し元" },
  "动作": { en: "Action", ja: "アクション" },
  "结果": { en: "Result", ja: "結果" },
  "延迟": { en: "Latency", ja: "遅延" },
  "事件 ID": { en: "Event ID", ja: "イベント ID" },
  "手动健康检查": { en: "Manual health check", ja: "手動ヘルスチェック" },
  "LLM API 请求": { en: "LLM API request", ja: "LLM API リクエスト" },
  "本地外观、网络与安全边界": { en: "Local appearance, network, and security boundaries", ja: "ローカル表示、ネットワーク、セキュリティ境界" },
  "外观": { en: "Appearance", ja: "外観" },
  "主题偏好保存在本机": { en: "Preferences are stored locally", ja: "設定はローカルに保存されます" },
  "显示模式": { en: "Display mode", ja: "表示モード" },
  "界面主题": { en: "Interface theme", ja: "インターフェーステーマ" },
  "系统": { en: "System", ja: "システム" },
  "浅色": { en: "Light", ja: "ライト" },
  "深色": { en: "Dark", ja: "ダーク" },
  "配色风格": { en: "Accent", ja: "アクセント" },
  "青峦": { en: "Forest", ja: "フォレスト" },
  "海岸": { en: "Ocean", ja: "オーシャン" },
  "暖阳": { en: "Amber", ja: "アンバー" },
  "界面行为": { en: "Interface behavior", ja: "インターフェース動作" },
  "调整刷新节奏、密度与动画": { en: "Adjust refresh cadence, density, and motion", ja: "更新間隔、密度、モーションを調整" },
  "界面语言": { en: "Interface language", ja: "表示言語" },
  "跟随系统语言，或固定一种语言": { en: "Follow the system language or select one", ja: "システム言語に従うか、言語を固定します" },
  "跟随系统": { en: "System", ja: "システム" },
  "自动刷新": { en: "Auto refresh", ja: "自動更新" },
  "控制状态轮询频率": { en: "Control-state polling interval", ja: "制御状態のポーリング間隔" },
  "秒": { en: "sec", ja: "秒" },
  "信息密度": { en: "Information density", ja: "情報密度" },
  "影响表格行高与页面间距": { en: "Affects table height and page spacing", ja: "表の行高とページ間隔に影響します" },
  "舒适": { en: "Comfortable", ja: "快適" },
  "紧凑": { en: "Compact", ja: "コンパクト" },
  "界面动效": { en: "Motion", ja: "モーション" },
  "精简模式会关闭循环和位移动画": { en: "Reduced mode disables looping and positional motion", ja: "モーション低減で反復・移動アニメーションを無効化" },
  "标准": { en: "Standard", ja: "標準" },
  "精简": { en: "Reduced", ja: "低減" },
  "安全方案": { en: "Security profiles", ja: "セキュリティプロファイル" },
  "新安装默认标准；迁移会先验证再切换": { en: "New installs default to Standard; migrations are verified before switching", ja: "新規インストールは標準が既定。移行は検証後に切り替えます" },
  "当前方案": { en: "Current profile", ja: "現在のプロファイル" },
  "待应用方案": { en: "Pending profile", ja: "適用待ちプロファイル" },
  "严格": { en: "Strict", ja: "厳格" },
  "便捷": { en: "Convenient", ja: "簡易" },
  "自定义": { en: "Custom", ja: "カスタム" },
  "高保护": { en: "High protection", ja: "高保護" },
  "推荐默认": { en: "Recommended default", ja: "推奨デフォルト" },
  "局域网暴露": { en: "LAN exposure", ja: "LAN 公開" },
  "自定义边界": { en: "Custom boundary", ja: "カスタム境界" },
  "推荐": { en: "Recommended", ja: "推奨" },
  "高级组合": { en: "Advanced combination", ja: "詳細組み合わせ" },
  "分别调整凭据保护与入口范围": { en: "Adjust credential protection and ingress separately", ja: "認証情報保護と入口範囲を個別に調整" },
  "凭据保护": { en: "Credential protection", ja: "認証情報保護" },
  "上游地址、账号、密码与代理认证": { en: "Upstream addresses, accounts, passwords, and proxy authentication", ja: "上流アドレス、アカウント、パスワード、プロキシ認証" },
  "0600 文件": { en: "0600 file", ja: "0600 ファイル" },
  "网络范围": { en: "Network scope", ja: "ネットワーク範囲" },
  "只影响数据入口，控制面始终仅当前用户": { en: "Affects data ingress only; control remains current-user only", ja: "データ入口のみに影響し、制御面は現在のユーザー専用です" },
  "仅本机": { en: "This Mac only", ja: "この Mac のみ" },
  "局域网": { en: "Private LAN", ja: "プライベート LAN" },
  "应用设置": { en: "Apply settings", ja: "設定を適用" },
  "正在迁移并重启": { en: "Migrating and restarting", ja: "移行して再起動中" },
  "网络与出口": { en: "Network and egress", ja: "ネットワークと出口" },
  "数据入口仅本机可访问": { en: "Data endpoints are local-only", ja: "データ入口はこの Mac からのみアクセス可能" },
  "数据入口已对局域网开放": { en: "Data endpoints are open to the private LAN", ja: "データ入口はプライベート LAN に公開中" },
  "HTTP 入口": { en: "HTTP endpoint", ja: "HTTP 入口" },
  "SSH 入口": { en: "SSH endpoint", ja: "SSH 入口" },
  "控制通道": { en: "Control channel", ja: "制御チャネル" },
  "更换": { en: "Change", ja: "変更" },
  "配置": { en: "Configure", ja: "設定" },
  "已配置": { en: "Configured", ja: "設定済み" },
  "未配置": { en: "Not configured", ja: "未設定" },
  "不变的安全边界": { en: "Invariant security boundaries", ja: "変わらないセキュリティ境界" },
  "便捷模式也不会放开控制面": { en: "Convenient mode never exposes control", ja: "簡易モードでも制御面は公開しません" },
  "路由元数据": { en: "Route metadata", ja: "ルートメタデータ" },
  "SSH 安全核心": { en: "SSH security core", ja: "SSH セキュリティコア" },
  "敏感录入": { en: "Sensitive entry", ja: "機密情報入力" },
  "关于": { en: "About", ja: "このアプリについて" },
  "为不受信任的调用方隔离真实凭据": { en: "Isolate real credentials from untrusted callers", ja: "信頼できない呼び出し元から実認証情報を分離" },
  "产品设计与核心开发": { en: "Product design and core development", ja: "プロダクト設計とコア開発" },
  "华南理工大学（SCUT）相关开发者 · 独立个人项目": { en: "South China University of Technology (SCUT) affiliated developer · independent personal project", ja: "華南理工大学（SCUT）に関わる開発者 · 独立した個人プロジェクト" },
  "Airlock 由 LouisonH 独立开发，不代表华南理工大学的官方项目、立场或背书。": { en: "Airlock is independently developed by LouisonH and does not represent an official SCUT project, position, or endorsement.", ja: "Airlock は LouisonH による独立した個人開発であり、華南理工大学の公式プロジェクト、立場、承認を表すものではありません。" },
  "Airlock 面向不受信任的 LLM、自动化工具与第三方 API 中转环境，让调用方只接触本地二次凭据，隔离真实 URL、账号、密码与 API Key。": { en: "Airlock is built for untrusted LLMs, automation, and third-party API relay services. Callers receive only local secondary credentials while real URLs, accounts, passwords, and API keys stay isolated.", ja: "Airlock は、信頼できない LLM、自動化ツール、サードパーティ API 中継サービス向けです。呼び出し元にはローカルの二次認証情報だけを渡し、実 URL、アカウント、パスワード、API Key を分離します。" },
  "访问 LouisonH 的网站": { en: "Visit LouisonH's website", ja: "LouisonH のサイトを開く" },
  "AI 协作": { en: "AI collaboration", ja: "AI 協働" },
  "Developer": { en: "Developer", ja: "開発者" },
  "AI 协作 · GPT-5.6 Sol": { en: "AI collaboration · GPT-5.6 Sol", ja: "AI 協働 · GPT-5.6 Sol" },
  "Technical Preview": { en: "Technical Preview", ja: "テクニカルプレビュー" },
  "Local security relay": { en: "Local security relay", ja: "ローカルセキュリティリレー" },
  "loopback only": { en: "loopback only", ja: "ループバックのみ" },
  "LAN relay": { en: "LAN relay", ja: "LAN リレー" },
  "操作": { en: "Actions", ja: "操作" },
  "活动摘要": { en: "Activity summary", ja: "アクティビティ概要" },
  "脱敏记录仅保留类别、结果与耗时；不记录目标、凭据、请求正文或模型内容": { en: "Sanitized records keep only category, result, and duration; targets, credentials, request bodies, and model content are never recorded", ja: "サニタイズ済み記録には分類、結果、所要時間のみを保存し、対象、認証情報、リクエスト本文、モデル内容は記録しません" },
  "安全等级": { en: "Security level", ja: "セキュリティレベル" },
  "Keychain · 仅本机": { en: "Keychain · this Mac only", ja: "Keychain · この Mac のみ" },
  "0600 文件 · 仅本机": { en: "0600 file · this Mac only", ja: "0600 ファイル · この Mac のみ" },
  "0600 文件 · 局域网": { en: "0600 file · private LAN", ja: "0600 ファイル · プライベート LAN" },
  "适合长期保存高价值凭据。Secret 由 macOS 加密并控制访问，系统可能要求验证登录密码。": { en: "For long-term storage of high-value credentials. macOS encrypts and controls access to secrets and may ask for your login password.", ja: "高価値の認証情報を長期保存する用途向けです。Secret は macOS により暗号化・アクセス制御され、ログインパスワードの確認を求められる場合があります。" },
  "默认方案。启动时不读取 Keychain，不弹授权框；Secret 未加密，由当前账户和文件权限隔离。": { en: "Default profile. It does not read Keychain at startup or show an authorization prompt. Secrets are not encrypted and rely on the current account and file permissions.", ja: "既定のプロファイルです。起動時に Keychain を読み取らず、認証ダイアログも表示しません。Secret は暗号化されず、現在のアカウントとファイル権限で分離されます。" },
  "用于受信任私网中转。持有路由凭据的局域网设备可访问入口，控制面仍保持本机专用。": { en: "For relaying on a trusted private network. LAN devices with route credentials can use data endpoints, while control remains local-only.", ja: "信頼できるプライベートネットワークでの中継向けです。ルート認証情報を持つ LAN デバイスはデータ入口を利用できますが、制御面はローカル専用のままです。" },
  "系统钥匙串": { en: "System Keychain", ja: "システム Keychain" },
  "当前用户文件": { en: "Current-user file", ja: "現在のユーザーファイル" },
  "私有局域网": { en: "Private LAN", ja: "プライベート LAN" },
  "存储": { en: "Store", ja: "保存先" },
  "入口": { en: "Ingress", ja: "入口" },
  "系统加密保护，会按需授权": { en: "System encryption with authorization when needed", ja: "システム暗号化保護と必要時の認証" },
  "免钥匙串提示，但入口对私网开放": { en: "No Keychain prompt, but endpoints are open to the private LAN", ja: "Keychain の確認なし、入口はプライベート LAN に公開" },
  "免钥匙串提示，但 Secret 不加密": { en: "No Keychain prompt, but secrets are not encrypted", ja: "Keychain の確認なし、Secret は暗号化されません" },
  "macOS 决定何时显示密码框，Airlock 不能绕过该系统授权。": { en: "macOS decides when to show a password prompt; Airlock cannot bypass this system authorization.", ja: "パスワードダイアログを表示するタイミングは macOS が決定し、Airlock がこのシステム認証を回避することはできません。" },
  "Secret 仅由当前 macOS 账户与 0600 文件权限隔离；同账户的其他进程可能读取。": { en: "Secrets rely only on the current macOS account and 0600 file permissions; other processes under the same account may read them.", ja: "Secret は現在の macOS アカウントと 0600 ファイル権限だけで分離され、同じアカウントの別プロセスから読み取られる可能性があります。" },
  "请只在受信任局域网使用，绝不要映射到公网。": { en: "Use this only on a trusted LAN and never expose it to the public internet.", ja: "信頼できる LAN でのみ使用し、インターネットには決して公開しないでください。" },
  "为什么调试包更容易弹出系统密码框？": { en: "Why do development builds trigger system password prompts more often?", ja: "開発ビルドでシステムのパスワード確認が増える理由" },
  "本地开发包采用 ad-hoc 签名；每次重建后，macOS 可能把新的 airlockd 视为不同程序并重新验证钥匙串访问。选择“始终允许”只对当前构建有效。正式稳定签名可减少询问，但 Keychain 仍保留最终授权决定。": { en: "Local development builds use ad-hoc signing. After each rebuild, macOS may treat the new airlockd as a different program and verify Keychain access again. Always Allow applies only to the current build. Stable production signing can reduce prompts, but Keychain retains the final authorization decision.", ja: "ローカル開発ビルドは ad-hoc 署名を使用します。再ビルド後、macOS が新しい airlockd を別のプログラムとみなし、Keychain アクセスを再確認する場合があります。「常に許可」は現在のビルドにだけ有効です。安定した正式署名で確認回数は減らせますが、最終的な認証判断は Keychain が保持します。" },
  "应用后会校验迁移结果并短暂重启 airlockd": { en: "Airlock will verify the migration and briefly restart airlockd", ja: "移行結果を検証して airlockd を短時間再起動します" },
  "标准模式启动时不会读取 macOS Keychain": { en: "Standard mode does not read macOS Keychain at startup", ja: "標準モードは起動時に macOS Keychain を読み取りません" },
  "已与当前运行设置一致": { en: "Matches the active runtime settings", ja: "現在の実行設定と一致しています" },
  "0.0.0.0:4768 · 请使用本机局域网 IP": { en: "0.0.0.0:4768 · use this Mac's LAN IP", ja: "0.0.0.0:4768 · この Mac の LAN IP を使用" },
  "0.0.0.0:4770 · 局域网": { en: "0.0.0.0:4770 · private LAN", ja: "0.0.0.0:4770 · プライベート LAN" },
  "127.0.0.1:4770 · 已就绪": { en: "127.0.0.1:4770 · ready", ja: "127.0.0.1:4770 · 準備完了" },
  "等待 airlockd": { en: "Waiting for airlockd", ja: "airlockd を待機中" },
  "Unix Socket · 仅当前用户": { en: "Unix socket · current user only", ja: "Unix Socket · 現在のユーザー専用" },
  "Clash / SOCKS5 出口": { en: "Clash / SOCKS5 egress", ja: "Clash / SOCKS5 出口" },
  "清除代理出口": { en: "Clear proxy egress", ja: "プロキシ出口を消去" },
  "0600 · 不包含明文本地密码": { en: "0600 · no plaintext local password", ja: "0600 · 平文のローカルパスワードを含みません" },
  "双会话隔离 · Shell/PTY 默认拒绝": { en: "Dual-session isolation · Shell/PTY denied by default", ja: "二重セッション分離 · Shell/PTY は既定で拒否" },
  "SSH 内嵌录入 · 仅发送到本机核心": { en: "Embedded SSH entry · sent only to the local core", ja: "SSH 組み込み入力 · ローカルコアだけに送信" },
  "SSH 命令范围": { en: "SSH command scope", ja: "SSH コマンド範囲" },
  "本地 SSH 用户名": { en: "Local SSH username", ja: "ローカル SSH ユーザー名" },
  "这个本地 SSH 用户名已映射到其他对象，请换一个。": { en: "This local SSH username already selects another route. Use a different one.", ja: "このローカル SSH ユーザー名は別のルートに割り当て済みです。別の名前を使用してください。" },
  "同一目标 IP 可以重复使用；请用不同的本地用户名选择不同映射，本地密码也可以相同。": { en: "The same target IP may be reused. Use a different local username for each mapping; the local password may be shared.", ja: "同じ対象 IP を再利用できます。マッピングごとに異なるローカルユーザー名を使用してください。ローカルパスワードは共通でも構いません。" },
  "同一监听地址通过不同用户名选择路由；修改后旧用户名立即失效。": { en: "Different usernames select routes on the same listener; the old username stops working immediately after a change.", ja: "同じリスナーでユーザー名ごとにルートを選択します。変更後、古いユーザー名は直ちに無効になります。" },
  "连接同一个 Airlock 地址时，用不同用户名选择不同的上游 SSH 主机；本地密码可与其他映射相同。": { en: "Use different usernames on the same Airlock address to select upstream SSH hosts. The local password may be shared with another mapping.", ja: "同じ Airlock アドレスで異なるユーザー名を使い、別々の上流 SSH ホストを選択します。ローカルパスワードは他のマッピングと同じでも構いません。" },
  "指定命令": { en: "Specific command", ja: "指定コマンド" },
  "只开放一个完整 exec 命令": { en: "Allow one exact exec command only", ja: "完全一致する exec コマンドを 1 つだけ許可" },
  "所有命令": { en: "All commands", ja: "すべてのコマンド" },
  "任意 exec，仍拒绝 Shell 与 PTY": { en: "Any exec; Shell and PTY remain denied", ja: "任意の exec。Shell と PTY は引き続き拒否" },
  "唯一允许命令": { en: "Only allowed command", ja: "唯一許可するコマンド" },
  "例如：uptime": { en: "Example: uptime", ja: "例: uptime" },
  "按完整字符串匹配，不要在命令参数中填写密码或 Token。": { en: "Matched as an exact string. Do not put passwords or tokens in command arguments.", ja: "文字列全体で照合します。コマンド引数にパスワードや Token を含めないでください。" },
  "该模式接近远程命令执行权限。保存时还会出现一次 macOS 原生风险确认。": { en: "This is close to remote command execution access. Saving requires an additional native macOS risk confirmation.", ja: "リモートコマンド実行に近い権限です。保存時に macOS ネイティブのリスク確認がもう一度表示されます。" },
  "记录执行命令": { en: "Record executed commands", ja: "実行コマンドを記録" },
  "完整命令保存在本机 0600 审计文件，参数可能包含敏感内容。": { en: "Full commands are stored in a local 0600 audit file; arguments may contain sensitive data.", ja: "完全なコマンドをローカルの 0600 監査ファイルに保存します。引数に機密情報が含まれる可能性があります。" },
  "等待原生确认": { en: "Waiting for native confirmation", ja: "ネイティブ確認を待機中" },
  "保存权限": { en: "Save permissions", ja: "権限を保存" },
  "固定上游": { en: "Pinned upstream", ja: "固定上流" },
  "允许模型": { en: "Allowed models", ja: "許可モデル" },
  "最大输出 Token": { en: "Maximum output tokens", ja: "最大出力 Token" },
  "每分钟请求": { en: "Requests per minute", ja: "1 分あたりのリクエスト" },
  "并发请求": { en: "Concurrent requests", ja: "同時リクエスト" },
  "统计调用与 Token": { en: "Track calls and tokens", ja: "呼び出し数と Token を集計" },
  "只读取上游 usage 数字，不记录提示词或响应正文；统计随 airlockd 重启归零。": { en: "Reads only upstream usage numbers and never records prompts or response bodies. Counters reset when airlockd restarts.", ja: "上流の usage 数値だけを読み取り、プロンプトやレスポンス本文は記録しません。集計は airlockd の再起動時にリセットされます。" },
  "LLM 使用量统计": { en: "LLM usage statistics", ja: "LLM 使用量統計" },
  "调用": { en: "Calls", ja: "呼び出し" },
  "输入 Token": { en: "Input tokens", ja: "入力 Token" },
  "输出 Token": { en: "Output tokens", ja: "出力 Token" },
  "清零统计": { en: "Reset statistics", ja: "統計をリセット" },
  "二次 API Key": { en: "Secondary API key", ja: "二次 API Key" },
  "轮换后旧 Key 立即失效；上游 API Key 不会改变。": { en: "The old key becomes invalid immediately after rotation; the upstream API key is unchanged.", ja: "ローテーション後、古い Key は直ちに無効になります。上流 API Key は変更されません。" },
  "等待原生窗口": { en: "Waiting for native window", ja: "ネイティブウィンドウを待機中" },
  "轮换 Key": { en: "Rotate key", ja: "Key をローテーション" },
  "正在保存": { en: "Saving", ja: "保存中" },
  "保存访问边界": { en: "Save access boundary", ja: "アクセス境界を保存" },
  "目标与认证在 Airlock 内完成，仅发送到本机核心": { en: "Target and credentials stay in Airlock and are sent only to the local core", ja: "対象と認証情報は Airlock 内で入力し、ローカルコアだけに送信します" },
  "敏感信息仅发送到本机核心并受隔离存储保护": { en: "Sensitive values are sent only to the local core and kept in isolated storage", ja: "機密情報はローカルコアだけに送信され、分離ストレージで保護されます" },
  "本地身份": { en: "Local identity", ja: "ローカルアイデンティティ" },
  "安全录入": { en: "Secure entry", ja: "セキュア入力" },
  "选择入口类型": { en: "Choose endpoint type", ja: "入口タイプを選択" },
  "固定 URL · GET / HEAD": { en: "Pinned URL · GET / HEAD", ja: "固定 URL · GET / HEAD" },
  "双会话隔离 · 可控 exec": { en: "Dual-session isolation · controlled exec", ja: "二重セッション分離 · 制御された exec" },
  "模型白名单 · Key 隔离": { en: "Model allowlist · key isolation", ja: "モデル許可リスト · Key 分離" },
  "本地别名": { en: "Local alias", ja: "ローカル別名" },
  "这个本地别名已被使用，请换一个。": { en: "This local alias is already in use. Choose another one.", ja: "このローカル別名は使用済みです。別の名前を選択してください。" },
  "协议预设": { en: "Protocol preset", ja: "プロトコルプリセット" },
  "逗号分隔": { en: "comma-separated", ja: "カンマ区切り" },
  "逗号分隔 ·": { en: "comma-separated ·", ja: "カンマ区切り ·" },
  "最大并发": { en: "Maximum concurrency", ja: "最大同時実行数" },
  "默认关闭，不记录提示词或响应正文": { en: "Off by default; prompts and response bodies are never recorded", ja: "既定では無効。プロンプトやレスポンス本文は記録しません" },
  "命令权限": { en: "Command permissions", ja: "コマンド権限" },
  "记录命令": { en: "Record commands", ja: "コマンドを記録" },
  "所有命令是高风险能力，创建时需要在 Airlock 内明确确认。": { en: "All commands is a high-risk capability and requires explicit confirmation inside Airlock.", ja: "すべてのコマンドは高リスク機能です。Airlock 内で明示的な確認が必要です。" },
  "出口策略": { en: "Egress policy", ja: "出口ポリシー" },
  "直连": { en: "Direct", ja: "直接" },
  "代理": { en: "Proxy", ja: "プロキシ" },
  "自动": { en: "Auto", ja: "自動" },
  "代理出口尚未在设置中安全配置。": { en: "A proxy egress has not been configured securely in Settings.", ja: "プロキシ出口が設定で安全に構成されていません。" },
  "完成受保护配置": { en: "Complete protected configuration", ja: "保護された設定を完了" },
  "上游连接与本地凭据": { en: "Upstream connection and local credential", ja: "上流接続とローカル認証情報" },
  "同一上游地址可以创建多个映射；本地用户名负责选择不同的上游账号。": { en: "One upstream address can have multiple mappings; the local username selects the upstream account.", ja: "同じ上流アドレスに複数のマッピングを作成でき、ローカルユーザー名で上流アカウントを選択します。" },
  "仅本机处理": { en: "Local processing only", ja: "ローカル処理のみ" },
  "上游 SSH 地址": { en: "Upstream SSH address", ja: "上流 SSH アドレス" },
  "上游 SSH 主机": { en: "Upstream SSH host", ja: "上流 SSH ホスト" },
  "端口": { en: "Port", ja: "ポート" },
  "默认 22": { en: "Default: 22", ja: "既定値: 22" },
  "支持主机名、IP 与 IPv6；不能填写 Airlock 自己的监听地址。": { en: "Supports hostnames, IP addresses, and IPv6. Do not enter Airlock's own listener.", ja: "ホスト名、IP アドレス、IPv6 に対応します。Airlock 自身のリスナーは指定できません。" },
  "新的 SSH 主机": { en: "New SSH host", ja: "新しい SSH ホスト" },
  "支持主机名、IP 和可选端口；不能填写 Airlock 自己的监听地址。": { en: "Supports a hostname, IP, and optional port. Do not enter Airlock's own listener.", ja: "ホスト名、IP、任意のポートに対応します。Airlock 自身のリスナーは指定できません。" },
  "上游用户名": { en: "Upstream username", ja: "上流ユーザー名" },
  "上游密码": { en: "Upstream password", ja: "上流パスワード" },
  "输入真实上游密码": { en: "Enter the real upstream password", ja: "実際の上流パスワードを入力" },
  "只发送到本机 airlockd，并按当前凭据保护方式保存。": { en: "Sent only to local airlockd and stored using the active credential protection.", ja: "ローカル airlockd だけに送信し、現在の認証情報保護方式で保存します。" },
  "本地登录凭据": { en: "Local login credential", ja: "ローカルログイン認証情報" },
  "调用方只使用这组凭据，不会接触上游密码": { en: "Callers use only this credential and never receive the upstream password", ja: "呼び出し元はこの認証情報だけを使用し、上流パスワードにはアクセスしません" },
  "随机生成": { en: "Generate", ja: "ランダム生成" },
  "自定义密码": { en: "Custom password", ja: "カスタムパスワード" },
  "Airlock 将生成高强度本地凭据，并只在完成页显示一次。": { en: "Airlock generates a strong local credential and shows it once on completion.", ja: "Airlock が強力なローカル認証情報を生成し、完了画面に一度だけ表示します。" },
  "本地登录密码": { en: "Local login password", ja: "ローカルログインパスワード" },
  "至少 12 个字节": { en: "At least 12 bytes", ja: "12 バイト以上" },
  "确认本地密码": { en: "Confirm local password", ja: "ローカルパスワードを確認" },
  "再次输入": { en: "Enter again", ja: "もう一度入力" },
  "两次输入不一致": { en: "Passwords do not match", ja: "パスワードが一致しません" },
  "Airlock 只保存摘要": { en: "Airlock stores only a digest", ja: "Airlock はダイジェストだけを保存" },
  "SSH Host Key": { en: "SSH Host Key", ja: "SSH Host Key" },
  "通过可信渠道核对后再确认": { en: "Confirm only after checking through a trusted channel", ja: "信頼できる経路で照合してから確認してください" },
  "先连接上游并读取公开指纹，不会尝试登录": { en: "Connects only to read the public fingerprint; no login is attempted", ja: "公開フィンガープリントの取得だけを行い、ログインは試行しません" },
  "我已核对并信任此 Host Key": { en: "I verified and trust this Host Key", ja: "この Host Key を照合し、信頼します" },
  "正在检测": { en: "Testing", ja: "検出中" },
  "检测 Host Key": { en: "Check Host Key", ja: "Host Key を検出" },
  "确认开放所有非交互 exec 命令": { en: "Confirm all non-interactive exec commands", ja: "すべての非対話 exec コマンドを許可" },
  "Shell、PTY、SFTP 与端口转发仍拒绝，但命令拥有上游账号可访问的数据与操作权限。": { en: "Shell, PTY, SFTP, and port forwarding stay denied, but commands can use the data and operations available to the upstream account.", ja: "Shell、PTY、SFTP、ポート転送は拒否されたままですが、コマンドは上流アカウントが利用できるデータと操作権限を使用できます。" },
  "Host Key 已读取，确认后创建并进行真实连接测试": { en: "Host Key read; confirm it to create the route and run a real connection test", ja: "Host Key を取得済み。確認後にルートを作成し、実接続をテストします" },
  "先检测 Host Key，再保存凭据和路由": { en: "Check the Host Key before saving credentials and the route", ja: "認証情報とルートを保存する前に Host Key を検出します" },
  "正在创建并测试": { en: "Creating and testing", ja: "作成してテスト中" },
  "信任并创建路由": { en: "Trust and create route", ja: "信頼してルートを作成" },
  "系统安全引导": { en: "Secure system guidance", ja: "安全なシステムガイド" },
  "macOS 原生引导": { en: "Native macOS guidance", ja: "macOS ネイティブガイド" },
  "按步骤填写上游账号与密码，可自定义完全隔离的本地登录密码，随后核对 Host Key。": { en: "Enter the upstream account and password, optionally set a fully isolated local login password, then verify the Host Key.", ja: "上流アカウントとパスワードを入力し、完全に分離されたローカルログインパスワードを任意で設定してから、Host Key を確認します。" },
  "录入上游 Base URL 与 API Key，然后自定义或随机生成完全隔离的本地 API Key。": { en: "Enter the upstream Base URL and API key, then create a fully isolated local API key manually or randomly.", ja: "上流 Base URL と API Key を入力し、完全に分離されたローカル API Key をカスタムまたはランダムで作成します。" },
  "完整 URL 与 Authorization 按当前凭据保护方式存储。": { en: "The complete URL and Authorization value are stored using the active credential protection mode.", ja: "完全な URL と Authorization は現在の認証情報保護方式で保存されます。" },
  "等待系统窗口": { en: "Waiting for system window", ja: "システムウィンドウを待機中" },
  "开始安全设置": { en: "Start secure setup", ja: "安全設定を開始" },
  "airlockd 未连接，暂时无法保存。": { en: "airlockd is disconnected; saving is temporarily unavailable.", ja: "airlockd が未接続のため、現在は保存できません。" },
  "路由已启用": { en: "Route enabled", ja: "ルートを有効化しました" },
  "Base URL 与本地 API Key 已在安全窗口中确认。": { en: "The Base URL and local API key were confirmed in a secure window.", ja: "Base URL とローカル API Key を安全なウィンドウで確認しました。" },
  "本地凭据只显示这一次，请仅交给需要访问该路由的客户端。": { en: "This local credential is shown once. Give it only to the client that needs this route.", ja: "このローカル認証情報は一度だけ表示されます。このルートが必要なクライアントだけに渡してください。" },
  "请使用刚才设置的本地密码登录；Airlock 不会回显该密码。": { en: "Sign in with the local password you just set; Airlock will not reveal it.", ja: "先ほど設定したローカルパスワードでログインしてください。Airlock はパスワードを再表示しません。" },
  "本地访问入口已创建。": { en: "The local endpoint was created.", ja: "ローカル入口を作成しました。" },
  "一次性本地凭据": { en: "One-time local credential", ja: "一度だけ表示するローカル認証情報" },
  "关闭此页面后无法再次查看。": { en: "It cannot be viewed again after this page closes.", ja: "このページを閉じると再表示できません。" },
  "显示密码": { en: "Show password", ja: "パスワードを表示" },
  "隐藏密码": { en: "Hide password", ja: "パスワードを隠す" },
  "请先检测并确认上游 SSH Host Key": { en: "Check and confirm the upstream SSH Host Key first", ja: "先に上流 SSH Host Key を検出して確認してください" },
  "SSH 上游地址指向 Airlock 本地监听地址": { en: "The SSH upstream address points to Airlock's local listener", ja: "SSH 上流アドレスが Airlock のローカルリスナーを指しています" },
  "上游 SSH 服务未返回 Host Key，请检查地址、端口和出口策略": { en: "The upstream SSH service did not return a Host Key; check the address, port, and egress", ja: "上流 SSH サービスが Host Key を返しませんでした。アドレス、ポート、出口を確認してください" },
  "本地路由别名已存在；相同上游 IP 可以复用，请更换本地别名": { en: "This local route alias already exists. The upstream IP may be reused; choose another local alias.", ja: "このローカルルート別名は既に存在します。上流 IP は再利用できるため、別のローカル別名を選択してください。" },
  "本地 SSH 用户名已存在；相同上游 IP 可以复用，请更换本地 SSH 用户名": { en: "This local SSH username already selects another route. The upstream IP may be reused; choose another local SSH username.", ja: "このローカル SSH ユーザー名は別のルートに割り当て済みです。上流 IP は再利用できるため、別のローカル SSH ユーザー名を選択してください。" },
  "上一步": { en: "Back", ja: "戻る" },
  "Direct": { en: "Direct", ja: "直接" },
  "Proxy": { en: "Proxy", ja: "プロキシ" },
  "Auto": { en: "Auto", ja: "自動" },
  "models": { en: "models", ja: "モデル" },
  "concurrent": { en: "concurrent", ja: "同時" },
  "calls": { en: "calls", ja: "呼び出し" },
  "all exec commands · high risk": { en: "all exec commands · high risk", ja: "全 exec コマンド · 高リスク" },
  "all exec commands · high risk · recorded": { en: "all exec commands · high risk · recorded", ja: "全 exec コマンド · 高リスク · 記録あり" },
  "1 exact command · stdin denied": { en: "1 exact command · stdin denied", ja: "完全一致コマンド 1 件 · stdin 拒否" },
  "1 exact command · stdin denied · recorded": { en: "1 exact command · stdin denied · recorded", ja: "完全一致コマンド 1 件 · stdin 拒否 · 記録あり" },
  "代理出口已安全保存": { en: "Proxy egress saved securely", ja: "プロキシ出口を安全に保存しました" },
  "代理出口已清除": { en: "Proxy egress cleared", ja: "プロキシ出口を消去しました" },
  "SSH 命令权限已更新": { en: "SSH command permissions updated", ja: "SSH コマンド権限を更新しました" },
  "SSH 宿主机与受保护凭据已更新": { en: "SSH host and protected credentials updated", ja: "SSH ホストと保護された認証情報を更新しました" },
  "SSH 本地凭据已轮换，旧凭据立即失效": { en: "SSH local credential rotated; the old credential is now invalid", ja: "SSH ローカル認証情報をローテーションし、古い認証情報を無効化しました" },
  "SSH 映射不可用": { en: "SSH mapping is unavailable", ja: "SSH マッピングは利用できません" },
  "LLM 访问边界已更新": { en: "LLM access boundary updated", ja: "LLM アクセス境界を更新しました" },
  "二次 API Key 已轮换，旧 Key 已失效": { en: "Secondary API key rotated; the old key is invalid", ja: "二次 API Key をローテーションし、古い Key を無効化しました" },
  "LLM 使用量统计已清零": { en: "LLM usage statistics reset", ja: "LLM 使用量統計をリセットしました" },
  "健康检查完成": { en: "Health check complete", ja: "ヘルスチェック完了" },
  "安全设置已更新": { en: "Security settings updated", ja: "セキュリティ設定を更新しました" },
  "已启用": { en: "Enabled", ja: "有効" },
  "已停用": { en: "Disabled", ja: "無効" },
  "已阻止": { en: "Blocked", ja: "拒否" },
  "已允许": { en: "Allowed", ja: "許可" },
  "失败": { en: "Failed", ja: "失敗" },
  "未测试": { en: "Not tested", ja: "未テスト" },
  "检测中": { en: "Testing", ja: "テスト中" },
  "检查": { en: "Check", ja: "確認" },
  "检测连接": { en: "Test connection", ja: "接続をテスト" },
  "代理健康检查完成": { en: "Proxy health check complete", ja: "プロキシのヘルスチェック完了" },
  "本地代理 TCP 端口可达": { en: "Local proxy TCP port is reachable", ja: "ローカルプロキシの TCP ポートに到達できます" },
  "立即检查连接": { en: "Check connection", ja: "接続を確認" },
  "新增宿主映射": { en: "Add host mapping", ja: "ホストマッピングを追加" },
  "删除宿主映射": { en: "Delete host mapping", ja: "ホストマッピングを削除" },
  "映射身份与出口": { en: "Mapping identity and egress", ja: "マッピング ID と出口" },
  "一个本地用户名对应一个受保护 SSH 宿主关系": { en: "One local username selects one protected SSH host mapping", ja: "1 つのローカルユーザー名が 1 つの保護 SSH ホストマッピングを選択します" },
  "映射名称": { en: "Mapping name", ja: "マッピング名" },
  "修改后旧用户名立即失效。": { en: "The old username stops working immediately after the change.", ja: "変更後、古いユーザー名は直ちに無効になります。" },
  "命令权限与记录": { en: "Command permissions and recording", ja: "コマンド権限と記録" },
  "停用路由的连接尝试始终进入脱敏活动；命令正文记录由下方开关控制": { en: "Connection attempts to disabled routes are always logged as sanitized activity; command text recording is controlled below", ja: "無効なルートへの接続試行は常にサニタイズ済みアクティビティとして記録され、コマンド本文の記録は下の設定に従います" },
  "受保护宿主机": { en: "Protected host", ja: "保護されたホスト" },
  "真实地址、上游账号和密码默认不回显；替换时重新输入": { en: "The real address, upstream account, and password are never shown; enter replacements again", ja: "実アドレス、上流アカウント、パスワードは再表示されません。置換時に再入力します" },
  "收起": { en: "Collapse", ja: "閉じる" },
  "替换宿主机": { en: "Replace host", ja: "ホストを置換" },
  "新的 SSH 地址": { en: "New SSH address", ja: "新しい SSH アドレス" },
  "新的上游用户名": { en: "New upstream username", ja: "新しい上流ユーザー名" },
  "新的上游密码": { en: "New upstream password", ja: "新しい上流パスワード" },
  "输入新的上游密码": { en: "Enter the new upstream password", ja: "新しい上流パスワードを入力" },
  "只保存到当前受保护凭据存储": { en: "Stored only in the active protected credential store", ja: "現在の保護認証情報ストアにのみ保存します" },
  "新的 SSH Host Key": { en: "New SSH Host Key", ja: "新しい SSH Host Key" },
  "通过可信渠道核对": { en: "Verify through a trusted channel", ja: "信頼できる経路で照合" },
  "检测新宿主 Host Key": { en: "Check new host key", ja: "新しいホストの Host Key を確認" },
  "替换成功后，原宿主凭据会被覆盖": { en: "The previous host credential is overwritten after replacement", ja: "置換後、以前のホスト認証情報は上書きされます" },
  "正在替换": { en: "Replacing", ja: "置換中" },
  "确认替换宿主机": { en: "Confirm host replacement", ja: "ホスト置換を確認" },
  "轮换后旧密码或随机凭据立即失效": { en: "The old password or generated credential becomes invalid immediately after rotation", ja: "ローテーション後、古いパスワードまたは生成認証情報は直ちに無効になります" },
  "新的本地密码": { en: "New local password", ja: "新しいローカルパスワード" },
  "确认新密码": { en: "Confirm new password", ja: "新しいパスワードを確認" },
  "只保存摘要": { en: "Only a digest is stored", ja: "ダイジェストのみ保存" },
  "正在轮换": { en: "Rotating", ja: "ローテーション中" },
  "轮换本地凭据": { en: "Rotate local credential", ja: "ローカル認証情報をローテーション" },
  "新的本地凭据，仅显示一次": { en: "New local credential, shown once", ja: "新しいローカル認証情報（一度だけ表示）" },
  "保存映射设置": { en: "Save mapping settings", ja: "マッピング設定を保存" },
  "SSH connection to disabled route": { en: "Connection attempt to disabled SSH route", ja: "無効な SSH ルートへの接続試行" },
  "SSH 认证测试不可用": { en: "SSH authentication test is unavailable", ja: "SSH 認証テストは利用できません" },
  "上游 SSH 认证测试失败": { en: "Upstream SSH authentication test failed", ja: "上流 SSH 認証テストに失敗しました" },
  "上游 SSH 账号或密码被拒绝": { en: "The upstream SSH username or password was rejected", ja: "上流 SSH のユーザー名またはパスワードが拒否されました" },
  "上游 SSH Host Key 校验失败": { en: "The upstream SSH Host Key check failed", ja: "上流 SSH Host Key の検証に失敗しました" },
  "上游 SSH 服务不可达": { en: "The upstream SSH service is unreachable", ja: "上流 SSH サービスに到達できません" },
  "Manual proxy health check": { en: "Manual proxy health check", ja: "手動プロキシヘルスチェック" },
  "SSH 路由策略不可用": { en: "SSH route policy is unavailable", ja: "SSH ルートポリシーは利用できません" },
  "未找到 SSH 路由": { en: "SSH route was not found", ja: "SSH ルートが見つかりません" },
  "SSH 命令策略无效": { en: "Invalid SSH command policy", ja: "SSH コマンドポリシーが無効です" },
  "无法保存 SSH 映射策略": { en: "Could not save the SSH mapping policy", ja: "SSH マッピングポリシーを保存できません" },
  "SSH 宿主机更新参数无效": { en: "Invalid SSH host update", ja: "SSH ホスト更新パラメーターが無効です" },
  "SSH 宿主地址无效或指向 Airlock 自身": { en: "The SSH host address is invalid or points to Airlock itself", ja: "SSH ホストアドレスが無効か、Airlock 自身を指しています" },
  "SSH Host Key 无效": { en: "Invalid SSH Host Key", ja: "SSH Host Key が無効です" },
  "无法替换受保护 SSH 宿主机": { en: "Could not replace the protected SSH host", ja: "保護された SSH ホストを置換できません" },
  "本地 SSH 凭据无效": { en: "Invalid local SSH credential", ja: "ローカル SSH 認証情報が無効です" },
  "无法生成本地 SSH 凭据": { en: "Could not generate a local SSH credential", ja: "ローカル SSH 認証情報を生成できません" },
  "无法轮换本地 SSH 凭据": { en: "Could not rotate the local SSH credential", ja: "ローカル SSH 認証情報をローテーションできません" },
  "无法保存本地 SSH 凭据": { en: "Could not save the local SSH credential", ja: "ローカル SSH 認証情報を保存できません" },
  "请输入有效的 SSH 映射名称": { en: "Enter a valid SSH mapping name", ja: "有効な SSH マッピング名を入力してください" },
  "SSH 宿主机更新意外终止": { en: "SSH host update stopped unexpectedly", ja: "SSH ホスト更新が予期せず終了しました" },
  "请先检测并确认新的 SSH Host Key": { en: "Check and confirm the new SSH Host Key first", ja: "先に新しい SSH Host Key を確認してください" },
  "airlockd 未返回轮换后的 SSH 凭据": { en: "airlockd did not return the rotated SSH credential", ja: "airlockd がローテーション後の SSH 認証情報を返しませんでした" },
  "airlockd 返回了不一致的 SSH 凭据": { en: "airlockd returned an inconsistent SSH credential", ja: "airlockd が一貫しない SSH 認証情報を返しました" },
  "SSH 本地凭据轮换意外终止": { en: "SSH local credential rotation stopped unexpectedly", ja: "SSH ローカル認証情報のローテーションが予期せず終了しました" },
  "airlockd 未返回代理健康检查结果": { en: "airlockd did not return a proxy health-check result", ja: "airlockd がプロキシのヘルスチェック結果を返しませんでした" },
  "刚刚": { en: "Just now", ja: "たった今" },
  "从未": { en: "Never", ja: "未使用" },
  "关闭": { en: "Close", ja: "閉じる" },
  "继续": { en: "Continue", ja: "続行" },
  "完成": { en: "Done", ja: "完了" },
};

const originalText = new WeakMap<Text, { source: string; rendered: string }>();
const originalAttributes = new WeakMap<Element, Map<string, { source: string; rendered: string }>>();
const translatedAttributes = ["aria-label", "title", "placeholder"];
let activeLocale: AppLocale = resolveLocale(getLocalePreference());
let observer: MutationObserver | undefined;

export function getLocalePreference(): LocalePreference {
  const stored = localStorage.getItem(storageKey);
  return stored === "zh-CN" || stored === "en" || stored === "ja" ? stored : "system";
}

export function resolveLocale(preference: LocalePreference): AppLocale {
  if (preference !== "system") return preference;
  const candidates = navigator.languages.length > 0 ? navigator.languages : [navigator.language];
  if (candidates.some((locale) => locale.toLowerCase().startsWith("zh"))) return "zh-CN";
  if (candidates.some((locale) => locale.toLowerCase().startsWith("ja"))) return "ja";
  return "en";
}

export function saveLocalePreference(preference: LocalePreference): AppLocale {
  localStorage.setItem(storageKey, preference);
  activeLocale = resolveLocale(preference);
  document.documentElement.lang = activeLocale;
  document.title = "Airlock";
  translateDocument();
  return activeLocale;
}

export function getResolvedLocale(): AppLocale {
  return activeLocale;
}

export function watchSystemLocale(onChange: () => void): () => void {
  window.addEventListener("languagechange", onChange);
  return () => window.removeEventListener("languagechange", onChange);
}

function translateValue(source: string): string {
  if (activeLocale === "zh-CN") return source;
  const exact = translations[source]?.[activeLocale];
  if (exact) return exact;
  const patterns: Array<[RegExp, (match: RegExpMatchArray) => string]> = [
    [/^(\d+) 条路由已开放$/, (m) => activeLocale === "ja" ? `${m[1]} ルート公開中` : `${m[1]} routes open`],
    [/^(\d+) 条 · (\d+) 条已开放$/, (m) => activeLocale === "ja" ? `${m[1]} ルート · ${m[2]} 公開中` : `${m[1]} routes · ${m[2]} open`],
    [/^共 (\d+) 条$/, (m) => activeLocale === "ja" ? `合計 ${m[1]}` : `${m[1]} total`],
    [/^(\d+) 条匹配$/, (m) => activeLocale === "ja" ? `${m[1]} 件一致` : `${m[1]} matches`],
    [/^(全部|HTTP|SSH|LLM|健康检查分类|异常) (\d+)$/, (m) => `${translateValue(m[1])} ${m[2]}`],
    [/^(健康检查完成) · (\d+)\/(\d+) 条通过$/, (m) => activeLocale === "ja" ? `ヘルスチェック完了 · ${m[2]}/${m[3]} 成功` : `Health checks complete · ${m[2]}/${m[3]} passed`],
    [/^(测试) (.+) (的连通性与健康状态)$/, (m) => activeLocale === "ja" ? `${m[2]} の接続性とヘルスをテスト` : `Test connectivity and health for ${m[2]}`],
    [/^(设置) (.+) (的访问边界)$/, (m) => activeLocale === "ja" ? `${m[2]} のアクセス境界を設定` : `Configure access boundary for ${m[2]}`],
    [/^(删除) (.+)$/, (m) => activeLocale === "ja" ? `${m[2]} を削除` : `Delete ${m[2]}`],
    [/^已删除 (.+) 并清理凭据$/, (m) => activeLocale === "ja" ? `${m[1]} と認証情報を削除しました` : `Deleted ${m[1]} and its credentials`],
    [/^SSH 权限 · (.+)$/, (m) => activeLocale === "ja" ? `SSH 権限 · ${m[1]}` : `SSH permissions · ${m[1]}`],
    [/^SSH 映射管理 · (.+)$/, (m) => activeLocale === "ja" ? `SSH マッピング管理 · ${m[1]}` : `SSH mapping management · ${m[1]}`],
    [/^LLM 访问边界 · (.+)$/, (m) => activeLocale === "ja" ? `LLM アクセス境界 · ${m[1]}` : `LLM access boundary · ${m[1]}`],
    [/^SSH 路由启用失败，已回滚凭据：(.+)$/, (m) => activeLocale === "ja" ? `SSH ルートを有効化できず、認証情報をロールバックしました: ${translateValue(m[1])}` : `Could not enable the SSH route; credentials were rolled back: ${translateValue(m[1])}`],
    [/^SSH 连接测试失败，路由与凭据已回滚：(.+)$/, (m) => activeLocale === "ja" ? `SSH 接続テストに失敗し、ルートと認証情報をロールバックしました: ${translateValue(m[1])}` : `SSH connection test failed; the route and credentials were rolled back: ${translateValue(m[1])}`],
    [/^SSH 认证测试失败，路由与凭据已回滚：(.+)$/, (m) => activeLocale === "ja" ? `SSH 認証テストに失敗し、ルートと認証情報をロールバックしました: ${translateValue(m[1])}` : `SSH authentication test failed; the route and credentials were rolled back: ${translateValue(m[1])}`],
    [/^SSH 路由已保存但保持停用：(.+)$/, (m) => activeLocale === "ja" ? `SSH ルートは保存されましたが、無効のままです: ${translateValue(m[1])}` : `The SSH route was saved but remains disabled: ${translateValue(m[1])}`],
    [/^新增 (HTTP|SSH|LLM) 路由$/, (m) => activeLocale === "ja" ? `${m[1]} ルートを追加` : `Add ${m[1]} route`],
    [/^(\d+) 秒$/, (m) => activeLocale === "ja" ? `${m[1]} 秒` : `${m[1]} sec`],
    [/^逗号分隔 · (\d+)\/(\d+)$/, (m) => activeLocale === "ja" ? `カンマ区切り · ${m[1]}/${m[2]}` : `comma-separated · ${m[1]}/${m[2]}`],
    [/^(GET|HEAD) 请求$/, (m) => activeLocale === "ja" ? `${m[1]} リクエスト` : `${m[1]} request`],
    [/^(Keychain|0600 文件) · 已配置$/, (m) => `${translateValue(m[1])} · ${translateValue("已配置")}`],
    [/^(\d+) 分钟前$/, (m) => activeLocale === "ja" ? `${m[1]} 分前` : `${m[1]} min ago`],
    [/^(\d+) 小时前$/, (m) => activeLocale === "ja" ? `${m[1]} 時間前` : `${m[1]} hr ago`],
  ];
  for (const [pattern, render] of patterns) {
    const match = source.match(pattern);
    if (match) return render(match);
  }
  return source;
}

export function translate(source: string): string {
  return translateValue(source);
}

function translateTextNode(node: Text): void {
  if (node.parentElement?.closest('[data-i18n="off"]')) return;
  const known = originalText.get(node);
  const source = known && node.data === known.rendered ? known.source : node.data;
  const leading = source.match(/^\s*/)?.[0] ?? "";
  const trailing = source.match(/\s*$/)?.[0] ?? "";
  const core = source.slice(leading.length, source.length - trailing.length);
  const rendered = `${leading}${translateValue(core)}${trailing}`;
  originalText.set(node, { source, rendered });
  if (node.data !== rendered) node.data = rendered;
}

function translateElement(element: Element): void {
  if (element.closest('[data-i18n="off"]')) return;
  let attributes = originalAttributes.get(element);
  if (!attributes) {
    attributes = new Map();
    originalAttributes.set(element, attributes);
  }
  for (const name of translatedAttributes) {
    const current = element.getAttribute(name);
    if (current === null) continue;
    const known = attributes.get(name);
    const source = known && current === known.rendered ? known.source : current;
    const rendered = translateValue(source);
    attributes.set(name, { source, rendered });
    if (current !== rendered) element.setAttribute(name, rendered);
  }
}

function translateTree(root: Node): void {
  if (root.nodeType === Node.TEXT_NODE) translateTextNode(root as Text);
  if (root.nodeType === Node.ELEMENT_NODE) translateElement(root as Element);
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    if (walker.currentNode.nodeType === Node.TEXT_NODE) translateTextNode(walker.currentNode as Text);
    else translateElement(walker.currentNode as Element);
  }
}

export function translateDocument(): void {
  if (!document.body) return;
  translateTree(document.body);
}

export function startLocalization(): () => void {
  saveLocalePreference(getLocalePreference());
  observer = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type === "characterData") translateTextNode(mutation.target as Text);
      if (mutation.type === "attributes") translateElement(mutation.target as Element);
      mutation.addedNodes.forEach(translateTree);
    }
  });
  observer.observe(document.body, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: translatedAttributes });
  return () => observer?.disconnect();
}

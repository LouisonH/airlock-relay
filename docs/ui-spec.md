# Airlock Desktop UI Specification

Airlock Desktop is a native control plane for the local relay. It is not a web administration site and does not manage LLM ecosystems, prompts, MCP servers, skills, or chat sessions.

## Information Architecture

- Overview: daemon state, local listeners, enabled routes, pending approvals, route health, and recent sanitized activity.
- Routes: filterable HTTP, SSH, and LLM route table with safe local endpoints and policy summaries.
- Activity: sanitized metadata by default. Full SSH commands appear only for routes with explicit command auditing enabled; targets, headers, credentials, and request/response bodies never appear.
- Settings: appearance, refresh cadence, density, motion, listener scope, SecretStore protection, proxy profiles, security defaults, and diagnostics.

The default window is 1120x720 with a minimum size of 960x640. Navigation uses a fixed sidebar that narrows at medium widths. Narrow windows hide secondary table columns rather than converting operational rows into cards; route actions must remain reachable without horizontal clipping.

## Security Rules

- Frontend IPC types must not contain target URLs, hosts, upstream usernames, credentials, cookies, or injected headers.
- Protected values are represented only by a state such as `configured`, `missing`, or `replace_required`.
- Secret entry uses a native secure flow. The WebView receives only completion status.
- A newly created capability is displayed once. Closing the dialog makes it unrecoverable; rotation is required.
- Sanitized events contain route alias, caller label, decision, latency, egress, and an opaque event ID.
- Proxy URLs and credentials are entered only in a native protected prompt; the WebView receives only configured/unconfigured state.
- Closing the window does not stop the daemon. Emergency stop is a separate confirmed action.

## Theme System

- Theme preference is `system`, `light`, or `dark`; `system` responds to live OS appearance changes.
- Store the preference under the UI-only `airlock.ui.theme` key. Never share this object with route or Secret data.
- Apply the resolved theme to `html[data-theme]`, `color-scheme`, and the native Tauri window theme.
- Light mode uses a quiet neutral sidebar rather than a permanently dark navigation rail. Dark mode avoids pure black and pure white.
- Accent, density, motion, and refresh cadence are UI-only local preferences. Reduced motion disables looping and positional animations; it never changes network behavior.

## Visual System

- System font, 12-14px body text, 11-13px tables, and compact 26px page headings.
- 4px spacing scale and restrained radii up to 8px.
- Light canvas `#F4F6F8`, surface `#FFFFFF`, text `#18202A`, border `#DFE4EA`.
- Dark canvas `#0F1215`, surface `#171B20`, text `#EDF2F7`, border `#2F3740`.
- Action blue, success green, warning amber, and danger red each have distinct foreground and soft-surface tokens.
- Status always uses an icon and text, never color alone.
- Lucide icons, visible 2px focus rings, tooltips for icon-only actions, and reduced-motion support.

## Route Creation

The in-window editor has three compact steps: local identity and non-secret policy, native protected entry, then one-time local access details. LLM routes expose provider preset, model allowlist, output limit, request rate, concurrency, and the optional usage-statistics switch in the WebView; native prompts collect the base URL and upstream API key, then require an explicit choice between a recommended random 256-bit local API key and a custom key entered twice. SSH routes expose one editable, exact-match `exec` command by default and allow it to be changed later from the policy editor; the UI warns against embedding secrets in command arguments. Existing LLM routes support policy editing, in-memory usage reset, and local API key rotation. Existing secrets are replace-only. Enabling broad SSH command access requires an explicit high-risk acknowledgement.

Route enablement uses a switch. Whole-service startup and emergency stop remain explicit commands. At medium widths the route table first hides recent-use metadata and tightens spacing; at widths up to 1050px route and activity tables hide additional secondary columns through table-specific selectors.

## LLM Usage Statistics

- Statistics are disabled by default per route and the preference is persisted with route policy.
- Only call count and upstream-reported input/output token numbers are retained; prompts and responses are never logged or persisted.
- JSON and SSE responses are supported. Temporary response capture is capped, cleared immediately after parsing, and discarded when truncated.
- Counters live only in `airlockd` memory, reset on daemon restart, and can be cleared manually from the LLM policy editor.

# Airlock Desktop UI Specification

Airlock Desktop is a native control plane for the local relay. It is not a web administration site and does not manage LLM ecosystems, prompts, MCP servers, skills, or chat sessions.

## Information Architecture

- Overview: daemon state, local listeners, enabled routes, pending approvals, route health, and recent sanitized activity.
- Routes: filterable HTTP, SSH, and LLM route table with safe local endpoints and policy summaries.
- Activity: sanitized metadata only. Never show request bodies, commands, targets, headers, or secrets.
- Settings: startup behavior, loopback listeners, proxy profiles, security defaults, and diagnostics.

The default window is 1120x720 with a minimum size of 960x640. Navigation uses a fixed 200px sidebar. Narrow windows hide secondary table columns rather than converting operational rows into cards.

## Security Rules

- Frontend IPC types must not contain target URLs, hosts, upstream usernames, credentials, cookies, or injected headers.
- Protected values are represented only by a state such as `configured`, `missing`, or `replace_required`.
- Secret entry uses a native secure flow. The WebView receives only completion status.
- A newly created capability is displayed once. Closing the dialog makes it unrecoverable; rotation is required.
- Sanitized events contain route alias, caller label, decision, latency, egress, and an opaque event ID.
- Closing the window does not stop the daemon. Emergency stop is a separate confirmed action.

## Visual System

- System font, 13-14px body text, 12-13px tables, 22px maximum page heading.
- 4px spacing scale and no radius above 6px.
- Background `#F6F7F8`, surface `#FFFFFF`, text `#171A1F`, border `#D9DEE5`.
- Action `#2563EB`, success `#16855B`, warning `#9A5B00`, danger `#C2363F`.
- Status always uses an icon and text, never color alone.
- Lucide icons, visible 2px focus rings, tooltips for icon-only actions, and reduced-motion support.

## Route Creation

The in-window editor has five steps: type and alias, protected target, policy and limits, egress, review and enable. Existing secrets are replace-only. Enabling shell, PTY, SFTP, agent forwarding, or port forwarding requires an explicit high-risk acknowledgement.

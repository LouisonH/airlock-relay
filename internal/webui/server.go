// Package webui exposes a deliberately small, separately authenticated
// operational view of a running Airlock core. It never accepts upstream
// credentials or route-creation data.
package webui

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/LouisonH/airlock-relay/internal/control"
)

const maxRequestBytes = 1024

var routeAlias = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Controller is satisfied by control.Client. It is deliberately narrower than
// the daemon control protocol so this package can only issue fixed operations.
type Controller interface {
	Do(context.Context, control.Request) (control.Response, error)
}

// Server provides an authenticated management handler.
type Server struct {
	token      string
	controller Controller
}

func New(token string, controller Controller) (*Server, error) {
	if len(token) < 32 || len(token) > 128 || controller == nil {
		return nil, errors.New("invalid web UI configuration")
	}
	return &Server{token: token, controller: controller}, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	setSecurityHeaders(writer)
	if request.Method == http.MethodGet && request.URL.Path == "/healthz" {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/" {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, shellHTML)
		return
	}
	if !s.authenticated(request) {
		writer.Header().Set("WWW-Authenticate", `Bearer realm="Airlock"`)
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "web UI authentication failed"})
		return
	}
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/status":
		s.dispatch(writer, request, control.Request{Action: "status"})
	case request.Method == http.MethodPost && request.URL.Path == "/api/stop-all":
		s.dispatch(writer, request, control.Request{Action: "stop_all"})
	case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/api/routes/"):
		s.handleRouteAction(writer, request)
	default:
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) authenticated(request *http.Request) bool {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	provided, ok := strings.CutPrefix(value, "Bearer ")
	if !ok || len(provided) != len(s.token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1
}

func (s *Server) handleRouteAction(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/routes/"), "/")
	if len(parts) != 2 || !routeAlias.MatchString(parts[0]) {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid route alias"})
		return
	}
	alias, action := parts[0], parts[1]
	switch action {
	case "health":
		s.dispatch(writer, request, control.Request{Action: "test_route_health", Alias: alias})
	case "enabled":
		var input struct {
			Enabled *bool `json:"enabled"`
		}
		body := http.MaxBytesReader(writer, request.Body, maxRequestBytes)
		defer body.Close()
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || input.Enabled == nil || ensureJSONEOF(decoder) != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid enabled state"})
			return
		}
		s.dispatch(writer, request, control.Request{Action: "set_route_enabled", Alias: alias, Enabled: *input.Enabled})
	default:
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) dispatch(writer http.ResponseWriter, request *http.Request, operation control.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()
	response, err := s.controller.Do(ctx, operation)
	if err != nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "airlockd control channel is unavailable"})
		return
	}
	if !response.OK {
		writeJSON(writer, http.StatusBadRequest, response)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return nil
}

func setSecurityHeaders(writer http.ResponseWriter) {
	header := writer.Header()
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

const shellHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Airlock Operations</title><style>
:root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}body{margin:0;background:#f4f5f7;color:#20242a}main{max-width:1040px;margin:0 auto;padding:32px 20px 52px}header{display:flex;align-items:center;justify-content:space-between;margin-bottom:28px}h1{font-size:24px;margin:0}p{color:#59616b}.panel{background:#fff;border:1px solid #dfe2e6;border-radius:10px;padding:18px;margin:14px 0;box-shadow:0 1px 2px #0000000a}.row{display:flex;gap:10px;align-items:center;flex-wrap:wrap}input{min-width:280px;padding:9px 10px;border:1px solid #b8bec6;border-radius:6px;font:inherit}button{padding:8px 11px;border:1px solid #b8bec6;border-radius:6px;background:#fff;color:inherit;font:inherit;cursor:pointer}button.primary{background:#bd503e;border-color:#bd503e;color:#fff}button:disabled{opacity:.55;cursor:wait}.muted{font-size:13px;color:#68727d}.error{color:#b42318}.ok{color:#087443}table{width:100%;border-collapse:collapse;margin-top:10px}th,td{text-align:left;padding:10px 7px;border-bottom:1px solid #e4e7eb;font-size:14px;vertical-align:middle}th{color:#68727d;font-weight:600}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}@media(prefers-color-scheme:dark){body{background:#17191d;color:#e7e9ec}.panel{background:#21242a;border-color:#363a41}input,button{background:#292d34;color:#e7e9ec;border-color:#555c66}th,td{border-color:#363a41}p,.muted,th{color:#afb7c0}}</style></head>
<body><main><header><div><h1>Airlock Operations</h1><p>Sanitized route status and safe operational controls.</p></div><button id="signOut">Sign out</button></header><section id="login" class="panel"><h2>Operator token</h2><div class="row"><input id="token" type="password" autocomplete="off" spellcheck="false" placeholder="Paste Web UI token"><button class="primary" id="connect">Connect</button></div><p class="muted">The token is kept only in this browser tab and is never sent to route targets.</p></section><section id="console" hidden><div class="row"><button id="refresh">Refresh</button><button id="stopAll">Stop all routes</button><span id="message" class="muted"></span></div><div class="panel"><div id="status" class="muted">Loading status...</div><table><thead><tr><th>Route</th><th>Type</th><th>Endpoint</th><th>State</th><th>Health</th><th>Actions</th></tr></thead><tbody id="routes"></tbody></table></div></section></main><script>
const key='airlock.web.token';const login=document.querySelector('#login');const consoleNode=document.querySelector('#console');const message=document.querySelector('#message');const status=document.querySelector('#status');const routes=document.querySelector('#routes');function token(){return sessionStorage.getItem(key)||''}function setMessage(text,bad){message.textContent=text;message.className=bad?'error':'muted'}function auth(){return {'Authorization':'Bearer '+token()}}async function call(path,options={}){const response=await fetch(path,{...options,headers:{...auth(),...(options.headers||{})}});const body=await response.json().catch(()=>({error:'invalid response'}));if(!response.ok||!body.ok)throw new Error(body.error||'operation failed');return body}function cell(row,text){const td=document.createElement('td');td.textContent=text||'--';row.append(td);return td}function action(label,handler){const button=document.createElement('button');button.textContent=label;button.onclick=handler;return button}async function load(){try{setMessage('Refreshing...');const data=await call('/api/status');status.textContent=data.running?'Core is running.':'Core status unavailable.';routes.replaceChildren();for(const route of data.routes||[]){const row=document.createElement('tr');cell(row,route.name||route.alias);cell(row,route.kind);cell(row,route.localEndpoint);cell(row,route.status);cell(row,route.health);const td=document.createElement('td');const toggle=action(route.status==='enabled'?'Disable':'Enable',async()=>{toggle.disabled=true;try{await call('/api/routes/'+encodeURIComponent(route.alias)+'/enabled',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({enabled:route.status!=='enabled'})});await load()}catch(error){setMessage(error.message,true)}finally{toggle.disabled=false}});td.append(toggle);const health=action('Health',async()=>{health.disabled=true;try{const result=await call('/api/routes/'+encodeURIComponent(route.alias)+'/health',{method:'POST'});setMessage((result.health_check||{}).message||'Health check finished')}catch(error){setMessage(error.message,true)}finally{health.disabled=false}});td.append(document.createTextNode(' '),health);row.append(td);routes.append(row)}setMessage('Updated')}catch(error){setMessage(error.message,true)}}function open(){login.hidden=true;consoleNode.hidden=false;load()}document.querySelector('#connect').onclick=()=>{const value=document.querySelector('#token').value.trim();if(value.length<32){setMessage('A valid token is required.',true);return}sessionStorage.setItem(key,value);open()};document.querySelector('#refresh').onclick=load;document.querySelector('#stopAll').onclick=async()=>{if(!confirm('Disable every route?'))return;try{await call('/api/stop-all',{method:'POST'});await load()}catch(error){setMessage(error.message,true)}};document.querySelector('#signOut').onclick=()=>{sessionStorage.removeItem(key);location.reload()};if(token())open();</script></body></html>`

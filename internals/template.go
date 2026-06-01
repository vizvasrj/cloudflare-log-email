package internals

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"
)

func buildTemplates() *template.Template {
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"seq": func(start, end int) []int {
			var s []int
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"paginationStart": func(page, total int) int {
			s := page - 2
			if s < 1 {
				s = 1
			}
			return s
		},
		"paginationEnd": func(page, total int) int {
			e := page + 2
			if e > total {
				e = total
			}
			return e
		},
		"fmtTime": func(t time.Time) string {
			return t.UTC().Format("2006-01-02 15:04:05")
		},
		"statusColor": func(s string) string {
			switch strings.ToLower(s) {
			case "delivered", "forwarded":
				return "badge-success"
			case "dropped", "failed", "rejected":
				return "badge-danger"
			case "deferred":
				return "badge-warning"
			default:
				return "badge-neutral"
			}
		},
		"authColor": func(s string) string {
			switch strings.ToLower(s) {
			case "pass":
				return "auth-pass"
			case "fail":
				return "auth-fail"
			case "none", "":
				return "auth-none"
			default:
				return "auth-neutral"
			}
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
		// buildQuery regenerates the query string preserving all filters, overriding page
		"buildQuery": func(q LogQuery, page int) template.URL {
			v := url.Values{}
			if q.Search != "" {
				v.Set("q", q.Search)
			}
			if q.FromAddr != "" {
				v.Set("from", q.FromAddr)
			}
			if q.ToAddr != "" {
				v.Set("to", q.ToAddr)
			}
			if q.Status != "" {
				v.Set("status", q.Status)
			}
			if q.Action != "" {
				v.Set("action", q.Action)
			}
			if q.IsSpam != nil {
				if *q.IsSpam {
					v.Set("spam", "true")
				} else {
					v.Set("spam", "false")
				}
			}
			if q.DateFrom != "" {
				v.Set("date_from", q.DateFrom)
			}
			if q.DateTo != "" {
				v.Set("date_to", q.DateTo)
			}
			v.Set("page", fmt.Sprintf("%d", page))
			v.Set("per_page", fmt.Sprintf("%d", q.PageSize))
			return template.URL(v.Encode())
		},
	}

	const loginTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CF Email Monitor · Login</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@300;400;500&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#0a0c0f;--surface:#12151a;--border:#1e2530;--border-bright:#2d3748;
  --text:#e2e8f0;--text-muted:#64748b;--accent:#f97316;--danger:#ef4444;
  --mono:'IBM Plex Mono',monospace;--sans:'IBM Plex Sans',sans-serif;
}
html,body{height:100%;background:var(--bg);color:var(--text);font-family:var(--sans)}
body{display:flex;align-items:center;justify-content:center;min-height:100vh}
.card{width:100%;max-width:380px;padding:2.5rem;background:var(--surface);border:1px solid var(--border);border-top:3px solid var(--accent)}
.logo{font-family:var(--mono);font-size:.7rem;letter-spacing:.15em;color:var(--accent);text-transform:uppercase;margin-bottom:2rem}
.logo span{color:var(--text-muted)}
h1{font-family:var(--mono);font-size:1.1rem;font-weight:600;margin-bottom:.4rem}
.sub{font-size:.8rem;color:var(--text-muted);margin-bottom:2rem}
label{display:block;font-family:var(--mono);font-size:.7rem;letter-spacing:.08em;color:var(--text-muted);text-transform:uppercase;margin-bottom:.5rem}
input[type=password]{width:100%;background:#0d1017;border:1px solid var(--border-bright);color:var(--text);font-family:var(--mono);font-size:.95rem;padding:.7rem .9rem;outline:none;transition:border-color .15s}
input[type=password]:focus{border-color:var(--accent)}
button{margin-top:1.2rem;width:100%;background:var(--accent);color:#fff;border:none;font-family:var(--mono);font-size:.85rem;font-weight:500;letter-spacing:.05em;padding:.75rem;cursor:pointer;text-transform:uppercase;transition:opacity .15s}
button:hover{opacity:.85}
.error{margin-top:1rem;padding:.6rem .8rem;background:#1a0a0a;border:1px solid var(--danger);color:var(--danger);font-family:var(--mono);font-size:.78rem}
.footer{margin-top:1.5rem;font-family:var(--mono);font-size:.65rem;color:var(--text-muted);text-align:center}
</style>
</head>
<body>
<div class="card">
  <div class="logo">▸ CF<span>/</span>EMAIL<span>/</span>MONITOR</div>
  <h1>Access Required</h1>
  <p class="sub">Enter your dashboard password to continue.</p>
  <form method="POST" action="/login">
    <label for="pw">Password</label>
    <input type="password" id="pw" name="password" autofocus autocomplete="current-password" required>
    {{if .Error}}<div class="error">⚠ {{.Error}}</div>{{end}}
    <button type="submit">→ Authenticate</button>
  </form>
  <div class="footer">Protected · max 5 attempts / 10 min</div>
</div>
</body>
</html>`

	const logsTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CF Email Monitor</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:ital,wght@0,400;0,500;0,600;1,400&family=IBM+Plex+Sans:wght@300;400;500;600&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#080a0d;--surface:#0d1017;--surface2:#111520;
  --border:#1a2030;--border-bright:#263040;
  --text:#dde4ed;--text-muted:#4a5568;--text-dim:#2d3748;
  --accent:#f97316;--accent2:#3b82f6;
  --success:#22c55e;--danger:#ef4444;--warning:#f59e0b;
  --mono:'IBM Plex Mono',monospace;--sans:'IBM Plex Sans',sans-serif;
}
html,body{min-height:100vh;background:var(--bg);color:var(--text);font-family:var(--sans);font-size:14px}
.header{background:var(--surface);border-bottom:1px solid var(--border);padding:.75rem 1.5rem;display:flex;align-items:center;gap:1rem;position:sticky;top:0;z-index:100;flex-wrap:wrap}
.header-logo{font-family:var(--mono);font-size:.75rem;letter-spacing:.12em;color:var(--accent)}
.header-logo span{color:var(--text-muted)}
.header-zone{font-family:var(--mono);font-size:.7rem;color:var(--text-muted);background:var(--surface2);border:1px solid var(--border);padding:.2rem .6rem}
.header-spacer{flex:1;min-width:1rem}
.header-count{font-family:var(--mono);font-size:.7rem;color:var(--text-muted)}
.header-count strong{color:var(--text)}
.logout-btn{font-family:var(--mono);font-size:.7rem;color:var(--text-muted);text-decoration:none;padding:.3rem .6rem;border:1px solid var(--border);transition:all .15s}
.logout-btn:hover{color:var(--accent);border-color:var(--accent)}
.main{padding:1.25rem 1.5rem;max-width:1600px;margin:0 auto}
.search-panel{background:var(--surface);border:1px solid var(--border);padding:1rem 1.25rem;margin-bottom:1.25rem}
.search-top{display:flex;gap:.75rem;align-items:flex-end;flex-wrap:wrap}
.sg{display:flex;flex-direction:column;gap:.3rem}
.sg label{font-family:var(--mono);font-size:.65rem;letter-spacing:.08em;color:var(--text-muted);text-transform:uppercase}
.sg input,.sg select{background:var(--bg);border:1px solid var(--border-bright);color:var(--text);font-family:var(--mono);font-size:.8rem;padding:.4rem .65rem;outline:none;transition:border-color .15s;border-radius:2px}
.sg input:focus,.sg select:focus{border-color:var(--accent)}
.sg select option{background:var(--surface2)}
.sg-main{flex:1;min-width:180px}
.sg-main input{width:100%}
.search-filters{display:flex;gap:.75rem;flex-wrap:wrap;margin-top:.75rem;padding-top:.75rem;border-top:1px solid var(--border);align-items:flex-end}
.btn{font-family:var(--mono);font-size:.75rem;padding:.4rem .9rem;border:1px solid var(--border-bright);background:var(--surface2);color:var(--text);cursor:pointer;letter-spacing:.03em;transition:all .15s;border-radius:2px;white-space:nowrap;text-decoration:none;display:inline-block}
.btn:hover{border-color:var(--accent);color:var(--accent)}
.btn-primary{background:var(--accent);border-color:var(--accent);color:#fff}
.btn-primary:hover{background:#ea6c0a;color:#fff;border-color:#ea6c0a}
.btn-ghost{border-color:transparent;background:transparent;color:var(--text-muted)}
.btn-ghost:hover{color:var(--text);border-color:var(--border-bright);background:var(--surface2)}
.table-wrap{overflow-x:auto;background:var(--surface);border:1px solid var(--border)}
table{width:100%;border-collapse:collapse;font-size:.8rem}
thead th{font-family:var(--mono);font-size:.65rem;letter-spacing:.08em;text-transform:uppercase;color:var(--text-muted);padding:.6rem .8rem;border-bottom:1px solid var(--border-bright);background:var(--surface2);white-space:nowrap;text-align:left}
tbody tr{border-bottom:1px solid var(--border);transition:background .1s;cursor:pointer}
tbody tr:hover{background:var(--surface2)}
tbody tr.is-spam{border-left:3px solid var(--danger)}
tbody tr.is-ndr{border-left:3px solid var(--warning)}
td{padding:.55rem .8rem;vertical-align:middle}
td.mono{font-family:var(--mono);font-size:.75rem}
td.trunc{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:220px}
td.time-col{white-space:nowrap;color:var(--text-muted);font-family:var(--mono);font-size:.72rem}
.badge{display:inline-block;font-family:var(--mono);font-size:.65rem;padding:.15rem .4rem;border-radius:2px;letter-spacing:.04em;white-space:nowrap}
.badge-success{background:#052e16;color:var(--success);border:1px solid #166534}
.badge-danger{background:#2d0a0a;color:var(--danger);border:1px solid #7f1d1d}
.badge-warning{background:#2d1f00;color:var(--warning);border:1px solid #78350f}
.badge-neutral{background:var(--surface2);color:var(--text-muted);border:1px solid var(--border-bright)}
.auth-pass{color:var(--success);font-family:var(--mono);font-size:.7rem}
.auth-fail{color:var(--danger);font-family:var(--mono);font-size:.7rem}
.auth-none{color:var(--text-dim);font-family:var(--mono);font-size:.7rem}
.auth-neutral{color:var(--text-muted);font-family:var(--mono);font-size:.7rem}
.spam-flag{color:var(--danger);font-family:var(--mono);font-size:.7rem}
.detail-row{display:none;background:var(--bg)}
.detail-row.open{display:table-row}
.detail-cell{padding:1rem 1.2rem;border-bottom:1px solid var(--border-bright)}
.detail-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(210px,1fr));gap:.6rem 2rem}
.df{display:flex;flex-direction:column;gap:.15rem}
.df-label{font-family:var(--mono);font-size:.6rem;letter-spacing:.1em;color:var(--text-muted);text-transform:uppercase}
.df-value{font-family:var(--mono);font-size:.75rem;color:var(--text);word-break:break-all}
.df-subject{grid-column:1/-1}
.df-subject .df-value{font-family:var(--sans);font-size:.9rem}
.pagination{display:flex;align-items:center;gap:.4rem;padding:1rem 0;flex-wrap:wrap}
.page-info{font-family:var(--mono);font-size:.72rem;color:var(--text-muted)}
.page-btn{font-family:var(--mono);font-size:.72rem;padding:.3rem .55rem;border:1px solid var(--border-bright);background:var(--surface2);color:var(--text-muted);cursor:pointer;text-decoration:none;transition:all .15s;border-radius:2px;display:inline-block}
.page-btn:hover{border-color:var(--accent);color:var(--accent)}
.page-btn.active{background:var(--accent);border-color:var(--accent);color:#fff}
.page-btn.disabled{pointer-events:none;opacity:.3;cursor:default}
.page-spacer{color:var(--text-dim);font-family:var(--mono);font-size:.7rem;padding:0 .2rem}
.per-page-wrap{display:flex;align-items:center;gap:.5rem;margin-left:auto}
.per-page-wrap label{font-family:var(--mono);font-size:.65rem;color:var(--text-muted)}
.per-page-wrap select{background:var(--bg);border:1px solid var(--border-bright);color:var(--text);font-family:var(--mono);font-size:.72rem;padding:.25rem .4rem;outline:none;border-radius:2px}
.empty{padding:4rem;text-align:center;color:var(--text-muted)}
.empty-icon{font-size:2rem;margin-bottom:1rem;opacity:.4}
.empty-text{font-family:var(--mono);font-size:.85rem}
@media(max-width:768px){.header{padding:.6rem 1rem}.main{padding:1rem}.hm{display:none}}
</style>
</head>
<body>
<header class="header">
  <span class="header-logo">▸ CF<span>/</span>EMAIL<span>/</span>MONITOR</span>
  <span class="header-zone">{{.ZoneTag}}</span>
  <span class="header-spacer"></span>
  <span class="header-count"><strong>{{.Page.TotalCount}}</strong> records</span>
  <a href="/logout" class="logout-btn">logout →</a>
</header>

<main class="main">
  <form method="GET" action="/" id="sf">
    <div class="search-panel">
      <div class="search-top">
        <div class="sg sg-main">
          <label>Search</label>
          <input type="text" name="q" value="{{.Query.Search}}" placeholder="from, to, subject, message-id…">
        </div>
        <div class="sg">
          <label>From</label>
          <input type="text" name="from" value="{{.Query.FromAddr}}" placeholder="sender@…" style="width:160px">
        </div>
        <div class="sg">
          <label>To</label>
          <input type="text" name="to" value="{{.Query.ToAddr}}" placeholder="recipient@…" style="width:160px">
        </div>
        <div class="sg">
          <label>Status</label>
          <select name="status">
            <option value="">Any</option>
            {{range .Statuses}}<option value="{{.}}"{{if eq . $.Query.Status}} selected{{end}}>{{.}}</option>{{end}}
          </select>
        </div>
        <div class="sg">
          <label>Action</label>
          <select name="action">
            <option value="">Any</option>
            {{range .Actions}}<option value="{{.}}"{{if eq . $.Query.Action}} selected{{end}}>{{.}}</option>{{end}}
          </select>
        </div>
        <div class="sg">
          <label>Spam</label>
          <select name="spam">
            <option value="">Any</option>
            <option value="false">Clean</option>
            <option value="true">Spam only</option>
          </select>
        </div>
        <button type="submit" class="btn btn-primary">Search</button>
        <a href="/" class="btn btn-ghost">Clear</a>
      </div>
      <div class="search-filters">
        <div class="sg">
          <label>Date from</label>
          <input type="datetime-local" name="date_from" value="{{.Query.DateFrom}}" style="width:180px">
        </div>
        <div class="sg">
          <label>Date to</label>
          <input type="datetime-local" name="date_to" value="{{.Query.DateTo}}" style="width:180px">
        </div>
        <input type="hidden" name="page" value="1">
        <input type="hidden" name="per_page" value="{{.Query.PageSize}}">
      </div>
    </div>
  </form>

  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>Time (UTC)</th>
          <th>From</th>
          <th>To</th>
          <th>Subject</th>
          <th>Status</th>
          <th class="hm">Action</th>
          <th class="hm">SPF</th>
          <th class="hm">DKIM</th>
          <th class="hm">DMARC</th>
          <th class="hm">Spam</th>
        </tr>
      </thead>
      <tbody>
        {{if .Page.Rows}}{{range .Page.Rows}}
        <tr onclick="toggle({{.ID}})" class="{{if .IsSpam}}is-spam{{else if .IsNDR}}is-ndr{{end}}">
          <td class="time-col">{{fmtTime .ReceivedAt}}</td>
          <td class="mono trunc" title="{{.FromAddr}}">{{truncate .FromAddr 38}}</td>
          <td class="mono trunc" title="{{.ToAddr}}">{{truncate .ToAddr 38}}</td>
          <td class="trunc" title="{{.Subject}}" style="max-width:260px">{{truncate .Subject 55}}</td>
          <td><span class="badge {{statusColor .Status}}">{{if .Status}}{{.Status}}{{else}}—{{end}}</span></td>
          <td class="mono hm">{{if .Action}}{{.Action}}{{else}}—{{end}}</td>
          <td class="hm"><span class="{{authColor .SPF}}">{{if .SPF}}{{.SPF}}{{else}}—{{end}}</span></td>
          <td class="hm"><span class="{{authColor .DKIM}}">{{if .DKIM}}{{.DKIM}}{{else}}—{{end}}</span></td>
          <td class="hm"><span class="{{authColor .DMARC}}">{{if .DMARC}}{{.DMARC}}{{else}}—{{end}}</span></td>
          <td class="hm">{{if .IsSpam}}<span class="spam-flag" title="score {{.SpamScore}}/{{.SpamThreshold}}">✕ spam</span>{{else}}<span style="color:var(--text-dim)">—</span>{{end}}</td>
        </tr>
        <tr class="detail-row" id="d{{.ID}}">
          <td colspan="10" class="detail-cell">
            <div class="detail-grid">
              <div class="df df-subject"><span class="df-label">Subject</span><span class="df-value">{{if .Subject}}{{.Subject}}{{else}}(no subject){{end}}</span></div>
              <div class="df"><span class="df-label">From</span><span class="df-value">{{.FromAddr}}</span></div>
              <div class="df"><span class="df-label">To</span><span class="df-value">{{.ToAddr}}</span></div>
              <div class="df"><span class="df-label">Received At</span><span class="df-value">{{fmtTime .ReceivedAt}} UTC</span></div>
              <div class="df"><span class="df-label">Status</span><span class="df-value"><span class="badge {{statusColor .Status}}">{{.Status}}</span></span></div>
              <div class="df"><span class="df-label">Action</span><span class="df-value">{{if .Action}}{{.Action}}{{else}}—{{end}}</span></div>
              <div class="df"><span class="df-label">SPF</span><span class="df-value"><span class="{{authColor .SPF}}">{{if .SPF}}{{.SPF}}{{else}}—{{end}}</span></span></div>
              <div class="df"><span class="df-label">DKIM</span><span class="df-value"><span class="{{authColor .DKIM}}">{{if .DKIM}}{{.DKIM}}{{else}}—{{end}}</span></span></div>
              <div class="df"><span class="df-label">DMARC</span><span class="df-value"><span class="{{authColor .DMARC}}">{{if .DMARC}}{{.DMARC}}{{else}}—{{end}}</span></span></div>
              <div class="df"><span class="df-label">ARC</span><span class="df-value"><span class="{{authColor .ARC}}">{{if .ARC}}{{.ARC}}{{else}}—{{end}}</span></span></div>
              <div class="df"><span class="df-label">Spam</span><span class="df-value">{{if .IsSpam}}Yes ({{.SpamScore}}/{{.SpamThreshold}}){{else}}No{{end}}</span></div>
              <div class="df"><span class="df-label">NDR</span><span class="df-value">{{if .IsNDR}}Yes{{else}}No{{end}}</span></div>
              {{if .ErrorDetail}}<div class="df" style="grid-column:1/-1"><span class="df-label">Error</span><span class="df-value" style="color:var(--danger)">{{.ErrorDetail}}</span></div>{{end}}
              <div class="df"><span class="df-label">Message-ID</span><span class="df-value">{{if .MessageID}}{{.MessageID}}{{else}}—{{end}}</span></div>
              <div class="df"><span class="df-label">Session-ID</span><span class="df-value">{{.SessionID}}</span></div>
            </div>
          </td>
        </tr>
        {{end}}{{else}}
        <tr><td colspan="10">
          <div class="empty">
            <div class="empty-icon">◌</div>
            <div class="empty-text">No records found{{if .Query.Search}} for "{{.Query.Search}}"{{end}}</div>
          </div>
        </td></tr>
        {{end}}
      </tbody>
    </table>
  </div>

  {{if gt .Page.TotalPages 1}}
  <div class="pagination">
    <span class="page-info">page {{.Page.Page}} of {{.Page.TotalPages}} · {{.Page.TotalCount}} total</span>
    {{if gt .Page.Page 1}}
      <a href="?{{buildQuery .Query 1}}" class="page-btn">«</a>
      <a href="?{{buildQuery .Query (sub .Page.Page 1)}}" class="page-btn">‹</a>
    {{else}}
      <span class="page-btn disabled">«</span>
      <span class="page-btn disabled">‹</span>
    {{end}}
    {{$cur := .Page.Page}}{{$tot := .Page.TotalPages}}
    {{$s := paginationStart $cur $tot}}{{$e := paginationEnd $cur $tot}}
    {{if gt $s 1}}<span class="page-spacer">…</span>{{end}}
    {{range seq $s $e}}
      <a href="?{{buildQuery $.Query .}}" class="page-btn{{if eq . $cur}} active{{end}}">{{.}}</a>
    {{end}}
    {{if lt $e $tot}}<span class="page-spacer">…</span>{{end}}
    {{if lt .Page.Page .Page.TotalPages}}
      <a href="?{{buildQuery .Query (add .Page.Page 1)}}" class="page-btn">›</a>
      <a href="?{{buildQuery .Query .Page.TotalPages}}" class="page-btn">»</a>
    {{else}}
      <span class="page-btn disabled">›</span>
      <span class="page-btn disabled">»</span>
    {{end}}
    <div class="per-page-wrap">
      <label>Per page</label>
      <select onchange="setPerPage(this.value)">
        {{range seq 1 4}}{{$v := mul . 25}}<option value="{{$v}}"{{if eq $v $.Query.PageSize}} selected{{end}}>{{$v}}</option>{{end}}
        <option value="100"{{if eq 100 $.Query.PageSize}} selected{{end}}>100</option>
      </select>
    </div>
  </div>
  {{end}}
</main>

<script>
function toggle(id){var r=document.getElementById('d'+id);if(r)r.classList.toggle('open');}
function setPerPage(v){var f=document.getElementById('sf');f.querySelector('[name=per_page]').value=v;f.querySelector('[name=page]').value=1;f.submit();}
document.addEventListener('keydown',function(e){
  if(e.key==='/'&&document.activeElement.tagName!=='INPUT'&&document.activeElement.tagName!=='SELECT'){e.preventDefault();document.querySelector('input[name=q]').focus();}
  if(e.key==='Escape'){document.querySelectorAll('.detail-row.open').forEach(function(r){r.classList.remove('open');});}
});
</script>
</body>
</html>`

	const errorTmpl = `<!DOCTYPE html>
<html><head><title>Error</title></head>
<body style="background:#080a0d;color:#ef4444;font-family:monospace;padding:2rem">
<h2>Error</h2><p>{{.Error}}</p><a href="/" style="color:#f97316">← Back</a>
</body></html>`

	t := template.Must(template.New("login.html").Funcs(funcs).Parse(loginTmpl))
	template.Must(t.New("logs.html").Funcs(funcs).Parse(logsTmpl))
	template.Must(t.New("error.html").Funcs(funcs).Parse(errorTmpl))
	return t
}

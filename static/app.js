"use strict";

/* When shown inside the pfSense web GUI, requests go through a PHP proxy that
   pfSense injects as window.NETMON_PROXY = {base, csrf, csrfName}. */
const PROXY = window.NETMON_PROXY || null;
const RANGES = { "24h": 864e5, "7d": 7 * 864e5, "30d": 30 * 864e5, "90d": 90 * 864e5 };
const MAX_POINTS = 600;
const PAGE_SIZE = 100;

const state = {
    view: "overview",
    range: store("range") || "24h",
    customFrom: "",
    customTo: "",
    status: null,
    lastRunEnd: null,
    summary: null,
    traceHost: null,
    historyKind: "speedtest",
    historyHost: "",
    historyRows: [],
    historyShown: PAGE_SIZE,
    config: null,
};
const charts = {};

/* ---------- Helpers ---------- */
const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

function store(key, value) {
    try {
        if (value === undefined) return localStorage.getItem("netmon." + key);
        localStorage.setItem("netmon." + key, value);
    } catch (e) { return null; }
}

function esc(v) {
    return String(v ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function url(path, params) {
    const qs = new URLSearchParams();
    Object.entries(params || {}).forEach(([k, v]) => { if (v !== undefined && v !== null && v !== "") qs.set(k, v); });
    const q = qs.toString();
    if (PROXY) return PROXY.base + encodeURIComponent(path) + (q ? "&" + q : "");
    return path + (q ? "?" + q : "");
}

async function apiGet(path, params) {
    const res = await fetch(url(path, params), { credentials: "same-origin", cache: "no-store" });
    if (!res.ok) throw new Error(await errorText(res));
    return res.json();
}

async function apiPost(path, body) {
    let opts;
    if (PROXY) {
        // pfSense checks its CSRF token on every POST, so send it as a form field.
        const form = new URLSearchParams();
        form.set(PROXY.csrfName || "__csrf_magic", PROXY.csrf || "");
        form.set("body", JSON.stringify(body));
        opts = { method: "POST", body: form };
    } else {
        opts = { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) };
    }
    const res = await fetch(url(path), { ...opts, credentials: "same-origin" });
    if (!res.ok) throw new Error(await errorText(res));
    return res.json();
}

async function errorText(res) {
    try { const j = await res.json(); return j.error || res.statusText; } catch (e) { return res.status + " " + res.statusText; }
}

function cssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function fmtMbps(v) {
    if (v === null || v === undefined || isNaN(v)) return "--";
    return v >= 100 ? v.toFixed(0) : v.toFixed(1);
}

function fmtMs(v) {
    if (v === null || v === undefined || isNaN(v) || v < 0) return "--";
    return v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2);
}

function fmtPct(v) {
    if (v === null || v === undefined || isNaN(v)) return "--";
    return (v >= 99.95 || v === 0 ? v.toFixed(0) : v.toFixed(1)) + "%";
}

function fmtDate(ts, withDate = true) {
    if (!ts) return "--";
    const d = new Date(ts);
    if (isNaN(d)) return ts;
    const time = d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
    if (!withDate) return time;
    return d.toLocaleDateString([], { year: "numeric", month: "short", day: "numeric" }) + " " + time;
}

function fmtRelative(ts) {
    const diff = (new Date(ts) - Date.now()) / 1000;
    const abs = Math.abs(diff);
    let txt;
    if (abs < 60) txt = "less than a minute";
    else if (abs < 3600) txt = Math.round(abs / 60) + " min";
    else if (abs < 86400) txt = (abs / 3600).toFixed(abs < 36000 ? 1 : 0).replace(/\.0$/, "") + " h";
    else txt = Math.round(abs / 86400) + " d";
    return diff >= 0 ? "in " + txt : txt + " ago";
}

function latencyClass(ms) {
    if (ms < 0) return "bad";
    if (ms < 50) return "good";
    if (ms < 100) return "warn";
    return "bad";
}

function toast(msg, bad = false) {
    const t = $("#toast");
    t.textContent = msg;
    t.className = "toast" + (bad ? " bad" : "");
    t.hidden = false;
    clearTimeout(toast.timer);
    toast.timer = setTimeout(() => { t.hidden = true; }, bad ? 6000 : 3000);
}

function rangeParams() {
    if (state.range === "all") return { from: "1970-01-01T00:00:00Z" };
    if (state.range === "custom" && state.customFrom) {
        const from = new Date(state.customFrom + "T00:00:00");
        const to = state.customTo ? new Date(state.customTo + "T23:59:59") : new Date();
        return { from: from.toISOString(), to: to.toISOString() };
    }
    return { from: new Date(Date.now() - (RANGES[state.range] || RANGES["24h"])).toISOString() };
}

function rangeSpanMs() {
    const p = rangeParams();
    return (p.to ? new Date(p.to) : Date.now()) - new Date(p.from);
}

/* ---------- Navigation ---------- */
function showView(view) {
    if (!["overview", "history", "settings"].includes(view)) view = "overview";
    state.view = view;
    $$(".view").forEach(v => { v.hidden = v.id !== "view-" + view; });
    $$(".tab").forEach(t => t.classList.toggle("active", t.dataset.view === view));
    $("#rangeBar").hidden = view === "settings";
    if (view === "overview") loadOverview();
    if (view === "history") loadHistory();
    if (view === "settings") loadSettings();
}

function setupRange() {
    $$(".chip").forEach(c => {
        c.classList.toggle("active", c.dataset.range === state.range);
        c.addEventListener("click", () => {
            $$(".chip").forEach(x => x.classList.toggle("active", x === c));
            $("#customRange").hidden = c.dataset.range !== "custom";
            if (c.dataset.range === "custom") {
                if (!$("#rangeFrom").value) {
                    const d = new Date(Date.now() - 7 * 864e5);
                    $("#rangeFrom").value = d.toLocaleDateString("en-CA");
                    $("#rangeTo").value = new Date().toLocaleDateString("en-CA");
                }
                return;
            }
            state.range = c.dataset.range;
            store("range", state.range);
            refreshData();
        });
    });
    if (state.range === "custom") state.range = "24h";
    $$(".chip").forEach(c => c.classList.toggle("active", c.dataset.range === state.range));
    $("#applyRange").addEventListener("click", () => {
        if (!$("#rangeFrom").value) return toast("Pick a start date", true);
        state.range = "custom";
        state.customFrom = $("#rangeFrom").value;
        state.customTo = $("#rangeTo").value;
        refreshData();
    });
}

function refreshData() {
    if (state.view === "overview") loadOverview();
    if (state.view === "history") loadHistory();
}

/* ---------- Status / run now ---------- */
async function pollStatus() {
    try {
        const s = await apiGet("api/status");
        state.status = s;
        renderStatus(s);
        if (state.lastRunEnd !== null && s.run.last_end && s.run.last_end !== state.lastRunEnd) {
            refreshData();
        }
        state.lastRunEnd = s.run.last_end || "";
        setTimeout(pollStatus, s.run.running ? 3000 : 15000);
    } catch (e) {
        $("#runStatus").innerHTML = '<span class="dot busy" style="background:var(--bad)"></span>Can\'t reach the monitor';
        setTimeout(pollStatus, 10000);
    }
}

function renderStatus(s) {
    const el = $("#runStatus");
    const btn = $("#runNow");
    if (s.run.running) {
        el.innerHTML = `<span class="dot busy"></span>Running ${esc(s.run.step || "tests")}…`;
        btn.disabled = true;
    } else {
        const next = s.next_run ? `Next test ${fmtRelative(s.next_run)}` : "";
        const last = s.run.last_end ? ` · last ${fmtRelative(s.run.last_end)}` : "";
        el.innerHTML = `<span class="dot"></span>${esc(next)}${esc(last)}`;
        el.title = s.next_run ? "Next test at " + fmtDate(s.next_run) + " (every " + s.run_every + ")" : "";
        btn.disabled = false;
    }
    $("#versionInfo").textContent = `Network Monitor ${s.version} · ${s.os}/${s.arch}`;

    const alerts = [];
    const deps = s.dependencies;
    if (!deps.ping) alerts.push(["bad", "ping is not installed", "Ping tests are skipped until it's available."]);
    if (!deps.traceroute) alerts.push(["warn", (s.os === "windows" ? "tracert" : "traceroute") + " is not installed", installHint("traceroute", s.os)]);
    if (!deps.speedtest || !deps.speedtest.name) alerts.push(["bad", "No speed test program found", installHint("speedtest", s.os)]);
    Object.entries(s.run.errors || {}).forEach(([tool, msg]) => {
        if (/not installed|no speedtest/i.test(msg)) return;
        alerts.push(["bad", `Last ${tool} run failed`, esc(msg)]);
    });
    $("#alerts").innerHTML = alerts.map(([cls, title, body]) =>
        `<div class="alert ${cls}"><strong>${esc(title)}</strong>${body}</div>`).join("");
}

function installHint(tool, os) {
    if (tool === "traceroute") {
        return os === "linux" ? "Install it with your package manager, e.g. <code>sudo apt install traceroute</code> or <code>sudo dnf install traceroute</code>." : "";
    }
    const hints = {
        windows: "Run <code>winget install Ookla.Speedtest.CLI</code>, or put <code>speedtest.exe</code> next to this program.",
        darwin: "Run <code>brew tap teamookla/speedtest &amp;&amp; brew install speedtest</code>.",
        freebsd: "On pfSense/FreeBSD run <code>pkg install -y py311-speedtest-cli</code> (or the Ookla FreeBSD package).",
        linux: 'Install the Ookla CLI from <a href="https://www.speedtest.net/apps/cli" target="_blank" rel="noopener">speedtest.net/apps/cli</a> (or <code>pip install speedtest-cli</code>).',
    };
    return hints[os] || hints.linux;
}

async function runNow() {
    const btn = $("#runNow");
    btn.disabled = true;
    try {
        await apiPost("api/run", {});
        toast("Test run started. Results will show up in a minute or two.");
        $("#runStatus").innerHTML = '<span class="dot busy"></span>Starting…';
        setTimeout(pollStatus, 1500);
    } catch (e) {
        toast(e.message, true);
        btn.disabled = false;
    }
}

/* ---------- Overview ---------- */
async function loadOverview() {
    try {
        const [summary, speeds, pings] = await Promise.all([
            apiGet("api/summary", rangeParams()),
            apiGet("api/speedtest", rangeParams()),
            apiGet("api/ping", rangeParams()),
        ]);
        state.summary = summary;
        renderKpis(summary);
        renderSpeedChart(speeds, summary.speedtest);
        renderPingChart(pings);
        renderHosts(summary.ping);
        renderLatestSpeed(summary.speedtest.latest);
        renderTraceroutes(summary.traceroutes);
    } catch (e) {
        toast("Couldn't load data: " + e.message, true);
    }
}

function kpi(label, value, unit, sub, meter) {
    let m = "";
    if (meter) {
        const pct = Math.max(0, Math.min(100, meter.pct));
        const cls = meter.pct >= 80 ? "" : meter.pct >= 50 ? "warn" : "bad";
        m = `<div class="meter"><span class="${cls}" style="width:${pct}%"></span></div><div class="kpi-sub">${meter.label}</div>`;
    }
    return `<div class="kpi"><div class="kpi-label">${esc(label)}</div>
        <div class="kpi-value">${value}${unit ? `<small>${esc(unit)}</small>` : ""}</div>
        <div class="kpi-sub">${sub}</div>${m}</div>`;
}

function renderKpis(s) {
    const st = s.speedtest, p = s.ping;
    const latest = st.latest;
    const tiles = [];

    const speedTile = (label, latestVal, stats, plan, metPct) => {
        let meter = null;
        if (plan > 0 && latestVal !== undefined) {
            const pct = latestVal / plan * 100;
            meter = { pct, label: `${pct.toFixed(0)}% of your ${fmtMbps(plan)} Mbps plan` + (metPct !== null && metPct !== undefined ? ` · ≥80% in ${fmtPct(metPct)} of tests` : "") };
        }
        const sub = stats.count ? `avg ${fmtMbps(stats.avg)} · min ${fmtMbps(stats.min)} · max ${fmtMbps(stats.max)}` : "no tests in range";
        return kpi(label, fmtMbps(latestVal), "Mbps", sub, meter);
    };
    tiles.push(speedTile("Download", latest?.download_mbps, st.download_mbps, st.plan_download_mbps, st.download_meets_plan_pct));
    tiles.push(speedTile("Upload", latest?.upload_mbps, st.upload_mbps, st.plan_upload_mbps, st.upload_meets_plan_pct));

    tiles.push(kpi("Latency", fmtMs(p.latency_ms.count ? p.latency_ms.avg : null), "ms",
        p.latency_ms.count ? `median ${fmtMs(p.latency_ms.median)} · max ${fmtMs(p.latency_ms.max)}` : "no pings in range"));
    tiles.push(kpi("Jitter", fmtMs(p.count ? p.avg_jitter_ms : null), "ms", "average variation between pings"));
    tiles.push(kpi("Packet loss", p.count ? `<span class="${p.avg_loss_pct > 1 ? "bad" : p.avg_loss_pct > 0 ? "warn" : "good"}">${fmtPct(p.avg_loss_pct)}</span>` : "--", "",
        "average across all hosts"));
    tiles.push(kpi("Uptime", p.count ? `<span class="${p.uptime_pct < 99 ? "bad" : p.uptime_pct < 100 ? "warn" : "good"}">${fmtPct(p.uptime_pct)}</span>` : "--", "",
        p.count ? `${p.count - p.success} failed of ${p.count} pings` : "no pings in range"));
    $("#kpis").innerHTML = tiles.join("");
}

function downsample(points) {
    if (points.length <= MAX_POINTS) return points;
    const size = Math.ceil(points.length / MAX_POINTS);
    const out = [];
    for (let i = 0; i < points.length; i += size) {
        const chunk = points.slice(i, i + size);
        const vals = chunk.filter(p => p.y !== null);
        out.push({ x: chunk[Math.floor(chunk.length / 2)].x, y: vals.length ? vals.reduce((a, b) => a + b.y, 0) / vals.length : null });
    }
    return out;
}

function timeAxis(xs) {
    const span = rangeSpanMs();
    return {
        type: "linear",
        min: xs && xs.length ? Math.min(...xs) : undefined,
        max: xs && xs.length ? Math.max(...xs) : undefined,
        grid: { color: cssVar("--grid") },
        ticks: {
            color: cssVar("--muted"),
            maxTicksLimit: 8,
            callback: v => {
                const d = new Date(v);
                return span <= 2 * 864e5
                    ? d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })
                    : d.toLocaleDateString([], { month: "short", day: "numeric" });
            },
        },
    };
}

function chartOptions(yTitle, xs, extra = {}) {
    return {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        parsing: false,
        interaction: { mode: "nearest", axis: "x", intersect: false },
        plugins: {
            legend: { position: "top", align: "end", labels: { color: cssVar("--text"), boxWidth: 12, boxHeight: 12, usePointStyle: true } },
            tooltip: { callbacks: { title: items => items.length ? fmtDate(items[0].parsed.x) : "" } },
        },
        scales: {
            x: timeAxis(xs),
            y: { beginAtZero: true, grid: { color: cssVar("--grid") }, ticks: { color: cssVar("--muted") }, title: { display: true, text: yTitle, color: cssVar("--muted") } },
            ...extra,
        },
    };
}

function drawChart(id, config) {
    if (charts[id]) charts[id].destroy();
    charts[id] = new Chart($("#" + id), config);
}

function renderSpeedChart(rows, st) {
    $("#speedEmpty").hidden = rows.length > 0;
    $("#speedChart").parentElement.hidden = rows.length === 0;
    $("#speedCount").textContent = rows.length ? `${rows.length} test${rows.length === 1 ? "" : "s"}` : "";
    if (!rows.length) return;

    const pts = key => downsample(rows.map(r => ({ x: Date.parse(r.timestamp), y: r[key] })));
    const line = (label, data, color) => ({
        label, data, borderColor: color, backgroundColor: color, borderWidth: 2,
        pointRadius: data.length > 60 ? 0 : 3, pointHoverRadius: 4, tension: 0.25,
    });
    const datasets = [line("Download", pts("download_mbps"), cssVar("--down")), line("Upload", pts("upload_mbps"), cssVar("--up"))];

    const xs = rows.map(r => Date.parse(r.timestamp));
    const planLine = (label, value, color) => ({
        label, data: [{ x: Math.min(...xs), y: value }, { x: Math.max(...xs), y: value }],
        borderColor: color, borderDash: [6, 5], borderWidth: 1.5, pointRadius: 0, fill: false,
    });
    if (st.plan_download_mbps > 0) datasets.push(planLine("Download plan", st.plan_download_mbps, cssVar("--down")));
    if (st.plan_upload_mbps > 0) datasets.push(planLine("Upload plan", st.plan_upload_mbps, cssVar("--up")));

    const opts = chartOptions("Mbps", xs);
    opts.plugins.tooltip.callbacks.label = c => `${c.dataset.label}: ${fmtMbps(c.parsed.y)} Mbps`;
    drawChart("speedChart", { type: "line", data: { datasets }, options: opts });
}

const HOST_COLORS = ["#38bdf8", "#a78bfa", "#f472b6", "#fb923c", "#2dd4bf", "#facc15", "#60a5fa", "#4ade80"];

function renderPingChart(rows) {
    $("#pingEmpty").hidden = rows.length > 0;
    $("#pingChart").parentElement.hidden = rows.length === 0;
    $("#pingCount").textContent = rows.length ? `${rows.length} pings` : "";
    if (!rows.length) return;

    const byHost = {};
    const lossByMinute = {};
    rows.forEach(r => {
        const x = Date.parse(r.timestamp);
        (byHost[r.host] = byHost[r.host] || []).push({ x, y: r.success ? r.time_ms : null });
        const minute = Math.round(x / 60000) * 60000;
        const l = (lossByMinute[minute] = lossByMinute[minute] || { sum: 0, n: 0 });
        l.sum += r.loss_pct;
        l.n++;
    });

    const datasets = Object.keys(byHost).sort().map((host, i) => {
        const data = downsample(byHost[host]);
        const color = HOST_COLORS[i % HOST_COLORS.length];
        return { label: host, data, borderColor: color, backgroundColor: color, borderWidth: 1.5, pointRadius: data.length > 60 ? 0 : 2, tension: 0.2, spanGaps: false, yAxisID: "y" };
    });
    const loss = downsample(Object.entries(lossByMinute).map(([x, l]) => ({ x: +x, y: l.sum / l.n })).sort((a, b) => a.x - b.x));
    if (loss.some(p => p.y > 0)) {
        datasets.push({ type: "bar", label: "Packet loss %", data: loss, backgroundColor: cssVar("--loss"), yAxisID: "y2", barThickness: 4, order: 10 });
    }

    const opts = chartOptions("ms", rows.map(r => Date.parse(r.timestamp)), {
        y2: { position: "right", min: 0, max: 100, grid: { display: false }, ticks: { color: cssVar("--muted"), callback: v => v + "%" }, display: datasets.some(d => d.yAxisID === "y2") },
    });
    opts.plugins.tooltip.callbacks.label = c => c.dataset.yAxisID === "y2" ? `Loss: ${fmtPct(c.parsed.y)}` : `${c.dataset.label}: ${fmtMs(c.parsed.y)} ms`;
    drawChart("pingChart", { type: "line", data: { datasets }, options: opts });
}

function renderHosts(p) {
    const tbody = $("#hostTable");
    if (!p.hosts.length) {
        tbody.innerHTML = `<tr><td colspan="7" class="muted">No pings in this range.</td></tr>`;
        return;
    }
    tbody.innerHTML = p.hosts.map(h => {
        const last = h.last;
        const lastCell = last.success ? `<span class="${latencyClass(last.time_ms)}">${fmtMs(last.time_ms)} ms</span>` : `<span class="pill bad">down</span>`;
        const up = h.uptime_pct;
        return `<tr><td>${esc(h.host)}</td><td>${lastCell}</td>
            <td>${h.latency_ms.count ? fmtMs(h.latency_ms.avg) + " ms" : "--"}</td>
            <td>${h.latency_ms.count ? fmtMs(h.latency_ms.min) + " / " + fmtMs(h.latency_ms.max) : "--"}</td>
            <td>${h.latency_ms.count ? fmtMs(h.avg_jitter_ms) + " ms" : "--"}</td>
            <td class="${h.avg_loss_pct > 1 ? "bad" : h.avg_loss_pct > 0 ? "warn" : ""}">${fmtPct(h.avg_loss_pct)}</td>
            <td class="${up < 99 ? "bad" : up < 100 ? "warn" : "good"}">${fmtPct(up)}</td></tr>`;
    }).join("");
}

function renderLatestSpeed(t) {
    const el = $("#latestSpeed");
    if (!t) {
        el.innerHTML = `<dt>Status</dt><dd class="muted">No speed tests yet.</dd>`;
        return;
    }
    const rows = [
        ["When", fmtDate(t.timestamp)],
        ["Download", fmtMbps(t.download_mbps) + " Mbps"],
        ["Upload", fmtMbps(t.upload_mbps) + " Mbps"],
        ["Ping", fmtMs(t.ping_ms) + " ms" + (t.jitter_ms !== null ? ` (jitter ${fmtMs(t.jitter_ms)} ms)` : "")],
    ];
    if (t.packet_loss !== null) rows.push(["Packet loss", fmtPct(t.packet_loss)]);
    if (t.isp) rows.push(["ISP", esc(t.isp)]);
    rows.push(["Server", esc([t.server_location, t.server_id ? "#" + t.server_id : ""].filter(Boolean).join(" ") || "--")]);
    if (t.result_url && /^https:\/\//.test(t.result_url)) {
        rows.push(["Result", `<a href="${esc(t.result_url)}" target="_blank" rel="noopener">View on speedtest.net ↗</a>`]);
    }
    el.innerHTML = rows.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
}

function hopAvg(h) {
    const v = (h.times || []).filter(t => t !== null);
    return v.length ? v.reduce((a, b) => a + b, 0) / v.length : null;
}

function traceTable(t) {
    const max = Math.max(1, ...t.hops.map(h => hopAvg(h) || 0));
    if (!t.hops.length) return `<p class="muted">No hops recorded.</p>`;
    // Collapse the run of silent hops at the end (common when the target drops probes).
    let hops = t.hops;
    let tail = "";
    let end = hops.length;
    while (end > 0 && hops[end - 1].ip === "*") end--;
    if (hops.length - end > 1) {
        tail = `<tr><td>${hops[end].hop}–${hops[hops.length - 1].hop}</td><td colspan="4" class="muted">no reply (the destination or a router ignores traceroute probes)</td></tr>`;
        hops = hops.slice(0, end);
    }
    return `<div class="table-wrap"><table><thead><tr><th>Hop</th><th>IP</th><th>Avg</th><th>Times</th><th style="width:40%"></th></tr></thead><tbody>` +
        hops.map(h => {
            const avg = hopAvg(h);
            const times = (h.times || []).map(x => x === null ? "*" : fmtMs(x)).join(" / ");
            return `<tr><td>${h.hop}</td><td>${h.ip === "*" ? '<span class="muted">no reply</span>' : esc(h.ip)}</td>
                <td>${avg === null ? "--" : fmtMs(avg) + " ms"}</td><td class="muted">${esc(times)}</td>
                <td>${avg === null ? "" : `<span class="hop-bar" style="width:${(avg / max * 100).toFixed(1)}%"></span>`}</td></tr>`;
        }).join("") + tail + `</tbody></table></div>`;
}

function renderTraceroutes(list) {
    const seg = $("#traceHosts");
    const view = $("#traceView");
    if (!list.length) {
        seg.innerHTML = "";
        view.innerHTML = `<p class="empty">No traceroutes yet.</p>`;
        return;
    }
    if (!list.some(t => t.host === state.traceHost)) state.traceHost = list[0].host;
    seg.innerHTML = list.length > 1 ? list.map(t => `<button data-host="${esc(t.host)}" class="${t.host === state.traceHost ? "active" : ""}">${esc(t.host)}</button>`).join("") : "";
    $$("button", seg).forEach(b => b.addEventListener("click", () => { state.traceHost = b.dataset.host; renderTraceroutes(list); }));
    const t = list.find(x => x.host === state.traceHost);
    view.innerHTML = `<p class="muted" style="margin-top:0">${esc(t.host)} · ${fmtDate(t.timestamp)} · ${t.hops.length} hops</p>` + traceTable(t);
}

/* ---------- History ---------- */
function setupHistory() {
    $$("#historyTabs button").forEach(b => b.addEventListener("click", () => {
        $$("#historyTabs button").forEach(x => x.classList.toggle("active", x === b));
        state.historyKind = b.dataset.kind;
        loadHistory();
    }));
    $("#historyHost").addEventListener("change", e => { state.historyHost = e.target.value; loadHistory(); });
    $("#historyMore").addEventListener("click", () => { state.historyShown += PAGE_SIZE; renderHistory(); });
}

async function loadHistory() {
    const kind = state.historyKind;
    const params = rangeParams();
    const hostSel = $("#historyHost");
    hostSel.hidden = kind !== "ping";
    const exportParams = { ...params, host: kind === "ping" ? state.historyHost : "" };
    $("#exportCsv").href = url("api/export", { ...exportParams, type: kind, format: "csv" });
    $("#exportJson").href = url("api/export", { ...exportParams, type: kind, format: "json" });
    $("#exportAll").href = url("api/export", { ...params, type: "all", format: "json" });

    try {
        if (!state.config) state.config = await apiGet("api/config");
        if (kind === "ping") {
            const hosts = state.summary ? state.summary.ping.hosts.map(h => h.host) : [];
            const cfgHosts = state.config ? state.config.ping_host : [];
            const all = [...new Set([...hosts, ...cfgHosts])].sort();
            hostSel.innerHTML = `<option value="">All hosts</option>` + all.map(h => `<option ${h === state.historyHost ? "selected" : ""}>${esc(h)}</option>`).join("");
        }
        const rows = kind === "traceroute"
            ? await apiGet("api/traceroute", params)
            : await apiGet("api/" + kind, { ...params, host: kind === "ping" ? state.historyHost : "" });
        state.historyRows = kind === "traceroute" ? rows : rows.reverse();
        state.historyShown = PAGE_SIZE;
        renderHistory();
    } catch (e) {
        toast("Couldn't load history: " + e.message, true);
    }
}

function renderHistory() {
    const rows = state.historyRows.slice(0, state.historyShown);
    const el = $("#historyTable");
    $("#historyMore").hidden = state.historyRows.length <= state.historyShown;
    if (!rows.length) {
        el.innerHTML = `<p class="empty">Nothing recorded in this range.</p>`;
        return;
    }
    const plan = state.config || {};
    if (state.historyKind === "speedtest") {
        el.innerHTML = `<table><thead><tr><th>Date</th><th>Download</th><th>Upload</th><th>Ping</th><th>Jitter</th><th>Loss</th><th>ISP</th><th>Server</th><th></th></tr></thead><tbody>` +
            rows.map(t => {
                const dCls = plan.plan_download_mbps > 0 && t.download_mbps < plan.plan_download_mbps * 0.8 ? "bad" : "";
                const uCls = plan.plan_upload_mbps > 0 && t.upload_mbps < plan.plan_upload_mbps * 0.8 ? "bad" : "";
                const link = t.result_url && /^https:\/\//.test(t.result_url) ? `<a href="${esc(t.result_url)}" target="_blank" rel="noopener">result ↗</a>` : "";
                return `<tr><td>${fmtDate(t.timestamp)}</td><td class="${dCls}">${fmtMbps(t.download_mbps)} Mbps</td><td class="${uCls}">${fmtMbps(t.upload_mbps)} Mbps</td>
                    <td>${fmtMs(t.ping_ms)} ms</td><td>${t.jitter_ms === null ? "--" : fmtMs(t.jitter_ms) + " ms"}</td><td>${t.packet_loss === null ? "--" : fmtPct(t.packet_loss)}</td>
                    <td>${esc(t.isp || "--")}</td><td>${esc(t.server_location || "--")}${t.server_id ? ` <span class="muted">#${esc(t.server_id)}</span>` : ""}</td><td>${link}</td></tr>`;
            }).join("") + `</tbody></table>`;
    } else if (state.historyKind === "ping") {
        el.innerHTML = `<table><thead><tr><th>Date</th><th>Host</th><th>Status</th><th>Latency</th><th>Jitter</th><th>Loss</th></tr></thead><tbody>` +
            rows.map(p => `<tr><td>${fmtDate(p.timestamp)}</td><td>${esc(p.host)}</td>
                <td>${p.success ? '<span class="pill good">up</span>' : '<span class="pill bad">down</span>'}</td>
                <td class="${latencyClass(p.time_ms)}">${p.success ? fmtMs(p.time_ms) + " ms" : "--"}</td>
                <td>${p.success ? fmtMs(p.jitter_ms) + " ms" : "--"}</td>
                <td class="${p.loss_pct > 0 ? "bad" : ""}">${fmtPct(p.loss_pct)}</td></tr>`).join("") + `</tbody></table>`;
    } else {
        el.innerHTML = rows.map(t => {
            const last = t.hops.length ? t.hops[t.hops.length - 1] : null;
            const total = last ? hopAvg(last) : null;
            return `<details class="trace"><summary>${fmtDate(t.timestamp)} · <strong>${esc(t.host)}</strong> · ${t.hops.length} hops${total !== null ? " · " + fmtMs(total) + " ms" : ""}</summary>${traceTable(t)}</details>`;
        }).join("");
    }
}

/* ---------- Settings ---------- */
async function loadSettings() {
    try {
        const cfg = await apiGet("api/config");
        state.config = cfg;
        fillForm(cfg);
    } catch (e) {
        toast("Couldn't load settings: " + e.message, true);
    }
}

function fillForm(cfg) {
    const f = $("#settingsForm");
    Object.entries(cfg).forEach(([k, v]) => {
        const input = f.elements[k];
        if (!input) return;
        if (input.type === "checkbox") input.checked = !!v;
        else if (Array.isArray(v)) input.value = v.join("\n");
        else if (input.type === "password") input.value = "";
        else input.value = v ?? "";
    });
    f.elements.clear_ui_password.checked = false;
    $("#clearUiPasswordWrap").hidden = !cfg.ui_password_set;
    f.elements.ui_password.placeholder = cfg.ui_password_set ? "Unchanged" : "No password";
    f.elements.sql_password.placeholder = cfg.sql_password_set ? "Unchanged" : "";
    $("#sqlFields").hidden = !cfg.run_sql;
    const s = state.status;
    if (s) {
        $("#configPath").textContent = "Saved to " + s.config_path;
        $("#dbLocation").textContent = "Currently using " + s.database;
        const sp = s.dependencies.speedtest;
        $("#speedtestFound").textContent = sp && sp.name ? `Using ${sp.name === "ookla" ? "Ookla CLI" : "speedtest-cli"} at ${sp.path}` : "No speed test program found.";
    }
}

function setupSettings() {
    const f = $("#settingsForm");
    f.elements.run_sql.addEventListener("change", e => { $("#sqlFields").hidden = !e.target.checked; });
    f.addEventListener("submit", async e => {
        e.preventDefault();
        const body = {};
        const lists = ["ping_host", "traceroute_host"];
        const numbers = ["ping_count", "retention_days", "plan_download_mbps", "plan_upload_mbps"];
        Array.from(f.elements).forEach(input => {
            if (!input.name) return;
            if (input.type === "checkbox") body[input.name] = input.checked;
            else if (lists.includes(input.name)) body[input.name] = input.value.split(/[\s,]+/).filter(Boolean);
            else if (numbers.includes(input.name)) body[input.name] = input.value === "" ? 0 : Number(input.value);
            else body[input.name] = input.value.trim();
        });
        const btn = $("button[type=submit]", f);
        btn.disabled = true;
        try {
            const res = await apiPost("api/config", body);
            state.config = res.config;
            fillForm(res.config);
            toast(res.restart_required ? "Saved. Restart the monitor to use the new listen address." : "Settings saved.");
            pollStatusOnce();
        } catch (err) {
            toast(err.message, true);
        } finally {
            btn.disabled = false;
        }
    });
}

async function pollStatusOnce() {
    try { state.status = await apiGet("api/status"); renderStatus(state.status); } catch (e) { /* next poll will retry */ }
}

/* ---------- Theme ---------- */
function setupTheme() {
    $("#themeToggle").addEventListener("click", () => {
        const current = document.documentElement.dataset.theme ||
            (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
        const next = current === "dark" ? "light" : "dark";
        document.documentElement.dataset.theme = next;
        store("theme", next);
        if (state.view === "overview" && state.summary) loadOverview();
    });
}

/* ---------- Start ---------- */
(function init() {
    if (PROXY || window.self !== window.top) document.body.classList.add("embedded");
    if (window.self !== window.top && window.ResizeObserver) {
        // Lets the pfSense page size its frame to fit (only delivered to the same origin).
        const report = () => window.parent.postMessage({ netmonHeight: document.documentElement.scrollHeight }, location.origin);
        new ResizeObserver(report).observe(document.body);
    }
    $$(".api-link").forEach(a => { a.href = url(a.getAttribute("href")); a.target = "_blank"; });
    $("#runNow").addEventListener("click", runNow);
    setupTheme();
    setupRange();
    setupHistory();
    setupSettings();
    apiGet("api/config").then(c => { state.config = c; }).catch(() => {});
    window.addEventListener("hashchange", () => showView(location.hash.slice(1)));
    pollStatus();
    showView(location.hash.slice(1) || "overview");
})();

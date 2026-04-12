const REFRESH_MS = 5000;
let logs = { speed: [], ping: [], trace: [] };
window.speedChart = null;
window.pingChart = null;

/* ---------- Helpers ---------- */
function mbps(bytesPerSecond) {
    return ((bytesPerSecond * 8) / 1_000_000).toFixed(1) + ' Mbps';
}
function latencyClass(ms) {
    if (ms < 50) return 'good';
    if (ms < 100) return 'warn';
    return 'bad';
}

function formatDate(ts) {
    if (!ts) return "-";
    const d = new Date(ts);
    if (isNaN(d)) return ts;

    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');

    let hours = d.getHours();
    const minutes = String(d.getMinutes()).padStart(2, '0');
    const ampm = hours >= 12 ? 'pm' : 'am';
    hours = hours % 12;
    if (hours === 0) hours = 12;

    return `${year}-${month}-${day} ${hours}:${minutes}${ampm}`;
}

/* ---------- DASHBOARD PAGE ---------- */
if (document.getElementById("download")) {
    async function loadSpeedtest() {
        const res = await fetch("/speedtest");
        const data = await res.json();
        const latest = data[data.length - 1];

        document.getElementById("download").innerText = "Download: " + mbps(parseFloat(latest.download));
        document.getElementById("upload").innerText = "Upload: " + mbps(parseFloat(latest.upload));
        document.getElementById("speedPing").innerText = "Ping: " + latest.ping + " ms";
        document.getElementById("serverLocation").innerText = "Server: " + (latest.serverLocation || '--');
    }

    async function loadPing() {
        const res = await fetch("/ping");
        const data = await res.json();
        const tbody = document.getElementById("pingTable");
        tbody.innerHTML = "";
        const latestPerHost = {};
        data.forEach(p => latestPerHost[p.host] = p);
        Object.values(latestPerHost).forEach(p => {
            const row = document.createElement("tr");
            const cls = latencyClass(parseInt(p.timems));
            row.innerHTML = `<td>${p.host}</td><td class="${cls}">${p.timems} ms</td><td>${p.success === "true" ? "✅" : "❌"}</td>`;
            tbody.appendChild(row);
        });
    }

    async function loadTraceroute() {
        const res = await fetch("/traceroute");
        const data = await res.json();
        const latest = data[0];
        const tbody = document.getElementById("traceTable");
        tbody.innerHTML = "";
        latest.hops.forEach(h => {
            if (h.ip === "*") return;
            const valid = h.times.filter(t => t !== null);
            const avg = valid.length ? (valid.reduce((a, b) => a + b, 0) / valid.length).toFixed(1) : "-";
            const row = document.createElement("tr");
            row.innerHTML = `<td>${h.hop}</td><td>${h.ip}</td><td>${avg} ms</td>`;
            tbody.appendChild(row);
        });
    }

    async function refreshDashboard() {
        try {
            await Promise.all([loadSpeedtest(), loadPing(), loadTraceroute()]);
        } catch (e) { console.error(e); }
    }

    refreshDashboard();
    setInterval(refreshDashboard, REFRESH_MS);
}

/* ---------- CHARTS ---------- */
function updateCharts() {
    const speedCtx = document.getElementById('speedChart').getContext('2d');
    const speedLabels = logs.speed.map(s => {
        return formatDate(s.timestamp).split(', ')[1] || formatDate(s.timestamp);
    });
    const downloadData = logs.speed.map(s => parseFloat(s.download) / 1000000);
    const uploadData = logs.speed.map(s => parseFloat(s.upload) / 1000000);

    if (window.speedChart) window.speedChart.destroy();
    window.speedChart = new Chart(speedCtx, {
        type: 'line',
        data: {
            labels: speedLabels,
            datasets: [
                { label: 'Download Mbps', data: downloadData, borderColor: '#22c55e', fill: false },
                { label: 'Upload Mbps', data: uploadData, borderColor: '#38bdf8', fill: false }
            ]
        },
        options: {
            responsive: true,
            plugins: { legend: { position: 'top' } },
            scales: { y: { beginAtZero: true } }
        }
    });

    const pingCtx = document.getElementById('pingChart').getContext('2d');
    const pingLabels = logs.ping.map(p => {
        return formatDate(p.timestamp).split(', ')[1] || formatDate(p.timestamp);
    });
    const pingData = logs.ping.map(p => parseFloat(p.timems));

    if (window.pingChart) window.pingChart.destroy();
    window.pingChart = new Chart(pingCtx, {
        type: 'line',
        data: {
            labels: pingLabels,
            datasets: [
                { label: 'Ping ms', data: pingData, borderColor: '#facc15', fill: false }
            ]
        },
        options: {
            responsive: true,
            plugins: { legend: { position: 'top' } },
            scales: { y: { beginAtZero: false } }
        }
    });
}

/* ---------- LOGS PAGE ---------- */
if (document.getElementById("speedTable")) {
    // ← FIX: removed duplicate `let logs` declaration here so updateCharts()
    //         reads from the same outer `logs` that loadLogs() populates.

    async function loadLogs() {
        try {
            const [speedRes, pingRes, traceRes] = await Promise.all([
                fetch("/speedtest"),
                fetch("/ping"),
                fetch("/traceroute")
            ]);

            logs.speed = await speedRes.json() || [];
            logs.ping = await pingRes.json() || [];
            logs.trace = await traceRes.json() || [];
            renderLogs();
            updateCharts();
        } catch (e) {
            console.error("Error loading logs:", e);
        }
    }
document.getElementById("applyFilter").addEventListener("click", () => {
    const dateStr = document.getElementById("filterDate").value; // yyyy-mm-dd
    if (!dateStr) {
        renderLogs();
        return;
    }
    renderLogs(dateStr);
});
function renderLogs(filterDate = null) {
    const speedTable = document.querySelector('#speedTable tbody');
    const pingTable = document.querySelector('#pingLogs tbody');
    const traceTable = document.querySelector('#traceLogs tbody');

    speedTable.innerHTML = '';
    pingTable.innerHTML = '';
    traceTable.innerHTML = '';

    const filterDateObj = filterDate ? new Date(filterDate) : null;

    // SPEED LOGS
    logs.speed
        .filter(s => {
            if (!filterDateObj) return true;
            const ts = new Date(s.timestamp);
            return ts.getFullYear() === filterDateObj.getFullYear() &&
                   ts.getMonth() === filterDateObj.getMonth() &&
                   ts.getDate() === filterDateObj.getDate();
        })
        .forEach(s => {
            const d = formatDate(s.timestamp);
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${d}</td>
                <td>${mbps(s.download)}</td>
                <td>${mbps(s.upload)}</td>
                <td>${s.ping} ms</td>
                <td>${s.serverId || '-'}</td>
                <td>${s.serverHost || '-'}</td>
                <td>${s.serverLocation || '-'}</td>
            `;
            speedTable.appendChild(row);
        });

    // PING LOGS
    logs.ping
        .filter(p => {
            if (!filterDateObj) return true;
            const ts = new Date(p.timestamp);
            return ts.getFullYear() === filterDateObj.getFullYear() &&
                   ts.getMonth() === filterDateObj.getMonth() &&
                   ts.getDate() === filterDateObj.getDate();
        })
        .forEach(p => {
            const d = formatDate(p.timestamp);
            const row = document.createElement('tr');
            row.innerHTML = `<td>${d}</td><td>${p.host}</td><td class='${latencyClass(p.timems)}'>${p.timems} ms</td><td>${p.success === 'true' ? '✅' : '❌'}</td>`;
            pingTable.appendChild(row);
        });

    // TRACEROUTE LOGS
    logs.trace
        .filter(t => {
            if (!filterDateObj) return true;
            const ts = new Date(t.timestamp);
            return ts.getFullYear() === filterDateObj.getFullYear() &&
                   ts.getMonth() === filterDateObj.getMonth() &&
                   ts.getDate() === filterDateObj.getDate();
        })
        .forEach(t => {
            const d = formatDate(t.timestamp);
            t.hops.forEach(h => {
                if (h.ip === '*') return;
                const valid = h.times.filter(x => x !== null);
                const avg = valid.length ? (valid.reduce((a, b) => a + b, 0) / valid.length).toFixed(1) : '-';
                const row = document.createElement('tr');
                row.innerHTML = `<td>${d}</td><td>${t.id}</td><td>${h.hop}</td><td>${h.ip}</td><td>${avg} ms</td>`;
                traceTable.appendChild(row);
            });
        });
}

    loadLogs();
    setInterval(loadLogs, REFRESH_MS);
}

const downloadBtn = document.getElementById("downloadBtn");
const downloadOptions = document.getElementById("downloadOptions");

downloadBtn.addEventListener("click", () => {
    downloadOptions.style.display = downloadOptions.style.display === "block" ? "none" : "block";
});

window.addEventListener("click", (e) => {
    if (!downloadBtn.contains(e.target)) {
        downloadOptions.style.display = "none";
    }
});

downloadOptions.querySelectorAll("button").forEach(btn => {
    btn.addEventListener("click", () => {
        const format = btn.dataset.format;
        downloadOptions.style.display = "none";
        downloadLogs(format);
    });
});

function downloadLogs(format) {
    const allLogs = { speed: logs.speed, ping: logs.ping, trace: logs.trace };
    let content = '';
    let type = '';
    let filename = `network_logs_${new Date().toISOString().split("T")[0]}`;

    switch(format) {
        case 'json':
            content = JSON.stringify(allLogs, null, 2);
            type = "application/json";
            filename += ".json";
            break;

        case 'csv':
            if (logs.speed.length > 0) {
                const headers = Object.keys(logs.speed[0]);
                const rows = logs.speed.map(r => headers.map(h => r[h]).join(","));
                content = headers.join(",") + "\n" + rows.join("\n");
            }
            type = "text/csv";
            filename += ".csv";
            break;

        case 'txt':
            content = `=== SPEED LOGS ===\n${JSON.stringify(logs.speed, null, 2)}\n\n` +
                      `=== PING LOGS ===\n${JSON.stringify(logs.ping, null, 2)}\n\n` +
                      `=== TRACE LOGS ===\n${JSON.stringify(logs.trace, null, 2)}`;
            type = "text/plain";
            filename += ".txt";
            break;

        case 'html':
            content = `<html><body><h2>Speed Logs</h2><table border="1"><tr>` +
                      Object.keys(logs.speed[0] || {}).map(h => `<th>${h}</th>`).join("") +
                      `</tr>` +
                      logs.speed.map(r => `<tr>${Object.values(r).map(v => `<td>${v}</td>`).join("")}</tr>`).join("") +
                      `</table></body></html>`;
            type = "text/html";
            filename += ".html";
            break;
    }

    const blob = new Blob([content], { type });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}
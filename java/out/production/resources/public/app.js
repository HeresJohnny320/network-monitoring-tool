const REFRESH_MS = 5000;

/* ---------- Helpers ---------- */
function mbps(bits) {
    return (bits / 1000000).toFixed(1) + ' Mbps';
}
function latencyClass(ms) {
    if (ms < 50) return 'good';
    if (ms < 100) return 'warn';
    return 'bad';
}
function formatDate(ts) {
    if (!ts) return "-";

    // Try parsing as ISO first
    let d = new Date(ts);
    if (!isNaN(d)) return d.toLocaleString();

    // Try SQL-like "YYYY-MM-DD HH:MM:SS" or "YYYYMMDD HHMMSS"
    const cleaned = ts.replace(/[^0-9]/g, ''); // remove non-numeric
    if (cleaned.length >= 14) {
        const year = parseInt(cleaned.slice(0, 4));
        const month = parseInt(cleaned.slice(4, 6)) - 1; // 0-indexed
        const day = parseInt(cleaned.slice(6, 8));
        const hour = parseInt(cleaned.slice(8, 10));
        const min = parseInt(cleaned.slice(10, 12));
        const sec = parseInt(cleaned.slice(12, 14));
        d = new Date(year, month, day, hour, min, sec);
        if (!isNaN(d)) return d.toLocaleString();
    }

    // Fallback: return raw string
    return ts;
}

/* ---------- DASHBOARD PAGE ---------- */
if(document.getElementById("download")){
async function loadSpeedtest(){
    const res = await fetch("/speedtest");
    const data = await res.json();
    const latest = data[data.length-1];

    document.getElementById("download").innerText = "Download: " + mbps(parseFloat(latest.download));
    document.getElementById("upload").innerText = "Upload: " + mbps(parseFloat(latest.upload));
    document.getElementById("speedPing").innerText = "Ping: " + latest.ping + " ms";
    document.getElementById("serverLocation").innerText = "Server: " + (latest.serverLocation || '--');
}

    async function loadPing(){
        const res = await fetch("/ping");
        const data = await res.json();
        const tbody = document.getElementById("pingTable");
        tbody.innerHTML = "";
        const latestPerHost = {};
        data.forEach(p => latestPerHost[p.host]=p);
        Object.values(latestPerHost).forEach(p=>{
            const row = document.createElement("tr");
            const cls = latencyClass(parseInt(p.timems));
            row.innerHTML = `<td>${p.host}</td><td class="${cls}">${p.timems} ms</td><td>${p.success==="true"?"✅":"❌"}</td>`;
            tbody.appendChild(row);
        });
    }

    async function loadTraceroute(){
        const res = await fetch("/traceroute");
        const data = await res.json();
        const latest = data[0];
        const tbody = document.getElementById("traceTable");
        tbody.innerHTML="";
        latest.hops.forEach(h=>{
            if(h.ip==="*") return;
            const valid=h.times.filter(t=>t!==null);
            const avg=valid.length ? (valid.reduce((a,b)=>a+b,0)/valid.length).toFixed(1) : "-";
            const row=document.createElement("tr");
            row.innerHTML=`<td>${h.hop}</td><td>${h.ip}</td><td>${avg} ms</td>`;
            tbody.appendChild(row);
        });
    }

    async function refreshDashboard(){
        try{
            await Promise.all([loadSpeedtest(), loadPing(), loadTraceroute()]);
        }catch(e){console.error(e);}
    }

    refreshDashboard();
    setInterval(refreshDashboard, REFRESH_MS);
}

/* ---------- CHARTS ---------- */
function updateCharts() {
    if (!logs.speed.length || !logs.ping.length) return;

    // Speed Chart
    const speedCtx = document.getElementById('speedChart').getContext('2d');
    const speedLabels = logs.speed.map(s => formatDate(s.timestamp).split(', ')[1] || formatDate(s.timestamp));
    const downloadData = logs.speed.map(s => parseFloat(s.download)/1000000);
    const uploadData = logs.speed.map(s => parseFloat(s.upload)/1000000);

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
            scales: { y: { beginAtZero: true } }
        }
    });

    // Ping Chart
    const pingCtx = document.getElementById('pingChart').getContext('2d');
    const pingLabels = logs.ping.map(p => formatDate(p.timestamp).split(', ')[1] || formatDate(p.timestamp));
    const pingData = logs.ping.map(p => parseFloat(p.timems));

    if (window.pingChart) window.pingChart.destroy();
    window.pingChart = new Chart(pingCtx, {
        type: 'line',
        data: { labels: pingLabels, datasets: [{ label: 'Ping ms', data: pingData, borderColor: '#facc15', fill: false }] },
        options: { responsive: true, scales: { y: { beginAtZero: false } } }
    });
}


/* ---------- LOGS PAGE ---------- */

function updateCharts() {
    // Speed Chart
    const speedCtx = document.getElementById('speedChart').getContext('2d');
    const speedLabels = logs.speed.map(s => {
        return formatDate(s.timestamp).split(', ')[1] || formatDate(s.timestamp); // only show time
    });
    const downloadData = logs.speed.map(s => parseFloat(s.download)/1000000); // Mbps
    const uploadData = logs.speed.map(s => parseFloat(s.upload)/1000000);   // Mbps

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
            scales: {
                y: { beginAtZero: true }
            }
        }
    });

    // Ping Chart
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
            scales: {
                y: { beginAtZero: false }
            }
        }
    });
}



if (document.getElementById("speedTable")) {
    let logs = { speed: [], ping: [], trace: [] };

    async function loadLogs() {
        try {
            const [speedRes, pingRes, traceRes] = await Promise.all([
                fetch("/speedtest"),
                fetch("/ping"),
                fetch("/traceroute")
            ]);

            logs.speed = await speedRes.json();
            logs.ping = await pingRes.json();
            logs.trace = await traceRes.json();

            renderLogs();
            updateCharts();
        } catch (e) {
            console.error("Error loading logs:", e);
        }
    }

function renderLogs(filterDate = null, filterMonth = null, filterYear = null) {
    const speedTable = document.querySelector('#speedTable tbody');
    const pingTable = document.querySelector('#pingLogs tbody');
    const traceTable = document.querySelector('#traceLogs tbody');

    speedTable.innerHTML = '';
    pingTable.innerHTML = '';
    traceTable.innerHTML = '';

    // Convert filter inputs to Date for easier comparison
    const filterDateObj = filterDate ? new Date(filterDate) : null;
    const filterMonthObj = filterMonth ? new Date(filterMonth + '-01') : null;

    // SPEED TEST
    logs.speed
        .filter(s => {
            if (!filterDateObj) return true;
            const ts = new Date(s.timestamp);
            return ts.toDateString() === filterDateObj.toDateString();
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

    // PING
    logs.ping
        .filter(p => {
            const ts = new Date(p.timestamp);
            if (filterDateObj && ts.toDateString() !== filterDateObj.toDateString()) return false;
            if (filterMonthObj && (ts.getFullYear() !== filterMonthObj.getFullYear() || ts.getMonth() !== filterMonthObj.getMonth())) return false;
            if (filterYear && ts.getFullYear() !== filterYear) return false;
            return true;
        })
        .forEach(p => {
            const d = formatDate(p.timestamp);
            const row = document.createElement('tr');
            row.innerHTML = `<td>${d}</td><td>${p.host}</td><td class='${latencyClass(p.timems)}'>${p.timems} ms</td><td>${p.success==='true'?'✅':'❌'}</td>`;
            pingTable.appendChild(row);
        });

    // TRACEROUTE
logs.trace
    .filter(t => {
        if (!filterDateObj) return true;
        const ts = new Date(t.timestamp);
        return ts.toDateString() === filterDateObj.toDateString();
    })
    .forEach(t => {
        const d = formatDate(t.timestamp);
        t.hops.forEach(h => {
            if (h.ip === '*') return;
            const valid = h.times.filter(x => x !== null);
            const avg = valid.length ? (valid.reduce((a,b)=>a+b,0)/valid.length).toFixed(1) : '-';
            const row = document.createElement('tr');
            // Include the id as the first column
            row.innerHTML = `<td>${d}</td><td>${t.id}</td><td>${h.hop}</td><td>${h.ip}</td><td>${avg} ms</td>`;
            traceTable.appendChild(row);
        });
    });

}

    loadLogs();
    setInterval(loadLogs, REFRESH_MS);
}
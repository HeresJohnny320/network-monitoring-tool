# 🌐 Network Monitoring Tool (Go)  

![Go Version](https://img.shields.io/badge/Go-1.20%2B-blue) ![Release](https://img.shields.io/badge/Release-v1.4-green) ![License](https://img.shields.io/github/license/HeresJohnny320/network-monitoring-tool) ![OS](https://img.shields.io/badge/OS-Windows%20%7C%20Linux%20%7C%20macOS%20%7C%20FreeBSD-lightgrey)  

A **cross-platform network monitoring tool** with a **live web dashboard**, built in **Go**.  
Monitor your network automatically and view results in your browser.  

Works on: **Windows, Linux, macOS (Intel & ARM), FreeBSD and PFsense**  

---

## 📸 Screenshots  

![Dashboard Screenshot](https://github.com/HeresJohnny320/network-monitoring-tool/blob/main/go/Screenshot_20260404_093345.png)  
*Live dashboard showing ping, traceroute, and speedtest data*  

![Logs Screenshot](https://github.com/HeresJohnny320/network-monitoring-tool/blob/main/go/Screenshot_20260404_093428.png)  
*Historical logs and charts for analysis*  

---

## 🧠 How It Works  

1. Checks for required tools: `ping`, `traceroute`, `speedtest`  
2. Starts a **local web server**  
3. Every few minutes (configurable):  
   * Ping your hosts  
   * Run traceroute  
   * Run a speed test  
4. Saves results to a database  
5. Shows a **live dashboard**  

---

## 📦 Download & Setup  

### 1️⃣ Download Prebuilt Binaries  

Go to the **[Releases](https://github.com/HeresJohnny320/network-monitoring-tool/releases)** tab and download your platform:  

* Windows  
* Linux  
* macOS Intel & ARM  
* FreeBSD (even PFsense)

No Go installation or compilation needed.  

---

### 2️⃣ Install Speedtest CLI
Download speedtest cli in console / powershell
| **Windows** | `winget install --id=Ookla.Speedtest.CLI -e` |
then for the rest of you
https://www.speedtest.net/apps/cli


Check installation:

```bash
speedtest --version
```

---

### 3️⃣ Config & Database Locations  

| Platform | Path |
|----------|------|
| **Windows** | `%APPDATA%\network_monitoring_tool` |
| **Linux** | `$HOME/.config/network_monitoring_tool` |
| **macOS** | `$HOME/Library/Application Support/network_monitoring_tool` |
| **FreeBSD** | `$HOME/.config/network_monitoring_tool` |

Stored files:  

* `config.json` → configuration  
* `database.db` → SQLite database (if MySQL not enabled)  

> The app automatically reads/writes here. Missing `config.json` → **default file is created**.  

---

### 4️⃣ Example `config.json`  

```json
{
  "run_every": "5m",
  "ping_hosts": ["google.com", "cloudflare.com"],
  "traceroute_hosts": ["google.com"],
  "speedtest_server_id": "",
  "use_mysql": false,
  "mysql_user": "",
  "mysql_password": "",
  "mysql_host": "localhost",
  "mysql_port": "3306",
  "mysql_database": "network_tool"
}
```

---

### 5️⃣ Run the App  

#### Linux/macOS/FreeBSD

```bash
./network-tool-go
```

#### Windows  

Double-click `.exe` or run in PowerShell:

```powershell
network-tool-go.exe
```

---

### 6️⃣ Open Dashboard  

```text
http://localhost:8080
```

---

## ⏱️ Scheduling  

Set test frequency in `config.json`:

```json
"run_every": "5m"
```

Examples:  

* `"30s"` → every 30 seconds  
* `"5m"` → every 5 minutes  
* `"1h"` → every hour  
* `"1h30m"` → every 1 hour 30 minutes  

> First test runs immediately on startup.  

---

## 📊 Dashboard Features  

* ⚡ Latest speed test  
* 📡 Ping status per host  
* 🧭 Latest traceroute  
* Auto-updates every **5 seconds**  

---

## 📜 Logs  

* View all past speed tests, pings, and traceroutes  
* Filter by date  
* Charts:  
  * Speed over time  
  * Ping over time  

---

## 🔌 API  

Access raw JSON:  

* `http://localhost:8080/ping`  
* `http://localhost:8080/speedtest`  
* `http://localhost:8080/traceroute`  

Speedtest is in bytes/sec → convert to Mbps:

```go
func Mbps(bytesPerSecond float64) string {
    return fmt.Sprintf("%.1f Mbps", (bytesPerSecond*8)/1_000_000)
}
```

---

## ❗ Common Issues  

### Missing Tools  

```bash
ping
traceroute
speedtest
```

Should report:

```text
Speedtest-cli available: true
Ping available: true
Traceroute available: true
Everything needed Installed :)
```

---

### No Data / Empty UI  

* Wait for the first scheduled run  
* Check API (`http://localhost:8080/ping`)  

---

## 💡 Tips  

* Start with `"run_every": "1m"` for testing  
* Add multiple hosts to monitor more targets  
* Keep server running for long-term tracking  
* Use logs to analyze trends  

---

## 🔒 Safety  

* Runs **locally only** (`localhost`)  
* No external data sharing  
* Free & open-source  
* Uses system commands only  

---

## 🚀 Use Cases  

* Detect internet instability  
* Monitor servers/websites  
* Observe latency spikes  
* Track ISP performance over time  
* Home lab monitoring  

---

## 🚧 Future Features  

* Alerts (Discord / Email)  
* Docker support  
* Enhanced UI  

---

## 🎉 Done!  

Run it once, leave it open, and your **network monitoring dashboard** automatically:  

* Collects data  
* Updates the live dashboard  
* Provides insights over time  

---

## 🔗 Links  

* [Project Repository](https://github.com/HeresJohnny320/network-monitoring-tool/)  
* [Latest Release](https://github.com/HeresJohnny320/network-monitoring-tool/releases)


# 🌐 Network Tool — Easy Network Monitoring Dashboard

A simple, automatic network monitoring tool with a built-in web dashboard.

It checks your internet and network health by running:

* 📡 Ping (is a site reachable? how fast?)
* 🛰️ Traceroute (where does your connection go?)
* ⚡ Speed tests (download, upload, latency)

Everything is:

* ✅ Logged to a database
* 📊 Shown in a live dashboard
* 🔁 Run automatically on a schedule

---

# 🧠 What This App Does (Simple Explanation)

Once you start the app, it will:

1. Check if required tools are installed (`ping`, `traceroute`, `speedtest`)
2. Start a local web server
3. Every few minutes (you choose):

    * Ping your selected websites
    * Run traceroute
    * Run a speed test
4. Save all results
5. Show everything in a dashboard you can open in your browser

---

# 🖥️ What You See (The UI)

Open this in your browser:

```text
http://localhost:8080
```

---

## 📊 Dashboard Tab

This is your **live view**:

* ⚡ Latest speed test
* 📡 Ping status (per host)
* 🧭 Latest traceroute

Updates automatically every **5 seconds**

---

## 📜 Logs Tab

This is your **history**:

* All past speed tests
* All ping logs
* All traceroutes
* 📅 Filter by date
* 📈 Charts:

    * Speed over time
    * Ping over time

---

# ⚙️ Requirements (IMPORTANT)

You MUST have these installed:

### ✅ Java

* Java 17 or newer

### ✅ Required system tools

Test these in your terminal:

```bash
ping google.com
traceroute google.com   # or tracert on Windows
speedtest
```

If any fail → install them before running the app.

---

# 📦 Setup (Step-by-Step)

## 1. Download the project

```bash
git clone https://github.com/your-username/network-tool.git
cd network-tool
```

---

## 2. Install Speedtest CLI

### Windows

Download:
https://www.speedtest.net/apps/cli

### Linux (Ubuntu/Debian)

```bash
sudo apt install speedtest-cli
```

### macOS

```bash
brew install speedtest-cli
```

---

## 3. Create config file

Create this file:

```text
src/main/resources/config.json
```

Paste this:

```json
{
  "run_every": "5m",

  "ping_host": ["google.com", "cloudflare.com"],
  "traceroute_host": ["google.com"],

  "speedtest_server_id": "",

  "run_sql": false,

  "sql_user": "",
  "sql_password": "",
  "sql_host": "localhost",
  "sql_port": "3306",
  "sql_database": "network_tool"
}
```

---

## 4. Start the app

```bash
mvn spring-boot:run
```

---

## 5. Open the dashboard

Go to:

```text
http://localhost:8080
```

---

# ⏱️ How Scheduling Works

You control how often tests run:

```json
"run_every": "5m"
```

Examples:

* `"30s"` → every 30 seconds
* `"5m"` → every 5 minutes
* `"1h"` → every hour
* `"1h30m"` → every 1 hour 30 minutes

---

# 🔌 API (For Advanced Users)

You can also access raw data:

* http://localhost:8080/ping
* http://localhost:8080/speedtest
* http://localhost:8080/traceroute

All return JSON.

---

# 🗄️ Data Storage

By default:

* Uses **SQLite**
* Creates a file:

```text
database.db
```

Optional:

* You can enable MySQL in config

---

# ❗ Common Problems & Fixes

## App closes immediately

👉 Missing tools

Run:

```bash
ping
traceroute
speedtest
```

Fix anything that fails.

---

## No data showing

👉 Wait for first scheduled run
(or shorten `run_every` to test)

---

## UI loads but empty

Check API works:

```text
http://localhost:8080/ping
```

If that works → UI will work

---

## Speedtest fails

Make sure this works:

```bash
speedtest --version
```

---

# 📌 Tips

* Start with `"run_every": "1m"` for testing
* Add multiple hosts to monitor more targets
* Leave server running for long-term tracking
* Use Logs tab to analyze trends

---

# 🔒 Is This Safe?

* Runs locally only (`localhost`)
* No external data sharing
* Uses system commands (ping, traceroute, etc.)

---

# 💡 What This Is Good For

* Checking if your internet is unstable
* Monitoring servers or websites
* Seeing latency spikes
* Tracking ISP performance over time
* Home lab monitoring

---

# 🚧 Possible Future Features

* Alerts (Discord / Email)
* Docker support
* Better UI
* Authentication
* Multi-device monitoring

---

# 🎉 Done!

Once running, just leave it open and it will:

* keep collecting data
* keep updating the dashboard
* give you a full picture of your network over time

---

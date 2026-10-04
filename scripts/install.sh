#!/bin/sh
# Network Monitor installer for Linux, macOS, FreeBSD and pfSense.
#
#   curl -fsSL https://raw.githubusercontent.com/HeresJohnny320/network-monitoring-tool/main/scripts/install.sh | sudo sh
#
# pfSense (Diagnostics > Command Prompt, or SSH option 8):
#   fetch -o - https://raw.githubusercontent.com/HeresJohnny320/network-monitoring-tool/main/scripts/install.sh | sh
#
# It downloads the right build for this machine, installs the Ookla speedtest CLI
# if no speed test program is found, and sets it up as a service that starts on boot.
#
# Options (append after "sh -s --" when piping, e.g. "| sudo sh -s -- --port 9090"):
#   --version v1.5.0   install a specific release (default: latest)
#   --port 8080        port for the web dashboard
#   --archive FILE     install from an already-downloaded release archive
#   --no-service       just install the binary, don't set up a service
#   --no-speedtest     don't install a speed test program
#   --lan              pfSense only: also serve the dashboard on the LAN (default: only via the pfSense GUI)
#   --uninstall        remove the program and service (keeps your data)
#   --purge            with --uninstall, also delete config and data

set -eu

REPO="HeresJohnny320/network-monitoring-tool"
APP="network_monitor_tool"
BIN_DIR="/usr/local/bin"
OOKLA_VERSION="1.2.0"

VERSION="latest"
PORT=""
ARCHIVE=""
SERVICE=1
SPEEDTEST=1
LAN=0
UNINSTALL=0
PURGE=0

while [ $# -gt 0 ]; do
    case "$1" in
        --version) VERSION="$2"; shift ;;
        --port) PORT="$2"; shift ;;
        --archive) ARCHIVE="$2"; shift ;;
        --no-service) SERVICE=0 ;;
        --no-speedtest) SPEEDTEST=0 ;;
        --lan) LAN=1 ;;
        --uninstall) UNINSTALL=1 ;;
        --purge) PURGE=1 ;;
        -h|--help) sed -n '2,25p' "$0" 2>/dev/null || true; exit 0 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
    shift
done

say() { printf '\033[36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33m!!\033[0m %s\n' "$*" >&2; }
die() { printf '\033[31mError:\033[0m %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

download() { # url dest
    if have curl; then
        curl -fsSL "$1" -o "$2"
    elif have fetch; then
        fetch -q -o "$2" "$1"
    elif have wget; then
        wget -qO "$2" "$1"
    else
        die "need curl, fetch or wget to download files"
    fi
}

[ "$(id -u)" -eq 0 ] || die "please run as root (e.g. pipe into 'sudo sh')"

# ---------- Detect platform ----------
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    armv7*|armv8l) ARCH=armv7 ;;
    armv6*) ARCH=armv6 ;;
    i386|i686) ARCH=386 ;;
    *) die "unsupported CPU: $(uname -m)" ;;
esac
PFSENSE=0
if [ "$OS" = freebsd ] && { grep -qi pfsense /etc/platform 2>/dev/null || [ -f /usr/local/sbin/pfSsh.php ]; }; then
    PFSENSE=1
fi
case "$OS" in
    linux|darwin|freebsd) ;;
    *) die "unsupported OS: $OS (on Windows use install.ps1)" ;;
esac

# Where config.json and database.db live when running as a service.
case "$OS" in
    linux) DATA_DIR="/var/lib/netmon" ;;
    darwin) DATA_DIR="/Library/Application Support/netmon" ;;
    freebsd) DATA_DIR="/var/db/netmon" ;;
esac
# pfSense can keep /var in RAM, so store data with the rest of the config.
[ "$PFSENSE" -eq 1 ] && DATA_DIR="/usr/local/etc/netmon"

SYSTEMD_UNIT="/etc/systemd/system/netmon.service"
LAUNCHD_PLIST="/Library/LaunchDaemons/com.heresjohnny320.netmon.plist"
RC_SCRIPT="/usr/local/etc/rc.d/netmon"
[ "$PFSENSE" -eq 1 ] && RC_SCRIPT="/usr/local/etc/rc.d/netmon.sh"

# ---------- Uninstall ----------
if [ "$UNINSTALL" -eq 1 ]; then
    say "Removing Network Monitor"
    case "$OS" in
        linux)
            if [ -f "$SYSTEMD_UNIT" ]; then
                systemctl disable --now netmon 2>/dev/null || true
                rm -f "$SYSTEMD_UNIT"
                systemctl daemon-reload
            fi ;;
        darwin)
            launchctl bootout system "$LAUNCHD_PLIST" 2>/dev/null || true
            rm -f "$LAUNCHD_PLIST" ;;
        freebsd)
            [ -f "$RC_SCRIPT" ] && "$RC_SCRIPT" onestop 2>/dev/null || true
            rm -f "$RC_SCRIPT"
            have sysrc && sysrc -x netmon_enable 2>/dev/null || true
            if [ "$PFSENSE" -eq 1 ]; then
                [ -f /usr/local/share/netmon/netmon_menu.php ] && php -f /usr/local/share/netmon/netmon_menu.php uninstall || true
                rm -f /usr/local/www/status_netmon.php /usr/local/www/netmon_proxy.php
                rm -rf /usr/local/share/netmon
            fi ;;
    esac
    rm -f "$BIN_DIR/$APP"
    if [ "$PURGE" -eq 1 ]; then
        rm -rf "$DATA_DIR"
        say "Deleted $DATA_DIR"
    else
        say "Your data was kept in $DATA_DIR (use --purge to delete it)"
    fi
    say "Uninstalled."
    exit 0
fi

# ---------- Download & install the binary ----------
TMP=$(mktemp -d 2>/dev/null || mktemp -d -t netmon)
trap 'rm -rf "$TMP"' EXIT INT TERM

ASSET="${APP}_${OS}_${ARCH}.tar.gz"
if [ -z "$ARCHIVE" ]; then
    if [ "$VERSION" = latest ]; then
        URL="https://github.com/$REPO/releases/latest/download/$ASSET"
    else
        URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"
    fi
    say "Downloading $ASSET ($VERSION)"
    download "$URL" "$TMP/$ASSET" || die "download failed: $URL"
    ARCHIVE="$TMP/$ASSET"
fi
mkdir -p "$TMP/pkg"
tar -xzf "$ARCHIVE" -C "$TMP/pkg"
[ -f "$TMP/pkg/$APP" ] || die "archive doesn't contain $APP"

# Stop a running copy before replacing the binary.
case "$OS" in
    linux) [ -f "$SYSTEMD_UNIT" ] && systemctl stop netmon 2>/dev/null || true ;;
    darwin) launchctl bootout system "$LAUNCHD_PLIST" 2>/dev/null || true ;;
    freebsd) [ -f "$RC_SCRIPT" ] && "$RC_SCRIPT" onestop >/dev/null 2>&1 || true ;;
esac

mkdir -p "$BIN_DIR"
cp "$TMP/pkg/$APP" "$BIN_DIR/$APP"
chmod 755 "$BIN_DIR/$APP"
say "Installed $("$BIN_DIR/$APP" -version)"

# ---------- Dependencies ----------
has_speedtest() { have speedtest || have speedtest-cli; }

install_ookla() { # platform-suffix
    say "Installing the Ookla Speedtest CLI $OOKLA_VERSION (license: https://www.speedtest.net/about/eula)"
    download "https://install.speedtest.net/app/cli/ookla-speedtest-$OOKLA_VERSION-$1.tgz" "$TMP/speedtest.tgz" &&
        tar -xzf "$TMP/speedtest.tgz" -C "$TMP" speedtest &&
        cp "$TMP/speedtest" "$BIN_DIR/speedtest" && chmod 755 "$BIN_DIR/speedtest"
}

install_pkg() { # package name, using whatever package manager exists
    if have apt-get; then apt-get install -y "$1"
    elif have dnf; then dnf install -y "$1"
    elif have yum; then yum install -y "$1"
    elif have pacman; then pacman -S --noconfirm "$1"
    elif have zypper; then zypper --non-interactive install "$1"
    elif have apk; then apk add "$1"
    else return 1
    fi
}

if [ "$OS" = linux ] && ! have traceroute; then
    say "Installing traceroute"
    install_pkg traceroute >/dev/null 2>&1 || warn "couldn't install traceroute; install it with your package manager"
fi
if [ "$OS" = linux ] && ! have ping; then
    install_pkg iputils-ping >/dev/null 2>&1 || install_pkg iputils >/dev/null 2>&1 || warn "couldn't install ping"
fi

if [ "$SPEEDTEST" -eq 1 ] && ! has_speedtest; then
    case "$OS/$ARCH" in
        linux/amd64) install_ookla linux-x86_64 ;;
        linux/arm64) install_ookla linux-aarch64 ;;
        linux/armv7) install_ookla linux-armhf ;;
        linux/armv6) install_ookla linux-armel ;;
        linux/386) install_ookla linux-i386 ;;
        darwin/*) install_ookla macosx-universal ;;
        freebsd/*)
            # pfSense/FreeBSD: the Python speedtest-cli is in the package repo.
            PKGNAME=$(pkg search -q -x '^py[0-9]+-speedtest-cli-[0-9]' 2>/dev/null | sed 's/-[0-9][^-]*$//' | sort | tail -1 || true)
            if [ -n "$PKGNAME" ]; then
                say "Installing $PKGNAME"
                pkg install -y "$PKGNAME" || true
            fi ;;
    esac || true
    has_speedtest || warn "no speed test program installed; speed tests will be skipped until you install one (https://www.speedtest.net/apps/cli)"
fi

# ---------- Data dir & config ----------
mkdir -p "$DATA_DIR"
chmod 700 "$DATA_DIR"

# Bring over config/data from running it by hand before (as the sudo user).
if [ ! -f "$DATA_DIR/config.json" ] && [ -n "${SUDO_USER:-}" ]; then
    case "$OS" in
        darwin) OLD="/Users/$SUDO_USER/Library/Application Support/heresjohnnys320_network_monitor_tool" ;;
        *) OLD="$(eval echo "~$SUDO_USER")/.config/heresjohnnys320_network_monitor_tool" ;;
    esac
    if [ -f "$OLD/config.json" ]; then
        say "Copying your existing config and data from $OLD"
        cp "$OLD/config.json" "$DATA_DIR/"
        [ -f "$OLD/database.db" ] && cp "$OLD/database.db" "$DATA_DIR/"
    fi
fi

if [ ! -f "$DATA_DIR/config.json" ]; then
    LISTEN=":${PORT:-8080}"
    # On pfSense the dashboard is reached through the pfSense GUI, so keep it off the network.
    [ "$PFSENSE" -eq 1 ] && [ "$LAN" -eq 0 ] && LISTEN="127.0.0.1:${PORT:-8080}"
    printf '{\n  "listen": "%s"\n}\n' "$LISTEN" > "$DATA_DIR/config.json"
elif [ -n "$PORT" ]; then
    warn "config.json already exists; change the port in Settings or $DATA_DIR/config.json"
fi

if [ "$SERVICE" -eq 0 ]; then
    say "Done. Run it with: $BIN_DIR/$APP -data-dir \"$DATA_DIR\""
    exit 0
fi

# ---------- Service ----------
case "$OS" in
linux)
    have systemctl || die "systemd not found; run '$BIN_DIR/$APP -data-dir $DATA_DIR' with your init system"
    cat > "$SYSTEMD_UNIT" <<EOF
[Unit]
Description=Network Monitor (ISP speed, ping and traceroute history)
Documentation=https://github.com/$REPO
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=$BIN_DIR/$APP -data-dir $DATA_DIR -no-browser
Environment=HOME=$DATA_DIR
Restart=on-failure
RestartSec=10
NoNewPrivileges=yes
ProtectSystem=full
ProtectHome=yes
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now netmon
    say "Service 'netmon' is running. Logs: journalctl -u netmon -f"
    ;;

darwin)
    cat > "$LAUNCHD_PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.heresjohnny320.netmon</string>
    <key>ProgramArguments</key>
    <array>
        <string>$BIN_DIR/$APP</string>
        <string>-data-dir</string><string>$DATA_DIR</string>
        <string>-no-browser</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key><string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
        <key>HOME</key><string>$DATA_DIR</string>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>/var/log/netmon.log</string>
    <key>StandardErrorPath</key><string>/var/log/netmon.log</string>
</dict>
</plist>
EOF
    launchctl bootstrap system "$LAUNCHD_PLIST"
    say "Service is running. Logs: /var/log/netmon.log"
    ;;

freebsd)
    ENABLE_DEFAULT=NO
    [ "$PFSENSE" -eq 1 ] && ENABLE_DEFAULT=YES # pfSense starts rc.d/*.sh at boot without rc.conf
    cat > "$RC_SCRIPT" <<EOF
#!/bin/sh
#
# PROVIDE: netmon
# REQUIRE: NETWORKING DAEMON
# KEYWORD: shutdown

. /etc/rc.subr

name="netmon"
rcvar="netmon_enable"
load_rc_config \$name

: \${netmon_enable:="$ENABLE_DEFAULT"}
: \${netmon_data_dir:="$DATA_DIR"}

pidfile="/var/run/\${name}.pid"
procname="$BIN_DIR/$APP"
command="/usr/sbin/daemon"
command_args="-f -p \${pidfile} -T \${name} /usr/bin/env HOME=\${netmon_data_dir} \${procname} -data-dir \${netmon_data_dir} -no-browser"

run_rc_command "\$1"
EOF
    chmod 755 "$RC_SCRIPT"

    if [ "$PFSENSE" -eq 1 ]; then
        [ -d "$TMP/pkg/pfsense" ] || die "archive is missing the pfsense/ files"
        cp "$TMP/pkg/pfsense/status_netmon.php" "$TMP/pkg/pfsense/netmon_proxy.php" /usr/local/www/
        mkdir -p /usr/local/share/netmon
        cp "$TMP/pkg/pfsense/netmon_menu.php" /usr/local/share/netmon/
        php -f /usr/local/share/netmon/netmon_menu.php install
        "$RC_SCRIPT" start
    else
        sysrc netmon_enable=YES >/dev/null
        service netmon start
    fi
    say "Service 'netmon' is running. Logs go to syslog (tag: netmon)."
    ;;
esac

LISTEN=$(sed -n 's/.*"listen": *"\([^"]*\)".*/\1/p' "$DATA_DIR/config.json" | head -1)
PORT_NOW=${LISTEN##*:}
echo
if [ "$PFSENSE" -eq 1 ]; then
    say "Open the pfSense web GUI and go to Status > Network Monitor."
    case "$LISTEN" in 127.0.0.1:*) ;; *) say "It's also available at http://<this-firewall>:$PORT_NOW" ;; esac
else
    say "Dashboard: http://localhost:$PORT_NOW (or http://<this-machine's-IP>:$PORT_NOW from another device)"
fi
say "Config: $DATA_DIR/config.json (you can change everything from the Settings tab)"

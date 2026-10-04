# Network Monitor installer for Windows.
#
# Run in PowerShell *as Administrator*:
#   irm https://raw.githubusercontent.com/HeresJohnny320/network-monitoring-tool/main/scripts/install.ps1 | iex
#
# It installs to "C:\Program Files\NetworkMonitor", downloads the Ookla speedtest CLI
# next to it, and registers a scheduled task that starts the monitor at boot (it runs
# in the background, even when nobody is logged in). Data lives in C:\ProgramData\NetworkMonitor.
#
# To uninstall:  & ([scriptblock]::Create((irm <url above>))) -Uninstall

param(
    [string]$Version = "latest",
    [int]$Port = 8080,
    [switch]$Uninstall,
    [switch]$Purge
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue" # makes Invoke-WebRequest much faster

$Repo = "HeresJohnny320/network-monitoring-tool"
$App = "network_monitor_tool"
$InstallDir = Join-Path $env:ProgramFiles "NetworkMonitor"
$DataDir = Join-Path $env:ProgramData "NetworkMonitor"
$TaskName = "NetworkMonitor"
$OoklaVersion = "1.2.0"

function Say($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Please run PowerShell as Administrator (right-click > Run as administrator)."
}

function Stop-Monitor {
    if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    }
    Get-Process -Name $App -ErrorAction SilentlyContinue | Stop-Process -Force
}

if ($Uninstall) {
    Say "Removing Network Monitor"
    Stop-Monitor
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    Get-NetFirewallRule -DisplayName "Network Monitor" -ErrorAction SilentlyContinue | Remove-NetFirewallRule
    Remove-Item -Recurse -Force $InstallDir -ErrorAction SilentlyContinue
    if ($Purge) {
        Remove-Item -Recurse -Force $DataDir -ErrorAction SilentlyContinue
        Say "Deleted $DataDir"
    } else {
        Say "Your data was kept in $DataDir (add -Purge to delete it)"
    }
    Say "Uninstalled."
    return
}

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "${App}_windows_$arch.zip"
$url = if ($Version -eq "latest") {
    "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    "https://github.com/$Repo/releases/download/$Version/$asset"
}

$tmp = Join-Path $env:TEMP ("netmon-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Say "Downloading $asset ($Version)"
    Invoke-WebRequest -Uri $url -OutFile "$tmp\$asset" -UseBasicParsing
    Expand-Archive -Path "$tmp\$asset" -DestinationPath "$tmp\pkg"

    Stop-Monitor
    New-Item -ItemType Directory -Force -Path $InstallDir, $DataDir | Out-Null
    Copy-Item "$tmp\pkg\*" $InstallDir -Recurse -Force
    $exe = Join-Path $InstallDir "$App.exe"
    Say ("Installed " + (& $exe -version))

    # The monitor looks for speedtest.exe next to itself first, which works no matter
    # which account the scheduled task runs as.
    if (-not (Test-Path (Join-Path $InstallDir "speedtest.exe"))) {
        Say "Downloading the Ookla Speedtest CLI $OoklaVersion (license: https://www.speedtest.net/about/eula)"
        $zip = "$tmp\speedtest.zip"
        Invoke-WebRequest -Uri "https://install.speedtest.net/app/cli/ookla-speedtest-$OoklaVersion-win64.zip" -OutFile $zip -UseBasicParsing
        Expand-Archive -Path $zip -DestinationPath "$tmp\speedtest"
        Copy-Item "$tmp\speedtest\speedtest.exe" $InstallDir -Force
    }

    # Bring over config/data from running it by hand before.
    $old = Join-Path $env:APPDATA "heresjohnnys320_network_monitor_tool"
    if (-not (Test-Path "$DataDir\config.json") -and (Test-Path "$old\config.json")) {
        Say "Copying your existing config and data from $old"
        Copy-Item "$old\config.json" $DataDir
        if (Test-Path "$old\database.db") { Copy-Item "$old\database.db" $DataDir }
    }
    if (-not (Test-Path "$DataDir\config.json")) {
        Set-Content -Path "$DataDir\config.json" -Value "{`n  `"listen`": `":$Port`"`n}" -Encoding ascii
    }

    # Start at boot as SYSTEM, restart if it crashes, never stop because it ran "too long".
    $action = New-ScheduledTaskAction -Execute $exe -Argument "-data-dir `"$DataDir`" -no-browser" -WorkingDirectory $InstallDir
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
        -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -StartWhenAvailable
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings `
        -Principal $taskPrincipal -Description "Network Monitor: ISP speed, ping and traceroute history" -Force | Out-Null
    Start-ScheduledTask -TaskName $TaskName

    # Let other devices on the home network open the dashboard.
    if (-not (Get-NetFirewallRule -DisplayName "Network Monitor" -ErrorAction SilentlyContinue)) {
        New-NetFirewallRule -DisplayName "Network Monitor" -Direction Inbound -Program $exe -Action Allow -Profile Private,Domain | Out-Null
    }

    $listen = ((Get-Content "$DataDir\config.json" -Raw) | ConvertFrom-Json).listen
    $portNow = $listen.Split(":")[-1]
    Say "Network Monitor is running and will start automatically with Windows."
    Say "Dashboard: http://localhost:$portNow"
    Say "Config: $DataDir\config.json (you can change everything from the Settings tab)"
    Start-Sleep -Seconds 2
    Start-Process "http://localhost:$portNow"
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

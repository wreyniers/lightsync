param(
    [Parameter(Position = 0)]
    [ValidateSet('start', 'stop', 'restart', 'status', 'log', 'build')]
    [string]$Action = 'start'
)

$ErrorActionPreference = 'Stop'

$env:Path = "C:\Program Files\Go\bin;$env:USERPROFILE\go\bin;" + $env:Path

$PidFile = Join-Path $PSScriptRoot '.dev.pid'
$LogFile = Join-Path $PSScriptRoot '.dev.log'
$Wails3Exe = Join-Path "$env:USERPROFILE\go\bin" 'wails3.exe'

function Find-DevProcesses {
    $procs = @()

    # The wails3 CLI process
    $procs += Get-Process -Name 'wails3' -ErrorAction SilentlyContinue

    # The compiled Go app spawned by wails dev (lightsync-dev in dev mode)
    $procs += Get-Process -Name 'lightsync*' -ErrorAction SilentlyContinue

    # Node processes running Vite from our frontend directory
    $frontendDir = (Join-Path $PSScriptRoot 'frontend').ToLower()
    Get-CimInstance Win32_Process -Filter "Name = 'node.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.ToLower().Contains($frontendDir) } |
        ForEach-Object {
            $p = Get-Process -Id $_.ProcessId -ErrorAction SilentlyContinue
            if ($p) { $procs += $p }
        }

    return $procs | Sort-Object -Property Id -Unique
}

function Test-DevRunning {
    $procs = Find-DevProcesses
    return ($procs.Count -gt 0)
}

function Start-Dev {
    if (-not (Test-Path $Wails3Exe)) {
        Write-Host "wails3 not found at $Wails3Exe" -ForegroundColor Red
        Write-Host "Install with: go install github.com/wailsapp/wails/v3/cmd/wails3@latest" -ForegroundColor Yellow
        return
    }
    if (Test-DevRunning) {
        Write-Host 'Dev server is already running' -ForegroundColor Yellow
        return
    }

    Write-Host 'Starting dev server (Wails v3)...' -ForegroundColor Cyan

    $proc = Start-Process -FilePath $Wails3Exe -ArgumentList 'dev', '-config', './build/config.yml' `
        -WorkingDirectory $PSScriptRoot `
        -NoNewWindow `
        -RedirectStandardOutput $LogFile `
        -RedirectStandardError "$LogFile.err" `
        -PassThru

    $proc.Id | Set-Content $PidFile

    Write-Host "Dev server started (PID: $($proc.Id))" -ForegroundColor Green
    Write-Host 'Streaming log (Ctrl+C to stop tailing)...' -ForegroundColor DarkGray
    Get-Content $LogFile -Wait
}

function Stop-Dev {
    $procs = Find-DevProcesses
    if ($procs.Count -eq 0) {
        Write-Host 'Dev server is not running' -ForegroundColor Yellow
        return
    }

    Write-Host "Killing $($procs.Count) process(es)..." -ForegroundColor Cyan
    foreach ($p in $procs) {
        if (Get-Process -Id $p.Id -ErrorAction SilentlyContinue) {
            Write-Host "  Killing $($p.ProcessName) (PID: $($p.Id))" -ForegroundColor DarkGray
            $ErrorActionPreference = 'Continue'
            & taskkill /F /T /PID $p.Id 2>&1 | Out-Null
            $ErrorActionPreference = 'Stop'
        }
    }
    $global:LASTEXITCODE = 0

    if (Test-Path $PidFile) { Remove-Item $PidFile -Force }
    Write-Host 'Dev server stopped' -ForegroundColor Green
}

function Show-Status {
    $procs = Find-DevProcesses
    if ($procs.Count -gt 0) {
        Write-Host 'Dev server is running:' -ForegroundColor Green
        foreach ($p in $procs) {
            Write-Host "  $($p.ProcessName) (PID: $($p.Id))" -ForegroundColor DarkGray
        }
    }
    else {
        Write-Host 'Dev server is not running' -ForegroundColor Yellow
    }
}

function Show-Log {
    if (-not (Test-Path $LogFile)) {
        Write-Host 'No log file found. Start the dev server first.' -ForegroundColor Yellow
        return
    }
    Write-Host 'Streaming log (Ctrl+C to stop tailing)...' -ForegroundColor DarkGray
    Get-Content $LogFile -Wait
}

function Build-App {
    if (-not (Test-Path $Wails3Exe)) {
        Write-Host "wails3 not found at $Wails3Exe" -ForegroundColor Red
        Write-Host "Install with: go install github.com/wailsapp/wails/v3/cmd/wails3@latest" -ForegroundColor Yellow
        return
    }

    Write-Host 'Building production executable (Wails v3)...' -ForegroundColor Cyan

    Push-Location $PSScriptRoot
    try {
        & $Wails3Exe task windows:build:production
        if ($LASTEXITCODE -eq 0) {
            $exe = Join-Path $PSScriptRoot 'bin\lightsync.exe'
            if (Test-Path $exe) {
                $size = [math]::Round((Get-Item $exe).Length / 1MB, 1)
                Write-Host "Build complete: bin\lightsync.exe (${size} MB)" -ForegroundColor Green

                # Replace old v2 binary in build/bin so shortcuts/scripts use the new build
                $buildBinExe = Join-Path $PSScriptRoot 'build\bin\lightsync.exe'
                $buildBinDir = Split-Path $buildBinExe
                if (-not (Test-Path $buildBinDir)) {
                    New-Item -ItemType Directory -Path $buildBinDir -Force | Out-Null
                }
                try {
                    Copy-Item $exe $buildBinExe -Force
                    Write-Host "Copied to build\bin\lightsync.exe" -ForegroundColor DarkGray
                }
                catch {
                    Write-Host "Could not copy to build\bin (close LightSync if running): $_" -ForegroundColor Yellow
                }
            }
        }
        else {
            Write-Host 'Build failed' -ForegroundColor Red
        }
    }
    finally {
        Pop-Location
    }
}

switch ($Action) {
    'start'   { Start-Dev }
    'stop'    { Stop-Dev }
    'restart' { Stop-Dev; Start-Dev }
    'status'  { Show-Status }
    'log'     { Show-Log }
    'build'   { Build-App }
}

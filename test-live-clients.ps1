<#
.SYNOPSIS
    Tests the Meraki /devices/{serial}/switch/liveTools/clients endpoint.
    This is an Early Access API - may 404 if not enabled for your org.

.PARAMETER Serial
    Switch serial number (e.g. Q2XX-XXXX-XXXX). Prompts if not supplied.

.PARAMETER MaxPoll
    Maximum number of 2-second poll attempts (default: 10).

.EXAMPLE
    .\test-live-clients.ps1 -Serial Q2HP-XXXX-XXXX
#>
param(
    [string]$Serial = "Q2DY-HRFF-L8UJ",
    [int]$MaxPoll = 10
)

# ---------------------------------------------------------------------------
# Load .env
# ---------------------------------------------------------------------------
$envFile = Join-Path $PSScriptRoot ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | Where-Object { $_ -match '^\s*[^#]\S+=\S' } | ForEach-Object {
        $key, $val = $_ -split '=', 2
        $val = $val -replace "^'|'$", ''   # strip single quotes
        [System.Environment]::SetEnvironmentVariable($key.Trim(), $val.Trim())
    }
}

$apiKey  = $env:MERAKI_API_KEY
$baseUrl = if ($env:MERAKI_BASE_URL) { $env:MERAKI_BASE_URL } else { "https://api.meraki.com/api/v1" }

if (-not $apiKey) {
    Write-Error "MERAKI_API_KEY not set (check .env)"
    exit 1
}

if (-not $Serial) {
    $Serial = Read-Host "Switch serial number"
}

$headers = @{
    "X-Cisco-Meraki-API-Key" = $apiKey
    "Content-Type"           = "application/json"
}

# ---------------------------------------------------------------------------
# Step 1 - Create job
# ---------------------------------------------------------------------------
$createUrl = "$baseUrl/devices/$Serial/switch/liveTools/clients"
Write-Host "`nPOST $createUrl" -ForegroundColor Cyan

try {
    $create = Invoke-RestMethod -Uri $createUrl -Method POST -Headers $headers -Body "{}" -ErrorAction Stop
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "FAILED ($status): $_" -ForegroundColor Red
    if ($status -eq 404) {
        Write-Host "404 = endpoint not available on this org (Early Access not enabled, or unsupported firmware)." -ForegroundColor Yellow
    }
    exit 1
}

Write-Host "Response:" -ForegroundColor Green
$create | ConvertTo-Json -Depth 5

# Try to extract a job ID - key name unknown until we see a real response
$jobId = $create.clientsId ?? $create.id ?? $create.jobId ?? $null
if (-not $jobId) {
    Write-Host "`nNo job ID found in response - cannot poll. Raw response above." -ForegroundColor Yellow
    exit 0
}

Write-Host "`nJob ID: $jobId" -ForegroundColor Cyan

# ---------------------------------------------------------------------------
# Step 2 - Poll for results
# ---------------------------------------------------------------------------
$pollUrl = "$baseUrl/devices/$Serial/switch/liveTools/clients/$jobId"
for ($i = 1; $i -le $MaxPoll; $i++) {
    Start-Sleep -Seconds 2
    Write-Host "Poll $i/$MaxPoll ..." -ForegroundColor DarkGray

    try {
        $result = Invoke-RestMethod -Uri $pollUrl -Method GET -Headers $headers -ErrorAction Stop
    } catch {
        Write-Host "Poll error: $_" -ForegroundColor Red
        break
    }

    $st = $result.status
    Write-Host "  status: $st"

    if ($st -eq "complete") {
        Write-Host "`nResults:" -ForegroundColor Green
        $result | ConvertTo-Json -Depth 10
        exit 0
    } elseif ($st -eq "failed") {
        Write-Host "Job failed." -ForegroundColor Red
        $result | ConvertTo-Json -Depth 5
        exit 1
    }
}

Write-Host "`nTimed out after $MaxPoll polls. Last response:" -ForegroundColor Yellow
$result | ConvertTo-Json -Depth 5

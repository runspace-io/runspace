$ErrorActionPreference = 'Stop'

Write-Host 'Checking Runspace local services...'
$checks = @(
  @{ Name = 'web'; Url = 'http://localhost:3000' },
  @{ Name = 'gateway'; Url = 'http://localhost:8080/healthz' },
  @{ Name = 'metrics'; Url = 'http://localhost:8080/metrics' },
  @{ Name = 'nats'; Url = 'http://localhost:8222/healthz' }
)

foreach ($check in $checks) {
  try {
    $response = Invoke-WebRequest -Uri $check.Url -UseBasicParsing -TimeoutSec 5
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 400) { throw "HTTP $($response.StatusCode)" }
    Write-Host "PASS $($check.Name): $($response.StatusCode)"
  } catch {
    Write-Error "FAIL $($check.Name): $($_.Exception.Message)"
    exit 1
  }
}

docker compose --profile tools run --rm nats-box sh -ec 'nats sub chat.message --count=1 > /tmp/chat-event.out & subscriber=$!; sleep 1; nats pub chat.message "{\"type\":\"chat.message\",\"text\":\"smoke\"}"; wait $subscriber; grep -q smoke /tmp/chat-event.out'
if ($LASTEXITCODE -ne 0) { throw 'NATS publish/consume smoke check failed' }
Write-Host 'PASS nats chat.message publish/consume'

Write-Host 'All smoke checks passed.'

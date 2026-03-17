$ErrorActionPreference = "Stop"
$env:LEAFWIKI_JWT_SECRET="test-secret"
$env:LEAFWIKI_ADMIN_PASSWORD="test-admin"

Write-Host "--- Starting leafwiki for API testing ---"
$process = Start-Process -FilePath ".\leafwiki.exe" -ArgumentList "--data-dir .\test-wiki-data --port 9090 --api-key my-test-key" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

try {
    Write-Host "--- Testing POST API ---"
    $headers = @{ "Authorization" = "Bearer my-test-key"; "Content-Type" = "application/json" }
    $body = @{ title = "API Test Page"; slug = "api-test" } | ConvertTo-Json
    $response = Invoke-RestMethod -Uri "http://localhost:9090/api/pages" -Method Post -Headers $headers -Body $body
    Write-Host "POST Response:"
    $response | ConvertTo-Json | Write-Host

    Write-Host "`n--- Testing GET API ---"
    $pageId = $response.id
    $getResponse = Invoke-RestMethod -Uri "http://localhost:9090/api/pages/$pageId" -Method Get -Headers $headers
    Write-Host "GET Response:"
    $getResponse | ConvertTo-Json | Write-Host
} finally {
    Write-Host "`n--- Stopping leafwiki server ---"
    Stop-Process -Id $process.Id -Force
}

Write-Host "`n--- Script completed successfully ---"

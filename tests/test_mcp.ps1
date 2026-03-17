$processInfo = New-Object System.Diagnostics.ProcessStartInfo
$processInfo.FileName = ".\leafwiki.exe"
$processInfo.Arguments = "--data-dir .\test-wiki-data mcp"
$processInfo.WorkingDirectory = (Get-Location).Path
$processInfo.RedirectStandardInput = $true
$processInfo.RedirectStandardOutput = $true
$processInfo.RedirectStandardError = $true
$processInfo.UseShellExecute = $false
$processInfo.CreateNoWindow = $true

$process = New-Object System.Diagnostics.Process
$process.StartInfo = $processInfo
$process.Start() | Out-Null

$payload = '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test-client", "version": "1.0.0"}}}'
$process.StandardInput.WriteLine($payload)
$process.StandardInput.Flush()

Start-Sleep -Seconds 2

while ($process.StandardOutput.Peek() -gt -1) {
    $out = $process.StandardOutput.ReadLine()
    Write-Host "Output:" $out
}

while ($process.StandardError.Peek() -gt -1) {
    $errOutput = $process.StandardError.ReadLine()
    Write-Host "Errors:" $errOutput
}

if (-not $process.HasExited) {
    $process.Kill()
}

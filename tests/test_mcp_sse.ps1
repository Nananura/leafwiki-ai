# Test MCP SSE endpoint over HTTP using curl
$baseUrl = "http://127.0.0.1:9090"
$apiKey = "my-test-key"

Write-Host "--- Testing MCP SSE endpoint ---"
Write-Host ""
Write-Host "1. Connecting to SSE endpoint (reading first event)..."

# Use curl to connect to SSE and grab only the first 2 lines (the endpoint event)
$sseOutput = curl.exe --silent --max-time 5 -H "Authorization: Bearer $apiKey" "$baseUrl/api/mcp/sse" 2>&1
Write-Host "   SSE output:"
Write-Host "   $sseOutput"

# Extract message endpoint from the SSE event
if ($sseOutput -match "data:\s*(.+)") {
    $messageEndpoint = $Matches[1].Trim()
    Write-Host ""
    Write-Host "2. Message endpoint: $messageEndpoint"
    
    # Send initialize request
    $initPayload = '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'
    
    Write-Host ""
    Write-Host "3. Sending initialize request to message endpoint..."
    $msgResponse = curl.exe --silent -X POST -H "Authorization: Bearer $apiKey" -H "Content-Type: application/json" -d $initPayload "$baseUrl$messageEndpoint"
    Write-Host "   Response: $msgResponse"
} else {
    Write-Host ""
    Write-Host "   Could not extract message endpoint from SSE output."
    Write-Host "   (This may happen if the connection timed out before receiving the event)"
}

Write-Host ""
Write-Host "--- Test Complete ---"
Write-Host ""
Write-Host "To use with VS Code, add this to .vscode/mcp.json:"
Write-Host '{
  "servers": {
    "leafwiki": {
      "type": "sse",
      "url": "' -NoNewline
Write-Host "$baseUrl/api/mcp/sse" -NoNewline
Write-Host '",
      "headers": {
        "Authorization": "Bearer ' -NoNewline
Write-Host "$apiKey" -NoNewline
Write-Host '"
      }
    }
  }
}'

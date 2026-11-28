# Script to seed notification CloudEvents directly into Kafka
param (
    [string]$UserId = "4014742f-94b5-4dd2-964e-2e8de009c966",
    [string]$Scope = "DIRECT", # "DIRECT", "GROUP", "BROADCAST"
    [string[]]$GroupUserIds = @(),
    [string]$GroupId = "",
    [string]$Title = "Thông báo thử nghiệm từ Kafka",
    [string]$Body = "Nội dung kiểm thử hệ thống thông báo đa kênh",
    [string[]]$Channels = @("IN_APP", "PUSH", "EMAIL"),
    [string]$Email = "dangviethung213@gmail.com",
    [string]$Phone = "+84912345678",
    [string]$Topic = "notification.events.high-priority",
    [string]$Priority = "HIGH"
)

# Normalize Channels if passed as comma-separated string
if ($Channels.Count -eq 1 -and $Channels[0] -match ',') {
    $Channels = $Channels[0].Split(',') | ForEach-Object { $_.Trim().ToUpper() }
} else {
    $Channels = $Channels | ForEach-Object { $_.Trim().ToUpper() }
}

$eventId = [guid]::NewGuid().ToString()
$eventType = if ($Priority -eq "HIGH") {
    "contracts.generic.notification.v1.event.HighPriorityNotificationRequested"
} else {
    "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested"
}

# Build target payload based on scope
$targetObj = @{}
switch ($Scope.ToUpper()) {
    "BROADCAST" {
        $targetObj = @{
            allUsers = $true
        }
    }
    "GROUP" {
        $targetObj = @{
            groupId = if ($GroupId) { $GroupId } else { "test-group-1" }
            userList = @{
                userIds = if ($GroupUserIds.Count -gt 0) { $GroupUserIds } else { @($UserId) }
            }
        }
    }
    Default {
        $targetObj = @{
            userId = $UserId
        }
    }
}

$payloadObj = @{
    specversion     = "1.0"
    id              = $eventId
    source          = "manual/seed-script"
    type            = $eventType
    time            = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    datacontenttype = "application/json"
    data            = @{
        target   = $targetObj
        title    = $Title
        body     = $Body
        channels = $Channels
        data     = @{
            email = $Email
            phone = $Phone
        }
    }
}

$payload = $payloadObj | ConvertTo-Json -Depth 6 -Compress

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "Publishing CloudEvent [$eventId] to Kafka" -ForegroundColor Cyan
Write-Host "Topic   : $Topic" -ForegroundColor DarkGray
Write-Host "Scope   : $Scope" -ForegroundColor DarkGray
Write-Host "Channels: $($Channels -join ', ')" -ForegroundColor DarkGray
Write-Host "Payload : $payload" -ForegroundColor DarkGray
Write-Host "==========================================================" -ForegroundColor Cyan

$payload | docker exec -i fitai-kafka-test /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 --topic $Topic

if ($LASTEXITCODE -eq 0) {
    Write-Host "`n>>> SUCCESS: Event published into Kafka successfully! <<<" -ForegroundColor Green
    Write-Host "`n1. Check in-app notifications in DB:" -ForegroundColor Yellow
    Write-Host "   docker exec fitai-postgres-test psql -U postgres -d fitai -c ""SELECT id, user_id, title, body, created_at FROM notification.in_app_notifications ORDER BY created_at DESC LIMIT 3;"""
    Write-Host "`n2. Check outbox_log (idempotency & dispatch state):" -ForegroundColor Yellow
    Write-Host "   docker exec fitai-postgres-test psql -U postgres -d fitai -c ""SELECT event_id, status, error_message, processed_at FROM notification.outbox_log ORDER BY processed_at DESC LIMIT 3;"""
    Write-Host "`n3. Check consumer & email/push dispatch logs in fitai-ai:" -ForegroundColor Yellow
    Write-Host "   docker logs --tail 40 fitai-ai"
} else {
    Write-Host "`n>>> FAILED: Failed to publish event to Kafka. <<<" -ForegroundColor Red
}

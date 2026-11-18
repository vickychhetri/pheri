param(
    [string]$CsvPath = "c:\Users\vicky\Desktop\pj\pheri\pheri_feature_roadmap_tracker.csv",
    [string]$Repo = "vickychhetri/pheri"
)

# 1. Obtain GitHub authentication token from Git Credential Manager
$cred = "protocol=https`nhost=github.com" | git credential fill
$token = ($cred | Where-Object { $_ -match '^password=(.+)$' } | ForEach-Object { $Matches[1] })

if (-not $token) {
    Write-Error "Failed to retrieve GitHub authentication token from Git Credential Manager."
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $token"
    "Accept"        = "application/vnd.github+json"
    "User-Agent"    = "PowerShell-Pheri-Roadmap-Sync"
}

# 2. Read tasks from CSV
if (-not (Test-Path $CsvPath)) {
    Write-Error "CSV file not found at: $CsvPath"
    exit 1
}

$tasks = Import-Csv $CsvPath

# 3. Fetch existing issues to avoid duplicate titles
$existingIssues = @{}
try {
    $page = 1
    while ($true) {
        $resp = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/issues?state=all&per_page=100&page=$page" -Headers $headers
        if (-not $resp -or $resp.Count -eq 0) { break }
        foreach ($iss in $resp) {
            $existingIssues[$iss.title.Trim()] = $iss
        }
        if ($resp.Count -lt 100) { break }
        $page++
    }
    Write-Host "Found $($existingIssues.Count) existing issues/PRs in $Repo" -ForegroundColor Cyan
} catch {
    Write-Warning "Could not fetch existing issues: $_"
}

# 4. Map categories and priorities to clean label names
function Get-Labels($task) {
    $labels = @()
    
    # Priority
    if ($task.Priority -match 'P0') { $labels += "priority: p0-critical" }
    elseif ($task.Priority -match 'P1') { $labels += "priority: p1-high" }
    elseif ($task.Priority -match 'P2') { $labels += "priority: p2-medium" }
    elseif ($task.Priority -match 'P3') { $labels += "priority: p3-low" }

    # Phase
    if ($task.Phase -match 'Phase 1') { $labels += "phase-1: reliability" }
    elseif ($task.Phase -match 'Phase 2') { $labels += "phase-2: transactions-crud" }
    elseif ($task.Phase -match 'Phase 3') { $labels += "phase-3: editor" }
    elseif ($task.Phase -match 'Phase 4') { $labels += "phase-4: security-connectivity" }
    elseif ($task.Phase -match 'Phase 5') { $labels += "phase-5: architecture-diagnostics" }

    # Category
    $cat = $task.Category.ToLower() -replace '\s+', '-'
    $labels += "category: $cat"

    return $labels
}

# 5. Process each task from CSV
$createdCount = 0
$skippedCount = 0

foreach ($task in $tasks) {
    $title = "[$($task.ID)] $($task.Feature)"
    
    # Check if issue already exists
    if ($existingIssues.ContainsKey($title)) {
        Write-Host "Skipping existing issue: $title (#$($existingIssues[$title].number))" -ForegroundColor Gray
        $skippedCount++
        continue
    }

    $labels = Get-Labels -task $task

    # Build rich Markdown description body
    $body = @"
## 📌 Overview & Feature Summary
**$($task.Feature)** ($($task.ID))

$($task.Notes)

---

## 🔍 Problem Statement / Current State
> $($task.CurrentState)

---

## 🎯 Proposed Solution & Changes
$($task.ProposedChange)

---

## 📋 Task Metadata
| Attribute | Value |
| :--- | :--- |
| **Task Code** | ``$($task.ID)`` |
| **Phase** | ``$($task.Phase)`` |
| **Category** | ``$($task.Category)`` |
| **Priority** | ``$($task.Priority)`` |
| **Estimated Effort** | ``$($task.Effort)`` |
| **Target Files** | ``$($task.TargetFiles)`` |

---

## 🏁 Acceptance Criteria
- [ ] Implementation complete in target files: ``$($task.TargetFiles)``
- [ ] Safe non-blocking execution with proper error handling
- [ ] Automated unit test added in ``ui/`` or relevant package
- [ ] Task documentation created at ``docs/tasks/$($task.ID)_...md``
"@

    if ($task.Status -eq "Done") {
        $body += "`n`n> [!NOTE]`n> **Status**: Resolved & merged in Pull Request #15."
    }

    $payload = @{
        title  = $title
        body   = $body
        labels = $labels
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/issues" -Method Post -Headers $headers -Body $payload
        Write-Host "Created issue #$($response.number): $title ($($response.html_url))" -ForegroundColor Green
        $createdCount++

        # If already Done, close the issue and link to PR #15
        if ($task.Status -eq "Done") {
            $closePayload = @{ state = "closed"; state_reason = "completed" } | ConvertTo-Json
            Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/issues/$($response.number)" -Method Patch -Headers $headers -Body $closePayload | Out-Null
            Write-Host "  -> Marked issue #$($response.number) as closed (Resolved in PR #15)" -ForegroundColor Yellow
        }

        # Polite delay to respect GitHub API rate limits
        Start-Sleep -Milliseconds 800
    } catch {
        Write-Error "Failed to create issue for $($task.ID): $_"
        Start-Sleep -Seconds 2
    }
}

Write-Host "`nSummary: $createdCount issues created, $skippedCount skipped." -ForegroundColor Cyan

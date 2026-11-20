param(
    [string]$Search = "PHERI-04",
    [string]$Repo = "vickychhetri/pheri"
)

$cred = "protocol=https`nhost=github.com" | git credential fill
$token = ($cred | Where-Object { $_ -match '^password=(.+)$' } | ForEach-Object { $Matches[1] })

$headers = @{
    "Authorization" = "Bearer $token"
    "Accept"        = "application/vnd.github+json"
    "User-Agent"    = "PowerShell"
}

$issues = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/issues?per_page=100&state=all" -Headers $headers
$matches = $issues | Where-Object { $_.title -like "*$Search*" -or $_.body -like "*$Search*" }
foreach ($m in $matches) {
    [PSCustomObject]@{
        Number   = $m.number
        Title    = $m.title
        State    = $m.state
        HTML_URL = $m.html_url
        Body     = $m.body
    } | Format-List
}

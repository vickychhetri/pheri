param(
    [int]$PRNumber = 47,
    [string]$Repo = "vickychhetri/pheri"
)

$cred = "protocol=https`nhost=github.com" | git credential fill
$token = ($cred | Where-Object { $_ -match '^password=(.+)$' } | ForEach-Object { $Matches[1] })

$headers = @{
    "Authorization" = "Bearer $token"
    "Accept"        = "application/vnd.github+json"
    "User-Agent"    = "PowerShell"
}

$pr = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/pulls/$PRNumber" -Headers $headers
[PSCustomObject]@{
    Number    = $pr.number
    Title     = $pr.title
    State     = $pr.state
    Head      = $pr.head.ref
    Base      = $pr.base.ref
    Commits   = $pr.commits
    Mergeable = $pr.mergeable
    HTML_URL  = $pr.html_url
} | Format-List

# ipad_sim.ps1 — Simulates iPad CalDAV discovery using raw .NET WebRequest
# so that custom HTTP methods (PROPFIND) work under PowerShell 5.1.
#
# Usage: .\ipad_sim.ps1 [-Base http://localhost] [-CustomName "Mein Kalender"]

param(
    [string]$Base = "http://localhost",
    [string]$CustomName = ""
)

$allpropXml = '<?xml version="1.0" encoding="UTF-8"?><A:propfind xmlns:A="DAV:"><A:allprop/></A:propfind>'
$xmlBytes   = [System.Text.Encoding]::UTF8.GetBytes($allpropXml)

function WebDAV([string]$method, [string]$url, [string]$depth = "0", [byte[]]$body = $xmlBytes) {
    $req = [System.Net.HttpWebRequest]::Create($url)
    $req.Method = $method
    $req.Headers.Add("Depth", $depth)
    $req.ContentType = "application/xml; charset=utf-8"
    $req.AllowAutoRedirect = $false
    $req.Timeout = 10000

    if ($body -and $body.Length -gt 0) {
        $req.ContentLength = $body.Length
        $stream = $req.GetRequestStream()
        $stream.Write($body, 0, $body.Length)
        $stream.Close()
    }

    try {
        $resp = $req.GetResponse()
    } catch [System.Net.WebException] {
        $resp = $_.Exception.Response
    }

    $code    = [int]$resp.StatusCode
    $loc     = $resp.Headers["Location"]
    $davHdr  = $resp.Headers["DAV"]
    $bodyStream = $resp.GetResponseStream()
    $content = (New-Object System.IO.StreamReader($bodyStream)).ReadToEnd()
    $resp.Close()
    return [PSCustomObject]@{ Status = $code; Location = $loc; DavHeader = $davHdr; Body = $content }
}

function OK  ([string]$msg) { Write-Host "  [OK]   $msg" -ForegroundColor Green }
function FAIL([string]$msg) { Write-Host "  [FAIL] $msg" -ForegroundColor Red }
function INFO([string]$msg) { Write-Host "  [INFO] $msg" -ForegroundColor Yellow }

function Check([string]$body, [string]$pattern) {
    if ($body.IndexOf($pattern, [System.StringComparison]::OrdinalIgnoreCase) -ge 0) {
        OK $pattern ; return $true
    } else {
        FAIL "MISSING: $pattern" ; return $false
    }
}

function DisplayNames([string]$body) {
    [regex]::Matches($body, '<[Dd]:displayname>([^<]*)</[Dd]:displayname>') |
        ForEach-Object { $_.Groups[1].Value } | Where-Object { $_ -ne "" }
}

Write-Host ""
Write-Host "======================================================" -ForegroundColor Cyan
Write-Host " iPad CalDAV Discovery Simulation  ->  $Base" -ForegroundColor Cyan
Write-Host "======================================================" -ForegroundColor Cyan

# --- Current settings ---------------------------------------------------------
Write-Host "`n[Settings] GET /admin/api/v1/settings"
try {
    $sResp = Invoke-RestMethod -Uri "$Base/admin/api/v1/settings" -Method GET -UseBasicParsing
    Write-Host "  default_calendar_name = '$($sResp.default_calendar_name)'"
    Write-Host "  sender_name           = '$($sResp.sender_name)'"
} catch {
    INFO "settings endpoint: $_"
}

# --- Step 1: /.well-known/caldav ---------------------------------------------
Write-Host "`n[Step 1] PROPFIND /.well-known/caldav  (expect 301 -> /caldav/)"
$r1 = WebDAV "PROPFIND" "$Base/.well-known/caldav"
Write-Host "  Status: $($r1.Status)  Location: $($r1.Location)"
if ($r1.Status -eq 301 -and $r1.Location -match "/caldav/") { OK "301 redirect to /caldav/" }
else { FAIL "Expected 301 -> /caldav/, got $($r1.Status) -> $($r1.Location)" }

# --- Step 2: PROPFIND / ------------------------------------------------------
Write-Host "`n[Step 2] PROPFIND /  (expect 207 + current-user-principal)"
$r2 = WebDAV "PROPFIND" "$Base/"
Write-Host "  Status: $($r2.Status)"
Check $r2.Body "current-user-principal" | Out-Null
Check $r2.Body "/caldav/user/"          | Out-Null

# --- Step 3: PROPFIND /caldav/user/ ------------------------------------------
Write-Host "`n[Step 3] PROPFIND /caldav/user/  (expect 207 + principal props)"
$r3 = WebDAV "PROPFIND" "$Base/caldav/user/"
Write-Host "  Status: $($r3.Status)"
Write-Host "  DAV header: $($r3.DavHeader)"
foreach ($p in @("calendar-home-set","schedule-inbox-URL","schedule-outbox-URL",
                 "schedule-default-calendar-URL","calendar-user-address-set",
                 "calendar-user-type","INDIVIDUAL")) {
    Check $r3.Body $p | Out-Null
}
# calendar-auto-schedule lives in the DAV: response header, not the XML body
if ($r3.DavHeader -match "calendar-auto-schedule") { OK "DAV header contains calendar-auto-schedule" }
else { FAIL "DAV header missing calendar-auto-schedule (got: '$($r3.DavHeader)')" }

# --- Step 4: Depth:1 calendar home -------------------------------------------
Write-Host "`n[Step 4] PROPFIND /caldav/user/calendars/ Depth:1  (calendar list)"
$r4 = WebDAV "PROPFIND" "$Base/caldav/user/calendars/" "1"
Write-Host "  Status: $($r4.Status)"
$n4 = DisplayNames $r4.Body
Write-Host "  Display names: $(($n4 | ForEach-Object { `"'$_'`" }) -join ', ')"
foreach ($p in @("current-user-privilege-set","write-content","free-busy-query")) {
    Check $r4.Body $p | Out-Null
}

# --- Step 5: Depth:0 individual collection -----------------------------------
Write-Host "`n[Step 5] PROPFIND /caldav/user/calendars/default/ Depth:0"
$r5 = WebDAV "PROPFIND" "$Base/caldav/user/calendars/default/"
Write-Host "  Status: $($r5.Status)"
$n5 = DisplayNames $r5.Body
Write-Host "  Display names: $(($n5 | ForEach-Object { `"'$_'`" }) -join ', ')"
foreach ($p in @("schedule-calendar-transp","free-busy-query","current-user-privilege-set","write-content")) {
    Check $r5.Body $p | Out-Null
}

# --- Phase 2: change name, re-check ------------------------------------------
if ($CustomName -ne "") {
    Write-Host ""
    Write-Host "======================================================" -ForegroundColor Magenta
    Write-Host " Changing calendar name -> '$CustomName'" -ForegroundColor Magenta
    Write-Host "======================================================" -ForegroundColor Magenta

    $putBody = "{`"default_calendar_name`":`"$CustomName`"}"
    try {
        $putResp = Invoke-RestMethod -Uri "$Base/admin/api/v1/settings" -Method PUT `
            -ContentType "application/json" -Body $putBody -UseBasicParsing
        $got = $putResp.default_calendar_name
        Write-Host "  PUT response: default_calendar_name = '$got'"
        if ($got -eq $CustomName) { OK "settings API reflects new name immediately" }
        else                      { FAIL "settings API returned '$got', expected '$CustomName'" }
    } catch {
        FAIL "PUT /admin/api/v1/settings: $_"
    }

    Write-Host "`n[Re-check Step 4] Depth:1 home after name change"
    $rr4 = WebDAV "PROPFIND" "$Base/caldav/user/calendars/" "1"
    $rn4 = DisplayNames $rr4.Body
    Write-Host "  Display names: $(($rn4 | ForEach-Object { `"'$_'`" }) -join ', ')"
    if ($rn4 -contains $CustomName) { OK "'$CustomName' present in Depth:1 home PROPFIND" }
    else                            { FAIL "'$CustomName' NOT found in Depth:1 home PROPFIND" }
    if ($rn4 -notcontains "Default Calendar") { OK "'Default Calendar' no longer a display name" }
    else { FAIL "'Default Calendar' still appears as a display name after rename" }

    Write-Host "`n[Re-check Step 5] Depth:0 /default/ after name change"
    $rr5 = WebDAV "PROPFIND" "$Base/caldav/user/calendars/default/"
    $rn5 = DisplayNames $rr5.Body
    Write-Host "  Display names: $(($rn5 | ForEach-Object { `"'$_'`" }) -join ', ')"
    if ($rn5 -contains $CustomName) { OK "'$CustomName' present in Depth:0 collection PROPFIND" }
    else                            { FAIL "'$CustomName' NOT found in Depth:0 collection PROPFIND" }
    if ($rn5 -notcontains "Default Calendar") { OK "'Default Calendar' no longer a display name" }
    else { FAIL "'Default Calendar' still appears as a display name after rename" }
}

Write-Host ""
Write-Host "======================================================" -ForegroundColor Cyan
Write-Host " Done" -ForegroundColor Cyan
Write-Host "======================================================" -ForegroundColor Cyan

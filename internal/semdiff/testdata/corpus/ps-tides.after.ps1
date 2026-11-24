# Tide tables
$max = 48
function Get-Tide([int]$n, [int]$step) {
    if ($n -gt $max) { throw 'too many readings' }
    $readings = @()
    foreach ($i in 1..$n) { $readings += $i * $step }
    return $readings
}
get-childitem -path $env:TEMP | where-object { $_.Length -gt 10 }

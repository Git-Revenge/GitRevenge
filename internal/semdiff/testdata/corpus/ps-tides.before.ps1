# Tide tables
$Max = 0x30
function Get-Tide([int]$n) {
    if ($n -GT $Max) { throw 'too many' }
    $out = @()
    foreach ($i in 1..$n) { $out += $i * 3600 }
    return $out
}
Get-ChildItem -Path $env:TEMP | Where-Object { $_.Length -gt 10 }

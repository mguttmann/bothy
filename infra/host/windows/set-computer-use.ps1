# SPDX-License-Identifier: Apache-2.0
#
# Sets host_service.computer_use to true in the configuration file of the host service
# (docs/spec/27-computer-use.md 9.3, docs/decisions.d/CU-02.md D-CU-02-10). The MSI runs it as a
# deferred custom action when the feature ComputerHelper is selected; it does what
# ../set-computer-use.sh does on macOS, line for line:
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File set-computer-use.ps1 -Config <file>
#
# A missing or empty file becomes "host_service:" with the key. Otherwise the key goes in as the
# first child of the top-level mapping host_service, with the indentation of its other children,
# or an existing computer_use child of that mapping gets the value true; comments and every other
# line stay. A file without host_service gets the mapping appended. A host_service in flow style
# ({...}) is left alone with exit code 3. The file is written in place (UTF-8 without a byte order
# mark, LF line ends), so it keeps its owner and ACL.
param([Parameter(Mandatory = $true)][string]$Config)

$ErrorActionPreference = 'Stop'
$utf8 = New-Object System.Text.UTF8Encoding($false)
$key = 'computer_use: true'

if (-not (Test-Path -LiteralPath $Config -PathType Leaf) -or (Get-Item -LiteralPath $Config).Length -eq 0) {
    [System.IO.File]::WriteAllText($Config, "host_service:`n  $key`n", $utf8)
    exit 0
}

$text = [System.IO.File]::ReadAllText($Config, $utf8)
$lines = [System.Collections.Generic.List[string]]::new()
$parts = $text -split "`r?`n"
# A trailing line break leaves one empty item after the split; it is not a line of the file.
$count = $parts.Count
if ($count -gt 0 -and $parts[$count - 1] -eq '') { $count-- }
for ($i = 0; $i -lt $count; $i++) { $lines.Add($parts[$i]) }

$found = -1; $child = 0; $cu = -1; $inBlock = $false
for ($i = 0; $i -lt $lines.Count; $i++) {
    $line = $lines[$i]
    if ($found -lt 0) {
        if ($line -match '^host_service:[ \t]*(#.*)?$') { $found = $i; $inBlock = $true; continue }
        if ($line -match '^host_service:') {
            [Console]::Error.WriteLine("set-computer-use: host_service in $Config is not a block mapping; set host_service.computer_use: true by hand")
            exit 3
        }
        continue
    }
    if (-not $inBlock) { continue }
    if ($line -match '^[ \t]*(#.*)?$') { continue }
    $indent = $line.Length - $line.TrimStart(' ').Length
    if ($indent -eq 0) { $inBlock = $false; continue }
    if ($child -eq 0) { $child = $indent }
    if ($indent -eq $child -and $line -match '^ *computer_use:') { $cu = $i }
}

if ($found -lt 0) {
    $lines.Add('host_service:')
    $lines.Add("  $key")
} elseif ($cu -ge 0) {
    $lines[$cu] = (' ' * $child) + $key
} else {
    $width = if ($child -gt 0) { $child } else { 2 }
    $lines.Insert($found + 1, (' ' * $width) + $key)
}

[System.IO.File]::WriteAllText($Config, (($lines -join "`n") + "`n"), $utf8)

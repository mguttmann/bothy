# SPDX-License-Identifier: Apache-2.0
#
# Reads the UI Automation tree of a running app, as Narrator does (brief M4-32 Design note 2),
# and prints one JSON array with an object per element: its control type ("role") and its
# name. Used by integration_test/a11y_platform_tree_test.dart on Windows, which passes the
# process id of the app under test. Read only: it changes nothing in the app.
#
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File tool/uia_probe.ps1 -ProcessId <pid>
param([Parameter(Mandatory = $true)][int]$ProcessId)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$auto = [System.Windows.Automation.AutomationElement]
$condition = New-Object System.Windows.Automation.PropertyCondition(
  $auto::ProcessIdProperty, $ProcessId)
$windows = $auto::RootElement.FindAll(
  [System.Windows.Automation.TreeScope]::Children, $condition)

$elements = New-Object System.Collections.Generic.List[object]
foreach ($window in $windows) {
  $all = $window.FindAll(
    [System.Windows.Automation.TreeScope]::Subtree,
    [System.Windows.Automation.Condition]::TrueCondition)
  foreach ($element in $all) {
    try {
      $current = $element.Current
      $elements.Add([pscustomobject]@{
          role = $current.ControlType.ProgrammaticName
          name = $current.Name
        })
    } catch {
      # An element that went away while the tree was read is skipped.
    }
  }
}
ConvertTo-Json -InputObject @($elements) -Compress -Depth 3

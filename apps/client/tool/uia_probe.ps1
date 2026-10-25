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
try {
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
  Add-Type -AssemblyName UIAutomationClient
  Add-Type -AssemblyName UIAutomationTypes

  # The top-level windows of the process. A PropertyCondition built by New-Object receives the
  # process id wrapped as a PowerShell object and fails with "Argument types do not match", so
  # the windows are filtered by their ProcessId instead.
  $all = [System.Windows.Automation.Condition]::TrueCondition
  $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll(
    [System.Windows.Automation.TreeScope]::Children, $all) |
    Where-Object { $_.Current.ProcessId -eq $ProcessId }

  $elements = New-Object System.Collections.Generic.List[object]
  foreach ($window in $windows) {
    $subtree = $window.FindAll([System.Windows.Automation.TreeScope]::Subtree, $all)
    foreach ($element in $subtree) {
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
} catch {
  # The position and the exception, so a failure on a runner says where it happened.
  [Console]::Error.WriteLine($_.InvocationInfo.PositionMessage)
  [Console]::Error.WriteLine($_.Exception.ToString())
  exit 1
}

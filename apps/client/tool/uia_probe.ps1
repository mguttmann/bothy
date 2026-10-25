# SPDX-License-Identifier: Apache-2.0
#
# Reads the accessibility tree of a running app, as Narrator does (brief M4-32 Design note 2),
# and prints one JSON array with an object per element: its role and its name. Used by
# integration_test/a11y_platform_tree_test.dart on Windows, which passes the process id of the
# app under test. Read only: it changes nothing in the app.
#
# The Flutter engine on Windows answers the request of a screen reader for the client object
# of its view (WM_GETOBJECT with OBJID_CLIENT, Active Accessibility) and turns its semantics on
# with the first one; UI Automation reaches the same tree through its proxy. The probe asks for
# that object, as Narrator does, and reads both: the UI Automation elements of the process
# ("ControlType.*") and the Active Accessibility tree of each Flutter view ("msaa:<role>").
#
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File tool/uia_probe.ps1 -ProcessId <pid>
param([Parameter(Mandatory = $true)][int]$ProcessId)

$ErrorActionPreference = 'Stop'
try {
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
  Add-Type -AssemblyName UIAutomationClient
  Add-Type -AssemblyName UIAutomationTypes
  Add-Type -ReferencedAssemblies Accessibility -TypeDefinition @"
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using Accessibility;

public static class AccessibilityProbe {
  [DllImport("oleacc.dll")]
  static extern int AccessibleObjectFromWindow(IntPtr hwnd, uint id, ref Guid iid,
      [MarshalAs(UnmanagedType.Interface)] out object accessible);

  [DllImport("oleacc.dll")]
  static extern int AccessibleChildren(IAccessible container, int start, int count,
      [Out] object[] children, out int obtained);

  const uint ObjidClient = 0xFFFFFFFC;

  public static IAccessible Root(IntPtr hwnd) {
    Guid iid = typeof(IAccessible).GUID;
    object accessible;
    return AccessibleObjectFromWindow(hwnd, ObjidClient, ref iid, out accessible) == 0
        ? accessible as IAccessible
        : null;
  }

  public static List<string[]> Walk(IntPtr hwnd) {
    var elements = new List<string[]>();
    var root = Root(hwnd);
    if (root != null) Visit(root, 0, elements);
    return elements;
  }

  static void Visit(IAccessible node, int depth, List<string[]> elements) {
    if (depth > 80) return;
    Add(node, 0, elements);
    int count;
    try { count = node.accChildCount; } catch { return; }
    if (count <= 0) return;
    var children = new object[count];
    int obtained;
    if (AccessibleChildren(node, 0, count, children, out obtained) != 0) return;
    for (int i = 0; i < obtained; i++) {
      var child = children[i] as IAccessible;
      if (child != null) {
        Visit(child, depth + 1, elements);
      } else if (children[i] is int) {
        Add(node, (int)children[i], elements);
      }
    }
  }

  static void Add(IAccessible node, int id, List<string[]> elements) {
    string role = "";
    string name = "";
    try { role = "msaa:" + Convert.ToString(node.get_accRole(id)); } catch { }
    try { name = node.get_accName(id) ?? ""; } catch { }
    elements.Add(new[] { role, name });
  }
}
"@

  # The top-level windows of the process, filtered by their ProcessId: a PropertyCondition
  # built by New-Object fails on Windows PowerShell 5.1 with "Argument types do not match".
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
        # The view of Flutter: its Active Accessibility tree, which the request also enables.
        if ($current.ClassName -eq 'FLUTTERVIEW') {
          try {
            foreach ($pair in [AccessibilityProbe]::Walk([IntPtr]$current.NativeWindowHandle)) {
              $elements.Add([pscustomobject]@{ role = $pair[0]; name = $pair[1] })
            }
          } catch {
            # Shown in the tree the test prints when it fails, so the cause is visible.
            $elements.Add([pscustomobject]@{ role = 'msaa:error'; name = "$_" })
          }
        }
      } catch {
        # An element that went away while the tree was read is skipped.
      }
    }
  }
  # ToArray, not @(...): Windows PowerShell 5.1 fails on @() over a generic list with
  # "Argument types do not match".
  ConvertTo-Json -InputObject $elements.ToArray() -Compress -Depth 3
} catch {
  # The position and the exception, so a failure on a runner says where it happened.
  [Console]::Error.WriteLine($_.InvocationInfo.PositionMessage)
  [Console]::Error.WriteLine($_.Exception.ToString())
  exit 1
}

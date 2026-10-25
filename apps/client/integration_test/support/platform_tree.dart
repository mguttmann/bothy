// SPDX-License-Identifier: Apache-2.0

// The probes of the platform accessibility tree (brief M4-32 Design note 2): what VoiceOver
// reads on macOS (the app's own accessibility hierarchy through the channel
// `bothy/a11y_probe` of the debug runner) and what Narrator reads on Windows (UI Automation of
// the app's process through `tool/uia_probe.ps1`), and the comparison with the semantics tree
// of Flutter.

import 'dart:convert';
import 'dart:io';

import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// One element of the platform tree: its role (an `AX` role on macOS, a UI Automation control
/// type on Windows) and the texts a screen reader reads as its name.
typedef NativeElement = ({String role, List<String> names});

/// What the platform must show for one node of the semantics tree.
typedef ExpectedElement = ({String kind, String label});

/// The probe of the running system, or null where the system has none (Linux, the phones).
abstract class PlatformTreeProbe {
  /// The probe of the running system, or null.
  static PlatformTreeProbe? current() {
    if (Platform.isMacOS) return _MacProbe();
    if (Platform.isWindows) return _WindowsProbe();
    return null;
  }

  /// Turns the platform accessibility of the app on, as a screen reader does when it starts.
  Future<void> enable();

  /// The elements of the app's windows.
  Future<List<NativeElement>> read();

  /// The roles a node of [kind] (`button`, `text field` or `group`) may have.
  Set<String> rolesOf(String kind);
}

class _MacProbe extends PlatformTreeProbe {
  static const MethodChannel _channel = MethodChannel('bothy/a11y_probe');

  @override
  Future<void> enable() => _channel.invokeMethod<void>('enable');

  @override
  Future<List<NativeElement>> read() async {
    final raw = await _channel.invokeListMethod<Object?>('tree') ?? const [];
    return [
      for (final item in raw.cast<Map<Object?, Object?>>())
        (
          role: '${item['role'] ?? ''}',
          names: [
            for (final key in const [
              'description',
              'title',
              'value',
              'placeholder',
            ])
              if (item[key] case final String text) text,
          ],
        ),
    ];
  }

  @override
  Set<String> rolesOf(String kind) => switch (kind) {
    'button' => const {
      'AXButton',
      'AXPopUpButton',
      'AXMenuButton',
      'AXMenuItem',
      'AXCheckBox',
      'AXRadioButton',
      'AXDisclosureTriangle',
    },
    'text field' => const {
      'AXTextField',
      'AXTextArea',
      'AXComboBox',
      'AXSearchField',
    },
    _ => const {},
  };
}

class _WindowsProbe extends PlatformTreeProbe {
  @override
  Future<void> enable() async {
    // UI Automation turns the accessibility of the engine on with its first request.
    await read();
  }

  @override
  Future<List<NativeElement>> read() async {
    final result = await Process.run(
      'powershell.exe',
      [
        '-NoProfile',
        '-NonInteractive',
        '-ExecutionPolicy',
        'Bypass',
        '-File',
        _script().path,
        '-ProcessId',
        '$pid',
      ],
      stdoutEncoding: utf8,
      stderrEncoding: utf8,
    );
    if (result.exitCode != 0) {
      throw StateError('uia_probe.ps1 failed: ${result.stderr}');
    }
    final text = (result.stdout as String).trim();
    if (text.isEmpty) return const [];
    final decoded = jsonDecode(text);
    final items = decoded is List ? decoded : [decoded];
    return [
      for (final item in items.cast<Map<String, Object?>>())
        (
          role: '${item['role'] ?? ''}',
          names: [if (item['name'] case final String name) name],
        ),
    ];
  }

  /// `tool/uia_probe.ps1` of the app: the test binary lives in the build folder below the app.
  static File _script() {
    var dir = File(Platform.resolvedExecutable).parent;
    while (true) {
      final candidate = File(
        '${dir.path}${Platform.pathSeparator}tool'
        '${Platform.pathSeparator}uia_probe.ps1',
      );
      if (candidate.existsSync()) return candidate;
      final parent = dir.parent;
      if (parent.path == dir.path) {
        throw StateError('tool/uia_probe.ps1 not found above the test binary');
      }
      dir = parent;
    }
  }

  @override
  Set<String> rolesOf(String kind) => switch (kind) {
    'button' => const {
      'ControlType.Button',
      'ControlType.SplitButton',
      'ControlType.MenuItem',
      'ControlType.CheckBox',
      'ControlType.RadioButton',
      'ControlType.ComboBox',
    },
    'text field' => const {
      'ControlType.Edit',
      'ControlType.Document',
      'ControlType.ComboBox',
    },
    _ => const {},
  };
}

/// [text] without bidi isolates and with single spaces, for comparing names.
String normalizeName(String text) => text
    .replaceAll(RegExp('[\u2066-\u2069]'), '')
    .replaceAll(RegExp(r'\s+'), ' ')
    .trim();

/// The buttons and text fields with a label in the current semantics tree, and every
/// container whose label matches [containers] (the live box summary, for example).
List<ExpectedElement> expectedElements(
  WidgetTester tester, {
  RegExp? containers,
}) {
  final out = <ExpectedElement>[];
  void visit(SemanticsNode node) {
    if (!node.isMergedIntoParent && !node.isInvisible) {
      final data = node.getSemanticsData();
      final flags = data.flagsCollection;
      final label = normalizeName(data.label);
      if (!flags.isHidden && label.isNotEmpty) {
        if (flags.isTextField) {
          out.add((kind: 'text field', label: label));
        } else if (flags.isButton) {
          out.add((kind: 'button', label: label));
        } else if (containers != null && containers.hasMatch(label)) {
          out.add((kind: 'group', label: label));
        }
      }
    }
    node.visitChildren((child) {
      visit(child);
      return true;
    });
  }

  for (final view in tester.binding.renderViews) {
    final root = view.owner?.semanticsOwner?.rootSemanticsNode;
    if (root != null) visit(root);
  }
  return out;
}

/// Whether [element] is what [expected] asks of the platform: a role of its kind (any role
/// for a container) and a name that is its label (a platform may add the hint or cut a long
/// label, so one may start with the other).
bool matchesPlatform(
  PlatformTreeProbe probe,
  ExpectedElement expected,
  NativeElement element,
) {
  final roles = probe.rolesOf(expected.kind);
  if (roles.isNotEmpty && !roles.contains(element.role)) return false;
  for (final raw in element.names) {
    final name = normalizeName(raw);
    if (name.isEmpty) continue;
    if (name == expected.label ||
        name.startsWith(expected.label) ||
        (name.length >= 8 && expected.label.startsWith(name))) {
      return true;
    }
  }
  return false;
}

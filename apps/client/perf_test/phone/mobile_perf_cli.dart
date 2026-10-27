// SPDX-License-Identifier: Apache-2.0

// The helper of tool/mobile_perf.sh (brief M5-11 Design note 5), run as `dart <this file>`: it
// imports only the Dart core libraries, so it needs no package resolution and no build.
//
//   phones            reads the output of `flutter devices --machine` from stdin and prints one
//                     line `<id>\t<ios|android>\t<name>` per attached physical phone
//   report <summary>  prints the report of build/perf/mobile-summary.json and ends with the exit
//                     code of the run: 0 pass, 1 failure, 3 not a reference device
//
// The decision itself is made in the app (mobile_perf_summary.dart); the report reads it back
// and checks that its parts agree (a pass needs a reference device and every listed target met),
// so a summary whose fields contradict each other never passes.

import 'dart:convert';
import 'dart:io';

/// The exit code of a pass on a reference device.
const int exitPass = 0;

/// The exit code of a missed target, a failed run or an unreadable summary.
const int exitFail = 1;

/// The exit code of values from a device that is not a reference device of 15 19 (a distinct
/// result, not a success: nothing is proven).
const int exitNotProven = 3;

/// One attached physical phone of `flutter devices --machine`.
final class AttachedPhone {
  /// The device [id] of Flutter, its [platform] (`ios` or `android`) and its [name].
  const AttachedPhone(this.id, this.platform, this.name);

  /// The device ID that `flutter drive -d` takes.
  final String id;

  /// `ios` or `android`.
  final String platform;

  /// The name the device reports (on iOS the name its owner gave it).
  final String name;

  /// The line of the `phones` command: ID, platform and name, separated by tabs.
  String get line => '$id\t$platform\t$name';
}

/// The physical iOS and Android devices in [machineJson], the output of
/// `flutter devices --machine`, in its order. Only the `emulator` field tells a simulator or an
/// emulator from a device (flutter_tools reports `targetPlatform` `ios` for both); Android
/// devices have the `targetPlatform` `android-<abi>`. Text before the JSON list and output
/// without one give no phone.
List<AttachedPhone> attachedPhones(String machineJson) {
  final start = machineJson.indexOf('[');
  if (start < 0) return const [];
  final Object? decoded;
  try {
    decoded = jsonDecode(machineJson.substring(start));
  } on FormatException {
    return const [];
  }
  if (decoded is! List) return const [];
  return [
    for (final d in decoded)
      if (d is Map && d['emulator'] == false && d['id'] is String)
        if (_phonePlatform(d['targetPlatform']) case final platform?)
          AttachedPhone(
            d['id']! as String,
            platform,
            d['name'] is String ? d['name']! as String : d['id']! as String,
          ),
  ];
}

String? _phonePlatform(Object? target) => switch (target) {
  'ios' => 'ios',
  final String t when t.startsWith('android-') => 'android',
  _ => null,
};

/// The exit code of mobile_perf.sh for the recorded [summary] (null when it is missing or
/// unreadable): [exitPass] only for the result `pass` of a reference device whose every target
/// passed, [exitNotProven] for the result `not_proven` of another device, [exitFail] otherwise.
int summaryExitCode(Map<String, Object?>? summary) {
  if (summary == null) return exitFail;
  final reference = summary['reference_device'];
  final results = summary['results'];
  final allPassed =
      results is Map &&
      results.isNotEmpty &&
      results.values.every((r) => r is Map && r['pass'] == true);
  return switch (summary['result']) {
    'pass'
        when reference == true &&
            summary['pass'] == true &&
            summary['targets_met'] == true &&
            allPassed =>
      exitPass,
    'not_proven' when reference == false && results is Map => exitNotProven,
    _ => exitFail,
  };
}

/// The report of [summary]: the device, whether it is a reference device, every target with
/// its value, limit and verdict, and the result.
List<String> describeSummary(Map<String, Object?> summary) {
  final device = summary['device'];
  final reference = summary['reference'];
  final lines = <String>[
    if (device is Map)
      'device: ${device['manufacturer']} ${device['model']}, ${device['os']} '
          '(${device['platform']})',
    if (summary['reference_device'] == true && reference is Map)
      'reference device of 15 19: ${reference['name']}'
    else
      'not a reference device of 15 19: the values are reported, not proven',
  ];
  final results = summary['results'];
  if (results is Map) {
    for (final MapEntry(:key, :value) in results.entries) {
      if (value is! Map) continue;
      final verdict = value['pass'] == true ? 'met' : 'MISSED';
      final note = value['note'] is String ? ' (${value['note']})' : '';
      lines.add(
        '  $key: ${_number(value['value'])} ${value['unit']}, '
        '${value['comparison'] == 'atLeast' ? 'at least' : 'at most'} '
        '${_number(value['limit'])}, ${value['samples']} samples: $verdict$note',
      );
    }
  }
  lines.add('result: ${summary['result']}');
  return lines;
}

String _number(Object? v) => switch (v) {
  null => 'none',
  final num n when n == n.roundToDouble() => n.round().toString(),
  final num n => n.toStringAsFixed(3),
  _ => '$v',
};

/// Runs the command of [args]; see the head of this file.
Future<void> main(List<String> args) async {
  switch (args) {
    case ['phones']:
      final input = await stdin.transform(utf8.decoder).join();
      for (final p in attachedPhones(input)) {
        stdout.writeln(p.line);
      }
    case ['report', final path]:
      Map<String, Object?>? summary;
      final file = File(path);
      if (file.existsSync()) {
        try {
          final decoded = jsonDecode(file.readAsStringSync());
          if (decoded is Map) summary = decoded.cast<String, Object?>();
        } on FormatException {
          summary = null;
        }
      }
      if (summary == null) {
        stdout.writeln('no readable summary in $path');
      } else {
        describeSummary(summary).forEach(stdout.writeln);
      }
      await stdout.flush();
      exit(summaryExitCode(summary));
    default:
      stderr.writeln('usage: mobile_perf_cli.dart phones | report <summary>');
      exit(2);
  }
}

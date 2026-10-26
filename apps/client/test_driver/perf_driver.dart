// SPDX-License-Identifier: Apache-2.0

/// The driver of `flutter drive --profile` for the performance level of the desktop app (brief
/// M4-34 Design note 3, brief M4-38 Design note 2): it writes the summary that
/// perf_test/perf_test.dart or perf_test/terminal_perf_test.dart reports to
/// build/perf/summary.json and ends with exit code 1 when a target `client.perf.*` of that run
/// is missed, so `make perf-desktop` fails (15 19). With `PERF_SUMMARY_MERGE=1` (the terminal
/// part of make perf-desktop) the summary joins the one already in the file, so both parts end
/// up in one summary. A failed test ends the run with exit code 1 as well.
library;

import 'dart:convert';
import 'dart:io';

import 'package:integration_test/integration_test_driver.dart';

import '../perf_test/perf_summary.dart' show mergeSummaries;

/// Runs the performance level and checks its summary.
Future<void> main() => integrationDriver(
  responseDataCallback: (data) async {
    final summary = data ?? const <String, Object?>{};
    final file = File('build/perf/summary.json')..createSync(recursive: true);
    var written = summary;
    if (Platform.environment['PERF_SUMMARY_MERGE'] == '1') {
      final text = file.readAsStringSync();
      if (text.trim().isNotEmpty) {
        written = mergeSummaries(
          (jsonDecode(text) as Map).cast<String, Object?>(),
          summary,
        );
      }
    }
    file.writeAsStringSync(const JsonEncoder.withIndent('  ').convert(written));
    stdout.writeln('perf: summary in ${file.path}');
    if (summary['pass'] != true) {
      stderr.writeln(
        'perf: a target client.perf.* was missed (see ${file.path})',
      );
      exit(1);
    }
  },
);

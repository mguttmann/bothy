// SPDX-License-Identifier: Apache-2.0

/// The driver of `flutter drive --profile` for the performance level of the phone apps (brief
/// M5-11 Design note 5): it writes the summary that perf_test/phone/mobile_perf_test.dart reports
/// to the file `MOBILE_PERF_SUMMARY` names (build/perf/mobile-summary.json by default), also
/// after a failed test, so the values measured until then are kept. It ends with exit code 1
/// when the run failed or a target `client.perf.*` was missed on a reference device of 15 19.
/// `flutter drive` turns every non-zero exit of a driver into 1, so tool/mobile_perf.sh reads
/// the result from the summary: a pass, a failure, or values from a device that is not a
/// reference device (not proven).
library;

import 'dart:convert';
import 'dart:io';

import 'package:integration_test/integration_test_driver.dart';

/// Runs the measurement and writes its summary.
Future<void> main() => integrationDriver(
  writeResponseOnFailure: true,
  responseDataCallback: (data) async {
    if (data == null) {
      stderr.writeln('perf-mobile: the measurement reported no summary');
      return;
    }
    final file = File(
      Platform.environment['MOBILE_PERF_SUMMARY'] ??
          'build/perf/mobile-summary.json',
    )..createSync(recursive: true);
    file.writeAsStringSync(const JsonEncoder.withIndent('  ').convert(data));
    stdout.writeln('perf-mobile: summary in ${file.path}');
    if (data['result'] == 'fail') {
      stderr.writeln(
        'perf-mobile: a target client.perf.* was missed on a reference device '
        '(see ${file.path})',
      );
      exit(1);
    }
  },
);

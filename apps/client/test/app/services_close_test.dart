// SPDX-License-Identifier: Apache-2.0

// The close of the services of an instance is one close: the app begins it without waiting when
// it quits, and a later caller (a test that deletes the data folder; Windows refuses to delete
// an open database file) waits for that same close instead of starting a second one.

import 'dart:io';

import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';

void main() {
  late Directory root;

  setUp(() async => root = await Directory.systemTemp.createTemp('close'));
  tearDown(() async {
    if (await root.exists()) await root.delete(recursive: true);
  });

  test('every close call returns the one close begun first', () async {
    final services = await InstanceServices.open(
      testPorts(dataRoot: root),
      InstanceInfo(
        address: Uri.parse('https://chat.example.com'),
        instanceId: '0193f2a7-0000-7000-8000-0000000000ab',
        productName: 'Team Chat',
      ),
    );
    final begun = services.close();
    final later = services.close();
    expect(identical(begun, later), isTrue);
    await later;
    expect(identical(services.close(), begun), isTrue);
  });
}

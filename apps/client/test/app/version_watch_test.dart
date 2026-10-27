// SPDX-License-Identifier: Apache-2.0

// The version gate of one instance (brief M7-13 Design note 1): every attempt asks discovery,
// a new decision is published, a refusal is published every time (also when it repeats or
// stays in force without an answer), so the app always leaves the instance below the minimum.

import 'package:client/app/discovery.dart';
import 'package:client/app/update/version_watch.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

/// Answers `checkVersion` with [next]; null means no usable answer.
final class Decisions implements DiscoveryClient {
  ClientVersionDecision? next;

  @override
  Future<ClientVersionDecision?> checkVersion(Uri address) async => next;

  @override
  Future<DiscoveryOutcome> discover(Uri address) async =>
      const DiscoveryNoInstance();
}

void main() {
  final address = Uri.parse('https://chat.example.com');

  test('admits and publishes only new decisions', () async {
    final discovery = Decisions();
    final watch = VersionWatch(discovery: discovery, address: address);
    final seen = <ClientVersionDecision>[];
    watch.changes.listen(seen.add);
    discovery.next = const ClientVersionAllowed();
    expect(await watch.admit(), isTrue);
    discovery.next = const ClientVersionUpdateRecommended('1.5.0');
    expect(await watch.admit(), isTrue);
    expect(await watch.admit(), isTrue);
    discovery.next = null;
    expect(await watch.admit(), isTrue);
    expect(seen, [const ClientVersionUpdateRecommended('1.5.0')]);
    expect(watch.current, const ClientVersionUpdateRecommended('1.5.0'));
    await watch.dispose();
  });

  test('a refusal is published every time it holds the app back', () async {
    final discovery = Decisions()
      ..next = const ClientVersionBelowMinimum('2.0.0');
    final watch = VersionWatch(
      discovery: discovery,
      address: address,
      initial: const ClientVersionBelowMinimum('2.0.0'),
    );
    final seen = <ClientVersionDecision>[];
    watch.changes.listen(seen.add);
    expect(await watch.admit(), isFalse);
    // Without an answer the refusal stays in force and is published again.
    discovery.next = null;
    expect(await watch.admit(), isFalse);
    expect(seen, [
      const ClientVersionBelowMinimum('2.0.0'),
      const ClientVersionBelowMinimum('2.0.0'),
    ]);
    await watch.dispose();
    // After the dispose an attempt still answers, without publishing.
    expect(await watch.admit(), isFalse);
  });
}

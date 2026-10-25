// SPDX-License-Identifier: Apache-2.0

// What every flow of the desktop E2E suite shares (brief M4-34 Design note 2): the world, the
// network of the process and the two actors, set up once per run in `desktop_e2e_test.dart`.

import 'package:flutter_test/flutter_test.dart';

import 'actors.dart';
import 'e2e_network.dart';
import 'e2e_world.dart';

/// The shared state of a run.
final class E2eContext {
  /// The context of [world]; [setUp] and [tearDown] are registered by the entry.
  E2eContext(this.world);

  /// The world of the run.
  final E2eWorld world;

  /// The tunnel and TLS trust of the process.
  late final E2eNetwork network;

  ApiActor? _owner;
  ApiActor? _sam;

  /// Alex's other device: issues codes, approves and revokes devices, creates posts.
  ApiActor get owner => _owner!;

  /// Sam's device.
  ApiActor get sam => _sam!;

  /// Registers the set-up of the run: the network first, then the two actors.
  void register() {
    setUpAll(() async {
      network = await E2eNetwork.start(world.server, world.caPem);
      network.install();
      _owner = await ApiActor.enroll(world, world.alex.code, name: 'Alex desk');
      _sam = await ApiActor.enroll(world, world.sam.code, name: 'Sam laptop');
    });
    tearDownAll(() async {
      await _sam?.dispose();
      await _owner?.dispose();
      await network.stop();
    });
  }

  /// A fresh enrollment code of Alex for a new app (the last issued code is the only valid one,
  /// so every app asks for its own right before it signs in).
  Future<String> alexCode() => owner.issueCode(world.alex.id);
}

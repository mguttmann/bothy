// SPDX-License-Identifier: Apache-2.0

// A scripted create API for the tests of "Create space..." and "Create channel..." (01 6.25).

import 'package:client/structure/structure_port.dart';
import 'package:domain/domain.dart';

import 'structure.dart';

/// A [StructurePort] that records its calls; a failure in [failures] is thrown by the next
/// call instead of its answer.
final class FakeStructurePort implements StructurePort {
  /// The calls in order, for example `space Dev key-1` or `channel <space> general key-1`.
  final List<String> calls = [];

  /// Failures thrown by the next calls, in order.
  final List<StructureFailure> failures = [];

  /// The space [createSpace] answers.
  SpaceId space = spaceId(7);

  /// The channel [createTextChannel] answers.
  ChannelId channel = channelId(7);

  void _fail() {
    if (failures.isNotEmpty) throw failures.removeAt(0);
  }

  @override
  Future<SpaceId> createSpace(
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('space $name $idempotencyKey');
    _fail();
    return space;
  }

  @override
  Future<ChannelId> createTextChannel(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('channel ${space.value} $name $idempotencyKey');
    _fail();
    return channel;
  }
}

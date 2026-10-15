// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

void main() {
  const v1 = ResourceVersion(1);
  const v2 = ResourceVersion(2);

  test('a newer version applies', () {
    expect(
      decideApply(local: v1, incoming: v2, typeCarriesVersion: true),
      ApplyDecision.apply,
    );
  });

  test('equal and older versions are ignored', () {
    expect(
      decideApply(local: v2, incoming: v2, typeCarriesVersion: true),
      ApplyDecision.ignoreStale,
    );
    expect(
      decideApply(local: v2, incoming: v1, typeCarriesVersion: true),
      ApplyDecision.ignoreStale,
    );
  });

  test('an absent local version applies', () {
    expect(
      decideApply(local: null, incoming: v1, typeCarriesVersion: true),
      ApplyDecision.apply,
    );
  });

  test('a versioned type without a version is not applied', () {
    expect(
      decideApply(local: v1, incoming: null, typeCarriesVersion: true),
      ApplyDecision.ignoreStale,
    );
  });

  test('types without resource.version apply in seq order', () {
    for (final type in ['reaction.added', 'pin.added', 'read_state.updated']) {
      expect(
        decideApplyForEvent(type: type, local: v2, incoming: null),
        ApplyDecision.applyInSeqOrder,
        reason: type,
      );
    }
  });

  test('message.updated carries a version', () {
    expect(
      decideApplyForEvent(type: 'message.updated', local: v2, incoming: v1),
      ApplyDecision.ignoreStale,
    );
    expect(
      decideApplyForEvent(type: 'message.updated', local: v1, incoming: v2),
      ApplyDecision.apply,
    );
  });

  test('an unknown type is unknown', () {
    expect(
      decideApplyForEvent(type: 'rocket.launched', local: v1, incoming: v2),
      ApplyDecision.unknownType,
    );
    expect(EventCatalog.lookup('rocket.launched'), isNull);
  });
}

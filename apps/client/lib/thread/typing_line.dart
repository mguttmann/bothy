// SPDX-License-Identifier: Apache-2.0

/// The typing line of the composer (04 12 item 1; components.md 9: "Sam is typing", "Several
/// people are typing"): the names of the people typing in the thread, never the reader.
library;

import 'package:domain/domain.dart';

/// The display names of [typing] from [principals], without [me], sorted; agents never type
/// (their run shows in the live box, 04 12 item 1).
List<String> typingNames(
  Set<PrincipalId> typing,
  Map<PrincipalId, Principal> principals, {
  required PrincipalId? me,
}) => [
  for (final id in typing)
    if (id != me)
      if (principals[id] case final p? when p.kind == PrincipalKind.user)
        p.displayName,
]..sort();

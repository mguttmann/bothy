// SPDX-License-Identifier: Apache-2.0

/// Appliers of `user.updated` (no `resource.version`: `seq` order) and `agent.updated` (by
/// version) (04 9 item 3).
library;

import 'package:domain/domain.dart';

import '../dispatcher.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> principalAppliers = {
  'user.updated': (c) =>
      c.db.principalsDao.put(userFromJson(obj(c.payload, 'user'))),
  'agent.updated': _agent,
};

Future<void> _agent(ApplyContext c) async {
  final card = agentFromJson(obj(c.payload, 'agent'));
  if (!await c.db.principalsDao.upsertAgentIfNewer(card)) return;
  // The agent is also the author of messages; its principal row names it in threads.
  await c.db.principalsDao.put(
    Principal(
      id: card.id,
      kind: PrincipalKind.fromCode('agent'),
      handle: card.handle,
      displayName: card.displayName,
      avatarFileId: card.avatarFileId,
    ),
  );
}

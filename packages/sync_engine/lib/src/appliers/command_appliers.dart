// SPDX-License-Identifier: Apache-2.0

/// Applier of `command.changed` (04 9 item 3): a registered or updated command by version, a
/// removed one deleted.
library;

import '../dispatcher.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> commandAppliers = {
  'command.changed': (c) async {
    final command = commandFromJson(obj(c.payload, 'command'));
    if (str(c.payload, 'change') == 'removed') {
      await c.db.commandsDao.delete(command.id);
    } else {
      await c.db.commandsDao.upsertIfNewer(command);
    }
  },
};

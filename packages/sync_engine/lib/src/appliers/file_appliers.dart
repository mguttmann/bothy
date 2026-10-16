// SPDX-License-Identifier: Apache-2.0

/// Applier of `file.updated` (no `resource.version`: `seq` order, 04 9 item 3): the file
/// metadata; a redacted file keeps only its neutral members (04 2.2 item 5).
library;

import '../dispatcher.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> fileAppliers = {
  'file.updated': (c) =>
      c.db.messagesDao.putFile(fileFromJson(obj(c.payload, 'file'))),
};

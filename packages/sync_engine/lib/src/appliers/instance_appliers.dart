// SPDX-License-Identifier: Apache-2.0

/// Applier of `instance.updated`: the payload names only the changed keys, so the instance
/// information is loaded again through `getInstance` after the commit.
library;

import '../dispatcher.dart';

/// The appliers of this group by event type.
final Map<String, Applier> instanceAppliers = {
  'instance.updated': (c) async {
    final sync = c.sync;
    c.after.reads.add(() async {
      final info = await sync.fetcher.instance();
      await sync.db.instanceDao.put(info);
    });
  },
};

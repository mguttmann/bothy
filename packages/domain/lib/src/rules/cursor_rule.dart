// SPDX-License-Identifier: Apache-2.0

/// The cursor rule of 04 9 item 2: events at or below the stored cursor are duplicates.
library;

import '../seq.dart';

/// Whether an event with [seq] is a duplicate for a client whose stored cursor is [cursor]
/// (04 9 item 2, 04 7.1). Without a cursor nothing is a duplicate.
bool isDuplicate(GlobalSeq seq, GlobalSeq? cursor) =>
    cursor != null && seq <= cursor;

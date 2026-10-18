// SPDX-License-Identifier: Apache-2.0

/// Entities, value objects and pure rules of the apps, without Flutter (15 1 principle 5,
/// 15 3.1 rule 1).
///
/// Typed IDs and the UUIDv7 generator, the entities of the local schema mirror (15 6.1), the
/// sync rules of 04 (version, cursor, read state, loaded ranges, unread counting), the display
/// rules of 01 (grouping, forum order, cost labels), the generated event catalogue and
/// `AppConfig`. No I/O, no clock and no randomness except through injected functions.
library;

export 'src/coded.dart' show Coded;
export 'src/config/app_config.dart';
export 'src/entities/channel.dart';
export 'src/entities/command.dart';
export 'src/entities/component.dart';
export 'src/entities/file.dart';
export 'src/entities/instance.dart';
export 'src/entities/mention.dart';
export 'src/entities/message.dart';
export 'src/entities/personal.dart';
export 'src/entities/principal.dart';
export 'src/entities/reaction.dart';
export 'src/entities/run.dart';
export 'src/entities/space.dart';
export 'src/entities/thread.dart';
export 'src/ids.dart';
export 'src/rules/cost_label.dart';
export 'src/rules/cursor_rule.dart';
export 'src/rules/event_catalog.dart';
export 'src/rules/forum_order.dart';
export 'src/rules/fractional_index.dart';
export 'src/rules/loaded_ranges.dart';
export 'src/rules/message_grouping.dart';
export 'src/rules/read_state_rule.dart';
export 'src/rules/unread_counter.dart';
export 'src/rules/version_rule.dart';
export 'src/seq.dart';
export 'src/uuid7.dart';

// SPDX-License-Identifier: Apache-2.0

// The current schema against 15 6.1: every table, `host_key_pins` (M4-36) and the sidebar view
// state of M4-40 included, the columns a client may see with their 03 source, and none of the
// columns 15 6.1 excludes.

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

/// Expected columns per table. Mirror tables name the 03 table they come from; columns that are
/// not in 03 come from the API object named in the comment or are local (15 6.1, 15 6.3).
const Map<String, List<String>> expected = {
  // 03 `spaces`; `level`, `capabilities`: API `SpaceView`.
  'spaces': [
    'id', 'name', 'icon', 'color', 'sort_key', 'default_operator_agent_id', //
    'created_at', 'archived_at', 'version', 'level', 'capabilities',
  ],
  // 03 `user_space_states` (one person per database: no `user_id`).
  'user_space_states': [
    'space_id',
    'collapsed',
    'sort_key',
    'version',
    'updated_by_device_id',
  ],
  // 03 `channels` without `prompt` and the behaviour fields; `effective_operator_agent_id`:
  // API `ChannelOperator.effective_agent_id`; `main_thread_id`, `dm_participant_ids`,
  // `project_bound` (derived from 03 `project_bindings`, R-DM-54): API `Channel`; `level`,
  // `capabilities`: API `ChannelView`.
  'channels': [
    'id', 'space_id', 'kind', 'name', 'topic', 'sort_key', //
    'effective_operator_agent_id',
    'forum_default_sort',
    'default_notification_level',
    'read_only',
    'main_thread_id',
    'dm_participant_ids',
    'created_at',
    'archived_at',
    'version', 'settings_version', 'level', 'capabilities', 'project_bound',
  ],
  // API `ChannelOperator.mode` and 03 `channels.operator_agent_id`.
  'channel_operators': ['channel_id', 'operator_mode', 'operator_agent_id'],
  // 03 `forum_tags` without `deleted_at`.
  'forum_tags': ['id', 'channel_id', 'name', 'color', 'sort_key', 'restricted'],
  // 03 `threads` without the search columns; `billing_mode`: API `Thread`; local
  // `last_accessed_at` (15 6.3).
  'threads': [
    'id',
    'channel_id',
    'kind',
    'parent_message_id',
    'title',
    'title_source',
    'state', //
    'closed_at',
    'closed_by',
    'operator_agent_id',
    'billing_mode',
    'last_message_seq',
    'unread_floor_seq',
    'last_activity_at',
    'message_count',
    'created_by',
    'created_at',
    'moved_at', 'version', 'last_accessed_at',
  ],
  // 03 `thread_tags`.
  'thread_tags': ['thread_id', 'tag_id'],
  // 03 `messages` without `client_payload_sha256`, the import and search columns;
  // `content_truncated`, `mention_ids`, `pinned`: API `Message`.
  'messages': [
    'id', 'thread_id', 'channel_id', 'thread_seq', 'author_principal_id', //
    'client_msg_id',
    'origin',
    'kind',
    'system_code',
    'content_md',
    'content_format',
    'content_truncated', 'reply_to_message_id', 'run_id', 'trigger_state',
    'trigger_state_reason',
    'trigger_state_at',
    'silent',
    'mention_ids',
    'pinned',
    'version', 'edited_at', 'deleted_at', 'created_at',
  ],
  // 03 `reactions` summarized per emoji.
  'reactions_summary': ['message_id', 'emoji', 'count', 'me'],
  // 03 `mentions` of the person.
  'my_mentions': ['message_id', 'principal_id', 'thread_id'],
  // 03 `message_components`.
  'message_components': [
    'id',
    'message_id',
    'custom_id',
    'type',
    'definition',
    'position',
    'single_use', //
    'required_capability',
    'restricted_to',
    'run_id',
    'state',
    'expires_at',
    'used_by',
    'used_at', 'version',
  ],
  // 03 `files` without `object_key`, scan engine, transcript text and search columns;
  // `thumbnails`: 03 `file_thumbnails`.
  'files': [
    'id',
    'sha256',
    'size_bytes',
    'mime_type',
    'original_name',
    'uploaded_by', //
    'purpose',
    'scope_channel_id',
    'upload_state',
    'scan_status',
    'width',
    'height',
    'duration_ms',
    'transcript_status',
    'transcript_language',
    'transcript_source',
    'thumbnails', 'created_at',
  ],
  // 03 `message_attachments`.
  'message_attachments': [
    'message_id',
    'file_id',
    'position',
    'thread_id',
    'channel_id',
  ],
  // 03 `runs`, summary only; `channel_id`, `step_count`: API `RunSummary`.
  'runs': [
    'id', 'thread_id', 'channel_id', 'agent_id', 'triggered_by', 'state', //
    'state_reason',
    'version',
    'billing_mode_expected',
    'billing_mode_used',
    'status_line',
    'summary',
    'cost_micros',
    'currency',
    'cost_is_estimate',
    'step_count',
    'duration_ms',
    'queued_at', 'started_at', 'finished_at',
  ],
  // 03 `agents` card plus `principals` display fields.
  'agents': [
    'principal_id',
    'handle',
    'display_name',
    'avatar_file_id',
    'description', //
    'billing_mode',
    'tool_class',
    'reported_runtime_state',
    'reported_billing_mode',
    'version',
  ],
  // 03 `principals` (API `PrincipalRef`).
  'principals': ['id', 'kind', 'handle', 'display_name', 'avatar_file_id'],
  // 03 `commands`; `scope_id`: API `Command`.
  'commands': [
    'id',
    'owner_principal_id',
    'name',
    'description',
    'scope_kind',
    'scope_id', //
    'options', 'required_level', 'control', 'version',
  ],
  // 03 `read_states`; `mention_count`, `unread_count`: API `ReadState`.
  'read_states': [
    'thread_id',
    'channel_id',
    'last_read_seq',
    'mention_count',
    'unread_count', //
    'version', 'updated_by_device_id', 'updated_at',
  ],
  // 03 `drafts`.
  'drafts': [
    'id',
    'thread_id',
    'channel_id',
    'content_md',
    'reply_to_message_id',
    'file_ids', //
    'version', 'updated_by_device_id', 'updated_at',
  ],
  // 03 `user_settings`.
  'user_settings': [
    'key',
    'value',
    'version',
    'updated_by_device_id',
    'updated_at',
  ],
  // 03 `notification_preferences`.
  'notification_preferences': [
    'target_kind',
    'target_id',
    'level',
    'muted_until',
    'version',
  ],
  // 03 `quiet_hours`.
  'quiet_hours': [
    'id',
    'weekdays',
    'start_local',
    'end_local',
    'time_zone',
    'enabled',
    'version',
  ],
  // 03 `instance_settings` (branding) and API `InstanceInfo`.
  'instance': [
    'instance_id',
    'product_name',
    'accent_color',
    'logo_file_id',
    'branding_version', //
    'locale_default', 'spec_version', 'features', 'limits',
    'app_lock', // M5-08
  ],
  // Local only (15 6.1, Design note 1).
  'outbox': [
    'id',
    'kind',
    'thread_id',
    'depends_on',
    'payload',
    'expected_version',
    'state', //
    'attempts', 'next_attempt_at', 'last_error_code', 'last_error_reason',
    'conflict_current', 'created_at', 'state_changed_at',
  ],
  'sync_state': [
    'id', 'cursor', 'sync_epoch', 'head_seq', 'permission_version', //
    'gateway_session_id', 'disconnected_at', 'last_snapshot_at',
  ],
  'loaded_ranges': ['thread_id', 'from_seq', 'to_seq'],
  'file_cache': [
    'file_id',
    'variant',
    'path',
    'size_bytes',
    'last_accessed_at',
  ],
  'search_index': [
    'kind',
    'target_id',
    'thread_id',
    'channel_id',
    'space_id',
    'body',
  ],
  'share_inbox': [
    'id',
    'target_thread_id',
    'payload',
    'file_paths',
    'created_at',
  ],
  // Local only (M4-19): the Done section per forum on this device (01 6.1).
  'forum_view_states': ['channel_id', 'done_collapsed'],
  // Local only (M4-23): the recently visited targets of the quick switcher (01 D6).
  'recent_targets': ['kind', 'target_id', 'visited_at'],
  // Local only (M4-36): the pinned host keys of this device (15 23.3).
  'host_key_pins': [
    'host_id',
    'noise_public_key',
    'key_generation',
    'key_fingerprint',
    'first_seen_at',
  ],
  // Local only (M4-40): the collapsed thread group per channel on this device (01 6.8 item 7).
  'channel_view_states': ['channel_id', 'group_collapsed'],
  // Local only (M4-40, M4-42): the pinned forum overview and the hidden channel column next to
  // an open thread on this device, one row (01 D1, D2, D15).
  'shell_view_state': ['id', 'overview_pinned', 'channel_column_hidden'],
};

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  Future<List<String>> columns(String table) async {
    final rows = await db.customSelect("PRAGMA table_info('$table')").get();
    return [for (final r in rows) r.read<String>('name')];
  }

  test('schema version 7 has exactly the tables of 15 6.1', () async {
    expect(db.schemaVersion, 7);
    final rows = await db
        .customSelect(
          "SELECT name FROM sqlite_master WHERE type = 'table' "
          "AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'search_index_%'",
        )
        .get();
    final tables = {for (final r in rows) r.read<String>('name')};
    expect(tables, expected.keys.toSet());
  });

  for (final entry in expected.entries) {
    test('${entry.key} has the documented columns', () async {
      expect(await columns(entry.key), unorderedEquals(entry.value));
    });
  }

  test(
    'no table has a column readable only with manage or kept on the server',
    () async {
      for (final table in expected.keys) {
        final names = await columns(table);
        for (final banned in ['prompt', 'skills', 'client_payload_sha256']) {
          expect(names, isNot(contains(banned)), reason: '$table.$banned');
        }
      }
    },
  );

  test(
    'the search index is an FTS5 table with the unicode61 tokenizer',
    () async {
      final row = await db
          .customSelect(
            "SELECT sql FROM sqlite_master WHERE name = 'search_index'",
          )
          .getSingle();
      final sql = row.read<String>('sql');
      expect(sql, contains('fts5'));
      expect(sql, contains('unicode61 remove_diacritics 2'));
    },
  );
}

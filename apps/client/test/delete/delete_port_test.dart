// SPDX-License-Identifier: Apache-2.0

// The delete port over the generated client (01 6.18; openapi `deleteChannel`, `restoreSpace`,
// `getChannelDeletion`): `ChannelDelete.trash_project_folder` is always sent, false without the
// box; a space and a channel carry the step-up purpose, a chat none; `DeletionInfo` maps to the
// counts, `undo_until` and the folder; a refused undo keeps the server's code and reason.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiClient, ProblemInterceptor;
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:client/delete/delete_port.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

const String spaceId = '0192f4a0-7b1c-72d4-8bae-000000000001';
const String forumId = '0192f4a0-7b1c-72d4-8bae-000000000002';
const String threadId = '0192f4a0-7b1c-72d4-8bae-000000000003';

Map<String, Object?> deletion(String kind, String id, {String? state}) => {
  'target_kind': kind,
  'target_id': id,
  'state': state ?? 'pending',
  'deleted_at': '2026-11-01T09:00:00Z',
  'undo_until': '2026-11-01T09:00:30Z',
  'contents': {
    'channels': 0,
    'threads': 2,
    'messages': 140,
    'files': 3,
    'sessions': 2,
  },
  'project_folder': {
    'workspace_ref': 'website',
    'trash_check': {'ok': false, 'reason': 'unpushed_commits'},
    if (state == 'final') 'trash_state': 'refused',
    if (state == 'final') 'trash_reason': 'uncommitted_changes',
  },
};

/// Answers every request from [routes] (`METHOD path` to status and body) and records it.
final class RecordingAdapter implements HttpClientAdapter {
  RecordingAdapter(this.routes);

  final Map<String, (int, Object?)> routes;

  /// The requests: `METHOD path`, the decoded body and the request's extra.
  final List<(String, Object?, Map<String, Object?>)> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final key = '${options.method} ${options.uri.path}';
    final bytes = requestStream == null
        ? <int>[]
        : [for (final chunk in await requestStream.toList()) ...chunk];
    seen.add((
      key,
      bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes)),
      options.extra,
    ));
    final (status, body) = routes[key] ?? (404, {'code': 'not_found'});
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [
          status >= 400 ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  const project = DeleteTarget(
    kind: DeleteKind.project,
    id: forumId,
    name: 'website',
  );
  const channel = DeleteTarget(
    kind: DeleteKind.channel,
    id: forumId,
    name: 'general',
  );
  const chat = DeleteTarget(kind: DeleteKind.chat, id: threadId, name: 'x');
  const space = DeleteTarget(kind: DeleteKind.space, id: spaceId, name: 'T');

  late RecordingAdapter adapter;
  late ApiDeletePort port;

  setUp(() {
    adapter = RecordingAdapter({
      'POST /v1/channels/$forumId/delete': (200, deletion('channel', forumId)),
      'GET /v1/channels/$forumId/deletion': (
        200,
        deletion('channel', forumId, state: 'final'),
      ),
      'POST /v1/threads/$threadId/delete': (200, deletion('thread', threadId)),
      'POST /v1/spaces/$spaceId/delete': (200, deletion('space', spaceId)),
      'POST /v1/spaces/$spaceId/restore': (
        409,
        {
          'type': 'about:blank',
          'title': 'Conflict',
          'status': 409,
          'code': 'conflict',
          'reason': 'undo_window_passed',
        },
      ),
    });
    port = ApiDeletePort(
      ApiClient(
        dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com/v1'))
          ..httpClientAdapter = adapter
          // As the app's client: an error answer becomes its problem.
          ..interceptors.add(ProblemInterceptor()),
        interceptors: const [],
      ),
      newKey: () => 'k',
    );
  });

  test('a project delete sends the box and the step-up purpose', () async {
    for (final trash in [false, true]) {
      await port.delete(
        project,
        trashFolder: trash,
        idempotencyKey: 'k',
        purpose: 'Delete the project website',
      );
    }
    expect(
      [for (final s in adapter.seen) s.$2],
      [
        {'trash_project_folder': false},
        {'trash_project_folder': true},
      ],
    );
    expect(
      adapter.seen.first.$3[stepUpPurposeExtra],
      'Delete the project website',
    );
  });

  test('a channel without a binding never asks to move a folder', () async {
    await port.delete(
      channel,
      trashFolder: true,
      idempotencyKey: 'k',
      purpose: 'p',
    );
    expect(adapter.seen.single.$2, {'trash_project_folder': false});
  });

  test('a chat delete carries no step-up purpose', () async {
    final view = await port.delete(
      chat,
      trashFolder: false,
      idempotencyKey: 'k',
      purpose: '',
    );
    expect(adapter.seen.single.$1, 'POST /v1/threads/$threadId/delete');
    expect(adapter.seen.single.$3.containsKey(stepUpPurposeExtra), isFalse);
    expect(view.undoUntil, DateTime.utc(2026, 11, 1, 9, 0, 30));
    expect(
      (view.chats, view.messages, view.files, view.sessions),
      (2, 140, 3, 2),
    );
  });

  test('the folder state maps the check and the outcome', () async {
    final view = await port.info(project);
    final folder = view.folder!;
    expect(folder.name, 'website');
    expect((folder.checkOk, folder.checkReason), (false, 'unpushed_commits'));
    expect(
      (folder.trashState, folder.trashReason),
      (FolderTrashState.refused, 'uncommitted_changes'),
    );
  });

  test('a restore after the window keeps the code and the reason', () async {
    await expectLater(
      port.restore(space, idempotencyKey: 'k', purpose: 'Restore the space T'),
      throwsA(
        isA<NewProjectFailure>()
            .having((f) => f.code, 'code', 'conflict')
            .having((f) => f.reason, 'reason', 'undo_window_passed'),
      ),
    );
    expect(adapter.seen.single.$3[stepUpPurposeExtra], 'Restore the space T');
  });
}

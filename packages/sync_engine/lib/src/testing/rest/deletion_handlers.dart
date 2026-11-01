// SPDX-License-Identifier: Apache-2.0

/// Handlers of deleting and restoring spaces, channels and threads and of their deletion info
/// (01 6.18, D-DEL-1 to D-DEL-6): the world's `DeletionWorld` does the checks and the events,
/// the step-up comes from the action catalogue (`space.delete` and `channel.delete` with
/// their restores ask for it, `thread.delete` does not).
library;

import '../action_catalog.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// Registers the deletion handlers on [rest], with the step-up of [catalog].
void registerDeletionHandlers(RestFake rest, ActionCatalog catalog) {
  final world = rest.world;

  FakeResponse ok(JsonMap body) => FakeResponse(200, body);

  void stepUp(FakeRequest r) => requireCatalogStepUp(r, rest.contract, catalog);

  rest
    ..handle(
      'getSpaceDeletion',
      (r) => ok(world.deletionInfo(r.person.id, 'space', _space(r))),
    )
    ..handle('deleteSpace', (r) {
      final id = _space(r);
      stepUp(r);
      return ok(world.deleteSpace(r.person.id, id));
    })
    ..handle('restoreSpace', (r) {
      final id = _space(r);
      stepUp(r);
      return ok(world.restoreSpace(r.person.id, id));
    })
    ..handle(
      'getChannelDeletion',
      (r) => ok(world.deletionInfo(r.person.id, 'channel', _channel(r))),
    )
    ..handle('deleteChannel', (r) {
      final id = _channel(r);
      stepUp(r);
      return ok(
        world.deleteChannel(
          r.person.id,
          id,
          trashProjectFolder: r.json['trash_project_folder'] == true,
        ),
      );
    })
    ..handle('restoreChannel', (r) {
      final id = _channel(r);
      stepUp(r);
      return ok(world.restoreChannel(r.person.id, id));
    })
    ..handle(
      'getThreadDeletion',
      (r) => ok(world.deletionInfo(r.person.id, 'thread', _thread(r))),
    )
    ..handle(
      'deleteThread',
      (r) => ok(world.deleteThread(r.person.id, r.param('thread_id'))),
    )
    ..handle(
      'restoreThread',
      (r) => ok(world.restoreThread(r.person.id, r.param('thread_id'))),
    );
}

/// The space of [r] when the person sees it (also a deleted one); else `404 not_found`.
String _space(FakeRequest r) {
  final id = r.param('space_id');
  if (!r.world.spaces.containsKey(id) || !r.world.seesSpace(r.person.id, id)) {
    throw const FakeProblem(404, 'not_found');
  }
  return id;
}

/// The channel of [r] when the person sees it (also a deleted one); else `404 not_found`.
String _channel(FakeRequest r) {
  final id = r.param('channel_id');
  if (!r.world.seesChannel(r.person.id, id)) {
    throw const FakeProblem(404, 'not_found');
  }
  return id;
}

/// The thread of [r] when the person sees its channel (also a deleted thread); else
/// `404 not_found`.
String _thread(FakeRequest r) {
  final id = r.param('thread_id');
  final thread = r.world.threads[id];
  if (thread == null ||
      !r.world.seesChannel(r.person.id, thread['channel_id']! as String)) {
    throw const FakeProblem(404, 'not_found');
  }
  return id;
}

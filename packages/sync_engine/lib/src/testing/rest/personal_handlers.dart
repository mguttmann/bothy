// SPDX-License-Identifier: Apache-2.0

/// Handlers of the personal state: read states, drafts and settings (04 14).
library;

import '../rest_fake.dart';

/// Registers the personal handlers on [rest].
void registerPersonalHandlers(RestFake rest) {
  final world = rest.world;
  rest
    ..handle(
      'putReadState',
      (r) => FakeResponse(
        200,
        world.putReadState(
          r.person.id,
          r.param('thread_id'),
          r.json['last_read_seq']! as int,
          deviceId: r.session!.deviceId,
        ),
      ),
    )
    ..handle(
      'putThreadDraft',
      (r) => FakeResponse(
        200,
        world.putDraft(r.person.id, r.param('thread_id'), r.json, deviceId: r.session!.deviceId),
      ),
    )
    ..handle('deleteThreadDraft', (r) {
      world.deleteDraft(r.person.id, r.param('thread_id'));
      return const FakeResponse.noContent();
    })
    ..handle('listMyReadStates', (r) {
      final channel = r.query('channel_id');
      return FakeResponse(200, {
        'items': [
          for (final MapEntry(:key, :value) in world.readStates.entries)
            if (key.startsWith('${r.person.id}|') && value['channel_id'] == channel) value,
        ],
      });
    })
    ..handle(
      'listMyDrafts',
      (r) => FakeResponse(200, {
        'items': [
          for (final MapEntry(:key, :value) in world.drafts.entries)
            if (key.startsWith('${r.person.id}|')) value,
        ],
      }),
    )
    ..handle(
      'listMySettings',
      (r) => FakeResponse(200, {
        'items': [...?world.settings[r.person.id]?.values],
      }),
    )
    ..handle(
      'putMySetting',
      (r) => FakeResponse(
        200,
        world.putSetting(r.person.id, r.param('key'), r.json['value'], deviceId: r.session!.deviceId),
      ),
    )
    ..handle('deleteMySetting', (r) {
      world.deleteSetting(r.person.id, r.param('key'));
      return const FakeResponse.noContent();
    });
}

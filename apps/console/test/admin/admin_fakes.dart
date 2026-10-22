// SPDX-License-Identifier: Apache-2.0

/// Answers of the administration operations for the console tests: Alex (owner) and Sam, the
/// spaces Support and Engineering with one channel each, and a stored view.
library;

import '../support/fakes.dart';

/// Alex, the signed-in owner.
const alexId = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';

/// Sam.
const samId = '0192f4a0-7b1c-7f89-8d6b-6960453241bd';

/// The space Support.
const supportId = '0192f4a0-7b1c-7881-8522-0a58634abc14';

/// The space Engineering.
const engineeringId = '0192f4a0-7b1c-7881-8522-0a58634abc15';

/// The channel Support/general.
const generalId = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';

/// The channel Engineering/builds.
const buildsId = '0192f4a0-7b1c-72d4-8bae-7661d2b2f204';

Map<String, Object?> _user(String id, String handle, String name, bool owner) =>
    {
      'id': id,
      'handle': handle,
      'display_name': name,
      'status': 'active',
      'is_owner': owner,
      'locale': 'en',
      'time_zone': 'UTC',
      'created_at': '2026-10-10T09:30:00.000Z',
    };

Map<String, Object?> _space(String id, String name, String level) => {
  'space': {
    'id': id,
    'name': name,
    'sort_key': 'a0',
    'version': 3,
    'created_at': '2026-10-10T09:30:00.000Z',
  },
  'level': level,
  'capabilities': <String>[],
};

Map<String, Object?> _channel(
  String id,
  String spaceId,
  String name,
  String level,
) => {
  'channel': {
    'id': id,
    'space_id': spaceId,
    'kind': 'text',
    'name': name,
    'topic': '',
    'sort_key': 'a1',
    'read_only': false,
    'operator': {'mode': 'inherit'},
    'version': 3,
    'settings_version': 1,
    'created_at': '2026-10-10T09:30:00.000Z',
  },
  'level': level,
  'capabilities': <String>[],
};

/// A ConsoleSession body with a step-up and, with [viewAsEnds], a stored view of Sam.
Map<String, Object?> adminSession({DateTime? viewAsEnds}) => {
  ...consoleSession(stepUp: true),
  if (viewAsEnds != null)
    'view_as': {
      'user_id': samId,
      'display_name': 'Sam',
      'started_at': viewAsEnds
          .subtract(const Duration(minutes: 15))
          .toUtc()
          .toIso8601String(),
      'ends_at': viewAsEnds.toUtc().toIso8601String(),
    },
};

/// Registers the read operations of the people list, the owner flag, the spaces of the owner
/// and the snapshot of `viewAsUser` for Sam on [server].
void adminRoutes(FakeServer server) {
  server.routes['GET /api/v1/users'] = (_) => Answer(200, {
    'items': [
      _user(alexId, 'alex', 'Alex', true),
      _user(samId, 'sam', 'Sam', false),
    ],
  });
  server.routes['GET /api/v1/users/$samId'] = (_) =>
      Answer(200, _user(samId, 'sam', 'Sam', false));
  server.routes['GET /api/v1/users/$alexId/devices'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  server.routes['GET /api/v1/users/$samId/devices'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  server.routes['GET /api/v1/me'] = (_) => const Answer(200, {
    'principal': {
      'id': alexId,
      'kind': 'user',
      'handle': 'alex',
      'display_name': 'Alex',
    },
    'is_owner': true,
    'time_zone': 'UTC',
    'permission_version': 1,
  });
  server.routes['GET /api/v1/spaces'] = (_) => Answer(200, {
    'items': [
      _space(supportId, 'Support', 'owner'),
      _space(engineeringId, 'Engineering', 'owner'),
    ],
  });
  server.routes['GET /api/v1/spaces/$supportId/channels'] = (_) => Answer(200, {
    'items': [_channel(generalId, supportId, 'general', 'owner')],
  });
  server.routes['GET /api/v1/spaces/$engineeringId/channels'] = (_) =>
      Answer(200, {
        'items': [_channel(buildsId, engineeringId, 'builds', 'owner')],
      });
  server.routes['GET /api/v1/users/$samId/view-as'] = (_) => Answer(200, {
    'user': {
      'id': samId,
      'kind': 'user',
      'handle': 'sam',
      'display_name': 'Sam',
    },
    'expires_at': '2026-10-10T09:45:00.000Z',
    'spaces': [_space(supportId, 'Support', 'write')],
    'channels': [_channel(generalId, supportId, 'general', 'write')],
  });
}

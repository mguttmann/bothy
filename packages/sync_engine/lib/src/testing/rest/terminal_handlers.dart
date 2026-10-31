// SPDX-License-Identifier: Apache-2.0

/// Handlers of the terminal sessions (docs/spec/24-terminal.md 6, 8, 13; brief M4-36 Scope):
/// `listTerminalSessions`, `getTerminalSession`, `openTerminalSession`,
/// `attachTerminalSession` and `closeTerminalSession` over the [FakeTerminal] of the world.
///
/// A small model of the server: opening needs the capability `terminal` from a grant, SR10
/// (the assignee), SR12 (an own session user the host reports), a known profile and, for
/// `claude`, an index into the own working directories (ruling T13), a connected host and a
/// fresh, unused step-up; attaching needs one too, except from the same device and login
/// session within `terminal.quick_reattach_window` after the detach (24 6.2), and an attach
/// from another device closes the open stream with `4103`. Every change appends
/// `terminal_session.changed`. The fake decides nothing beyond these rules.
library;

import '../fake_host.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'self_handlers.dart' show requireStepUp;

/// Registers the terminal handlers on [rest].
void registerTerminalHandlers(RestFake rest) {
  final world = rest.world;
  final terminal = world.terminal;

  FakeTerminalSession own(FakeRequest r) {
    final s = terminal.sessions[r.param('session_id')];
    if (s == null || (s.userId != r.person.id && !r.person.isOwner)) {
      throw const FakeProblem(404, 'not_found');
    }
    return s;
  }

  /// A fresh, unused step-up (08 6.3): the confirmation is consumed by this request.
  void freshStepUp(FakeRequest r) {
    requireStepUp(r, fresh: true);
    r.session!.stepUpAt = null;
  }

  bool holdsTerminal(String userId, FakeHost h) => world.grants.values.any(
    (g) =>
        g['revoked_at'] == null &&
        g['target_kind'] == 'host' &&
        g['target_id'] == h.id &&
        g['subject_kind'] == 'user' &&
        g['subject_id'] == userId &&
        ((g['capabilities'] as List?) ?? const []).contains('terminal'),
  );

  JsonMap grantOf(FakeTerminalSession s, FakeHost h) {
    final token = 'tst.${world.nextId()}';
    s.token = token;
    return {
      'session': s.json,
      'session_token': token,
      'session_token_expires_at': world.stamp(
        world.now().add(terminal.sessionTokenTtl),
      ),
      'stream_path': '/api/v1/terminal/sessions/${s.id}/stream',
      'noise_public_key': h.noisePublicKey,
      'key_fingerprint': h.fingerprint,
      'key_generation': h.keyGeneration,
      'noise_protocol': 'Noise_NK_25519_ChaChaPoly_SHA256',
      'prologue_version': 'bothy-terminal/1',
    };
  }

  Never denied(String reason) =>
      throw FakeProblem(403, 'terminal_denied', reason: reason);

  rest
    ..handle('listTerminalSessions', (r) {
      final me = r.person;
      final host = r.query('host_id');
      final state = r.query('state');
      final items = [
        for (final s in terminal.sessions.values)
          if ((me.isOwner || s.userId == me.id) &&
              (host == null || s.hostId == host) &&
              (state == null || s.state == state))
            s.json,
      ];
      return FakeResponse(200, {'items': items});
    })
    ..handle('getTerminalSession', (r) => FakeResponse(200, own(r).json))
    ..handle('openTerminalSession', (r) {
      final me = r.person;
      final h =
          world.hosts[r.param('host_id')] ??
          (throw const FakeProblem(404, 'not_found'));
      if (!holdsTerminal(me.id, h)) {
        throw const FakeProblem(403, 'forbidden', reason: 'capability_missing');
      }
      final assignee = h.assignedUserId;
      if (assignee != null && assignee != me.id) denied('not_host_assignee');
      final osUser = h.sessionUsers[me.id]?.$1;
      if (osUser == null || !(h.reported ?? const []).contains(osUser)) {
        denied('no_session_user');
      }
      final body = r.json;
      final profileName = body['profile']! as String;
      final profiles = (h.json['profiles']! as List).cast<JsonMap>().where(
        (p) => p['name'] == profileName,
      );
      if (profiles.isEmpty) denied('profile_unknown');
      final index = body['workdir_index'] as int?;
      final claude = profiles.first['kind'] == 'claude';
      final own = h.workdirs[osUser] ?? const <String>[];
      if (claude != (index != null) ||
          (index != null && (index < 0 || index >= own.length))) {
        denied('profile_unknown');
      }
      final open = terminal.sessions.values.where(
        (s) => s.userId == me.id && !s.closed,
      );
      if (open.length >= terminal.maxSessionsPerPerson) {
        denied('session_limit');
      }
      if (h.state != 'active' || !h.connected) {
        throw const FakeProblem(409, 'host_unavailable');
      }
      freshStepUp(r);
      final s = FakeTerminalSession(
        id: world.nextId(),
        hostId: h.id,
        userId: me.id,
        deviceId: r.session!.deviceId,
        loginSessionId: r.session!.id,
        profile: profileName,
        openedAt: world.stamp(),
      );
      terminal.sessions[s.id] = s;
      world.terminalSessionChanged(s);
      return FakeResponse(201, grantOf(s, h));
    })
    ..handle('attachTerminalSession', (r) {
      final s = own(r);
      if (s.userId != r.person.id) throw const FakeProblem(404, 'not_found');
      if (s.closed) throw const FakeProblem(409, 'invalid_state_transition');
      final h = world.hosts[s.hostId]!;
      final session = r.session!;
      final detached = s.detachedAt;
      final quick =
          s.deviceId == session.deviceId &&
          s.loginSessionId == session.id &&
          detached != null &&
          world.now().difference(detached) <= terminal.quickReattachWindow;
      if (!quick) freshStepUp(r);
      if (s.deviceId != session.deviceId) {
        // 24 6.2: the connection of the other device ends with 4103.
        terminal.closeStream(s.id, 4103);
        s.deviceId = session.deviceId;
      }
      s.loginSessionId = session.id;
      world.terminalSessionChanged(s);
      return FakeResponse(200, grantOf(s, h));
    })
    ..handle('closeTerminalSession', (r) {
      final s = own(r);
      world.endTerminalSession(
        s,
        s.userId == r.person.id ? 'client_closed' : 'closed_by_owner',
      );
      return FakeResponse(200, s.json);
    });
}

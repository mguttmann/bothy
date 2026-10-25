// SPDX-License-Identifier: Apache-2.0

/// Handlers of grants, their preview, "Who has access?" and "View as person" (brief M4-27
/// Scope; 07 2, 6, 9.1 to 9.3; 05 `grant.*`, `access.*`).
///
/// A small model of the server: a grant to a person or agent on a channel or space sets its
/// level in the channels' access lists of the world, and revoking removes it. The preview
/// answers the example request of 07 9.3 with the text of the specification, read at run time
/// ([grantPreviewExample], D-W-94); other requests get a short English text. H1 (07 6) is
/// derived from the access lists: an agent that sees a channel the subject does not see. Since
/// M4-28 every grant runs through [FakeI1Guard] (10 5.4, 11): `resolve_i1`, the I1 check after
/// the change, the audit rows; the preview reports a violation as `invariant_violations`.
library;

import 'dart:io';

import '../action_catalog.dart';
import '../audit_log.dart';
import '../fake_host.dart';
import '../rest_fake.dart';
import '../schema_validator.dart' show embeddedPermissionsChapter;
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;
import 'agent_handlers.dart';

/// The example text of 07 9.3: the first fenced block between the `language-check` markers
/// after the heading `### 9.3` of `docs/spec/07-permissions.md` of the repository that holds
/// [from] (default: the working directory). Without such a repository (an app under test in
/// the macOS app sandbox) it reads the copy compiled into the fake
/// (`contract_documents.g.dart`, docs/DECISIONS.md D-M4-27-7).
///
/// Throws a [StateError] when the chapter has no marked example.
String grantPreviewExample({Directory? from}) {
  var dir = (from ?? Directory.current).absolute;
  while (true) {
    final file = File('${dir.path}/docs/spec/07-permissions.md');
    if (file.existsSync()) return _example(file.readAsStringSync());
    if (dir.parent.path == dir.path) {
      return _example(embeddedPermissionsChapter());
    }
    dir = dir.parent;
  }
}

String _example(String spec) {
  final start = spec.indexOf('\n### 9.3');
  final off = start < 0 ? -1 : spec.indexOf('language-check: off', start);
  final on = off < 0 ? -1 : spec.indexOf('language-check: on', off);
  if (on < 0) {
    throw StateError('07 9.3 has no example between language-check markers');
  }
  final block = spec.substring(off, on);
  final open = block.indexOf('```text\n') + '```text\n'.length;
  return block.substring(open, block.indexOf('\n```', open));
}

const List<String> _levels = ['none', 'read', 'write', 'trigger'];

String _max(String a, String b) =>
    _levels.indexOf(a) >= _levels.indexOf(b) ? a : b;

/// Registers the grant and access handlers on [rest], with the step-up of [catalog]; the
/// audit rows of `createGrant` go to [audit].
void registerAccessHandlers(
  RestFake rest,
  ActionCatalog catalog,
  FakeAuditLog audit,
) {
  final world = rest.world;
  final guard = FakeI1Guard(world, audit);

  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
  }

  void stepUp(FakeRequest r) => requireCatalogStepUp(r, rest.contract, catalog);

  /// The channels a target covers; `404` for a target the world does not have.
  List<FakeChannel> covered(String kind, String? id) => switch (kind) {
    'instance' => world.channels.values.toList(),
    'space' when world.spaces.containsKey(id) => [
      for (final c in world.channels.values)
        if (c.spaceId == id) c,
    ],
    'channel' when world.channels.containsKey(id) => [world.channels[id]!],
    _ => throw const FakeProblem(404, 'not_found'),
  };

  String levelOn(String principal, List<FakeChannel> channels) =>
      channels.map((c) => c.access[principal] ?? 'none').fold('none', _max);

  String targetPath(String kind, String? id) => switch (kind) {
    'channel' => () {
      final c = world.channels[id]!;
      return '${world.spaces[c.spaceId]?['name']}/${c.json['name']}';
    }(),
    'space' => world.spaces[id]!['name']! as String,
    _ => 'instance',
  };

  /// H1 (07 6): for [person] with `trigger` on [channels], every agent there and every channel
  /// the agent sees that [person] does not.
  List<JsonMap> readHorizon(String person, List<FakeChannel> channels) => [
    for (final agent in world.people.values)
      if (agent.kind == 'agent' &&
          channels.any((c) => c.access.containsKey(agent.id)))
        for (final other in world.channels.values)
          if (other.access.containsKey(agent.id) &&
              !other.access.containsKey(person) &&
              !channels.contains(other))
            {
              'code': 'read_horizon',
              'agent_id': agent.id,
              'principal_id': person,
              'channel_id': other.id,
            },
  ];

  JsonMap grantOf(FakeRequest r) =>
      world.grants[r.param('grant_id')] ??
      (throw const FakeProblem(404, 'not_found'));

  /// The entries of "Who has access?" for [channels] (one channel, or a space's channels).
  List<JsonMap> explain(List<FakeChannel> channels, {required bool space}) {
    final entries = <JsonMap>[];
    final seen = <String>{};
    for (final c in channels) {
      seen.addAll(c.access.keys);
    }
    for (final id in seen) {
      final p = world.people[id];
      if (p == null) continue;
      final ids = {for (final c in channels) c.id};
      final spaceIds = {for (final c in channels) c.spaceId};
      final grants = [
        for (final g in world.grants.values)
          if (g['revoked_at'] == null && g['subject_id'] == id)
            if ((g['target_kind'] == 'channel' &&
                    ids.contains(g['target_id'])) ||
                (g['target_kind'] == 'space' &&
                    spaceIds.contains(g['target_id'])) ||
                g['target_kind'] == 'instance')
              g,
      ];
      final sources = <JsonMap>[
        if (p.isOwner) {'kind': 'owner'},
        for (final g in grants)
          if (g['target_kind'] == 'channel' ||
              (space && g['target_kind'] == 'space'))
            {
              'kind': 'direct',
              'grant_id': g['id'],
              'expires_at': ?g['expires_at'],
              'reason': g['reason'],
            }
          else
            {
              'kind': 'inherited',
              'grant_id': g['id'],
              'from_target_kind': g['target_kind'],
              'from_target_id': ?g['target_id'],
              'expires_at': ?g['expires_at'],
              'reason': g['reason'],
            },
      ];
      if (sources.isEmpty) {
        // Access the seed set without a grant: the operator rule for an agent, else a
        // direct entry (the fake has no grant rows for its seed).
        sources.add(
          p.kind == 'agent'
              ? {'kind': 'operator', 'agent_id': p.id}
              : {'kind': 'direct'},
        );
      }
      entries.add({
        'principal': p.ref,
        'level': levelOn(id, channels),
        if (p.status != 'active') 'inactive': true,
        'capabilities': <Object?>[],
        'sources': sources,
      });
    }
    // H1 (07 6, 9.1): a person with trigger next to an agent here learns content through it.
    for (final p in world.people.values) {
      if (p.kind != 'user' || seen.contains(p.id)) continue;
      final agents = <String>{
        for (final c in world.channels.values)
          if (c.access[p.id] == 'trigger')
            for (final a in c.access.keys)
              if (world.people[a]?.kind == 'agent' &&
                  channels.any((x) => x.access.containsKey(a)))
                a,
      };
      if (agents.isEmpty) continue;
      entries.add({
        'principal': p.ref,
        'level': 'none',
        'capabilities': <Object?>[],
        'sources': [
          for (final a in agents) {'kind': 'via_agent', 'agent_id': a},
        ],
      });
    }
    return entries;
  }

  /// The host of a grant request; `404` for a host the world does not have.
  FakeHost hostOf(JsonMap body) =>
      world.hosts[body['target_id']] ??
      (throw const FakeProblem(404, 'not_found'));

  /// The preview of a grant `terminal` on a host (07 2.12, 9.3): SR10 (a non-owner who is not
  /// the assignee) and SR12 (no valid session user whose name the host reports) give the hint
  /// `terminal_not_effective` and a sentence with the cause, as the server's templates do.
  FakeResponse hostPreview(JsonMap body) {
    final h = hostOf(body);
    final subject = body['subject_id']! as String;
    final person = world.people[subject];
    final name = person?.displayName ?? subject;
    final notAssignee =
        person != null && !person.isOwner && h.assignedUserId != subject;
    final own = h.sessionUsers[subject];
    final noSessionUser =
        own == null || !(h.reported ?? const []).contains(own.$1);
    final cause = notAssignee
        ? 'Not effective: $name is not the assignee of this host (SR10).'
        : noSessionUser
        ? 'Not effective: no session user (SR12).'
        : null;
    return FakeResponse(200, {
      'allowed': true,
      'effects': <Object?>[],
      'invariant_violations': <Object?>[],
      'hints': [
        if (cause != null)
          {'code': 'terminal_not_effective', 'principal_id': subject},
      ],
      'text': [
        '$name gets the capability "terminal" on the host ${h.name}.',
        ?cause,
      ].join('\n'),
      'text_locale': 'en',
    });
  }

  /// Creates a grant `terminal` on a host: no level, I6 checked (24 7.3).
  FakeResponse hostGrant(FakeRequest r, JsonMap body) {
    final h = hostOf(body);
    final subject = body['subject_id']! as String;
    if (h.llmCredentials == 'personal' &&
        (body['subject_kind'] != 'user' || subject != h.credentialHolderId)) {
      throw const FakeProblem(
        409,
        'i6_violation',
        extra: {'hint': 'Only the holder of the personal login may get it.'},
      );
    }
    final id = world.nextId();
    final grant = <String, Object?>{
      'id': id,
      'subject_kind': body['subject_kind'],
      'subject_id': subject,
      'target_kind': 'host',
      'target_id': h.id,
      'capabilities': body['capabilities'] ?? <Object?>[],
      'expires_at': ?body['expires_at'],
      'reason': body['reason'],
      'granted_by': r.person.id,
      'granted_at': world.stamp(),
    };
    world.grants[id] = grant;
    world.permissionVersion++;
    return FakeResponse(201, grant);
  }

  /// Writes a grant of [body] on [channels] by [grantedBy]: the row, the levels of the
  /// subject in the access lists and the permission version; returns the `Grant`.
  JsonMap applyGrant(
    JsonMap body,
    List<FakeChannel> channels,
    String grantedBy,
  ) {
    final subject = body['subject_id']! as String;
    final level = body['level'] as String? ?? 'read';
    final id = world.nextId();
    final grant = <String, Object?>{
      'id': id,
      'subject_kind': body['subject_kind'],
      'subject_id': subject,
      'target_kind': body['target_kind'],
      'target_id': ?body['target_id'],
      'level': level,
      'capabilities': body['capabilities'] ?? <Object?>[],
      'expires_at': ?body['expires_at'],
      'reason': body['reason'],
      'granted_by': grantedBy,
      'granted_at': world.stamp(),
    };
    world.grants[id] = grant;
    if (world.people.containsKey(subject)) {
      for (final c in channels) {
        c.access[subject] = _max(c.access[subject] ?? 'none', level);
      }
    }
    world.permissionVersion++;
    return grant;
  }

  /// The details of the audit row `grant.create`.
  JsonMap grantDetails(JsonMap grant) => {
    'subject_kind': grant['subject_kind'],
    'subject_id': grant['subject_id'],
    'target_kind': grant['target_kind'],
    'target_id': ?grant['target_id'],
    'level': ?grant['level'],
    'reason': grant['reason'],
  };

  rest
    ..handle('previewGrant', (r) {
      owner(r);
      final body = r.json;
      if (body['target_kind'] == 'host') return hostPreview(body);
      final kind = body['target_kind']! as String;
      final target = body['target_id'] as String?;
      final channels = covered(kind, target);
      final subject = body['subject_id']! as String;
      final subjectKind = body['subject_kind']! as String;
      final level = body['level'] as String? ?? 'read';
      final person = world.people[subject];
      final before = levelOn(subject, channels);
      final expires = body['expires_at'] as String?;
      final isExample =
          person?.displayName == 'Sam' &&
          kind == 'channel' &&
          targetPath(kind, target) == 'Marketing/Kampagnen' &&
          level == 'trigger' &&
          expires != null;
      // 10 11 point 1: the grant as a dry run (with its `resolve_i1`), then rolled back.
      final conflicts = guard.dryRun(
        body,
        () => applyGrant(body, channels, r.person.id),
      );
      final text = isExample
          ? grantPreviewExample()
          : '${person?.displayName ?? subject} gets the level "$level" on '
                '${targetPath(kind, target)}.';
      return FakeResponse(200, {
        'allowed': conflicts.isEmpty,
        'effects': [
          if (person != null)
            {
              'principal': person.ref,
              'target_kind': kind,
              'target_id': ?target,
              'before_level': before,
              'after_level': _max(before, level),
              'capabilities_added': body['capabilities'] ?? <Object?>[],
            },
        ],
        'invariant_violations': guard.previewViolations(conflicts),
        'hints': [
          if (level == 'trigger' && person != null)
            ...readHorizon(subject, channels),
          if (subjectKind == 'user' && expires == null)
            {'code': 'expiry_missing', 'principal_id': subject},
        ],
        'text': [text, ...guard.previewSentences(conflicts)].join('\n'),
        'text_locale': isExample ? 'de' : 'en',
      });
    })
    ..handle('createGrant', (r) {
      owner(r);
      stepUp(r);
      final body = r.json;
      // The checks that write nothing come first (10 11: 404 before the agents of resolve_i1).
      final host = body['target_kind'] == 'host';
      final channels = host
          ? const <FakeChannel>[]
          : covered(
              body['target_kind']! as String,
              body['target_id'] as String?,
            );
      return guard.run(
        r,
        action: 'grant.create',
        resourceKind: 'grant',
        apply: () {
          final response = host
              ? hostGrant(r, body)
              : FakeResponse(201, applyGrant(body, channels, r.person.id));
          final grant = response.body! as JsonMap;
          return (
            response: response,
            resourceId: grant['id']! as String,
            details: grantDetails(grant),
          );
        },
      );
    })
    ..handle('listGrants', (r) {
      owner(r);
      final kind = r.query('target_kind');
      final target = r.query('target_id');
      final subjectKind = r.query('subject_kind');
      final subject = r.query('subject_id');
      return FakeResponse(200, {
        'items': [
          for (final g in world.grants.values)
            if (g['revoked_at'] == null &&
                (kind == null || g['target_kind'] == kind) &&
                (target == null || g['target_id'] == target) &&
                (subjectKind == null || g['subject_kind'] == subjectKind) &&
                (subject == null || g['subject_id'] == subject))
              g,
        ],
      });
    })
    ..handle('getGrant', (r) {
      owner(r);
      return FakeResponse(200, grantOf(r));
    })
    ..handle('revokeGrant', (r) {
      owner(r);
      final g = grantOf(r);
      stepUp(r);
      if (g['revoked_at'] != null) throw const FakeProblem(409, 'conflict');
      g
        ..['revoked_at'] = world.stamp()
        ..['revoked_by'] = r.person.id
        ..['revoke_reason'] = r.json['reason'];
      final subject = g['subject_id']! as String;
      if (g['target_kind'] == 'host') {
        world.permissionVersion++;
        return FakeResponse(200, g);
      }
      for (final c in covered(
        g['target_kind']! as String,
        g['target_id'] as String?,
      )) {
        c.access.remove(subject);
      }
      world.permissionVersion++;
      return FakeResponse(200, g);
    })
    ..handle('explainSpaceAccess', (r) {
      owner(r);
      final id = r.param('space_id');
      return FakeResponse(200, {
        'target_kind': 'space',
        'target_id': id,
        'entries': explain(covered('space', id), space: true),
      });
    })
    ..handle('explainChannelAccess', (r) {
      owner(r);
      final id = r.param('channel_id');
      return FakeResponse(200, {
        'target_kind': 'channel',
        'target_id': id,
        'entries': explain(covered('channel', id), space: false),
      });
    })
    ..handle('viewAsUser', (r) {
      owner(r);
      stepUp(r);
      final p = world.people[r.param('user_id')];
      if (p == null || p.kind != 'user') {
        throw const FakeProblem(404, 'not_found');
      }
      return FakeResponse(200, {
        'user': p.ref,
        'expires_at': world.stamp(world.now().add(const Duration(minutes: 15))),
        'spaces': world.spaceViews(p.id),
        'channels': world.channelViews(p.id),
      });
    });
}

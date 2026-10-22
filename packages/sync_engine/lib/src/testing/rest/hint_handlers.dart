// SPDX-License-Identifier: Apache-2.0

/// Handlers of the lists "Permission hints" and "Expiring soon" (brief M6-07; 07 6, 9.4, 9.5,
/// SR2; 05 `access.hints.read`, R-AZ-34): `listPermissionHints` and `listExpiringAccess`.
///
/// A small model of the server (`internal/authz/app/hints.go` and `expiring.go`) over the
/// world's access lists, grants, groups, operators and agents. Both lists are read only, never
/// ask for a step-up and page with an offset as cursor. Simplifications against the server:
///
/// - Read access is the access list of a channel (user grants write it), a valid group grant
///   that covers the channel, or the owner role; the fake has no expiry sweep of the lists.
/// - `manage` on a space is `manage` ([FakeChannel.capabilities]) in every channel of it; the
///   fake keeps no capability on the space itself.
/// - The fake has no project bindings and no dev environments, so SR3 never blocks and no
///   person passes SR2 by a ready environment: `trigger_blocked` only ever names `sr2`.
/// - The fake has no apps, so "Expiring soon" lists agent tokens only, and no archived
///   channels.
library;

import 'dart:math';

import '../action_catalog.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// The page size of both lists when the request names no `limit` (`api.page_size_default`,
/// 06 6 point 3).
const int fakeHintPageSizeDefault = 50;

/// The largest page of both lists; a larger `limit` is capped, not rejected
/// (`api.page_size_max`, 06 6 point 3).
const int fakeHintPageSizeMax = 200;

/// The kinds of "Permission hints" in the order of the list (07 9.5, `PermissionHintKind`).
const List<String> _hintKinds = [
  'h1_read_horizon',
  'grant_to_disabled',
  'trigger_blocked',
  'agent_personal',
];

/// The kinds of "Expiring soon" in the order of the tie break after the expiry.
const List<String> _expiringKinds = ['grant', 'membership', 'token'];

/// Registers `listPermissionHints` and `listExpiringAccess` on [rest]; [catalog] gives the
/// step-up of `access.hints.read`, which asks nothing today.
void registerHintHandlers(RestFake rest, ActionCatalog catalog) {
  final world = rest.world;

  bool valid(Object? expiresAt) =>
      expiresAt == null ||
      DateTime.parse(expiresAt as String).isAfter(world.now());

  bool inEffect(JsonMap g) => g['revoked_at'] == null && valid(g['expires_at']);

  int byId(FakeChannel a, FakeChannel b) => a.id.compareTo(b.id);

  /// The agent that operates [c] (`operator.effective_agent_id`), or null.
  String? operatorOf(FakeChannel c) =>
      (c.json['operator'] as JsonMap?)?['effective_agent_id'] as String?;

  /// Whether [c] is a channel of a space (a direct message has none and appears in no list)
  /// within [space], or in any space for the instance.
  bool inScope(FakeChannel c, String? space) =>
      c.spaceId != null && (space == null || c.spaceId == space);

  /// Whether the target of [g] covers the channel [c] (07 2.2).
  bool covers(JsonMap g, FakeChannel c) => switch (g['target_kind']) {
    'instance' => true,
    'space' => g['target_id'] == c.spaceId,
    'channel' => g['target_id'] == c.id,
    _ => false,
  };

  /// Whether the target of [g] lies in [space]: the space itself or one of its channels.
  bool targetIn(JsonMap g, String space) => switch (g['target_kind']) {
    'space' => g['target_id'] == space,
    'channel' => world.channels[g['target_id']]?.spaceId == space,
    _ => false,
  };

  /// Whether [g] belongs to the list of [space]: every target for the instance (a direct
  /// message channel excepted), else [targetIn].
  bool grantInScope(JsonMap g, String? space) {
    if (g['target_kind'] == 'channel') {
      final c = world.channels[g['target_id']];
      return c != null && inScope(c, space);
    }
    return space == null || targetIn(g, space);
  }

  /// Whether [person] is a member of [group] whose membership has not expired.
  bool memberOf(Object? group, String person) {
    final m = world.groupMembers[group]?[person];
    return m != null && valid(m['expires_at']);
  }

  /// Whether [person] may read [c]: an owner, an entry of its access list, or a valid grant
  /// to a group with a valid membership of [person] that covers [c].
  bool reads(String person, FakeChannel c) =>
      (world.people[person]?.isOwner ?? false) ||
      c.access.containsKey(person) ||
      world.grants.values.any(
        (g) =>
            g['subject_kind'] == 'group' &&
            inEffect(g) &&
            covers(g, c) &&
            memberOf(g['subject_id'], person),
      );

  /// Whether [agentId] serves [space]: it operates one of its channels or is its default
  /// operator.
  bool serves(String agentId, String space) =>
      world.spaces[space]?['default_operator_agent_id'] == agentId ||
      world.channels.values.any(
        (c) => c.spaceId == space && operatorOf(c) == agentId,
      );

  /// Whether [personId] holds `manage` on [space]: in every channel of it, as a grant with
  /// `manage` on the space gives it (07 2.2, 2.6).
  bool manages(String personId, String space) {
    final own = [
      for (final c in world.channels.values)
        if (c.spaceId == space) c,
    ];
    return own.isNotEmpty &&
        own.every((c) => c.capabilities[personId]?.contains('manage') ?? false);
  }

  /// `access.hints.read` on the instance (no [FakeRequest.query] `space_id`: owners only,
  /// `403 owner_only`) or on a space (unknown or not readable: `404`; readable without
  /// `manage`: `403`); returns the space, or null for the instance.
  String? scope(FakeRequest r) {
    final p = r.person;
    final space = r.query('space_id');
    if (space == null) {
      if (!p.isOwner) {
        throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
      }
    } else {
      if (!world.spaces.containsKey(space) ||
          (!p.isOwner && !world.seesSpace(p.id, space))) {
        throw const FakeProblem(404, 'not_found');
      }
      if (!p.isOwner && !manages(p.id, space)) {
        throw const FakeProblem(403, 'forbidden', reason: 'capability_missing');
      }
    }
    requireCatalogStepUp(r, rest.contract, catalog);
    return space;
  }

  /// H1 (07 6): for every active agent A, its read horizon H(A) (the channels whose access list
  /// holds A, and the channels it operates) within [space], and every person P of its trigger
  /// set T(A) ([FakeWorld.agentTriggerers]) who may not read a channel K of H(A): one hint
  /// (A, P, K). Owners read everything; SR2 lets no other person trigger a `host` agent, so
  /// such an agent gives none. In the order of agent, person and channel.
  List<JsonMap> readHorizon(String? space) {
    final agents = [
      for (final p in world.people.values)
        if (p.kind == 'agent' && p.status == 'active') p.id,
    ]..sort();
    return [
      for (final a in agents)
        if (world.agents[a]?.toolClass != 'host')
          ...() {
            final horizon = [
              for (final c in world.channels.values)
                if (inScope(c, space) &&
                    (c.access.containsKey(a) || operatorOf(c) == a))
                  c,
            ]..sort(byId);
            final triggerers = [
              for (final p in world.agentTriggerers(a))
                if (!world.people[p]!.isOwner) p,
            ]..sort();
            return [
              for (final p in triggerers)
                for (final k in horizon)
                  if (!reads(p, k))
                    {
                      'kind': 'h1_read_horizon',
                      'agent_id': a,
                      'person_id': p,
                      'space_id': k.spaceId,
                      'channel_id': k.id,
                    },
            ];
          }(),
    ];
  }

  /// The grants in effect within [space], by ID.
  List<JsonMap> grantsIn(String? space) => [
    for (final g in world.grants.values)
      if (inEffect(g) && grantInScope(g, space)) g,
  ]..sort((a, b) => (a['id']! as String).compareTo(b['id']! as String));

  /// `grant_to_disabled`: grants whose subject person or agent is `disabled` or `deleted` (a
  /// `pending` person's grants take effect, 07 SR7); a group never.
  List<JsonMap> toDisabled(String? space) => [
    for (final g in grantsIn(space))
      if (g['subject_kind'] != 'group' &&
          const {
            'disabled',
            'deleted',
          }.contains(world.people[g['subject_id']]?.status))
        {'kind': 'grant_to_disabled', 'grant': g},
  ];

  /// `trigger_blocked`: `trigger` grants to an active person who is not an owner, or to a group,
  /// that reach at least one channel with an operator, where SR2 blocks on every such channel:
  /// its operator has the tool class `host`. A disabled subject shows as `grant_to_disabled`.
  List<JsonMap> blocked(String? space) => [
    for (final g in grantsIn(space))
      if (g['level'] == 'trigger' &&
          switch (g['subject_kind']) {
            'user' => switch (world.people[g['subject_id']]) {
              final p? =>
                p.kind == 'user' && p.status == 'active' && !p.isOwner,
              null => false,
            },
            'group' => true,
            _ => false,
          })
        ...() {
          final operators = [
            for (final c in world.channels.values)
              if (c.spaceId != null && covers(g, c)) ?operatorOf(c),
          ];
          return [
            if (operators.isNotEmpty &&
                operators.every((a) => world.agents[a]?.toolClass == 'host'))
              {
                'kind': 'trigger_blocked',
                'grant': g,
                'rules': ['sr2'],
              },
          ];
        }(),
  ];

  /// `agent_personal`: the active agents in mode `personal` with the holder (when set) and the
  /// trigger set I1 asks to be exactly the holder, ascending; for a space only agents that
  /// serve it.
  List<JsonMap> personal(String? space) {
    final agents = world.agents.values.toList()
      ..sort((a, b) => a.id.compareTo(b.id));
    return [
      for (final a in agents)
        if (a.billingMode == 'personal' &&
            world.people[a.id]?.status == 'active' &&
            (space == null || serves(a.id, space)))
          {
            'kind': 'agent_personal',
            'agent_id': a.id,
            'credential_holder_id': ?a.credentialHolderId,
            'trigger_set': world.agentTriggerers(a.id)..sort(),
          },
    ];
  }

  /// The `AccessLabels` of [principals], [groups], [spaces] and [channels] (and the space of
  /// every channel), each ascending by ID; IDs the world does not know, and the host target of
  /// a grant, have no label.
  JsonMap labels({
    required Set<String> principals,
    required Set<String> groups,
    required Set<String> spaces,
    required Set<String> channels,
  }) {
    List<String> sorted(Set<String> ids) => ids.toList()..sort();
    final named = [
      for (final id in sorted(channels))
        if (world.channels[id] case final c? when c.spaceId != null) c,
    ];
    spaces.addAll([for (final c in named) c.spaceId!]);
    return {
      'principals': [
        for (final id in sorted(principals))
          if (world.people[id] case final p?) p.ref,
      ],
      'groups': [
        for (final id in sorted(groups))
          if (world.groups[id] case final g?) {'id': id, 'name': g['name']},
      ],
      'spaces': [
        for (final id in sorted(spaces))
          if (world.spaces[id] case final s?) {'id': id, 'name': s['name']},
      ],
      'channels': [
        for (final c in named)
          {'id': c.id, 'space_id': c.spaceId, 'name': c.json['name']},
      ],
    };
  }

  /// The labels of the hints and items of a page: every ID they name, the subject and target of
  /// every grant, the group and person of a membership and the holder of a token.
  JsonMap labelsOf(List<JsonMap> items) {
    final principals = <String>{};
    final groups = <String>{};
    final spaces = <String>{};
    final channels = <String>{};
    for (final x in items) {
      principals.addAll([
        for (final key in const [
          'agent_id',
          'person_id',
          'credential_holder_id',
          'user_id',
          'principal_id',
        ])
          ?(x[key] as String?),
        ...?(x['trigger_set'] as List<String>?),
      ]);
      if (x['space_id'] case final String s) spaces.add(s);
      if (x['channel_id'] case final String c) channels.add(c);
      if (x['group_id'] case final String g) groups.add(g);
      if (x['grant'] case final JsonMap g) {
        final subject = g['subject_id']! as String;
        (g['subject_kind'] == 'group' ? groups : principals).add(subject);
        final target = g['target_id'] as String?;
        if (g['target_kind'] == 'space') spaces.add(target!);
        if (g['target_kind'] == 'channel') channels.add(target!);
      }
    }
    return labels(
      principals: principals,
      groups: groups,
      spaces: spaces,
      channels: channels,
    );
  }

  /// One page of [items] after the cursor of [r] (an offset), with `limit` from 1 up to
  /// [fakeHintPageSizeMax] (default [fakeHintPageSizeDefault]), the labels of the page and
  /// `next_cursor` while more follow.
  FakeResponse page(FakeRequest r, List<JsonMap> items) {
    final start = (int.tryParse(r.query('cursor') ?? '') ?? 0).clamp(
      0,
      items.length,
    );
    final asked = int.tryParse(r.query('limit') ?? '');
    final limit = asked == null || asked < 1
        ? fakeHintPageSizeDefault
        : min(asked, fakeHintPageSizeMax);
    final end = min(start + limit, items.length);
    final slice = items.sublist(start, end);
    return FakeResponse(200, {
      'items': slice,
      'labels': labelsOf(slice),
      if (end < items.length) 'next_cursor': '$end',
    });
  }

  /// Every entry of "Expiring soon" within [space] whose expiry lies in (now, now + lead]
  /// (`authz.expiry_reminder_lead`), ordered by expiry, kind and ID: grants not revoked,
  /// memberships of groups that exist (for a space only of groups that hold a grant in effect
  /// in it), and for the instance the agent tokens not revoked, never their value or prefix.
  List<JsonMap> expiring(String? space) {
    final now = world.now();
    final until = now.add(world.expiryReminderLead);
    final entries = <(DateTime, int, String, JsonMap)>[];
    void add(Object? expiresAt, String kind, String id, JsonMap item) {
      if (expiresAt is! String) return;
      final at = DateTime.parse(expiresAt);
      if (!at.isAfter(now) || at.isAfter(until)) return;
      entries.add((
        at,
        _expiringKinds.indexOf(kind),
        id,
        {'kind': kind, 'expires_at': world.stamp(at), ...item},
      ));
    }

    for (final g in world.grants.values) {
      if (g['revoked_at'] == null && (space == null || targetIn(g, space))) {
        add(g['expires_at'], 'grant', g['id']! as String, {'grant': g});
      }
    }
    for (final MapEntry(key: group, value: members)
        in world.groupMembers.entries) {
      if (!world.groups.containsKey(group) ||
          (space != null &&
              !world.grants.values.any(
                (g) =>
                    g['subject_kind'] == 'group' &&
                    g['subject_id'] == group &&
                    inEffect(g) &&
                    targetIn(g, space),
              ))) {
        continue;
      }
      for (final MapEntry(key: user, value: m) in members.entries) {
        final id = world.membershipId(group, user);
        add(m['expires_at'], 'membership', id, {
          'membership_id': id,
          'group_id': group,
          'user_id': user,
        });
      }
    }
    if (space == null) {
      for (final a in world.agents.values) {
        for (final t in a.tokens.values) {
          if (t['revoked_at'] != null) continue;
          final id = t['id']! as String;
          add(t['expires_at'], 'token', id, {
            'token_id': id,
            'principal_id': a.id,
          });
        }
      }
    }
    entries.sort(
      (x, y) => switch (x.$1.compareTo(y.$1)) {
        0 => switch (x.$2.compareTo(y.$2)) {
          0 => x.$3.compareTo(y.$3),
          final c => c,
        },
        final c => c,
      },
    );
    return [for (final e in entries) e.$4];
  }

  rest
    ..handle('listPermissionHints', (r) {
      final space = scope(r);
      final kind = r.query('kind');
      final items = <JsonMap>[
        for (final k in _hintKinds)
          if (kind == null || kind == k)
            ...switch (k) {
              'h1_read_horizon' => readHorizon(space),
              'grant_to_disabled' => toDisabled(space),
              'trigger_blocked' => blocked(space),
              _ => personal(space),
            },
      ];
      return page(r, items);
    })
    ..handle('listExpiringAccess', (r) => page(r, expiring(scope(r))));
}

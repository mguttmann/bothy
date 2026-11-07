// SPDX-License-Identifier: Apache-2.0

/// Handlers of the built-in agents of stage A1 that the creation dialog calls (26 8.2; brief
/// A1-08 Scope): `createAgent` with `hosted_by`, `setAgentToolClass`, `getAgentDefinition`,
/// `putAgentDefinition`, `listAgentTemplates`, `interviewAgent`, and the channel steps
/// `createChannel` and `setChannelOperator`.
///
/// A small model of the server's rules, enough for the step order of 26 8.2 and 8.3: a
/// definition with a `stdio` MCP server for an agent below `host` is refused with
/// `400 validation_failed` and `tool_class_too_low` at the server's pointer (26 4.3 item 7);
/// `agent.tool_class.set` below `host` while the current definition has a `stdio` server is
/// `409 conflict` with `definition_needs_host` (R-BA-06), and above the hosting runner's
/// `hosts_agents` `409 conflict` with `host_tool_class_ceiling` (26 3.1 item 3). Every write
/// is its own action with its own audit row and nothing is rolled back (26 8.3), so a test sees
/// exactly the steps that ran. The other rules of 26 4.3 are the server's; the fake does not
/// repeat them.
library;

import 'dart:convert';

import '../action_catalog.dart';
import '../audit_log.dart';
import '../fake_agent.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// The rank of a tool class (`chat` < `scoped` < `host`, 07 2.11).
int _rank(String toolClass) => switch (toolClass) {
  'host' => 2,
  'scoped' => 1,
  _ => 0,
};

/// Whether [definition] has an MCP server with `transport = stdio` (26 4.4 item 4).
bool _hasStdio(JsonMap? definition) {
  final servers = definition?['mcp_servers'];
  if (servers is! List) return false;
  return servers.any((s) => s is Map && s['transport'] == 'stdio');
}

/// Registers the handlers on [rest], with the step-up of [catalog]; the audit rows go to
/// [audit].
void registerBuiltinAgentHandlers(
  RestFake rest,
  ActionCatalog catalog,
  FakeAuditLog audit,
) {
  final world = rest.world;

  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
  }

  void stepUp(FakeRequest r) => requireCatalogStepUp(r, rest.contract, catalog);

  FakeAgent agentOf(String id) =>
      world.agentOf(id) ?? (throw const FakeProblem(404, 'not_found'));

  int? ifMatchOf(FakeRequest r) {
    final match = RegExp(r'^"?(\d+)"?$')
        .firstMatch(r.headers['if-match'] ?? '');
    return match == null ? null : int.parse(match[1]!);
  }

  FakeResponse card(FakeAgent a, {int status = 200}) => FakeResponse(
    status,
    world.agentJson(a, owner: true),
    {'ETag': '"${a.version}"'},
  );

  JsonMap version(FakeAgent a, int n, {bool withDefinition = true}) => {
    'agent_id': a.id,
    'version': n,
    'origin_kind':
        (a.definitions[n - 1]['origin'] as Map?)?['kind'] ?? 'manual',
    'created_at': world.stamp(),
    'created_by': world.people.values.firstWhere((p) => p.isOwner).id,
    if (withDefinition) 'definition': a.definitions[n - 1],
  };

  void row(
    FakeRequest r,
    String action,
    String resourceKind,
    String? resourceId, [
    JsonMap details = const {},
  ]) => audit.add(
    world,
    action: action,
    actor: r.person.id,
    resourceKind: resourceKind,
    resourceId: resourceId,
    details: details,
  );

  rest
    ..handle('listAgentTemplates', (r) {
      owner(r);
      final templates = world.agentTemplates;
      if (templates == null) throw const FakeProblem(404, 'not_found');
      return FakeResponse(200, {'items': templates});
    })
    ..handle('interviewAgent', (r) {
      owner(r);
      world.interviewRequests.add(jsonDecode(jsonEncode(r.json)) as JsonMap);
      // 26 6.1 item 5: one audit row per request with counts only, never a text.
      final answer = world.interviewAnswers.isEmpty
          ? <String, Object?>{
              'outcome': 'fallback',
              'fallback_reason': 'builder_not_configured',
            }
          : world.interviewAnswers.removeAt(0);
      row(r, 'agent.interview', 'instance', null, {
        'outcome': answer['outcome'],
        'fallback_reason': ?answer['fallback_reason'],
        'turns': (r.json['turns'] as List?)?.length ?? 0,
      });
      return FakeResponse(200, answer);
    })
    ..handle('createAgent', (r) {
      owner(r);
      stepUp(r);
      final body = r.json;
      final handle = body['handle']! as String;
      final kind = body['adapter_kind']! as String;
      final hostedBy = body['hosted_by'] as String?;
      if ((kind == 'claude_agent') != (hostedBy != null)) {
        throw FakeProblem(
          400,
          'validation_failed',
          errors: [
            {
              'pointer': '/hosted_by',
              'code': hostedBy == null ? 'required' : 'not_allowed',
            },
          ],
        );
      }
      if (hostedBy != null) {
        final host = world.agentOf(hostedBy);
        if (host == null ||
            host.adapterKind != 'code_runner' ||
            host.hostedBy != null ||
            host.hostsAgents == null ||
            world.people[host.id]?.status != 'active') {
          throw const FakeProblem(
            400,
            'validation_failed',
            errors: [
              {'pointer': '/hosted_by', 'code': 'host_not_capable'},
            ],
          );
        }
      }
      if (world.people.values.any((p) => p.handle == handle)) {
        throw const FakeProblem(409, 'conflict', reason: 'handle_taken');
      }
      final a = world.addAgent(
        handle: handle,
        displayName: body['display_name']! as String,
        adapterKind: kind,
        hostedBy: hostedBy,
        billingModesSupported: null,
      )..description = body['description'] as String? ?? '';
      row(r, 'agent.create', 'agent', a.id, {
        'adapter_kind': kind,
        'hosted_by': ?hostedBy,
      });
      return card(a, status: 201);
    })
    ..handle('setAgentToolClass', (r) {
      owner(r);
      final a = agentOf(r.param('agent_id'));
      stepUp(r);
      final match = ifMatchOf(r);
      if (match == null) throw const FakeProblem(428, 'precondition_required');
      if (match != a.version) {
        throw FakeProblem(
          412,
          'version_conflict',
          current: world.agentJson(a, owner: true),
        );
      }
      final target = r.json['tool_class']! as String;
      final host = a.hostedBy == null ? null : world.agentOf(a.hostedBy!);
      if (host != null && _rank(target) > _rank(host.hostsAgents ?? 'chat')) {
        throw const FakeProblem(
          409,
          'conflict',
          reason: 'host_tool_class_ceiling',
        );
      }
      final current = a.definitions.isEmpty ? null : a.definitions.last;
      if (_rank(target) < _rank('host') && _hasStdio(current)) {
        throw const FakeProblem(
          409,
          'conflict',
          reason: 'definition_needs_host',
        );
      }
      final from = a.toolClass;
      if (from != target) {
        a
          ..toolClass = target
          ..version += 1;
        world.permissionVersion++;
      }
      row(r, 'agent.tool_class.set', 'agent', a.id, {
        'from_tool_class': from,
        'to_tool_class': target,
      });
      return card(a);
    })
    ..handle('getAgentDefinition', (r) {
      owner(r);
      final a = agentOf(r.param('agent_id'));
      if (a.adapterKind != 'claude_agent' || a.definitions.isEmpty) {
        throw const FakeProblem(404, 'not_found');
      }
      final n = a.definitions.length;
      return FakeResponse(200, version(a, n), {'ETag': '"$n"'});
    })
    ..handle('putAgentDefinition', (r) {
      owner(r);
      final a = agentOf(r.param('agent_id'));
      stepUp(r);
      if (a.adapterKind != 'claude_agent') {
        throw const FakeProblem(409, 'conflict', reason: 'not_a_builtin_agent');
      }
      final n = a.definitions.length;
      final match = ifMatchOf(r);
      if (n > 0 && match == null) {
        throw const FakeProblem(428, 'precondition_required');
      }
      if (match != null && match != n) {
        throw FakeProblem(
          412,
          'version_conflict',
          current: n == 0 ? null : version(a, n),
        );
      }
      final definition = jsonDecode(jsonEncode(r.json)) as JsonMap;
      final servers = (definition['mcp_servers'] as List?) ?? const [];
      if (a.toolClass != 'host') {
        final errors = [
          for (final (i, s) in servers.indexed)
            if (s is Map && s['transport'] == 'stdio')
              {
                'pointer': '/mcp_servers/$i/transport',
                'code': 'tool_class_too_low',
              },
        ];
        if (errors.isNotEmpty) {
          throw FakeProblem(400, 'validation_failed', errors: errors);
        }
      }
      // A body equal to the current definition stores no new version (26 4.1).
      if (n == 0 || jsonEncode(a.definitions.last) != jsonEncode(definition)) {
        a
          ..definitions.add(definition)
          ..version += 1;
        row(r, 'agent.update', 'agent', a.id, {
          'definition_version': a.definitions.length,
        });
      }
      final stored = a.definitions.length;
      return FakeResponse(200, version(a, stored), {'ETag': '"$stored"'});
    })
    ..handle('createChannel', (r) {
      final spaceId = r.param('space_id');
      if (!world.spaces.containsKey(spaceId) ||
          !(r.person.isOwner || world.seesSpace(r.person.id, spaceId))) {
        throw const FakeProblem(404, 'not_found');
      }
      stepUp(r);
      final name = r.json['name']! as String;
      if (world.channels.values.any(
        (c) => c.spaceId == spaceId && c.json['name'] == name,
      )) {
        throw const FakeProblem(409, 'conflict', reason: 'name_taken');
      }
      final id = world.createChannel(
        r.person.id,
        spaceId,
        name,
        {r.person.id: 'trigger'},
        kind: r.json['kind'] as String? ?? 'text',
        capabilities: {
          r.person.id: {'manage'},
        },
      );
      row(r, 'channel.create', 'channel', id, {'space_id': spaceId});
      return FakeResponse(201, world.channels[id]!.json);
    })
    ..handle('setChannelOperator', (r) {
      final id = r.param('channel_id');
      final channel = world.channels[id];
      if (channel == null ||
          !(r.person.isOwner || world.seesChannel(r.person.id, id))) {
        throw const FakeProblem(404, 'not_found');
      }
      stepUp(r);
      final match = ifMatchOf(r);
      if (match == null) throw const FakeProblem(428, 'precondition_required');
      if (match != channel.json['version']) {
        throw FakeProblem(412, 'version_conflict', current: channel.json);
      }
      final mode = r.json['mode']! as String;
      final agent = r.json['agent_id'] as String?;
      if (mode == 'agent') {
        if (agent == null || world.agentOf(agent) == null) {
          throw const FakeProblem(
            400,
            'validation_failed',
            errors: [
              {'pointer': '/agent_id', 'code': 'invalid'},
            ],
          );
        }
        world.setChannelOperator(id, agent);
      } else {
        world.setChannelOperator(id, null);
      }
      channel.json['version'] = (channel.json['version']! as int) + 1;
      row(r, 'channel.operator.set', 'channel', id, {
        'mode': mode,
        'agent_id': ?agent,
      });
      return FakeResponse(200, channel.json, {
        'ETag': '"${channel.json['version']}"',
      });
    });
}

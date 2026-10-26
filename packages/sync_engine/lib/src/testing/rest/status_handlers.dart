// SPDX-License-Identifier: Apache-2.0

/// Handlers of the seven operations of the status page (docs/spec/16-operations.md 12; brief
/// M7-21 Tests: the views against the fake server): `getStatus`, `getPublicStatus`,
/// `createStatusComponent`, `updateStatusComponent`, `deleteStatusComponent`,
/// `createStatusIncident` and `addStatusIncidentUpdate`.
///
/// The fake runs no checks: tests set the components and incidents in
/// [FakeWorld.statusComponents] and [FakeWorld.statusIncidents]. It keeps the server's rules a
/// view depends on: the component of an agent only for a person with `agent.read`
/// ([FakeWorld.agentReaders]), never on the public page and never in `overall_state`;
/// `getPublicStatus` is `404` unless [FakeWorld.statusPublicEnabled]; the owner operations
/// are `403` for anyone else, and each change appends one `status.updated`. Simplifications:
/// a name is checked for emptiness and uniqueness only, and a move is not supported.
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the seven operations of the status page on [rest].
void registerStatusHandlers(RestFake rest) {
  final world = rest.world;

  void owner(FakeRequest r) {
    if (!r.person.isOwner) throw const FakeProblem(403, 'forbidden');
  }

  Never invalid(String pointer, String code) => throw FakeProblem(
    400,
    'validation_failed',
    errors: [
      {'pointer': pointer, 'code': code},
    ],
  );

  JsonMap component(String id) =>
      world.statusComponents.where((c) => c['id'] == id).firstOrNull ??
      (throw const FakeProblem(404, 'not_found'));

  JsonMap incident(String id) =>
      world.statusIncidents.where((i) => i['id'] == id).firstOrNull ??
      (throw const FakeProblem(404, 'not_found'));

  void checkName(String name, String? except) {
    if (name.trim().isEmpty) invalid('/name', 'empty');
    final taken = world.statusComponents.any(
      (c) =>
          c['id'] != except &&
          (c['name']! as String).toLowerCase() == name.toLowerCase(),
    );
    if (taken) invalid('/name', 'taken');
  }

  void apply(JsonMap c, JsonMap input) {
    c['name'] = input['name'];
    for (final key in const ['description', 'state', 'health_source']) {
      if (input.containsKey(key)) c[key] = input[key];
    }
    if (input.containsKey('agent_id')) c['agent_id'] = input['agent_id'];
    c['updated_at'] = world.stamp();
  }

  rest
    ..handle(
      'getStatus',
      (r) => FakeResponse(200, world.statusPage(r.person.id)),
    )
    ..handle('getPublicStatus', (r) {
      if (!world.statusPublicEnabled) throw const FakeProblem(404, 'not_found');
      return FakeResponse(200, world.statusPage(null));
    })
    ..handle('createStatusComponent', (r) {
      owner(r);
      final input = r.json;
      checkName(input['name']! as String, null);
      final c = <String, Object?>{
        'id': world.nextId(),
        'description': '',
        'sort_key': world.statusComponents.length.toString().padLeft(6, '0'),
        'state': 'operational',
      };
      apply(c, input);
      final after = input['after_component_id'] as String?;
      if (after == null) {
        world.statusComponents.add(c);
      } else {
        final at = world.statusComponents.indexOf(component(after));
        world.statusComponents.insert(at + 1, c);
      }
      world.statusUpdated(componentIds: [c['id']! as String]);
      return FakeResponse(201, c);
    })
    ..handle('updateStatusComponent', (r) {
      owner(r);
      final c = component(r.param('component_id'));
      checkName(r.json['name']! as String, c['id'] as String?);
      apply(c, r.json);
      world.statusUpdated(componentIds: [c['id']! as String]);
      return FakeResponse(200, c);
    })
    ..handle('deleteStatusComponent', (r) {
      owner(r);
      final c = component(r.param('component_id'));
      world.statusComponents.remove(c);
      for (final i in world.statusIncidents) {
        (i['component_ids']! as List).remove(c['id']);
      }
      world.statusUpdated(componentIds: [c['id']! as String]);
      return const FakeResponse(204);
    })
    ..handle('createStatusIncident', (r) {
      owner(r);
      final input = r.json;
      final title = input['title']! as String;
      if (title.trim().isEmpty) invalid('/title', 'empty');
      final body = input['body_md']! as String;
      if (body.trim().isEmpty) invalid('/body_md', 'empty');
      final ids = (input['component_ids']! as List).cast<String>();
      for (final id in ids) {
        component(id);
      }
      final kind = input['kind']! as String;
      final state =
          input['state'] as String? ??
          (kind == 'maintenance' ? 'scheduled' : 'investigating');
      final i = <String, Object?>{
        'id': world.nextId(),
        'title': input['title'],
        'kind': kind,
        'impact': input['impact'],
        'state': state,
        'started_at': input['started_at'],
        if (state == 'resolved') 'resolved_at': world.stamp(),
        'component_ids': ids,
        'updates': [
          {
            'id': world.nextId(),
            'state': state,
            'body_md': body,
            'created_at': world.stamp(),
          },
        ],
      };
      world.statusIncidents.add(i);
      world.statusUpdated(incidentIds: [i['id']! as String]);
      return FakeResponse(201, i);
    })
    ..handle('addStatusIncidentUpdate', (r) {
      owner(r);
      final i = incident(r.param('incident_id'));
      final body = r.json['body_md']! as String;
      if (body.trim().isEmpty) invalid('/body_md', 'empty');
      final state = r.json['state']! as String;
      i['state'] = state;
      if (state == 'resolved') {
        i['resolved_at'] ??= world.stamp();
      } else {
        i.remove('resolved_at');
      }
      (i['updates']! as List).add({
        'id': world.nextId(),
        'state': state,
        'body_md': body,
        'created_at': world.stamp(),
      });
      world.statusUpdated(incidentIds: [i['id']! as String]);
      return FakeResponse(201, i);
    });
}

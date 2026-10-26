// SPDX-License-Identifier: Apache-2.0

// The status page calls of the fake repository (brief M7-21 Tests): the page of 16 12 with the
// four standard components (api, push, files, search), the component of the agent Lotse, an open
// incident, a maintenance window and a resolved incident; every owner call is recorded with its
// request body, so a test can check that a form sends only the fields of its operation.

import 'package:api_client/api_client.dart';

/// The IDs of the fake page.
const String fakeApiComponent = '0192f4a0-7b1c-7000-8a00-000000000301';

/// The component `push`.
const String fakePushComponent = '0192f4a0-7b1c-7000-8a00-000000000302';

/// The component `files`.
const String fakeFilesComponent = '0192f4a0-7b1c-7000-8a00-000000000303';

/// The component `search`.
const String fakeSearchComponent = '0192f4a0-7b1c-7000-8a00-000000000304';

/// The component of the agent Lotse.
const String fakeLotseComponent = '0192f4a0-7b1c-7000-8a00-000000000305';

/// The open incident.
const String fakeOpenIncident = '0192f4a0-7b1c-7000-8a00-000000000311';

/// The maintenance window.
const String fakeMaintenance = '0192f4a0-7b1c-7000-8a00-000000000312';

/// The resolved incident.
const String fakeResolvedIncident = '0192f4a0-7b1c-7000-8a00-000000000313';

/// The page as JSON with [overall], the components and the incidents.
Map<String, Object?> fakeStatusJson({
  String overall = 'degraded',
  bool withAgent = true,
}) => {
  'overall_state': overall,
  'components': [
    _component(fakeApiComponent, 'Messages', 'api', 'operational'),
    _component(fakePushComponent, 'Push', 'push', 'degraded'),
    _component(fakeFilesComponent, 'Files', 'files', 'maintenance'),
    _component(fakeSearchComponent, 'Search', 'search', 'partial_outage'),
    if (withAgent)
      {
        ..._component(
          fakeLotseComponent,
          'Lotse',
          'agent.presence',
          'major_outage',
        ),
        'agent_id': '0192f4a0-7b1c-7000-8a00-000000000399',
        'description': 'lotse',
      },
  ],
  'incidents': [
    {
      'id': fakeOpenIncident,
      'title': 'Push notifications delayed',
      'kind': 'incident',
      'impact': 'minor',
      'state': 'monitoring',
      'started_at': '2026-10-17T07:30:00.000Z',
      'component_ids': [fakePushComponent],
      'updates': [
        _update(
          '0192f4a0-7b1c-7000-8a00-000000000321',
          'investigating',
          'The push provider reports **delays**.',
          '2026-10-17T07:30:00.000Z',
        ),
        _update(
          '0192f4a0-7b1c-7000-8a00-000000000322',
          'monitoring',
          'A fix is out. We are watching:\n\n- iPhone\n- Android',
          '2026-10-17T08:15:00.000Z',
        ),
      ],
    },
    {
      'id': fakeMaintenance,
      'title': 'Storage upgrade',
      'kind': 'maintenance',
      'impact': 'none',
      'state': 'scheduled',
      'started_at': '2026-10-17T08:00:00.000Z',
      'component_ids': [fakeFilesComponent],
      'updates': [
        _update(
          '0192f4a0-7b1c-7000-8a00-000000000323',
          'scheduled',
          'Uploads pause for about one hour.',
          '2026-10-17T08:00:00.000Z',
        ),
      ],
    },
    {
      'id': fakeResolvedIncident,
      'title': 'Search slow',
      'kind': 'incident',
      'impact': 'major',
      'state': 'resolved',
      'started_at': '2026-10-16T12:00:00.000Z',
      'resolved_at': '2026-10-16T13:00:00.000Z',
      'component_ids': [fakeSearchComponent],
      'updates': [
        _update(
          '0192f4a0-7b1c-7000-8a00-000000000324',
          'resolved',
          'The index is rebuilt.',
          '2026-10-16T13:00:00.000Z',
        ),
      ],
    },
  ],
};

Map<String, Object?> _component(
  String id,
  String name,
  String source,
  String state,
) => {
  'id': id,
  'name': name,
  'description': '',
  'sort_key': id.substring(id.length - 3),
  'state': state,
  'health_source': source,
  'updated_at': '2026-10-17T08:00:00.000Z',
};

Map<String, Object?> _update(String id, String state, String body, String at) =>
    {'id': id, 'state': state, 'body_md': body, 'created_at': at};

/// The page of [json].
StatusPage fakeStatusPage([Map<String, Object?>? json]) => standardSerializers
    .deserializeWith(StatusPage.serializer, json ?? fakeStatusJson())!;

/// The status calls of the fake.
mixin FakeStatus {
  /// The page `getStatus` answers.
  StatusPage statusPage = fakeStatusPage();

  /// The error `getStatus` throws instead, or null.
  Object? statusError;

  /// Every owner call: the operation and its request body (or the ID of a delete).
  final List<(String, Object)> statusCalls = [];

  /// Reads of the page.
  int statusReads = 0;

  /// `getStatus`.
  Future<StatusPage> status() async {
    statusReads++;
    if (statusError case final e?) throw e;
    return statusPage;
  }

  /// `getPublicStatus`.
  Future<StatusPage> publicStatus() async => statusPage;

  /// `createStatusComponent`.
  Future<StatusComponent> createStatusComponent(
    StatusComponentInput input,
  ) async {
    statusCalls.add(('createStatusComponent', input));
    return statusPage.components.first;
  }

  /// `updateStatusComponent`.
  Future<StatusComponent> updateStatusComponent(
    String componentId,
    StatusComponentInput input,
  ) async {
    statusCalls.add(('updateStatusComponent $componentId', input));
    return statusPage.components.first;
  }

  /// `deleteStatusComponent`.
  Future<void> deleteStatusComponent(String componentId) async {
    statusCalls.add(('deleteStatusComponent', componentId));
  }

  /// `createStatusIncident`.
  Future<StatusIncident> createStatusIncident(
    StatusIncidentCreate input,
  ) async {
    statusCalls.add(('createStatusIncident', input));
    return statusPage.incidents.first;
  }

  /// `addStatusIncidentUpdate`.
  Future<StatusIncident> addStatusIncidentUpdate(
    String incidentId,
    StatusIncidentUpdateCreate input,
  ) async {
    statusCalls.add(('addStatusIncidentUpdate $incidentId', input));
    return statusPage.incidents.first;
  }
}

// SPDX-License-Identifier: Apache-2.0

/// The seven operations of the status page (docs/spec/16-operations.md 12; docs/api/openapi.yaml
/// tag `Status`; brief M7-21 Scope): reading the page of the signed-in person and the public
/// page, and the owner's changes of components and incidents (`status.manage`).
///
/// The forms hand over the generated request bodies (`StatusComponentInput`,
/// `StatusIncidentCreate`, `StatusIncidentUpdateCreate`), so a request carries only the fields
/// of its operation (brief M7-21 requirement 15). No visibility logic here: the server decides
/// which components and incidents a person sees (15 1 principle 1).
library;

import 'package:api_client/api_client.dart' as api;

import '../data/access_repository.dart' show adminCall;
import '../data/self_repository.dart' show SelfActionFailed;

/// The port of the status page to the server.
///
/// Errors: [SelfActionFailed] with the server's problem (`403` for a person without
/// `status.manage`, `404` for a gone component or incident or a public page that is off,
/// `400 validation_failed` with the field), or without a problem when no answer came.
///
/// Example:
/// ```dart
/// final page = await repository.status();
/// ```
abstract interface class StatusRepository {
  /// `getStatus`: the page as the signed-in person sees it.
  Future<api.StatusPage> status();

  /// `getPublicStatus`: the page without login; `404` unless `status.public_enabled`.
  Future<api.StatusPage> publicStatus();

  /// Whether the signed-in person is an owner (`getMe`, `is_owner`): only then the view offers
  /// the forms, because `status.manage` is an owner action (05). The server still decides
  /// each change.
  Future<bool> callerIsOwner();

  /// `createStatusComponent` with [input]; returns the new component.
  Future<api.StatusComponent> createStatusComponent(
    api.StatusComponentInput input,
  );

  /// `updateStatusComponent` of [componentId] with [input]; members [input] leaves out keep
  /// their value on the server.
  Future<api.StatusComponent> updateStatusComponent(
    String componentId,
    api.StatusComponentInput input,
  );

  /// `deleteStatusComponent` of [componentId].
  Future<void> deleteStatusComponent(String componentId);

  /// `createStatusIncident` with [input]: an incident or a maintenance window.
  Future<api.StatusIncident> createStatusIncident(
    api.StatusIncidentCreate input,
  );

  /// `addStatusIncidentUpdate` of [incidentId] with [input].
  Future<api.StatusIncident> addStatusIncidentUpdate(
    String incidentId,
    api.StatusIncidentUpdateCreate input,
  );
}

/// [StatusRepository] over the generated client [api.ApiClient] of the host (DPoP on the
/// apps, the cookie and CSRF header in the console); every write carries a new
/// `Idempotency-Key` from [newIdempotencyKey].
///
/// Example:
/// ```dart
/// final status = ApiStatusRepository(client, newIdempotencyKey: ids.next);
/// ```
final class ApiStatusRepository implements StatusRepository {
  /// Creates the repository over [_api].
  ApiStatusRepository(this._api, {required this.newIdempotencyKey});

  final api.ApiClient _api;

  /// A new UUIDv7 for the `Idempotency-Key` of each write.
  final String Function() newIdempotencyKey;

  api.StatusApi get _status => _api.getStatusApi();

  @override
  Future<api.StatusPage> status() =>
      adminCall(() async => (await _status.getStatus()).data!);

  @override
  Future<api.StatusPage> publicStatus() =>
      adminCall(() async => (await _status.getPublicStatus()).data!);

  @override
  Future<bool> callerIsOwner() =>
      adminCall(() async => (await _api.getMeApi().getMe()).data!.isOwner);

  @override
  Future<api.StatusComponent> createStatusComponent(
    api.StatusComponentInput input,
  ) => adminCall(
    () async => (await _status.createStatusComponent(
      statusComponentInput: input,
      idempotencyKey: newIdempotencyKey(),
    )).data!,
  );

  @override
  Future<api.StatusComponent> updateStatusComponent(
    String componentId,
    api.StatusComponentInput input,
  ) => adminCall(
    () async => (await _status.updateStatusComponent(
      componentId: componentId,
      statusComponentInput: input,
      idempotencyKey: newIdempotencyKey(),
    )).data!,
  );

  @override
  Future<void> deleteStatusComponent(String componentId) => adminCall(
    () async => _status.deleteStatusComponent(componentId: componentId),
  );

  @override
  Future<api.StatusIncident> createStatusIncident(
    api.StatusIncidentCreate input,
  ) => adminCall(
    () async => (await _status.createStatusIncident(
      statusIncidentCreate: input,
      idempotencyKey: newIdempotencyKey(),
    )).data!,
  );

  @override
  Future<api.StatusIncident> addStatusIncidentUpdate(
    String incidentId,
    api.StatusIncidentUpdateCreate input,
  ) => adminCall(
    () async => (await _status.addStatusIncidentUpdate(
      incidentId: incidentId,
      statusIncidentUpdateCreate: input,
      idempotencyKey: newIdempotencyKey(),
    )).data!,
  );
}

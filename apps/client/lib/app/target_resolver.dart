// SPDX-License-Identifier: Apache-2.0

/// Whether a target of a link may be shown (docs/spec/01-product.md 6.7; 15 11; brief M4-18
/// Design note 4).
library;

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:local_store/local_store.dart';

import 'router.dart';

/// What the main view does with a location.
enum TargetStatus {
  /// The target is in the local database or REST returned it.
  visible,

  /// The target does not exist or is invisible: "Not found or no access" (the same text for
  /// both, B7).
  notFound,

  /// REST could not be asked (offline); the view waits and tries again later.
  unknown,
}

/// Looks a target up first in the local database, then through REST (15 11: a link waits
/// until the target is loaded locally or through REST).
///
/// No permission logic (15 1 principle 1): only the answer of the server counts, `404` means
/// "Not found or no access".
final class TargetResolver {
  /// Creates the resolver over [db] and [api].
  TargetResolver(this._db, this._api);

  final LocalDatabase _db;
  final ApiClient _api;

  /// The status of [location]; a location without a target is visible, an [UnknownLocation]
  /// is not found.
  Future<TargetStatus> resolve(AppLocation location) async {
    switch (location) {
      case UnknownLocation():
        return TargetStatus.notFound;
      case SpaceLocation(:final spaceId):
        if (await _db.structureDao.spaceById(spaceId) != null) {
          return TargetStatus.visible;
        }
        return _remote(
          () => _api.getSpacesApi().getSpace(spaceId: spaceId.value),
        );
      case ChannelLocation(:final channelId):
        if (await _db.structureDao.channelById(channelId) != null) {
          return TargetStatus.visible;
        }
        return _remote(
          () => _api.getChannelsApi().getChannel(channelId: channelId.value),
        );
      case ThreadLocation(:final threadId):
        if (await _db.threadsDao.byId(threadId) != null) {
          return TargetStatus.visible;
        }
        return _remote(
          () => _api.getThreadsApi().getThread(threadId: threadId.value),
        );
      case HomeLocation() ||
          OnboardingLocation() ||
          ViewLocation() ||
          SectionLocation():
        return TargetStatus.visible;
    }
  }

  static Future<TargetStatus> _remote(
    Future<Response<Object?>> Function() call,
  ) async {
    try {
      await call();
      return TargetStatus.visible;
    } on DioException catch (e) {
      final status = e.response?.statusCode;
      if (status == 404 || status == 403) return TargetStatus.notFound;
      return TargetStatus.unknown;
    }
  }
}

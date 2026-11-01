// SPDX-License-Identifier: Apache-2.0

/// The provider of the delete port (01 6.18): the server side of the signed-in session, or
/// null before the login; tests override it with a fake.
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import 'delete_port.dart';

part 'providers.g.dart';

/// The [DeletePort] of the signed-in session, or null without one.
@riverpod
DeletePort? deletePort(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return ApiDeletePort(services.auth.api, newKey: services.ids.next);
}

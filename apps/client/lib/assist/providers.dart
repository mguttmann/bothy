// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the search assistant (docs/spec/14-search.md 4.10): the port over the
/// REST client of the signed-in instance.
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import 'assist_port.dart';

part 'providers.g.dart';

/// The search assistant of the signed-in instance (`assistSearch`), or null while signed out.
@riverpod
AssistPort? assistPort(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services == null ? null : ApiAssistPort(services.auth.api);
}

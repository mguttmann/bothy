// SPDX-License-Identifier: Apache-2.0

/// The Riverpod providers of creating a space and a channel (01 6.25; R-CL-03: widgets never
/// call the API themselves).
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import 'structure_port.dart';

part 'providers.g.dart';

/// The creates of the signed-in session, or null before the login; tests override it.
@riverpod
StructurePort? structurePort(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services == null ? null : ApiStructurePort(services.auth.api);
}

/// Makes an idempotency key for a create; tests override it.
@riverpod
String Function() structureKeys(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services?.ids.next ?? () => '';
}

// SPDX-License-Identifier: Apache-2.0

/// The status of the target the router shows (docs/spec/01-produkt.md 6.7).
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../router.dart';
import '../target_resolver.dart';
import 'session.dart';

part 'target.g.dart';

/// Whether the target of [path] (path and query) is visible; unknown while signed out.
@riverpod
Future<TargetStatus> targetStatus(Ref ref, String path) async {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return TargetStatus.unknown;
  return TargetResolver(
    services.database,
    services.auth.api,
  ).resolve(parseLocation(Uri.parse(path)));
}

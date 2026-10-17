// SPDX-License-Identifier: Apache-2.0

/// The outside world of the app as providers (docs/spec/15-client-architektur.md 4).
library;

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../bootstrap.dart';

part 'ports.g.dart';

/// The ports of the running app; `main.dart` and every test override it.
///
/// Throws an [UnimplementedError] without an override.
@Riverpod(keepAlive: true)
AppPorts appPorts(Ref ref) =>
    throw UnimplementedError('appPortsProvider must be overridden');

/// The client values of D-W-115 and D-W-116.
@Riverpod(keepAlive: true)
AppConfig appConfig(Ref ref) => ref.watch(appPortsProvider).config;

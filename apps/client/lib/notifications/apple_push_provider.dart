// SPDX-License-Identifier: Apache-2.0

/// The wiring of push on iOS and macOS (brief M5-04 Design note 5): the signed-in account and
/// its `notify.preview_mode` go to [ApplePush]; the app widget listens to its taps and tells it
/// when a thread opens.
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/bootstrap.dart';
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../notifications_settings/providers.dart';
import '../platform/push_apple.dart';
import '../telemetry/telemetry_providers.dart';

part 'apple_push_provider.g.dart';

/// [ApplePush] of the build kept in step with the signed-in account, or null where the build
/// has none (Windows, tests).
///
/// The account is handed over once per signed-in services and only after the personal
/// settings are read, so the extension never sees the default preview mode of a person who
/// chose `none`; later changes of `notify.preview_mode` follow.
@Riverpod(keepAlive: true)
ApplePush? applePush(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final push = ports.applePush;
  if (push == null) return null;
  InstanceServices? handed;
  void sync() {
    final services = ref.read(signedInServicesProvider);
    if (services == null) {
      handed = null;
      return;
    }
    if (identical(services, handed) ||
        !ref.read(personalSettingsLoadedProvider)) {
      return;
    }
    handed = services;
    unawaited(
      push.signedIn(
        PushAccount(
          instanceId: services.instance.instanceId,
          serverUrl: services.instance.address,
          secrets: services.secrets,
          registrations: ApiPushRegistrations(
            services.auth.api,
            services.ids.next,
          ),
          fetchTimeout: ports.config.pushFetchTimeout,
        ),
        previewMode: ref.read(notifySettingsProvider).previewMode,
      ),
    );
  }

  ref
    ..listen(signedInServicesProvider, (_, _) => sync())
    ..listen(personalSettingsLoadedProvider, (_, _) => sync())
    ..listen(notifySettingsProvider.select((s) => s.previewMode), (_, mode) {
      if (handed != null) unawaited(push.previewModeChanged(mode));
    });
  sync();
  return push;
}

// SPDX-License-Identifier: Apache-2.0

/// The wiring of push on iOS, macOS and Android (briefs M5-04 Design note 5 and M5-05 Design
/// note 4): the signed-in account and its `notify.preview_mode` go to the [PushPlatform] of the
/// build; the app widget listens to its taps and tells it when a thread opens.
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/bootstrap.dart';
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../notifications_settings/providers.dart';
import '../platform/push_platform.dart';
import '../telemetry/telemetry_providers.dart';

part 'push_provider.g.dart';

/// Whether the build receives pushes on this device; null where it has no push platform
/// (Windows, tests). D9 says so when it is false (brief M5-05 Design note 4).
@riverpod
Future<bool?> pushAvailable(Ref ref) async {
  final push = ref.watch(pushPlatformProvider);
  return push?.isAvailable();
}

/// The [PushPlatform] of the build kept in step with the signed-in account, or null where the
/// build has none (Windows, tests).
///
/// The account is handed over once per signed-in services and only after the personal
/// settings are read, so the extension or the push service never sees the default preview mode
/// of a person who chose `none`; later changes of `notify.preview_mode` follow.
@Riverpod(keepAlive: true)
PushPlatform? pushPlatform(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final push = ports.push;
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

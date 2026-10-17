// SPDX-License-Identifier: Apache-2.0

// The pure routing rule of a start (D-M4-18-15) over every stored state, discovery result and
// device answer; start_routing_test.dart runs the same rule through the controller.

import 'package:api_client/api_client.dart' show DeviceStateEnum;
import 'package:client/app/discovery.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/onboarding/start_route.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final found = DiscoveryFound(
    InstanceInfo(
      address: Uri.parse('https://chat.example.com'),
      instanceId: 'i',
      productName: 'Chat',
    ),
  );
  final discoveries = <DiscoveryOutcome>[
    found,
    const DiscoveryNoInstance(),
    const DiscoveryUnreachable(),
    const DiscoveryCertificateInvalid(),
    const DiscoverySetupRequired(),
    const DiscoveryAppTooOld('9.0.0'),
  ];
  final devices = <DeviceStateEnum?>[
    null,
    DeviceStateEnum.pending,
    DeviceStateEnum.active,
    DeviceStateEnum.revoked,
  ];

  (OnboardingStep, ServerStatus) want(
    StoredAccount stored,
    DiscoveryOutcome discovery,
    DeviceStateEnum? device,
  ) {
    if (discovery is DiscoveryCertificateInvalid) {
      return (OnboardingStep.server, ServerStatus.certificateInvalid);
    }
    if (discovery is DiscoverySetupRequired) {
      return (OnboardingStep.server, ServerStatus.notSetUp);
    }
    if (discovery is DiscoveryAppTooOld) {
      return (OnboardingStep.server, ServerStatus.appTooOld);
    }
    if (stored == StoredAccount.none) {
      return discovery is DiscoveryFound
          ? (OnboardingStep.login, ServerStatus.idle)
          : (OnboardingStep.server, ServerStatus.noInstance);
    }
    final waits = switch (device) {
      DeviceStateEnum.pending => true,
      DeviceStateEnum.active => false,
      _ => stored == StoredAccount.devicePending,
    };
    return (
      waits ? OnboardingStep.pending : OnboardingStep.signedIn,
      ServerStatus.idle,
    );
  }

  for (final stored in StoredAccount.values) {
    for (final discovery in discoveries) {
      for (final device in devices) {
        final expected = want(stored, discovery, device);
        test('${stored.name} x ${discovery.runtimeType} x ${device?.name} '
            '-> $expected', () {
          final route = routeStart(
            stored: stored,
            discovery: discovery,
            device: device,
          );
          expect((route.step, route.serverStatus), expected);
          expect(
            route.minVersion,
            discovery is DiscoveryAppTooOld ? '9.0.0' : isNull,
          );
        });
      }
    }
  }
}

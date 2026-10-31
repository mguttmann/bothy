// SPDX-License-Identifier: Apache-2.0

// Step-up of the instance administration (brief M6-09 Tests, fixed requirement 11): the fake
// asks for a step-up exactly as `docs/spec/actions.yaml` says for the action behind each call
// (`instance.settings.update`, `instance.branding.update`, `feature_flag.update`,
// `identity_provider.manage` all have `step_up: true`), and every write of the screens carries
// the purpose text the dialog shows; the reading calls never ask.

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:flutter_test/flutter_test.dart';

import '../audit/audit_step_up_test.dart' show catalogStepUp;
import 'support.dart';

/// Answers every step-up with yes and records its purpose.
class _Recording implements StepUpPrompt {
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }

  /// These tests confirm with the step-up key; a password prompt cancels.
  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

void main() {
  test('the fake asks for step-up exactly as actions.yaml says', () {
    for (final MapEntry(key: op, value: action) in instanceActionOf.entries) {
      expect(
        instanceStepUpOf[op],
        catalogStepUp(action) == 'true',
        reason: '$op ($action)',
      );
    }
    for (final action in [
      'instance.settings.read',
      'feature_flag.read',
      'identity_provider.read',
    ]) {
      expect(catalogStepUp(action), 'false', reason: action);
    }
  });

  test('every write asks once with its purpose; reads never ask', () async {
    final prompt = _Recording();
    final r = FakeAdminRepository(prompt: prompt);
    final p = r.seedProvider(ProviderKind.local, 'Local accounts', const {});
    final writes = <String, Future<void> Function()>{
      'putInstanceSetting': () => r.putInstanceSetting(
        'locale.default',
        'en',
        version: 0,
        purpose: 'putInstanceSetting',
      ),
      'updateBranding': () => r.updateBranding(
        productName: 'Team Chat',
        accentColor: 'loch',
        version: 0,
        purpose: 'updateBranding',
      ),
      'uploadLogo': () => r.uploadLogo(
        PickedFile('logo.png', fakeLogoBytes),
        purpose: 'uploadLogo',
      ),
      'removeLogo': () => r.removeLogo(purpose: 'removeLogo'),
      'putFeatureFlag': () => r.putFeatureFlag(
        r.flagsByKey['dm.enabled']!,
        enabled: true,
        reason: 'Pilot',
        purpose: 'putFeatureFlag',
      ),
      'createIdentityProvider': () => r.createIdentityProvider(
        const ProviderDraft(
          kind: ProviderKind.trustedHeader,
          name: 'Tailnet',
          config: {'login_header': 'Tailscale-User-Login'},
          enabled: true,
          allowProvisioning: false,
        ),
        purpose: 'createIdentityProvider',
      ),
      'updateIdentityProvider': () => r.updateIdentityProvider(
        p.id,
        const ProviderChange(enabled: false),
        purpose: 'updateIdentityProvider',
      ),
      'deleteIdentityProvider': () =>
          r.deleteIdentityProvider(p.id, purpose: 'deleteIdentityProvider'),
    };
    expect(writes.keys.toSet(), instanceStepUpOf.keys.toSet());
    for (final MapEntry(key: op, value: run) in writes.entries) {
      r.stepUpFresh = false;
      prompt.asked.clear();
      await run();
      expect(prompt.asked, [op], reason: op);
    }

    r.stepUpFresh = false;
    prompt.asked.clear();
    await r.instanceSettings();
    await r.instanceOverview();
    await r.featureFlags();
    await r.identityProviders();
    await r.logo();
    expect(prompt.asked, isEmpty);
  });
}

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Drives E1 to E7 from the outcomes of `LoginService` (Design note 3).
///
/// E1 [connect] runs discovery and composes the services of the instance; E2 and E3 run the
/// login flows; `pending` goes to E4 and polls; `signedIn` starts the engine and shows the main
/// view; every end of the account ([logout], [cancelPending], a rejected or expired device, a
/// revoked session, `4012`) runs the one [AccountWiper] and shows E7 with its cause.

@ProviderFor(OnboardingController)
final onboardingControllerProvider = OnboardingControllerProvider._();

/// Drives E1 to E7 from the outcomes of `LoginService` (Design note 3).
///
/// E1 [connect] runs discovery and composes the services of the instance; E2 and E3 run the
/// login flows; `pending` goes to E4 and polls; `signedIn` starts the engine and shows the main
/// view; every end of the account ([logout], [cancelPending], a rejected or expired device, a
/// revoked session, `4012`) runs the one [AccountWiper] and shows E7 with its cause.
final class OnboardingControllerProvider
    extends $NotifierProvider<OnboardingController, OnboardingState> {
  /// Drives E1 to E7 from the outcomes of `LoginService` (Design note 3).
  ///
  /// E1 [connect] runs discovery and composes the services of the instance; E2 and E3 run the
  /// login flows; `pending` goes to E4 and polls; `signedIn` starts the engine and shows the main
  /// view; every end of the account ([logout], [cancelPending], a rejected or expired device, a
  /// revoked session, `4012`) runs the one [AccountWiper] and shows E7 with its cause.
  OnboardingControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'onboardingControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$onboardingControllerHash();

  @$internal
  @override
  OnboardingController create() => OnboardingController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(OnboardingState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<OnboardingState>(value),
    );
  }
}

String _$onboardingControllerHash() =>
    r'a5da07991755866f74112ce48f90a69f784b6423';

/// Drives E1 to E7 from the outcomes of `LoginService` (Design note 3).
///
/// E1 [connect] runs discovery and composes the services of the instance; E2 and E3 run the
/// login flows; `pending` goes to E4 and polls; `signedIn` starts the engine and shows the main
/// view; every end of the account ([logout], [cancelPending], a rejected or expired device, a
/// revoked session, `4012`) runs the one [AccountWiper] and shows E7 with its cause.

abstract class _$OnboardingController extends $Notifier<OnboardingState> {
  OnboardingState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<OnboardingState, OnboardingState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<OnboardingState, OnboardingState>,
              OnboardingState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

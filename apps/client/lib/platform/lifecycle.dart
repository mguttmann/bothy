// SPDX-License-Identifier: Apache-2.0

/// The app lifecycle of the phone: the privacy cover of E6 and the background state of the
/// sync engine (docs/spec/01-product.md 5.6 E6; docs/spec/15-client-architecture.md 7.2;
/// brief M5-02 Design note 6; docs/decisions.d/M5-02.md D-M5-02-6).
///
/// | Flutter state | Privacy cover | Sync engine (`FlutterAppLifecycle`) |
/// |---|---|---|
/// | `resumed` | removed | foreground: connects with `resume` from `Suspended` |
/// | `inactive` | shown, before the system takes the app switcher picture | foreground |
/// | `hidden`, `paused` | shown | background: closes with `1000`, state `Suspended` |
/// | `detached` | shown | background |
///
/// The engine part needs no code here: `FlutterAppLifecycle` reports the activity, and the
/// phone builds its engine without `keepConnectedInBackground` (`InstanceServices.open`).
library;

import 'package:flutter/widgets.dart';

/// Whether the privacy cover shows in [state]: in every state but `resumed`.
///
/// iOS takes the picture of the app switcher right after `inactive`, so the cover must be in
/// the next frame of that state already; a cover that waits for `paused` is too late.
bool coversContent(AppLifecycleState state) =>
    state != AppLifecycleState.resumed;

/// The privacy cover state of the phone, driven by Flutter's [AppLifecycleListener].
///
/// [covered] follows [coversContent]. The app lock of M5-08 decides later whether `resumed`
/// shows E6 instead of the content; this class only hides the content while the app is not
/// in front (01 5.6 E6: "The content is hidden, also in the app switcher").
///
/// Example:
/// ```dart
/// final cover = PrivacyCover();
/// cover.addListener(() => print(cover.covered));
/// ```
final class PrivacyCover extends ChangeNotifier {
  /// Creates the cover and starts listening; [initial] is the state at start (the binding's
  /// state by default, `resumed` when the binding has none yet).
  PrivacyCover({AppLifecycleState? initial})
    : _covered = coversContent(
        initial ??
            WidgetsBinding.instance.lifecycleState ??
            AppLifecycleState.resumed,
      ) {
    _listener = AppLifecycleListener(onStateChange: onState);
  }

  late final AppLifecycleListener _listener;
  bool _covered;

  /// Whether the content is hidden now.
  bool get covered => _covered;

  /// Records the Flutter [state]; notifies only when [covered] changes.
  @visibleForTesting
  void onState(AppLifecycleState state) {
    final next = coversContent(state);
    if (next == _covered) return;
    _covered = next;
    notifyListeners();
  }

  @override
  void dispose() {
    _listener.dispose();
    super.dispose();
  }
}

// SPDX-License-Identifier: Apache-2.0

/// The app lock of the phone (docs/spec/08-identity.md 9; docs/spec/01-product.md 5.6 E6;
/// brief M5-08 Design notes 1 to 5): the policy of `getInstance` cached with the instance row,
/// the person's device-local switch, the last activity on an injected clock, the states
/// `unlocked`, `locked` and `unavailable`, and the `locked` and `unlocked` events the terminal
/// view of M5-12 follows.
library;

import 'dart:async';

import 'package:auth/auth.dart' show Clock, SecretStore;
import 'package:domain/domain.dart' show AppLockPolicy;
import 'package:flutter/foundation.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity, AppLifecycle;

import 'app_lock_port.dart';
import 'lock_decision.dart';

/// The state of the lock.
enum AppLockStatus {
  /// The person's switch and the cached policy are being read at a cold start; the content
  /// stays hidden behind the privacy cover until the lock decided.
  checking,

  /// The content shows.
  unlocked,

  /// E6 shows; only the system authentication or "Log out" leave it.
  locked,

  /// The lock applies but the device has no device credential: only the notice and "Log out"
  /// show until one is set (Design note 4).
  unavailable,
}

/// An event of the lock for other parts of the app (Design note 5).
enum AppLockEvent {
  /// The lock closed; emitted before E6 appears. Open terminal sessions detach (01 5.6 E6;
  /// they keep running on the host).
  locked,

  /// The system authentication succeeded and the content shows again. The lock never
  /// reattaches anything itself; the terminal view decides after this event.
  unlocked,
}

/// The outcome of turning the switch on or off.
enum AppLockSwitchResult {
  /// The switch changed.
  changed,

  /// The system authentication failed or was cancelled; the switch stays.
  failed,

  /// The device has no device credential; the switch stays off.
  noCredential,

  /// `security.app_lock_required` holds; the switch cannot be turned off.
  required,
}

/// A clock that only moves forward, counts the time the device sleeps and that nobody can
/// set: the time since a fixed start (D-M5-08-8).
abstract interface class MonotonicClock {
  /// The time since the start, as of the last [sync] plus the time awake since.
  Duration get elapsed;

  /// Takes in the time the device slept since the last call; the lock calls it before every
  /// decision. Never throws.
  Future<void> sync();
}

/// The time since boot of the platform, counting sleep and not settable (Android
/// `SystemClock.elapsedRealtime`, iOS `CLOCK_MONOTONIC`), or null where the platform has none.
typedef SinceBoot = Future<Duration?> Function();

/// [MonotonicClock] over a [Stopwatch] and the platform's time since boot.
///
/// A [Stopwatch] reads a monotonic clock that stands still while the device sleeps (Android
/// `CLOCK_MONOTONIC`, iOS `CLOCK_MONOTONIC_RAW`), so on its own a sleeping device would not
/// count. [sync] compares it with [SinceBoot] and adds the difference, the time slept; where
/// [SinceBoot] gives nothing the clock is the stopwatch alone.
///
/// Example: `final clock = SleepAwareClock(ports.sinceBoot); await clock.sync();`
final class SleepAwareClock implements MonotonicClock {
  /// Creates the clock over [sinceBoot]; [watch] replaces the stopwatch in tests.
  SleepAwareClock(this._sinceBoot, {Duration Function()? watch})
    : _watch = watch ?? _started();

  final SinceBoot _sinceBoot;
  final Duration Function() _watch;
  Duration? _bootAtStart;
  Duration _slept = Duration.zero;

  static Duration Function() _started() {
    final watch = Stopwatch()..start();
    return () => watch.elapsed;
  }

  @override
  Duration get elapsed => _watch() + _slept;

  @override
  Future<void> sync() async {
    final Duration? boot;
    try {
      boot = await _sinceBoot();
    } on Object {
      return;
    }
    if (boot == null) return;
    final awake = _watch();
    final start = _bootAtStart ??= boot - awake;
    final slept = boot - start - awake;
    if (slept > _slept) _slept = slept;
  }
}

/// The person's choice of the lock on this device (Design note 4): a device-local preference,
/// not a synced `user_settings` key, because the lock protects this device and another device
/// may lack biometrics.
abstract interface class AppLockSwitchStore {
  /// Whether the person turned the lock on; false when never set.
  Future<bool> read();

  /// Stores [on]; durable when the future completes.
  Future<void> write(bool on);
}

/// [AppLockSwitchStore] in the secrets of the instance (secure storage), so the account's wipe
/// (R-SY-14) removes it with the tokens and keys.
final class SecretAppLockSwitch implements AppLockSwitchStore {
  /// Creates the store over [secrets].
  SecretAppLockSwitch(this._secrets);

  /// The key in the instance's secrets.
  static const String key = 'app_lock/enabled';

  final SecretStore _secrets;

  @override
  Future<bool> read() async => await _secrets.read(key) == 'true';

  @override
  Future<void> write(bool on) => _secrets.write(key, on ? 'true' : 'false');
}

/// Decides and holds the lock of one signed-in instance on a phone.
///
/// A cold start with a stored session ([lockAtStart]) begins in [AppLockStatus.checking] and
/// locks before the first frame with content when the lock applies (Design note 2). Afterwards
/// the lock closes when the time since the last activity ([recordActivity]) reaches
/// `client.app_lock_after` while the app is in front, and on the return from the background
/// when that time passed meanwhile. [unlock] runs the system authentication; a failure keeps
/// the lock with [failed] set, and the app counts no attempts (Design note 3).
///
/// Example:
/// ```dart
/// final lock = AppLockController(port: port, clock: clock, lifecycle: lifecycle,
///     store: store, policy: policies, lockAtStart: true);
/// lock.events.listen((e) { if (e == AppLockEvent.locked) detachTerminals(); });
/// ```
final class AppLockController extends ChangeNotifier {
  /// Creates the controller and starts reading the switch and the policy.
  ///
  /// [policy] emits the cached policy of the instance row and every change (null while none
  /// is known); [lifecycle] reports the foreground and the background.
  AppLockController({
    required this._port,
    required this._clock,
    required AppLifecycle lifecycle,
    required this._store,
    required Stream<AppLockPolicy?> policy,
    required bool lockAtStart,
    MonotonicClock? monotonic,
  }) : _monotonic = monotonic ?? SleepAwareClock(_noBootClock),
       _status = lockAtStart ? AppLockStatus.checking : AppLockStatus.unlocked {
    _lastActivity = _now();
    _inBackground = lifecycle.current == AppActivity.background;
    _lifecycle = lifecycle.changes.listen(_onActivity);
    unawaited(_start(policy, lockAtStart));
  }

  final AppLockPort _port;
  final Clock _clock;
  final MonotonicClock _monotonic;
  final AppLockSwitchStore _store;
  final StreamController<AppLockEvent> _events =
      StreamController<AppLockEvent>.broadcast(sync: true);
  late final StreamSubscription<AppActivity> _lifecycle;
  StreamSubscription<AppLockPolicy?>? _policies;
  Timer? _timer;
  AppLockStatus _status;
  late LockInstant _lastActivity;
  LockInstant? _backgroundSince;
  bool _inBackground = false;
  bool _enabled = false;
  bool _ready = false;
  bool _failed = false;
  bool _busy = false;
  bool _disposed = false;
  AppLockPolicy? _policy;

  /// The state of the lock.
  AppLockStatus get status => _status;

  static Future<Duration?> _noBootClock() async => null;

  /// Now on both clocks of the lock (D-M5-08-8).
  LockInstant _now() => LockInstant(_clock.now(), _monotonic.elapsed);

  /// Whether the content is hidden ([AppLockStatus.locked] or [AppLockStatus.unavailable]).
  bool get closed =>
      _status == AppLockStatus.locked || _status == AppLockStatus.unavailable;

  /// Whether the last unlock failed; E6 then shows "Unlocking failed." with "Try again".
  bool get failed => _failed;

  /// Whether a system prompt is open.
  bool get busy => _busy;

  /// The person's switch on this device.
  bool get enabled => _enabled;

  /// The cached policy of the instance, or null while none is known.
  AppLockPolicy? get policy => _policy;

  /// Whether `security.app_lock_required` holds; the switch then shows on and disabled.
  bool get required => _policy?.required ?? false;

  /// Whether the lock applies on this device.
  bool get applies => lockApplies(enabled: _enabled, policy: _policy);

  /// [AppLockEvent.locked] before E6 appears and [AppLockEvent.unlocked] after a successful
  /// unlock; synchronous, so a listener runs before the next frame.
  Stream<AppLockEvent> get events => _events.stream;

  Future<void> _start(Stream<AppLockPolicy?> policies, bool lockAtStart) async {
    final first = Completer<void>();
    _policies = policies.listen(
      (p) {
        final applied = applies;
        _policy = p;
        if (!first.isCompleted) {
          first.complete();
          return;
        }
        _onPolicy(applied);
      },
      onError: (Object _) {
        if (!first.isCompleted) first.complete();
      },
    );
    var on = false;
    try {
      on = await _store.read();
    } on Object {
      // An unreadable switch counts as off; the owner's policy still applies.
    }
    await first.future;
    await _monotonic.sync();
    if (_disposed) return;
    _enabled = on;
    _ready = true;
    if (_status == AppLockStatus.checking && applies) {
      await _lock();
    } else {
      if (_status == AppLockStatus.checking) {
        _status = AppLockStatus.unlocked;
        _lastActivity = _now();
      }
      notifyListeners();
      await _enforceCredential();
      _schedule();
    }
    if (!_decided.isCompleted) _decided.complete();
  }

  final Completer<void> _decided = Completer<void>();

  /// Completes once the switch and the cached policy were read and the start decided between
  /// [AppLockStatus.locked], [AppLockStatus.unavailable] and [AppLockStatus.unlocked].
  Future<void> get decided => _decided.future;

  void _onPolicy(bool appliedBefore) {
    if (_disposed) return;
    if (!applies && _status == AppLockStatus.unavailable) {
      // The owner no longer requires the lock and the person never turned it on.
      _open();
      return;
    }
    notifyListeners();
    if (!appliedBefore && applies) unawaited(_enforceCredential());
    _schedule();
  }

  /// Without a device credential an applying lock cannot be opened again, so the content
  /// hides behind the notice at once (fixed requirement 14).
  Future<void> _enforceCredential() async {
    if (!_ready || !applies || _status != AppLockStatus.unlocked) return;
    if (await _port.hasDeviceCredential() || _disposed) return;
    if (_status == AppLockStatus.unlocked && applies) await _lock();
  }

  void _onActivity(AppActivity activity) {
    if (activity == AppActivity.background) {
      _inBackground = true;
      _backgroundSince = _now();
      _timer?.cancel();
      return;
    }
    _inBackground = false;
    unawaited(check());
  }

  /// Records a pointer or key event of the person; ignored while the lock is closed.
  void recordActivity() {
    if (_status != AppLockStatus.unlocked) return;
    _lastActivity = _now();
  }

  /// Applies the lock decision now: closes the lock when the time without use passed, checks
  /// the device credential again while [AppLockStatus.unavailable], and schedules the next
  /// check in the foreground. The lifecycle and the timer call it; tests call it after moving
  /// the clock.
  Future<void> check() async {
    if (!_ready || _busy || _disposed) return;
    // The time the device slept counts (D-M5-08-8).
    await _monotonic.sync();
    if (_busy || _disposed) return;
    if (_status == AppLockStatus.unavailable) {
      if (await _port.hasDeviceCredential() &&
          _status == AppLockStatus.unavailable &&
          !_disposed) {
        _status = AppLockStatus.locked;
        _failed = false;
        notifyListeners();
      }
      return;
    }
    if (_status != AppLockStatus.unlocked || !applies) return;
    final after = lockAfter(_policy);
    final since = _backgroundSince;
    // Activity after the move to the background (the system prompt of an unlock sends some
    // apps to the background) shows the person was there.
    final returned = since != null && since.notBefore(_lastActivity);
    final idle = idleReached(
      lastActivity: _lastActivity,
      now: _now(),
      after: after,
    );
    // A zero time (no policy known) locks only on a return from the background, never while
    // the person uses the app.
    if (idle && (after > Duration.zero || returned)) {
      await _lock();
      return;
    }
    _schedule();
  }

  void _schedule() {
    _timer?.cancel();
    if (_inBackground || !_ready || _status != AppLockStatus.unlocked) return;
    if (!applies) return;
    final after = lockAfter(_policy);
    if (after <= Duration.zero) return;
    _timer = Timer(
      idleLeft(lastActivity: _lastActivity, now: _now(), after: after),
      () => unawaited(check()),
    );
  }

  Future<void> _lock() async {
    if (_disposed) return;
    _timer?.cancel();
    final wasClosed = closed;
    _status = AppLockStatus.locked;
    _failed = false;
    if (!wasClosed) _events.add(AppLockEvent.locked);
    notifyListeners();
    if (await _port.hasDeviceCredential() || _disposed) return;
    if (_status == AppLockStatus.locked) {
      _status = AppLockStatus.unavailable;
      notifyListeners();
    }
  }

  void _open() {
    _status = AppLockStatus.unlocked;
    _failed = false;
    _lastActivity = _now();
    _backgroundSince = null;
    _events.add(AppLockEvent.unlocked);
    notifyListeners();
    _schedule();
  }

  /// E6 "Unlock" and "Try again": asks the system with [reason] (the text of the prompt).
  /// Success opens the lock; a failure or a cancel keeps E6 with [failed]; a device without a
  /// credential shows the notice.
  Future<void> unlock(String reason) async {
    if (_status != AppLockStatus.locked || _busy || _disposed) return;
    _busy = true;
    notifyListeners();
    final result = await _port.authenticate(reason);
    _busy = false;
    if (_disposed) return;
    switch (result) {
      case AppLockAuthResult.success:
        _open();
      case AppLockAuthResult.failed:
        _failed = true;
        notifyListeners();
      case AppLockAuthResult.noCredential:
        _status = AppLockStatus.unavailable;
        notifyListeners();
    }
  }

  /// The switch in the settings (Design note 4). Turning it on asks for the system
  /// authentication once with [reason], so nobody locks themselves out on a device without a
  /// credential; turning it off is refused while `security.app_lock_required` holds.
  Future<AppLockSwitchResult> setEnabled(
    bool on, {
    required String reason,
  }) async {
    if (!on && required) return AppLockSwitchResult.required;
    if (on == _enabled) return AppLockSwitchResult.changed;
    if (!on) {
      await _store.write(false);
      _enabled = false;
      notifyListeners();
      _schedule();
      return AppLockSwitchResult.changed;
    }
    if (_busy) return AppLockSwitchResult.failed;
    if (!await _port.hasDeviceCredential()) {
      return AppLockSwitchResult.noCredential;
    }
    _busy = true;
    notifyListeners();
    final result = await _port.authenticate(reason);
    _busy = false;
    if (_disposed) return AppLockSwitchResult.failed;
    if (result != AppLockAuthResult.success) {
      notifyListeners();
      return result == AppLockAuthResult.noCredential
          ? AppLockSwitchResult.noCredential
          : AppLockSwitchResult.failed;
    }
    await _store.write(true);
    _enabled = true;
    _lastActivity = _now();
    notifyListeners();
    _schedule();
    return AppLockSwitchResult.changed;
  }

  @override
  void dispose() {
    _disposed = true;
    _timer?.cancel();
    unawaited(_lifecycle.cancel());
    unawaited(_policies?.cancel());
    unawaited(_events.close());
    super.dispose();
  }
}

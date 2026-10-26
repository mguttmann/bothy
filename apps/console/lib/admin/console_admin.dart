// SPDX-License-Identifier: Apache-2.0

/// The administration of the console: the `admin_ui` repository over the console's cookie
/// based API, the stored "View as person" and the time zone of the signed-in person (brief
/// M6-04 Design notes 1 and 3, docs/decisions.d/M6-04.md D-M6-04-1 and D-M6-04-3).
library;

import 'dart:math';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart' show Uuid7Generator;
import 'package:flutter/foundation.dart';

import '../routes.dart';
import '../session/console_controller.dart';
import 'console_route.dart';

/// The state of the administration that outlives a page of the console.
///
/// The repository is `ApiAdminRepository` over [ConsoleController.api], so every call carries
/// the cookie and the CSRF header and passes the action step-up adapter; nothing here decides
/// a permission (15 1 principle 1).
///
/// "View as person" (07 9.2): [startViewAs] calls `viewAsUser` from the console session, which
/// stores the view on the session on the server; the answer gives the visible spaces and
/// channels, which the console keeps in memory only for the structure page ([snapshot]). The
/// banner reads the stored view from `getConsoleSession` ([ConsoleSession.viewAs]), so it stays
/// on every page until [endViewAs] or the end time, also after a reload. The console never
/// opens channel content (ADR 0028).
///
/// Example:
/// ```dart
/// final admin = ConsoleAdmin(controller);
/// await admin.startViewAs(sam, purpose: l.stepUpPurposeViewAs('Sam'));
/// ```
class ConsoleAdmin extends ChangeNotifier {
  /// Creates the administration of [controller]; [repository] replaces the REST source in tests,
  /// [clock] the current time.
  ConsoleAdmin(
    this.controller, {
    AdminRepository? repository,
    DateTime Function()? clock,
    Random? random,
  }) : clock = clock ?? DateTime.now,
       repository =
           repository ??
           ApiAdminRepository(
             controller.api,
             newIdempotencyKey: _ids(random ?? Random.secure()).next,
           ) {
    controller.addListener(_onController);
    _onController();
  }

  /// The console's state.
  final ConsoleController controller;

  /// The calls of the administration.
  final AdminRepository repository;

  /// The current time.
  final DateTime Function() clock;

  ViewAsResult? _snapshot;
  List<String> _notVisible = const [];
  String? _viewName;
  bool _starting = false;
  bool _ending = false;
  Object? _failure;
  String _timeZone = 'UTC';
  bool _meRead = false;
  Uri? _grafanaEmbedUrl;
  String? _user;

  /// The answer of the last `viewAsUser` of this page load, or null (none yet, or a reload
  /// during a stored view).
  ViewAsResult? get snapshot => _snapshot;

  /// The names of the spaces the owner saw before the view and the viewed person does not.
  List<String> get notVisible => _notVisible;

  /// The display name of the person of the view: the stored one, else the one being started.
  String? get viewName => storedView?.displayName ?? _viewName;

  /// Whether a view is being started.
  bool get starting => _starting;

  /// Whether the view is being ended.
  bool get ending => _ending;

  /// The error of the last start or end, or null; the page shows it through `adminFailure`.
  Object? get failure => _failure;

  /// The time zone of the signed-in person (`getMe.time_zone`), UTC until it is read.
  String get timeZone => _timeZone;

  /// The dashboard of the page "Operations (Grafana)": `grafana_embed_url` of `getInstance`,
  /// which the server sends to owners only and only with `console.grafana_embed_url` set (brief
  /// M6-17 Design note 3); null until it is read, for anyone else and without the key.
  Uri? get grafanaEmbedUrl => _grafanaEmbedUrl;

  /// The view stored on the console session, or null.
  ConsoleSessionViewAs? get storedView => controller.session?.viewAs;

  /// Starts "View as person" for [person] and opens its structure page; [purpose] is the text
  /// of the step-up. A stored view ends first (07 9.2: starting a new view while one is stored
  /// is denied). The owner's spaces are read before the view starts, because afterwards every
  /// read of the session runs with the view's intersected rights; their difference to the answer
  /// is the "Not visible" list. A cancelled step-up shows nothing.
  Future<void> startViewAs(
    AdminPerson person, {
    required String purpose,
  }) async {
    controller.openRoute(const ConsoleRoute(ConsolePage.people, viewAs: true));
    _starting = true;
    _failure = null;
    _snapshot = null;
    _notVisible = const [];
    _viewName = person.displayName;
    notifyListeners();
    try {
      if (storedView != null) {
        await controller.api.getAuthApi().endViewAs();
        await controller.refreshSession();
      }
      List<AdminTarget> targets;
      try {
        targets = await repository.targets();
      } on Object {
        // Without the owner's list the structure still shows; only "Not visible" is missing.
        targets = const [];
      }
      final snapshot = await repository.viewAs(person.id, purpose: purpose);
      _snapshot = snapshot;
      _notVisible = notVisibleSpaces(targets, snapshot);
      await controller.refreshSession();
    } on Object catch (e) {
      _failure = e;
    } finally {
      _starting = false;
      notifyListeners();
    }
  }

  /// Ends the stored view (`endViewAs`, audit `ended_reason = ended`) and reads the session
  /// again, so the banner goes away.
  Future<void> endViewAs() async {
    _ending = true;
    _failure = null;
    notifyListeners();
    try {
      await controller.api.getAuthApi().endViewAs();
      _snapshot = null;
      _notVisible = const [];
      await controller.refreshSession();
    } on Object catch (e) {
      _failure = e;
    } finally {
      _ending = false;
      notifyListeners();
    }
  }

  /// The end time of the stored view has passed: the structure goes, and the session is read
  /// again; the server ends the view with the first request after its end (audit `ended_reason
  /// = expired`).
  Future<void> viewEnded() async {
    _snapshot = null;
    _notVisible = const [];
    notifyListeners();
    await controller.refreshSession(afterViewEnd: true);
  }

  void _onController() {
    final user = controller.session?.userId;
    if (user != _user) {
      // A session ended or another person signed in: nothing of the last view and profile may
      // stay (the structure is only for the owner who started it).
      _user = user;
      _snapshot = null;
      _notVisible = const [];
      _viewName = null;
      _failure = null;
      _timeZone = 'UTC';
      _meRead = false;
      _grafanaEmbedUrl = null;
    }
    if (controller.state is ConsoleSignedIn && !_meRead) {
      _meRead = true;
      _readMe();
      _readInstance();
    }
    notifyListeners();
  }

  Future<void> _readInstance() async {
    final user = _user;
    try {
      final info = (await controller.api.getAdminApi().getInstance()).data;
      final url = info?.grafanaEmbedUrl;
      final parsed = url == null ? null : Uri.tryParse(url);
      // Only an https URL is embedded; the server checks the key at the start as well.
      if (user == _user && parsed != null && parsed.isScheme('https')) {
        _grafanaEmbedUrl = parsed;
        notifyListeners();
      }
    } on Object {
      // Without the instance view the console shows no Grafana page.
    }
  }

  Future<void> _readMe() async {
    try {
      final zone = (await controller.api.getMeApi().getMe()).data?.timeZone;
      if (zone != null && zone.isNotEmpty) {
        _timeZone = zone;
        notifyListeners();
      }
    } on Object {
      // The administration shows its times in UTC without the profile.
    }
  }

  @override
  void dispose() {
    controller.removeListener(_onController);
    super.dispose();
  }

  static Uuid7Generator _ids(Random random) => Uuid7Generator(
    nowMillis: () => DateTime.now().millisecondsSinceEpoch,
    // Not `1 << bits`: in the web build a shift works on 32 bits, so `1 << 31` is negative and
    // `1 << 32` is 1; the power of two is exact up to 2^32, the limit of `nextInt`.
    random: (bits) => random.nextInt(pow(2, bits).toInt()),
  );
}

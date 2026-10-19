// SPDX-License-Identifier: Apache-2.0

/// Crash reports through `sentry_flutter`, without content (docs/spec/15-client-architektur.md
/// 16; ADR 0057 item 5; brief M4-31 Design note 4).
///
/// Sentry starts only when the build names a target (the define `CRASH_DSN`; there is no instance
/// setting for it, docs/DECISIONS.md D-M4-31-3) and the person leaves `privacy.crash_reports` on
/// (01 11; default on when a target exists). The options turn off everything that could carry
/// content: no default PII, no screenshots, no view hierarchy, no session replay, no breadcrumbs
/// of user interactions, no traces. [CrashReports.scrubEvent] and [CrashReports.scrubBreadcrumb]
/// remove message texts, exception values, titles, file names, request data with every header
/// (`Authorization`, `DPoP`, `DPoP-Nonce`, `Cookie`) and body, query strings and path segments
/// that look like IDs, so no access token, refresh token or proof reaches a report. A report
/// keeps only technical attributes: release, environment, platform, stack frames of the code and
/// the hashed instance ID as the tag `instance`.
library;

import 'dart:async';
import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

/// Starts Sentry with the options [configure] sets; production: `SentryFlutter.init`.
typedef SentryStart = Future<void> Function(
  FlutterOptionsConfiguration configure,
);

/// The crash path of the app.
///
/// Example:
/// ```dart
/// final crashes = CrashReports(dsn: const String.fromEnvironment('CRASH_DSN'),
///     release: 'client@1.4.0', environment: 'production');
/// await crashes.apply(enabled: settings.crashReports);
/// ```
final class CrashReports {
  /// Creates the crash path for the target [dsn] ("" for none); [start] and [close] replace
  /// `SentryFlutter.init` and `Sentry.close` in tests.
  CrashReports({
    required this.dsn,
    required this.release,
    required this.environment,
    SentryStart? start,
    Future<void> Function()? close,
  }) : _start = start ?? SentryFlutter.init,
       _close = close ?? Sentry.close;

  /// The target from the build configuration; empty for none.
  final String dsn;

  /// `release` of every report (the app version of the build).
  final String release;

  /// `environment` of every report.
  final String environment;

  final SentryStart _start;
  final Future<void> Function() _close;
  bool _running = false;
  String? _instanceTag;

  /// Whether the build names a target; only then Settings shows `privacy.crash_reports`.
  bool get hasTarget => dsn.isNotEmpty;

  /// Whether Sentry runs.
  bool get running => _running;

  /// Starts Sentry when a target exists and [enabled] (`privacy.crash_reports`) is on, closes
  /// it when the switch is turned off.
  Future<void> apply({required bool enabled}) async {
    if (!hasTarget) return;
    if (enabled && !_running) {
      _running = true;
      await _start(configure);
    } else if (!enabled && _running) {
      _running = false;
      await _close();
    }
  }

  /// Tags every following report with the hashed [instanceId] (never the ID itself).
  void setInstance(String instanceId) => _instanceTag = instanceTag(instanceId);

  /// Sets the options of a start: target, release, environment and everything off that could
  /// carry content (Design note 4).
  FutureOr<void> configure(SentryFlutterOptions o) {
    configureCommon(o, dsn: dsn, release: release, environment: environment);
    o
      ..attachScreenshot = false
      ..attachViewHierarchy = false
      ..enableUserInteractionBreadcrumbs = false
      ..enableUserInteractionTracing = false
      ..enableAutoNativeBreadcrumbs = false
      ..reportPackages = false;
    o.replay
      ..sessionSampleRate = 0
      ..onErrorSampleRate = 0;
    // The instance is known only after the login: the tag is read when a report leaves.
    o.beforeSend = (event, hint) {
      final scrubbed = scrubEvent(event, hint);
      final tag = _instanceTag;
      if (scrubbed != null && tag != null) {
        scrubbed.tags = {...?scrubbed.tags, 'instance': tag};
      }
      return scrubbed;
    };
  }

  /// The part of the options the Dart SDK has (`SentryOptions`), shared with the canary test,
  /// which runs the Dart pipeline with a recording transport.
  static void configureCommon(
    SentryOptions o, {
    required String dsn,
    required String release,
    required String environment,
  }) {
    o
      ..dsn = dsn
      ..release = release
      ..environment = environment
      ..sendDefaultPii = false
      ..tracesSampleRate = null
      ..beforeSend = scrubEvent
      ..beforeBreadcrumb = scrubBreadcrumb
      ..beforeSendTransaction = (transaction, hint) => null;
  }

  /// The tag value of [instanceId]: the first 16 hex characters of its SHA-256.
  static String instanceTag(String instanceId) =>
      sha256.convert(utf8.encode(instanceId)).toString().substring(0, 16);

  /// Removes every content from [event] (Design note 4) and keeps the technical rest.
  static SentryEvent? scrubEvent(SentryEvent event, Hint hint) {
    event
      ..message = null
      ..culprit = null
      ..user = null
      ..request = null
      ..serverName = null
      ..modules = null
      // Deprecated in the SDK, but a plugin may still fill it with content; cleared all the
      // same.
      // ignore: deprecated_member_use
      ..extra = null
      ..transaction = event.transaction == null
          ? null
          : scrubUrl(event.transaction!)
      ..tags = {if (event.tags?['instance'] case final String t) 'instance': t};
    for (final e in event.exceptions ?? const <SentryException>[]) {
      e.value = null;
      _scrubFrames(e.stackTrace);
    }
    for (final t in event.threads ?? const <SentryThread>[]) {
      t.name = null;
      _scrubFrames(t.stacktrace);
    }
    // An allow-list: a context set by a plugin or the app (any name) could carry content.
    event.contexts.removeWhere((key, _) => !technicalContexts.contains(key));
    event.contexts.device?.name = null;
    // The screen names of the app context are routes, which may carry IDs.
    event.contexts.app?.viewNames = null;
    final crumbs = event.breadcrumbs;
    if (crumbs != null) {
      event.breadcrumbs = [for (final b in crumbs) ?scrubBreadcrumb(b, Hint())];
    }
    return event;
  }

  /// The contexts a report keeps, each technical only: app, device (without its name),
  /// operating system, runtimes, GPU, trace and the Dart and Flutter build modes.
  static const Set<String> technicalContexts = {
    'app',
    'device',
    'os',
    'runtime',
    'runtimes',
    'gpu',
    'trace',
    'dart_context',
    'flutter_context',
  };

  /// The members of a breadcrumb's data that stay, each without content: HTTP method and
  /// status, the URL and the navigation targets (scrubbed by [scrubUrl]), the lifecycle state.
  static const Set<String> breadcrumbData = {
    'method',
    'status_code',
    'url',
    'from',
    'to',
    'state',
  };

  /// Removes every content from a breadcrumb: its message and every data member outside
  /// [breadcrumbData] (headers, bodies, labels, texts); URLs and routes through [scrubUrl].
  static Breadcrumb? scrubBreadcrumb(Breadcrumb? b, Hint hint) {
    if (b == null) return null;
    b.message = null;
    final data = b.data;
    if (data != null) {
      b.data = {
        for (final e in data.entries)
          if (breadcrumbData.contains(e.key))
            e.key: switch (e.key) {
              'url' || 'from' || 'to' => scrubUrl('${e.value}'),
              _ => e.value is num || e.value is bool ? e.value : '${e.value}',
            },
      };
      for (final key in const ['method', 'state']) {
        final v = b.data![key];
        if (v is String && !RegExp(r'^[A-Za-z_]{1,16}$').hasMatch(v)) {
          b.data!.remove(key);
        }
      }
    }
    return b;
  }

  static final RegExp _uuid = RegExp(
    r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
  );
  static final RegExp _token = RegExp(r'^(?=.*\d)[A-Za-z0-9_\-]{16,}$');

  /// [url] without user info, query and fragment, with every path segment that looks like an
  /// ID or a token (a UUID, 16 or more characters with a digit) or a file name (a dot, a space
  /// or an escape) replaced by `:id`. An unreadable URL becomes "".
  static String scrubUrl(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null) return '';
    final segments = [
      for (final s in uri.pathSegments)
        _uuid.hasMatch(s) ||
                _token.hasMatch(s) ||
                s.contains('.') ||
                s.contains(' ') ||
                s.contains('%')
            ? ':id'
            : s,
    ];
    final path = (uri.path.startsWith('/') ? '/' : '') + segments.join('/');
    if (!uri.hasAuthority) return path;
    return Uri(
          scheme: uri.scheme,
          host: uri.host,
          port: uri.hasPort ? uri.port : null,
        ).toString() +
        path;
  }

  static void _scrubFrames(SentryStackTrace? trace) {
    for (final f in trace?.frames ?? const <SentryStackFrame>[]) {
      final abs = f.absPath;
      final code =
          abs != null &&
          (abs.startsWith('package:') || abs.startsWith('dart:'));
      if (!code) {
        f
          ..absPath = null
          ..fileName = null;
      }
      // The client SDK has no source, so it fills no pre or post context and no variables;
      // the context line is cleared all the same.
      f.contextLine = null;
    }
  }
}

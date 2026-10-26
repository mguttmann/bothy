// SPDX-License-Identifier: Apache-2.0

/// The app widget: theme, languages, router and deep links (docs/spec/15-client-architecture.md
/// 4 and 11).
library;

import 'dart:async';
import 'dart:ui' show AppExitResponse;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../admin/admin_route.dart';
import '../desktop/providers.dart';
import '../keyboard/menu_bar.dart';
import '../lock/app_lock_controller.dart';
import '../lock/lock_screen.dart';
import '../lock/providers.dart';
import '../notifications/providers.dart';
import '../notifications/push_provider.dart';
import '../onboarding/onboarding_controller.dart';
import '../onboarding/onboarding_screen.dart';
import '../phone/phone_shell.dart';
import '../phone/privacy_cover.dart';
import '../platform/lifecycle.dart';
import '../settings/settings_providers.dart';
import '../settings/settings_shell.dart';
import '../share/providers.dart';
import '../share/share_route.dart';
import '../status/status_route.dart';
import '../telemetry/telemetry_providers.dart';
import 'deep_links.dart';
import 'form_factor.dart';
import 'layout/desktop_shell.dart';
import 'providers/ports.dart';
import 'providers/session.dart';
import 'providers/settings.dart';
import 'router.dart';

/// The app on the desktop and on the phone.
///
/// Builds one [GoRouter] whose redirect follows the [OnboardingController] (`redirectFor`):
/// signed out, every content path leads to the current onboarding screen and is kept until the
/// login; signed in, the main view shows the path: the columns of [DesktopShell] on the
/// desktop, the tabs and stacks of [PhoneShell] on the phone (the same paths of 15 11, the
/// layout by the form factor of the build; brief M5-02 Design note 2). Links of the URL scheme
/// from the runner ([DeepLinkSource]) go to the router. The window title is the product name of
/// the instance after discovery and the build's app name before (fixed requirement 20). On the
/// phone the privacy cover hides the content while the app is not in front (01 5.6 E6), and
/// the app lock of M5-08 sends every route to E6 while it is closed.
class ClientApp extends ConsumerStatefulWidget {
  /// Creates the app.
  const ClientApp({super.key});

  @override
  ConsumerState<ClientApp> createState() => _ClientAppState();
}

class _ClientAppState extends ConsumerState<ClientApp> {
  final ValueNotifier<OnboardingStep> _step = ValueNotifier(
    OnboardingStep.server,
  );
  late final GoRouter _router;
  StreamSubscription<Uri>? _links;
  StreamSubscription<String>? _notificationTargets;
  StreamSubscription<String>? _pushTargets;
  StreamSubscription<Object?>? _shares;
  String? _kept;
  late final FormFactor _formFactor = ref.read(appPortsProvider).formFactor;

  /// The privacy cover of the phone; the desktop has none (a window without focus stays
  /// visible and shows its content).
  late final PrivacyCover? _cover = _formFactor.isPhone ? PrivacyCover() : null;

  /// The app lock of the signed-in phone, or null (brief M5-08).
  AppLockController? _lock;

  /// Sends the collected app metrics before the app quits (brief M4-31 Design note 2).
  late final AppLifecycleListener _exit = AppLifecycleListener(
    onExitRequested: () async {
      await ref.read(metricsSenderProvider)?.flush();
      return AppExitResponse.exit;
    },
  );

  @override
  void initState() {
    super.initState();
    _syncStep(rebuild: false);
    ref.listenManual(appLockProvider, (_, _) => _syncStep());
    HardwareKeyboard.instance.addHandler(_onKey);
    _router = GoRouter(
      // The navigator of the ports: the step-up dialog of the auth session opens over it.
      navigatorKey: ref.read(appPortsProvider).navigatorKey,
      initialLocation: _step.value == OnboardingStep.signedIn
          ? '/'
          : _step.value.path,
      refreshListenable: _step,
      redirect: (context, state) => redirectFor(
        state.uri,
        _step.value,
        keep: (path) => _kept = path,
        takeKept: () {
          final kept = _kept;
          _kept = null;
          return kept;
        },
      ),
      routes: _formFactor.isPhone ? _phoneRoutes() : _desktopRoutes(),
      // Every unknown path: the shell parses it and shows "Not found or no access"
      // (01 6.7).
      errorPageBuilder: _formFactor.isPhone ? _phonePage : null,
      errorBuilder: _formFactor.isPhone ? null : _shell,
    );
    _exit;
    _cover;
    // A clicked notification opens its thread at the message through the router (15 11,
    // brief M4-30 Design note 2).
    _notificationTargets = ref
        .read(desktopNotificationsProvider)
        .targets
        .listen((path) {
          if (!mounted) return;
          // The window may be hidden in the notification area; the click brings it back.
          unawaited(ref.read(windowBehaviorProvider).open());
          _router.go(path, extra: const ExternalLink());
        });
    // iOS, macOS (M5-04) and Android (M5-05): a tapped push opens its thread at the message
    // (15 11); the first thread the person opens is the moment to ask for the notification
    // permission.
    final push = ref.read(pushPlatformProvider);
    if (push != null) {
      _pushTargets = push.targets.listen((path) {
        if (mounted) _router.go(path, extra: const ExternalLink());
      });
      unawaited(push.listen());
      _router.routerDelegate.addListener(_onRoute);
    }
    // Android (M5-06): a share from another app opens the target sheet; the share waits there
    // until the person sends or cancels it, also through a login.
    final android = ref.read(appPortsProvider).androidShare;
    if (android != null) {
      _shares = android.shares.listen((content) {
        ref.read(pendingShareProvider.notifier).put(content);
        if (mounted) _router.go('/share');
      });
      unawaited(android.listen());
    }
    final deepLinks = ref.read(appPortsProvider).deepLinks;
    _links = deepLinks.links.listen(_open);
    unawaited(
      deepLinks.takePending().then((links) {
        for (final link in links) {
          _open(link);
        }
      }),
    );
  }

  /// The onboarding screens. E6 comes without a page transition: a transition would keep the
  /// content it replaces painted while it runs (brief M5-08, fixed requirement 12).
  GoRoute _onboardingRoute() => GoRoute(
    path: '/onboarding/:step',
    pageBuilder: (context, state) {
      final location = parseLocation(state.uri);
      final step = location is OnboardingLocation
          ? location.step
          : OnboardingStep.server;
      final screen = OnboardingScreen(step: step);
      return step == OnboardingStep.locked
          ? NoTransitionPage<void>(key: state.pageKey, child: screen)
          : MaterialPage<void>(key: state.pageKey, child: screen);
    },
  );

  /// The phone: every content path, the settings and the administration show the one
  /// [PhoneShell] under one page key, so the shell and its tabs stay while the path changes.
  List<RouteBase> _phoneRoutes() => [
    _onboardingRoute(),
    for (final path in const [
      '/',
      '/s/:id',
      '/c/:id',
      '/t/:id',
      '/running',
      '/running/:id',
      '/mentions',
      '/search',
      '/settings',
      '/settings/:section',
      '/status',
      '/admin',
      '/admin/:section',
      '/admin/:section/:id',
    ])
      GoRoute(path: path, pageBuilder: _phonePage),
    GoRoute(
      path: '/share',
      pageBuilder: (context, state) =>
          const NoTransitionPage<void>(child: ShareRoute()),
    ),
  ];

  Page<void> _phonePage(BuildContext context, GoRouterState state) =>
      NoTransitionPage<void>(
        key: const ValueKey('phone.shell.page'),
        child: PhoneShell(
          location: state.uri,
          external: state.extra is ExternalLink,
        ),
      );

  List<RouteBase> _desktopRoutes() => [
    GoRoute(path: '/', builder: _shell),
    _onboardingRoute(),
    GoRoute(path: '/share', builder: (context, state) => const ShareRoute()),
    GoRoute(
      path: '/settings',
      redirect: (context, state) => SettingsSection.appearance.path,
    ),
    GoRoute(
      path: '/settings/:section',
      builder: (context, state) =>
          SettingsShell(segment: state.pathParameters['section']),
    ),
    // The status page of 16 12 (M7-21).
    GoRoute(
      path: '/status',
      builder: (context, state) => const DesktopStatusRoute(),
    ),
    // The administration of admin_ui (brief M4-26): the shell parses the rest of the
    // path and shows "Not found or no access" for a path without a screen.
    for (final path in const [
      '/admin',
      '/admin/:section',
      '/admin/:section/:id',
    ])
      GoRoute(
        path: path,
        builder: (context, state) => AdminRoute(uri: state.uri),
      ),
    for (final path in const [
      '/s/:id',
      '/c/:id',
      '/t/:id',
      '/running',
      '/running/:id',
      '/mentions',
      '/search',
      // The terminal view of a host and the hint without one (01 D11, M4-36).
      '/hosts',
      '/hosts/:id',
    ])
      GoRoute(path: path, builder: _shell),
  ];

  Widget _shell(BuildContext context, GoRouterState state) =>
      DesktopShell(location: state.uri);

  void _onRoute() {
    final path = _router.routerDelegate.currentConfiguration.uri.path;
    if (path.startsWith('/t/')) {
      unawaited(ref.read(pushPlatformProvider)?.threadOpened());
    }
  }

  /// The onboarding step for the router: E6 while the lock of the signed-in phone is closed
  /// (`routeStep`); follows the lock's changes. A new lock rebuilds the app at once, so a lock
  /// that is still deciding at a cold start covers the content in the same frame as the
  /// signed-in route ([rebuild] is false only in `initState`, before the first build).
  void _syncStep({bool rebuild = true}) {
    final lock = ref.read(appLockProvider);
    final changed = !identical(lock, _lock);
    if (changed) {
      _lock?.removeListener(_onLock);
      _lock = lock?..addListener(_onLock);
    }
    _step.value = routeStep(ref.read(onboardingControllerProvider).step, lock);
    if (changed && rebuild && mounted) setState(() {});
  }

  void _onLock() {
    _syncStep();
    if (mounted) setState(() {});
  }

  /// A key event is activity of the person for the app lock (Design note 2).
  bool _onKey(KeyEvent event) {
    _lock?.recordActivity();
    return false;
  }

  void _open(Uri link) {
    final ports = ref.read(appPortsProvider);
    final path = pathOfExternalLink(
      link,
      ports.branding.urlScheme,
      // Only an Android build declares App Links of its public domain (D-M5-03-2).
      appLinksHost: ports.platform == 'android'
          ? ports.branding.associatedDomain
          : '',
    );
    if (path != null && mounted) _router.go(path, extra: const ExternalLink());
  }

  @override
  void dispose() {
    unawaited(_links?.cancel());
    unawaited(_notificationTargets?.cancel());
    unawaited(_pushTargets?.cancel());
    unawaited(_shares?.cancel());
    if (_pushTargets != null) _router.routerDelegate.removeListener(_onRoute);
    HardwareKeyboard.instance.removeHandler(_onKey);
    _lock?.removeListener(_onLock);
    _exit.dispose();
    _cover?.dispose();
    _router.dispose();
    _step.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(
      onboardingControllerProvider.select((s) => s.step),
      (_, _) => _syncStep(),
    );
    // The sender while signed in and the crash path by the privacy switches (M4-31).
    ref.watch(appTelemetryProvider);
    // The window in the notification area and the tray icon on Windows (M4-30).
    ref.watch(desktopShellProvider);
    // Push on iOS, macOS and Android follows the signed-in account (M5-04, M5-05).
    ref.watch(pushPlatformProvider);
    // Sharing on iOS and macOS follows the signed-in account (M5-06).
    ref.watch(appleShareProvider);
    final instance = ref.watch(currentInstanceProvider);
    final branding = ref.watch(appPortsProvider).branding;
    final media = MediaQuery.maybeOf(context);
    final dsContext = ref.watch(
      appearanceProvider(
        media?.platformBrightness ?? Brightness.light,
        media?.disableAnimations ?? false,
      ),
    );
    // 01 9: the language follows `users.locale`; without one the system language applies.
    final userLocale = ref.watch(profileControllerProvider).value?.locale;
    return MaterialApp.router(
      locale: userLocale == null
          ? null
          : resolveAppLocale(userLocale: userLocale, systemLocales: const []),
      title: instance?.productName ?? branding.appName,
      debugShowCheckedModeBanner: false,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      localeListResolutionCallback: (locales, supported) =>
          resolveAppLocale(systemLocales: locales ?? const []),
      theme: buildDsTheme(dsContext),
      routerConfig: _router,
      // M4-24: one menu bar of macOS for the whole app; its items run in the shell on screen.
      builder: (context, child) {
        // One visible focus ring for every control at one place (01 8, WCAG 2.4.7).
        final content = DsFocusRing(
          child: AppMenuBar(
            appName: instance?.productName ?? branding.appName,
            child: child ?? const SizedBox.shrink(),
          ),
        );
        final cover = _cover;
        return FormFactorScope(
          formFactor: _formFactor,
          child: DsThemeScope(
            dsContext: dsContext,
            child: cover == null
                ? content
                : PrivacyCoverHost(
                    cover: cover,
                    productName: instance?.productName ?? branding.appName,
                    logo: instance?.logo,
                    hidden: lockHidesContent(_lock),
                    // Every pointer event is activity of the person for the app lock.
                    child: Listener(
                      behavior: HitTestBehavior.translucent,
                      onPointerDown: (_) => _lock?.recordActivity(),
                      onPointerMove: (_) => _lock?.recordActivity(),
                      onPointerSignal: (_) => _lock?.recordActivity(),
                      child: content,
                    ),
                  ),
          ),
        );
      },
    );
  }
}

// SPDX-License-Identifier: Apache-2.0

/// The app widget: theme, languages, router and deep links (docs/spec/15-client-architektur.md
/// 4 and 11).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../admin/admin_route.dart';
import '../onboarding/onboarding_controller.dart';
import '../onboarding/onboarding_screen.dart';
import '../settings/settings_providers.dart';
import '../settings/settings_shell.dart';
import 'deep_links.dart';
import 'layout/desktop_shell.dart';
import 'providers/ports.dart';
import 'providers/session.dart';
import 'providers/settings.dart';
import 'router.dart';

/// The desktop app.
///
/// Builds one [GoRouter] whose redirect follows the [OnboardingController] (`redirectFor`):
/// signed out, every content path leads to the current onboarding screen and is kept until the
/// login; signed in, the main view ([DesktopShell]) shows the path. Links of the URL scheme
/// from the runner ([DeepLinkSource]) go to the router. The window title is the product name of
/// the instance after discovery and the build's app name before (fixed requirement 20).
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
  String? _kept;

  @override
  void initState() {
    super.initState();
    _step.value = ref.read(onboardingControllerProvider).step;
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
      routes: [
        GoRoute(path: '/', builder: _shell),
        GoRoute(
          path: '/onboarding/:step',
          builder: (context, state) {
            final location = parseLocation(state.uri);
            return OnboardingScreen(
              step: location is OnboardingLocation
                  ? location.step
                  : OnboardingStep.server,
            );
          },
        ),
        GoRoute(
          path: '/settings',
          redirect: (context, state) => SettingsSection.appearance.path,
        ),
        GoRoute(
          path: '/settings/:section',
          builder: (context, state) =>
              SettingsShell(segment: state.pathParameters['section']),
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
          '/mentions',
          '/search',
        ])
          GoRoute(path: path, builder: _shell),
      ],
      // Every unknown path: the shell parses it and shows "Not found or no access"
      // (01 6.7).
      errorBuilder: _shell,
    );
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

  Widget _shell(BuildContext context, GoRouterState state) =>
      DesktopShell(location: state.uri);

  void _open(Uri link) {
    final path = pathOfExternalLink(
      link,
      ref.read(appPortsProvider).branding.urlScheme,
    );
    if (path != null && mounted) _router.go(path);
  }

  @override
  void dispose() {
    unawaited(_links?.cancel());
    _router.dispose();
    _step.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(
      onboardingControllerProvider.select((s) => s.step),
      (_, step) => _step.value = step,
    );
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
      builder: (context, child) => DsThemeScope(
        dsContext: dsContext,
        child: child ?? const SizedBox.shrink(),
      ),
    );
  }
}

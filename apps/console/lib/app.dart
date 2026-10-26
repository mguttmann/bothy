// SPDX-License-Identifier: Apache-2.0

/// The console app: theme from `design_system`, texts from `l10n`, and a router whose only
/// configuration is [ConsolePage] (brief M6-02 Design notes 1 and 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'admin/admin_page.dart';
import 'admin/console_admin.dart';
import 'admin/console_route.dart';
import 'admin/step_up_dialog.dart';
import 'login/logged_out_view.dart';
import 'login/login_view.dart';
import 'login/public_status_page.dart';
import 'login/step_up_view.dart';
import 'ops/grafana_page.dart';
import 'platform/browser.dart';
import 'routes.dart';
import 'session/console_controller.dart';
import 'shell/console_shell.dart';
import 'shell/status_view.dart';
import 'terminal/console_terminals.dart';
import 'terminal/hosts_page.dart';
import 'terminal/pin_store.dart';
import 'terminal/ports.dart';
import 'terminal/prompts.dart';
import 'terminal/step_up_prompt.dart';
import 'viz/console_live_page.dart';

/// The root widget of the console.
class ConsoleApp extends StatefulWidget {
  /// Creates the app for [environment]. [initialLocation] replaces the browser's location (the
  /// path below `/console` with its query) and [locale] the browser's language; both for tests.
  /// [terminalPorts] are the browser seams of the terminal (WebSocket, IndexedDB of the pins,
  /// new tab); null takes those of the platform ([ConsoleTerminalPorts.browser]).
  const ConsoleApp({
    super.key,
    required this.environment,
    this.initialLocation,
    this.locale,
    this.terminalPorts,
  });

  /// The browser ports and the origin.
  final ConsoleEnvironment environment;

  /// The browser seams of the terminal, or null for those of the platform.
  final ConsoleTerminalPorts? terminalPorts;

  /// The location to start at, or null for the browser's.
  final Uri? initialLocation;

  /// The language, or null to follow the browser.
  final Locale? locale;

  /// The controller of the console that [context] belongs to.
  static ConsoleController controllerOf(BuildContext context) =>
      context.findAncestorStateOfType<_ConsoleAppState>()!._controller;

  /// The administration state of the console that [context] belongs to.
  static ConsoleAdmin adminOf(BuildContext context) =>
      context.findAncestorStateOfType<_ConsoleAppState>()!._admin;

  @override
  State<ConsoleApp> createState() => _ConsoleAppState();
}

class _ConsoleAppState extends State<ConsoleApp> {
  late final ConsoleTerminalPorts _ports =
      widget.terminalPorts ?? ConsoleTerminalPorts.browser();
  // The pins of the terminal live in IndexedDB of the origin; the controller deletes them before
  // E7 (brief M6-18 Design note 3).
  late final ConsoleController _controller = ConsoleController(
    widget.environment,
    terminalPins: IndexedDbHostKeyPinStore(_ports.pins),
  );
  late final ConsoleAdmin _admin = ConsoleAdmin(_controller);
  late final ConsoleTerminals _terminals = ConsoleTerminals(
    console: _controller,
    ports: _ports,
    prompts: (me) => ConsoleTerminalPrompts(_navigatorContext, selfUserId: me),
    stepUp: PasskeyStepUpPrompt(
      api: () => _controller.api,
      passkeys: widget.environment.passkeys,
      methods: () => _controller.sessionStepUpMethods,
      prompt: () => _controller.stepUpPrompt,
      showSetupNotice: DialogPasskeySetupNotice(_navigatorContext).show,
    ),
  );
  late final _ConsoleRouterDelegate _delegate = _ConsoleRouterDelegate(
    _controller,
    _admin,
    _terminals,
  );

  BuildContext? _navigatorContext() => _delegate.navigatorKey.currentContext;
  late final PlatformRouteInformationProvider? _provider =
      widget.initialLocation == null
      ? null
      : PlatformRouteInformationProvider(
          initialRouteInformation: RouteInformation(
            uri: widget.initialLocation,
          ),
        );

  @override
  void initState() {
    super.initState();
    // The step-up of an action shows its dialog over the navigator of the router.
    _controller.stepUpPrompt = DialogStepUpPrompt(
      () => _delegate.navigatorKey.currentContext,
    );
  }

  @override
  void dispose() {
    _delegate.dispose();
    _provider?.dispose();
    _terminals.dispose();
    _admin.dispose();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: _controller,
      builder: (context, _) {
        final branding = _controller.discovery?.branding;
        final platform = MediaQuery.platformBrightnessOf(context);
        DsContext dsFor(Brightness brightness) => resolveDsContext(
          appearanceTheme: null,
          appearanceDensity: null,
          appearanceReduceMotion: null,
          platformBrightness: brightness,
          platformReduceMotion: MediaQuery.disableAnimationsOf(context),
          isTouchPlatform: false,
          accentKey: branding?.accentColor,
        );
        final locale =
            widget.locale ??
            resolveAppLocale(
              systemLocales: WidgetsBinding.instance.platformDispatcher.locales,
            );
        return MaterialApp.router(
          debugShowCheckedModeBanner: false,
          onGenerateTitle: (context) =>
              AppLocalizations.of(context)
                  .consoleTitle(branding?.productName ?? ''),
          locale: locale,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          theme: buildDsTheme(dsFor(Brightness.light)),
          darkTheme: buildDsTheme(dsFor(Brightness.dark)),
          themeMode: platform == Brightness.dark
              ? ThemeMode.dark
              : ThemeMode.light,
          routerDelegate: _delegate,
          routeInformationParser: const _ConsoleRouteParser(),
          routeInformationProvider: _provider,
          builder: (context, child) {
            // The language of the document follows the app language, so screen readers read
            // the semantics tree in it (brief M6-02 Design note 3).
            widget.environment.navigation.setDocumentLanguage(
              locale.toLanguageTag(),
            );
            return child ?? const SizedBox.shrink();
          },
        );
      },
    );
  }
}

/// Route information is the location below `/console` as it is.
class _ConsoleRouteParser extends RouteInformationParser<Uri> {
  const _ConsoleRouteParser();

  @override
  Future<Uri> parseRouteInformation(RouteInformation routeInformation) =>
      SynchronousFuture(routeInformation.uri);

  @override
  RouteInformation restoreRouteInformation(Uri configuration) =>
      RouteInformation(uri: configuration);
}

/// Shows the page of the controller's state and reports its path to the browser; a path the
/// person enters only counts when the state allows it (an administration page needs a step-up).
class _ConsoleRouterDelegate extends RouterDelegate<Uri>
    with ChangeNotifier, PopNavigatorRouterDelegateMixin<Uri> {
  _ConsoleRouterDelegate(this.controller, this.admin, this.terminals) {
    _changes.addListener(notifyListeners);
  }

  final ConsoleController controller;

  final ConsoleAdmin admin;

  final ConsoleTerminals terminals;

  late final Listenable _changes = Listenable.merge([controller, admin]);

  @override
  final GlobalKey<NavigatorState> navigatorKey = GlobalKey<NavigatorState>();

  @override
  Uri? get currentConfiguration {
    final path = controller.path;
    // While starting, the browser keeps its location (the callback query is still needed).
    if (controller.state is ConsoleStarting || path == null) return null;
    return Uri(path: path);
  }

  @override
  Future<void> setInitialRoutePath(Uri configuration) =>
      controller.start(configuration);

  @override
  Future<void> setNewRoutePath(Uri configuration) {
    final route = ConsoleRoute.parse(configuration.path);
    if (route != null) controller.openRoute(route);
    return SynchronousFuture(null);
  }

  @override
  void dispose() {
    _changes.removeListener(notifyListeners);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Navigator(
      key: navigatorKey,
      pages: [
        MaterialPage<void>(
          key: ValueKey(controller.page),
          child: Scaffold(body: _page(context)),
        ),
      ],
      onDidRemovePage: (_) {},
    );
  }

  Widget _page(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = controller;
    final product = c.discovery?.branding.productName ?? '';
    final oidc = [
      for (final p in c.providers)
        if (p.kind == ConsoleProviderKind.oidc) p,
    ];
    if (c.page == ConsolePage.publicStatus) {
      return PublicStatusPage(
        source: admin.publicStatusSource,
        onBack: c.leavePublicStatus,
      );
    }
    return switch (c.state) {
      ConsoleStarting() => StatusView(
        working:
            c.page == ConsolePage.loginCallback ||
                c.page == ConsolePage.stepUpCallback
            ? l.consoleCallbackWorking
            : null,
      ),
      ConsoleUnavailable() => StatusView(
        message: l.consoleLoginUnavailable,
        onReload: c.reload,
      ),
      ConsoleOutdated() => StatusView(
        message: l.consoleUpdatedReload,
        onReload: c.reload,
      ),
      ConsoleSignedOut() => LoginView(
        productName: product,
        providers: c.providers,
        localEnabled: c.localEnabled,
        passwordEnabled: c.passwordEnabled,
        notice: c.notice,
        busy: c.busy,
        onProvider: c.loginWithProvider,
        onPasskey: c.loginWithPasskey,
        onPassword: c.loginWithPassword,
        onPublicStatus: c.openPublicStatus,
      ),
      ConsoleNeedsStepUp() => StepUpView(
        methods: c.stepUpMethods,
        oidcProviders: oidc,
        notice: c.notice,
        busy: c.busy,
        onPasskey: c.stepUpWithPasskey,
        onOidc: c.stepUpWithOidc,
        onPassword: c.stepUpWithPassword,
      ),
      ConsoleLoggedOut(:final reasonCode) => LoggedOutView(
        reasonCode: reasonCode,
        onLogInAgain: c.logInAgain,
      ),
      ConsoleSignedIn(:final session) => ConsoleShell(
        productName: product,
        displayName: session.displayName,
        page: c.page!,
        onOpen: c.open,
        onLogout: c.logout,
        banner: ConsoleViewAsBanner(admin: admin),
        navigation: consoleNavigationFor(
          grafana: admin.grafanaEmbedUrl != null,
        ),
        body: switch (c.route.page) {
          // 25 9.2 (brief M6-17): the dashboard of grafana_embed_url, owners only.
          ConsolePage.grafana => ConsoleGrafanaPage(
            url: admin.grafanaEmbedUrl,
            links: c.environment.links,
          ),
          ConsolePage.hosts => ConsoleHostsPage(
            controller: c,
            admin: admin,
            terminals: terminals,
            route: c.route,
          ),
          // D12 and D13 (brief M6-16): the views with the frame polled from getStatsLive.
          ConsolePage.live => ConsoleLivePage(
            controller: c,
            admin: admin,
            links: c.environment.links,
            visibility: c.environment.visibility,
          ),
          _ => ConsoleAdminPage(
            controller: c,
            admin: admin,
            route: c.route,
            downloads: c.environment.downloads,
            files: c.environment.files,
            links: c.environment.links,
          ),
        },
      ),
    };
  }
}

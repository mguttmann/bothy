// SPDX-License-Identifier: Apache-2.0

/// The web console for administration, statistics and the terminal (ADR 0028, ADR 0070):
/// Flutter Web, served by the server under `/console/`, logged in with the cookie session of
/// M6-01 and a step-up directly after the login. No chat, no `sync_engine`, no `local_store`.
///
/// - [ConsoleApp], [runConsole]: the app and its start (semantics on).
/// - [ConsolePage], [consoleNavigation]: the route table.
/// - [ConsoleController]: the state of the session, the login and the step-up.
/// - [createConsoleApiClient]: the HTTP layer (cookie, CSRF header, no token).
/// - [ConsoleEnvironment] and its ports: the browser.
/// - [versionsMatch]: the version check against the server.
/// - [ConsoleAdmin], [ConsoleAdminPage], [ConsoleRoute]: the administration screens A1 to A6 of
///   `admin_ui` in the console, "View as person" with its banner, and the places below the pages
///   (brief M6-04).
/// - [ConsoleActionStepUp], [ConsoleStepUpInterceptor], [ConsoleStepUpDialog]: the step-up of
///   an action in the browser (brief M6-04 Design note 2).
library;

export 'admin/admin_page.dart';
export 'admin/console_admin.dart';
export 'admin/console_route.dart';
export 'admin/download_target.dart';
export 'admin/step_up.dart';
export 'admin/step_up_dialog.dart';
export 'app.dart';
export 'http/console_client.dart';
export 'login/logged_out_view.dart';
export 'login/login_view.dart';
export 'login/oidc_flow.dart';
export 'login/step_up_view.dart';
export 'login/webauthn_json.dart';
export 'platform/browser.dart';
export 'routes.dart';
export 'run.dart';
export 'session/console_controller.dart';
export 'shell/console_shell.dart';
export 'shell/thread_link.dart';
export 'version/version_check.dart';

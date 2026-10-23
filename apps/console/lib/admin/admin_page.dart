// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../platform/browser.dart';
import '../routes.dart';
import '../session/console_controller.dart';
import 'console_admin.dart';
import 'console_route.dart';
import 'download_target.dart';
import 'manifest_target.dart';

/// The page frame of an administration page of the console (brief M6-04 Scope): the screen of
/// `admin_ui` for the place of [route] at the full width of the browser (01 5.5), the tabs
/// "Groups" and "Access grants" on the groups page and "Apps" and "Incoming webhooks" on the
/// apps page (M6-08), the structure of "View as person", and the placeholder on a page whose
/// screen comes with a later ticket.
///
/// The screens are the desktop's ([AdminBody]); the console passes its repository, its time
/// zone, the browser download as the save target of the audit export, the browser's file input
/// as the manifest picker of the apps and its own entry to "View as person" (it opens the
/// structure page instead of the desktop's dialog).
///
/// Example: `ConsoleAdminPage(controller: c, admin: admin, route: c.route)`.
class ConsoleAdminPage extends StatelessWidget {
  /// Creates the page.
  const ConsoleAdminPage({
    super.key,
    required this.controller,
    required this.admin,
    required this.route,
    this.downloads,
    this.files,
  });

  /// The console's state; its places are changed through it.
  final ConsoleController controller;

  /// The administration state.
  final ConsoleAdmin admin;

  /// The place to show.
  final ConsoleRoute route;

  /// The browser download, or null (no audit export).
  final FileDownloads? downloads;

  /// The browser's file input, or null (no install or update of an app).
  final FileChooser? files;

  /// The tabs of a page that holds two sections, or null.
  static List<(AdminSection, String Function(AppLocalizations), String)>? _tabs(
    ConsolePage page,
  ) => switch (page) {
    ConsolePage.groups => [
      (
        AdminSection.groups,
        (l) => l.adminNavGroups,
        'console.groups.tab.groups',
      ),
      (
        AdminSection.grants,
        (l) => l.adminNavGrants,
        'console.groups.tab.grants',
      ),
    ],
    ConsolePage.apps => [
      (AdminSection.apps, (l) => l.adminNavApps, 'console.apps.tab.apps'),
      (
        AdminSection.webhooks,
        (l) => l.adminNavIncomingWebhooks,
        'console.apps.tab.incoming',
      ),
    ],
    _ => null,
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    if (route.viewAs) return ConsoleViewAsPage(admin: admin);
    final at = route.admin;
    if (at == null) {
      return Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Text(
          l.consolePlaceholderPage,
          style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
        ),
      );
    }
    final save = downloads;
    final chooser = files;
    final body = AdminBody(
      key: ValueKey('console.admin.${route.path}.${at.id}'),
      repository: admin.repository,
      location: at,
      maxWidth: null,
      timeZone: admin.timeZone,
      clock: admin.clock,
      saveTarget: save == null ? null : downloadSaveTarget(save),
      pickManifest: chooser == null ? null : manifestPickerOf(chooser),
      onNavigate: controller.openAdmin,
      onViewAs: (person) => unawaited(
        admin.startViewAs(
          person,
          purpose: l.stepUpPurposeViewAs(person.displayName),
        ),
      ),
    );
    final tabs = _tabs(route.page);
    if (tabs == null) return body;
    final pageKey = route.page == ConsolePage.groups ? 'groups' : 'apps';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(
            DsSpace.s24,
            DsSpace.s8,
            DsSpace.s24,
            0,
          ),
          child: Align(
            alignment: Alignment.centerLeft,
            child: SegmentedButton<AdminSection>(
              key: ValueKey('console.$pageKey.tabs'),
              showSelectedIcon: false,
              segments: [
                for (final (section, label, key) in tabs)
                  ButtonSegment(
                    value: section,
                    label: Text(label(l), key: ValueKey(key)),
                  ),
              ],
              // A detail (a group, an app) belongs to the first tab.
              selected: {
                tabs.any((t) => t.$1 == at.section)
                    ? at.section
                    : tabs.first.$1,
              },
              onSelectionChanged: (s) =>
                  controller.openAdmin(AdminLocation(s.first)),
            ),
          ),
        ),
        Expanded(child: body),
      ],
    );
  }
}

/// The structure of "View as person" in the console (07 9.2; brief M6-04 Design note 3): the
/// spaces and channels of the answer of `viewAsUser` and the "Not visible" list, as text only.
/// The console has no content routes (ADR 0028), so nothing opens a channel; the desktop app
/// keeps showing content read-only (M4-27).
class ConsoleViewAsPage extends StatelessWidget {
  /// Creates the page.
  const ConsoleViewAsPage({super.key, required this.admin});

  /// The administration state with the view.
  final ConsoleAdmin admin;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return ListenableBuilder(
      listenable: admin,
      builder: (context, _) {
        final failure = admin.failure;
        final text = failure == null ? null : adminFailure(l, failure)?.text;
        final name = admin.snapshot?.user.displayName ?? admin.viewName ?? '';
        return SingleChildScrollView(
          key: const ValueKey('console.viewAs.page'),
          padding: const EdgeInsets.all(DsSpace.s24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (admin.starting)
                const Align(
                  alignment: Alignment.centerLeft,
                  child: CircularProgressIndicator(
                    key: ValueKey('console.viewAs.starting'),
                  ),
                )
              else if (text != null)
                DsBanner(
                  key: const ValueKey('console.viewAs.failed'),
                  tone: DsTone.warning,
                  text: text,
                  liveRegion: true,
                )
              else if (admin.storedView != null || admin.snapshot != null)
                ViewAsStructure(
                  name: name,
                  snapshot: admin.snapshot,
                  notVisible: admin.notVisible,
                ),
            ],
          ),
        );
      },
    );
  }
}

/// The banner of the stored view on every page of the console (07 9.2: "The console shows a
/// banner with name and remaining time throughout"): name, remaining time (updated every second)
/// and "End view". When the end time passes, the console reads its session again and
/// the banner goes away.
class ConsoleViewAsBanner extends StatefulWidget {
  /// Creates the banner over [admin].
  const ConsoleViewAsBanner({super.key, required this.admin});

  /// The administration state with the stored view.
  final ConsoleAdmin admin;

  @override
  State<ConsoleViewAsBanner> createState() => _ConsoleViewAsBannerState();
}

class _ConsoleViewAsBannerState extends State<ConsoleViewAsBanner> {
  Timer? _tick;
  DateTime? _endedAt;

  void _check() {
    if (!mounted) return;
    final view = widget.admin.storedView;
    if (view != null &&
        !widget.admin.clock().isBefore(view.endsAt) &&
        _endedAt != view.endsAt) {
      // Once per view: the session read after the end removes it.
      _endedAt = view.endsAt;
      unawaited(widget.admin.viewEnded());
    }
    setState(() {});
  }

  @override
  void dispose() {
    _tick?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final admin = widget.admin;
    final view = admin.storedView;
    if (view == null) {
      // No timer without a view, so a console without one never wakes up every second.
      _tick?.cancel();
      _tick = null;
      return const SizedBox.shrink();
    }
    _tick ??= Timer.periodic(const Duration(seconds: 1), (_) => _check());
    return ViewAsBanner(
      key: const ValueKey('console.viewAs.banner'),
      name: view.displayName,
      endsAt: view.endsAt,
      now: admin.clock(),
      onEnd: admin.ending ? null : () => unawaited(admin.endViewAs()),
    );
  }
}

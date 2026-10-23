// SPDX-License-Identifier: Apache-2.0

/// What "Permission hints" and "Expiring soon" share: the scope filter (the instance or one
/// space), paging with "Load more", the row with the kind as icon and text, and the actions on
/// a grant: "Explain" opens "Who has access?" (A3), "Revoke" and "Extend" (07 9.4: a new grant
/// with a new reason, then the revocation of the old one; brief M6-07 Design note 4).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../access/who_has_access.dart';
import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../grants/grant_labels.dart';
import '../grants/grant_wizard.dart';
import '../grants/revoke_grant_dialog.dart';
import '../self/self_feedback.dart';
import '../shell/admin_routes.dart';
import 'hint_texts.dart';

/// The parameters both lists take.
///
/// [repository] serves the list and its actions; [onNavigate] opens another place of the
/// administration (an agent's page, a group); [timeZone] and [clock] show the ends; [onChanged]
/// runs after a change; [onViewAs] is handed to "Who has access?" (the console's view as a
/// person, brief M6-04 Design note 3; null keeps the dialog of the desktop).
abstract class AccessListView extends StatefulWidget {
  /// Creates the list.
  const AccessListView({
    super.key,
    required this.repository,
    required this.onNavigate,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
    this.onViewAs,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens another place of the administration.
  final ValueChanged<AdminLocation> onNavigate;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a change, or null.
  final VoidCallback? onChanged;

  /// Starts the view as a person in the host, or null for the dialog of [WhoHasAccess].
  final AdminViewAs? onViewAs;
}

/// One page as a list loads it.
typedef AccessListPage<I> = ({
  List<I> items,
  AccessPageLabels labels,
  String? nextCursor,
});

/// The loading, the filter of the scope and the grant actions of a list of items [I].
///
/// The state asks [fetch] for the first page whenever the scope changes ([reload]) and for the
/// next page with the cursor of the last one ([loadMore]); the labels of every page are joined,
/// so a later page may name what an earlier one named. An answer for an older request is
/// dropped. A refused list keeps the filter, so a manager can choose a space after the server
/// refused the instance (07 2.6). Nothing here decides a permission (15 1 principle 1).
mixin AccessListState<W extends AccessListView, I> on AdminActionState<W> {
  /// The spaces the scope filter offers next to the instance (from `targets`), or null while
  /// they load.
  List<AdminTarget>? spaces;

  /// The chosen space, or null for the instance.
  String? spaceId;

  /// The loaded items, or null while the first page loads or after a refusal.
  List<I>? items;

  /// The names of every loaded page.
  AccessPageLabels labels = const AccessPageLabels();

  /// The cursor of the next page, or null after the last one.
  String? nextCursor;

  /// The server's problem with the list, or null.
  ApiProblem? problem;

  /// Whether a next page loads.
  bool loadingMore = false;

  int _generation = 0;

  /// One page of the list after [cursor] in the current scope.
  Future<AccessListPage<I>> fetch(String? cursor);

  @override
  void initState() {
    super.initState();
    _loadSpaces();
    reload();
  }

  Future<void> _loadSpaces() async {
    List<AdminTarget> found;
    try {
      found = [
        for (final t in await widget.repository.targets())
          if (t.kind == AdminTargetKind.space) t,
      ];
    } on Object {
      // Without the spaces the filter offers the instance only.
      found = const [];
    }
    if (mounted) setState(() => spaces = found);
  }

  /// Loads the first page of the current scope again.
  Future<void> reload() async {
    final generation = ++_generation;
    setState(() {
      items = null;
      problem = null;
      nextCursor = null;
      labels = const AccessPageLabels();
    });
    try {
      final page = await fetch(null);
      if (!mounted || generation != _generation) return;
      setState(() {
        items = page.items;
        labels = page.labels;
        nextCursor = page.nextCursor;
      });
    } on Object catch (e) {
      if (!mounted || generation != _generation) return;
      setState(() => problem = loadProblem(e));
    }
  }

  /// Loads the page after the last one and appends it.
  Future<void> loadMore() async {
    final cursor = nextCursor;
    final loaded = items;
    if (cursor == null || loaded == null || loadingMore) return;
    final generation = _generation;
    setState(() => loadingMore = true);
    try {
      final page = await fetch(cursor);
      if (!mounted || generation != _generation) return;
      setState(() {
        items = [...loaded, ...page.items];
        labels = labels.merge(page.labels);
        nextCursor = page.nextCursor;
      });
    } on Object catch (e) {
      if (mounted && generation == _generation) {
        setState(() => problem = loadProblem(e));
      }
    } finally {
      if (mounted) setState(() => loadingMore = false);
    }
  }

  /// The filter of the scope under the key [prefix] (`<prefix>.space`): the whole instance or
  /// one space; a choice loads the list again.
  Widget scopeFilter(String prefix) {
    final l = AppLocalizations.of(context);
    final choices = spaces ?? const <AdminTarget>[];
    final value = choices.any((s) => s.id == spaceId) ? spaceId! : '';
    return filterField(
      l.hintsScopeLabel,
      DropdownButton<String>(
        key: ValueKey('$prefix.space'),
        isExpanded: true,
        value: value,
        items: [
          DropdownMenuItem(
            key: ValueKey('$prefix.space.instance'),
            value: '',
            child: Text(l.hintsScopeInstance),
          ),
          for (final s in choices)
            DropdownMenuItem(
              key: ValueKey('$prefix.space.${s.id}'),
              value: s.id,
              child: Text(targetLabel(l, s)),
            ),
        ],
        onChanged: (id) {
          final next = id == null || id.isEmpty ? null : id;
          if (next == spaceId) return;
          spaceId = next;
          reload();
        },
      ),
    );
  }

  /// A filter with its [label] above the [field], at the full width of the list, so a long
  /// choice wraps instead of overflowing at a large text scale.
  Widget filterField(String label, Widget field) {
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          label,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        field,
      ],
    );
  }

  /// The title [title] with the explanation [intro] below it.
  Widget heading(String title, String intro) {
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            title,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s4),
        Text(
          intro,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
      ],
    );
  }

  /// The list under the key [prefix]: the server's refusal, the progress, [empty] for no item,
  /// or one [row] per item with "Load more" (`<prefix>.more`) while a next page exists.
  List<Widget> listBody(
    String prefix, {
    required String empty,
    required Widget Function(I item) row,
  }) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final loaded = items;
    final refused = problem;
    if (refused != null) {
      return [ProblemBanner(problem: refused, onRetry: reload)];
    }
    if (loaded == null) {
      return [const Center(child: CircularProgressIndicator())];
    }
    if (loaded.isEmpty) {
      return [
        Text(
          empty,
          key: ValueKey('$prefix.empty'),
          style: dsTextStyle(DsType.body).copyWith(color: colors.textSecondary),
        ),
      ];
    }
    return [
      for (final item in loaded) ...[
        row(item),
        Divider(height: 1, color: colors.borderSubtle),
      ],
      if (nextCursor != null) ...[
        const SizedBox(height: DsSpace.s12),
        Align(
          alignment: Alignment.centerLeft,
          child: OutlinedButton(
            key: ValueKey('$prefix.more'),
            onPressed: loadingMore ? null : loadMore,
            child: Text(l.adminLoadMore),
          ),
        ),
      ],
    ];
  }

  /// One row under [key]: the icon [glyph] (decorative, because [kind] next to it says the
  /// same), the [kind], the [lines], an optional [detail] (the end) and [note], and the
  /// [actions].
  Widget accessRow({
    required String key,
    required DsGlyph glyph,
    required String kind,
    required List<String> lines,
    String? detail,
    String? note,
    List<Widget> actions = const [],
  }) {
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Padding(
      key: ValueKey(key),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          DsIcon(
            glyph,
            key: ValueKey('$key.icon'),
            decorative: true,
            color: colors.textSecondary,
          ),
          const SizedBox(width: DsSpace.s12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(kind, style: secondary),
                for (final line in lines)
                  Text(
                    line,
                    style: dsTextStyle(DsType.body)
                        .copyWith(color: colors.textPrimary),
                  ),
                if (detail != null) Text(detail, style: secondary),
                if (note != null) Text(note, style: secondary),
                if (actions.isNotEmpty)
                  Wrap(spacing: DsSpace.s8, children: actions),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// "Explain" under `<key>.explain` for [target]: only a space or a channel has
  /// "Who has access?"; null for any other target.
  Widget? explainAction(String key, AdminTarget target) {
    if (target.kind != AdminTargetKind.space &&
        target.kind != AdminTargetKind.channel) {
      return null;
    }
    return TextButton(
      key: ValueKey('$key.explain'),
      onPressed: () => explain(target),
      child: Text(AppLocalizations.of(context).hintsExplain),
    );
  }

  /// The actions on [grant] of the row [key]: "Explain", "Revoke" (hidden after a refusal)
  /// and "Extend" when the grant has an end.
  List<Widget> grantActions(String key, AdminGrant grant) {
    final l = AppLocalizations.of(context);
    return [
      ?explainAction(key, grantTarget(grant, labels)),
      if (!denied.contains('revoke.${grant.id}'))
        TextButton(
          key: ValueKey('$key.revoke'),
          onPressed: busy ? null : () => revoke(grant),
          child: Text(l.adminRevoke),
        ),
      if (grant.expiresAt != null)
        TextButton(
          key: ValueKey('$key.extend'),
          onPressed: busy ? null : () => extend(grant),
          child: Text(l.hintsExtend),
        ),
    ];
  }

  /// Opens "Who has access?" of [target] in a dialog (A3).
  Future<void> explain(AdminTarget target) {
    final l = AppLocalizations.of(context);
    final viewAs = widget.onViewAs;
    return showDialog<void>(
      context: context,
      builder: (dialog) => Dialog(
        key: const ValueKey('hints.explainDialog'),
        backgroundColor: dialog.dsColors.surfaceRaised,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 760),
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(DsSpace.s24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                WhoHasAccess(
                  repository: widget.repository,
                  target: target,
                  timeZone: widget.timeZone,
                  clock: widget.clock,
                  onChanged: () {
                    widget.onChanged?.call();
                    reload();
                  },
                  // The host's view leaves this page, so the dialog closes first.
                  onViewAs: viewAs == null
                      ? null
                      : (person) {
                          Navigator.of(dialog).pop();
                          viewAs(person);
                        },
                ),
                const SizedBox(height: DsSpace.s12),
                Align(
                  alignment: Alignment.centerRight,
                  child: TextButton(
                    key: const ValueKey('hints.explainClose'),
                    onPressed: () => Navigator.of(dialog).pop(),
                    child: Text(l.hintsClose),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// Revokes [grant] after the confirmation with a reason ([RevokeGrantDialog]); it takes
  /// effect immediately (07 9.5). The list loads again.
  Future<void> revoke(AdminGrant grant) async {
    final l = AppLocalizations.of(context);
    final name = grantSubjectName(grant, labels);
    final reason = await RevokeGrantDialog.show(context, name: name);
    if (reason == null || !mounted) return;
    final ok = await runChange('revoke.${grant.id}', () async {
      await widget.repository.revokeGrant(
        grant.id,
        reason: reason,
        purpose: l.stepUpPurposeRevokeGrant(name),
      );
      return SelfNotice(l.adminGrantRevoked(name));
    });
    if (!ok || !mounted) return;
    widget.onChanged?.call();
    await reload();
  }

  /// "Extend" (07 9.4): the [GrantWizard] with the subject, target, level and capabilities of
  /// [grant], an empty reason and no last day; when it created the new grant, the old one is
  /// revoked with the new grant's reason. The new grant comes first, so the access never lapses
  /// in between; both requests carry their step-up purpose, and one step-up covers both within
  /// `auth.step_up_max_age` (the interceptor asks only when the server answers
  /// `401 step_up_required`). A failed revocation says that the new grant applies.
  Future<void> extend(AdminGrant grant) async {
    final l = AppLocalizations.of(context);
    final name = grantSubjectName(grant, labels);
    final created = await showDialog<AdminGrant>(
      context: context,
      builder: (dialog) => Dialog(
        key: const ValueKey('hints.extendDialog'),
        backgroundColor: dialog.dsColors.surfaceRaised,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 760),
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(DsSpace.s24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                DsBanner(tone: DsTone.info, text: l.hintsExtendNote),
                const SizedBox(height: DsSpace.s12),
                GrantWizard(
                  repository: widget.repository,
                  subject: GrantSubjectChoice(
                    grant.subjectKind,
                    grant.subjectId,
                    name,
                  ),
                  target: grantTarget(grant, labels),
                  level: grant.level ?? AccessLevel.none,
                  capabilities: grant.capabilities.toSet(),
                  step: 2,
                  title: l.hintsExtendTitle(name),
                  timeZone: widget.timeZone,
                  clock: widget.clock,
                  onCreated: (g) => Navigator.of(dialog).pop(g),
                  onCancel: () => Navigator.of(dialog).pop(),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    if (created == null || !mounted) return;
    widget.onChanged?.call();
    setState(() => busy = true);
    SelfNotice result;
    try {
      await widget.repository.revokeGrant(
        grant.id,
        reason: created.reason,
        purpose: l.stepUpPurposeRevokeGrant(name),
      );
      result = SelfNotice(l.hintsExtended(name));
    } on Object catch (e) {
      // A cancelled step-up has no problem text; the step-up's own purpose explains it.
      final cause = adminFailure(l, e)?.text ?? l.stepUpGenericPurpose;
      result = SelfNotice(
        l.hintsExtendRevokeFailed(name, cause),
        tone: DsTone.danger,
      );
    }
    if (!mounted) return;
    setState(() {
      busy = false;
      notice = result;
    });
    await reload();
  }
}

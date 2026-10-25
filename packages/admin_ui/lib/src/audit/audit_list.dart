// SPDX-License-Identifier: Apache-2.0

/// A6 Audit: the audit log as a list with filters and cursor paging, the details of an entry,
/// "Check chain" and the export (docs/spec/01-product.md 5.2 A6; 03 `audit_events`;
/// ADR 0021; brief M4-29).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart' show StepUpCancelled;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import 'audit_entry_detail.dart';
import 'audit_export.dart';
import 'audit_filters.dart';
import 'audit_repository.dart';
import 'chain_check.dart';

/// The audit log of the instance.
///
/// The list is virtualized (only the visible rows are built), newest first, one page at a
/// time with the server's page size and its cursor ("Load more"). Times show in [timeZone];
/// the person column names the actor from the people of [repository], else its ID. A tap opens
/// the entry's details. Reading needs the step-up of `audit.read`: the server asks, the step-up
/// dialog appears, and a cancelled one leaves "Try again". A `403` shows the server's reason
/// and nothing else (15 1 principle 1). The export buttons appear only when the host passes a
/// [saveTarget]. Entries are never changed or deleted here.
///
/// The view needs a bounded height (the shell gives it the space of its body).
///
/// Example:
/// ```dart
/// AuditLogView(repository: admin, timeZone: 'Europe/Berlin', saveTarget: pickFile);
/// ```
class AuditLogView extends StatefulWidget {
  /// Creates the view.
  const AuditLogView({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.saveTarget,
  });

  /// The administration's calls, the audit log among them.
  final AdminRepository repository;

  /// The time zone of the signed-in person (`users.time_zone`).
  final String timeZone;

  /// The current time, for "Today" and "Yesterday".
  final DateTime Function() clock;

  /// The host's save dialog, or null for no export.
  final AuditSaveTarget? saveTarget;

  @override
  State<AuditLogView> createState() => _AuditLogViewState();
}

class _AuditLogViewState extends State<AuditLogView> {
  AuditFilter _filter = const AuditFilter();
  List<AuditEntry>? _entries;
  String? _next;
  ApiProblem? _problem;
  bool _cancelled = false;
  bool _loading = false;
  bool _lastMore = false;
  Map<String, String> _names = const {};
  List<AdminPerson> _people = const [];

  bool _started = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // The first load needs the purpose text of the step-up, which needs the localizations;
    // it starts after the first frame, so a step-up dialog never opens during a build.
    if (_started) return;
    _started = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _load();
      _loadPeople();
    });
  }

  Future<void> _loadPeople() async {
    try {
      final people = await allPeople(widget.repository);
      if (!mounted) return;
      setState(() {
        _people = people;
        _names = {for (final p in people) p.id: p.displayName};
      });
    } on Object {
      // Without people the person filter is empty and the list shows actor IDs.
    }
  }

  Future<void> _load({bool more = false}) async {
    final l = AppLocalizations.of(context);
    final filter = _filter;
    _lastMore = more;
    setState(() => _loading = true);
    try {
      final page = await widget.repository.auditEvents(
        filter,
        cursor: more ? _next : null,
        purpose: l.stepUpPurposeAuditRead,
      );
      if (!mounted || filter != _filter) return;
      setState(() {
        _entries = [if (more) ...?_entries, ...page.items];
        _next = page.nextCursor;
        _problem = null;
        _cancelled = false;
      });
    } on StepUpCancelled {
      if (mounted) {
        setState(() {
          _cancelled = true;
          _problem = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _problem = loadProblem(e);
          _cancelled = false;
        });
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _filterBy(AuditFilter filter) {
    setState(() {
      _filter = filter;
      _entries = null;
      _next = null;
    });
    _load();
  }

  /// The seq span of the listed entries when a filter is set, else null: "Check chain" then
  /// checks the range of the current filter, otherwise from the last checkpoint (D-M4-29-2).
  ({int from, int to})? _range() {
    final entries = _entries;
    if (_filter.isEmpty || entries == null || entries.isEmpty) return null;
    final seqs = entries.map((e) => e.seq);
    return (
      from: seqs.reduce((a, b) => a < b ? a : b),
      to: seqs.reduce((a, b) => a > b ? a : b),
    );
  }

  String _time(BuildContext context, DateTime at) {
    final format = L10nFormat(Localizations.localeOf(context));
    String render(String zone) =>
        '${format.relativeDay(at, now: widget.clock(), timeZone: zone)} '
        '${format.time(at, timeZone: zone)}';
    try {
      return render(widget.timeZone);
    } on ArgumentError {
      return render('UTC');
    }
  }

  String _actor(AuditEntry e) => switch (e.actorId) {
    null => '',
    final id => _names[id] ?? id,
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final title = Semantics(
      header: true,
      child: Text(
        l.auditTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
    );
    // A refusal (403, or 404 for no access) shows the server's reason and nothing else; any
    // other failure keeps the filters, so a filter the server rejected can be changed.
    if (_problem case final problem?
        when problem.status == 403 || problem.status == 404) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          title,
          const SizedBox(height: DsSpace.s12),
          ProblemBanner(problem: problem, onRetry: _load),
        ],
      );
    }
    final entries = _entries;
    final save = widget.saveTarget;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        title,
        const SizedBox(height: DsSpace.s12),
        AuditFilters(
          actions: {...?entries?.map((e) => e.action)}.toList()..sort(),
          people: _people,
          timeZone: widget.timeZone,
          onChanged: _filterBy,
        ),
        const SizedBox(height: DsSpace.s12),
        Wrap(
          spacing: DsSpace.s12,
          runSpacing: DsSpace.s8,
          children: [
            AuditChainCheckButton(
              // A new filter starts without the result of the old one.
              key: ValueKey(identityHashCode(_filter)),
              repository: widget.repository,
              range: _range,
            ),
            if (save != null)
              AuditExportBar(
                repository: widget.repository,
                saveTarget: save,
                from: _filter.from,
                to: _filter.to,
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        if (_problem case final problem?)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s8),
            child: ProblemBanner(
              problem: problem,
              onRetry: () => _load(more: _lastMore),
            ),
          )
        else if (_cancelled)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s8),
            child: DsBanner(
              key: const ValueKey('audit.stepUpNeeded'),
              tone: DsTone.info,
              text: l.auditStepUpNeeded,
              actionLabel: l.selfRetry,
              onAction: () => _load(more: _lastMore),
            ),
          ),
        Expanded(
          child: entries == null
              ? const SizedBox.shrink()
              : entries.isEmpty
              ? Text(
                  l.auditEmpty,
                  key: const ValueKey('audit.empty'),
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textSecondary),
                )
              : ListView.builder(
                  key: const ValueKey('audit.list'),
                  itemCount: entries.length + (_next == null ? 0 : 1),
                  itemBuilder: (context, i) {
                    if (i == entries.length) {
                      return Align(
                        alignment: Alignment.centerLeft,
                        child: TextButton(
                          key: const ValueKey('audit.more'),
                          onPressed: _loading ? null : () => _load(more: true),
                          child: Text(l.adminLoadMore),
                        ),
                      );
                    }
                    final e = entries[i];
                    final time = _time(context, e.occurredAt);
                    final actor = _actor(e);
                    return _AuditRow(
                      entry: e,
                      time: time,
                      actor: actor,
                      onOpen: () => AuditEntryDetail.show(
                        context,
                        entry: e,
                        actor: actor,
                        time: time,
                      ),
                    );
                  },
                ),
        ),
      ],
    );
  }
}

class _AuditRow extends StatelessWidget {
  const _AuditRow({
    required this.entry,
    required this.time,
    required this.actor,
    required this.onOpen,
  });

  final AuditEntry entry;
  final String time;
  final String actor;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final e = entry;
    final label = auditActionLabel(l, e.action);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final primary = dsTextStyle(DsType.body)
        .copyWith(color: colors.textPrimary);
    // A button for screen readers: an InkWell row has a tap but no role (brief M4-32).
    return DsTappable(
      key: ValueKey('audit.entry.${e.seq}'),
      onTap: onOpen,
      child: DecoratedBox(
        decoration: BoxDecoration(
          border: Border(bottom: BorderSide(color: colors.borderSubtle)),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
          child: Wrap(
            spacing: DsSpace.s12,
            runSpacing: DsSpace.s4,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(time, style: secondary),
              if (actor.isNotEmpty)
                Text(
                  actor,
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
              Text(label ?? e.action, style: primary),
              if (label != null) Text(e.action, style: secondary),
              DsBadge(
                text: auditResultText(l, e.result),
                background: e.result == AuditResult.allow
                    ? colors.accentSubtle
                    : colors.dangerSubtle,
                foreground: colors.textPrimary,
              ),
              if (auditResourceText(e) case final r when r.isNotEmpty)
                Text(r, style: secondary),
              if (e.reason case final reason?) Text(reason, style: primary),
            ],
          ),
        ),
      ),
    );
  }
}

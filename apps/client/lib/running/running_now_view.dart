// SPDX-License-Identifier: Apache-2.0

/// "Running now" (docs/spec/01-product.md 5.1 D10, route `/running` of 15 11): every active run
/// the person may see, with the filters Space, Agent and "mine only", rows with state, agent,
/// path, duration, context and costs with their suffix, Stop only where the server allows
/// `run.cancel`, and the footer with the organization sum as an estimate (01 6.7). The switch
/// "Cards" and "List" (addendum of 2026-10-24, brief M4-39) shows the control room cards by
/// default; "List" keeps the rows and is the form for screen readers.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../forum/post_list_header.dart' show FilterChipButton;
import '../sidebar/providers.dart';
import 'control_room.dart';
import 'providers.dart';
import 'running_sum.dart';

/// The view D10.
///
/// Opening it loads `listActiveRuns` once into the local table ([runningRefreshProvider]);
/// afterwards the rows follow the table, which `run.state_changed` keeps (04 11.3 item 1).
class RunningNowView extends ConsumerWidget {
  /// Creates the view.
  const RunningNowView({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final refreshed = ref.watch(runningRefreshProvider);
    final rows = ref.watch(filteredRunningRowsProvider);
    final all = ref.watch(runningRowsProvider).value ?? const <RunningRow>[];
    final sum = sumRunningCosts([for (final r in rows) r.run]);
    final form = runningFormOf(ref);
    return Material(
      key: const ValueKey('running.view'),
      color: c.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _Header(count: rows.length, all: all),
          if (refreshed.value == false)
            DsBanner(
              key: const ValueKey('running.failed'),
              tone: DsTone.info,
              text: l.runningFailed,
            ),
          Divider(
            height: DsBorderWidth.hairline,
            thickness: DsBorderWidth.hairline,
            color: c.borderSubtle,
          ),
          Expanded(
            child: rows.isEmpty
                ? Center(
                    child: Text(
                      l.runningEmpty,
                      key: const ValueKey('running.empty'),
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: c.textSecondary),
                    ),
                  )
                : form == RunningForm.cards
                ? ControlRoomCards(rows: rows)
                : ListView(
                    key: const ValueKey('running.list'),
                    children: [
                      for (final row in rows) RunningRowTile(row: row),
                    ],
                  ),
          ),
          if (sum.hasOrganization || sum.hasPersonal) _Footer(sum: sum),
        ],
      ),
    );
  }
}

/// The form Running now shows: the person's choice, cards by default, on the desktop (D10) and
/// on the phone (M11; docs/decisions.d/M5-14.md D-M5-14-2).
RunningForm runningFormOf(WidgetRef ref) =>
    ref.watch(runningFormChoiceProvider);

class _Header extends ConsumerWidget {
  const _Header({required this.count, required this.all});

  final int count;
  final List<RunningRow> all;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final filter = ref.watch(runningFilterProvider);
    final notifier = ref.read(runningFilterProvider.notifier);
    final form = runningFormOf(ref);
    final forms = ref.read(runningFormChoiceProvider.notifier);
    final phone = ref.watch(appPortsProvider).formFactor.isPhone;
    final spaces = ref.watch(storedSpacesProvider).value ?? const <Space>[];
    final agents = <PrincipalId, String>{
      for (final row in all) row.run.agentId: row.agentName,
    };
    final spaceName = spaces
        .where((s) => s.id == filter.spaceId)
        .map((s) => s.name)
        .firstOrNull;
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        DsSpace.s12,
        DsSpace.s16,
        DsSpace.s8,
      ),
      child: Wrap(
        alignment: WrapAlignment.spaceBetween,
        crossAxisAlignment: WrapCrossAlignment.center,
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s6,
        children: [
          Semantics(
            header: true,
            child: Text(
              l.runningTitle(count),
              key: const ValueKey('running.title'),
              style: dsTextStyle(DsType.heading).copyWith(color: c.textPrimary),
            ),
          ),
          Wrap(
            spacing: DsSpace.s6,
            runSpacing: DsSpace.s6,
            children: [
              FilterChipButton(
                key: const ValueKey('running.form.cards'),
                label: l.runningCards,
                selected: form == RunningForm.cards,
                onTap: () => forms.set(RunningForm.cards),
              ),
              FilterChipButton(
                key: const ValueKey('running.form.list'),
                label: l.runningList,
                selected: form == RunningForm.list,
                onTap: () => forms.set(RunningForm.list),
              ),
              // On the phone the filters open as a sheet (01 5.3 M11).
              if (phone)
                FilterChipButton(
                  key: const ValueKey('running.filters'),
                  label: l.runningFilters,
                  selected:
                      filter.spaceId != null ||
                      filter.agentId != null ||
                      filter.mineOnly,
                  glyph: DsIcons.actionFilter,
                  onTap: () => unawaited(
                    showModalBottomSheet<void>(
                      context: context,
                      useSafeArea: true,
                      isScrollControlled: true,
                      builder: (_) => RunningFilterSheet(all: all),
                    ),
                  ),
                ),
              if (!phone) ...[
                MenuAnchor(
                  menuChildren: [
                    MenuItemButton(
                      key: const ValueKey('running.space.all'),
                      onPressed: () => notifier.setSpace(null),
                      child: Text(l.runningFilterAll),
                    ),
                    for (final s in spaces)
                      MenuItemButton(
                        key: ValueKey('running.space.${s.id.value}'),
                        onPressed: () => notifier.setSpace(s.id),
                        child: Text(dsIsolate(s.name)),
                      ),
                  ],
                  builder: (context, menu, _) => FilterChipButton(
                    key: const ValueKey('running.filter.space'),
                    label:
                        '${l.runningFilterSpace}: '
                        '${spaceName == null ? l.runningFilterAll : dsIsolate(spaceName)}',
                    selected: filter.spaceId != null,
                    glyph: DsIcons.actionFilter,
                    onTap: () => menu.isOpen ? menu.close() : menu.open(),
                  ),
                ),
                MenuAnchor(
                  menuChildren: [
                    MenuItemButton(
                      key: const ValueKey('running.agent.all'),
                      onPressed: () => notifier.setAgent(null),
                      child: Text(l.runningFilterAll),
                    ),
                    for (final MapEntry(key: id, value: name) in agents.entries)
                      MenuItemButton(
                        key: ValueKey('running.agent.${id.value}'),
                        onPressed: () => notifier.setAgent(id),
                        child: Text(dsIsolate(name)),
                      ),
                  ],
                  builder: (context, menu, _) => FilterChipButton(
                    key: const ValueKey('running.filter.agent'),
                    label:
                        '${l.runningFilterAgent}: '
                        '${filter.agentId == null ? l.runningFilterAll : dsIsolate(agents[filter.agentId] ?? filter.agentId!.value)}',
                    selected: filter.agentId != null,
                    glyph: DsIcons.actionFilter,
                    onTap: () => menu.isOpen ? menu.close() : menu.open(),
                  ),
                ),
                FilterChipButton(
                  key: const ValueKey('running.filter.mine'),
                  label: l.runningMineOnly,
                  selected: filter.mineOnly,
                  onTap: () => notifier.setMineOnly(!filter.mineOnly),
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}

/// The filters of Running now on the phone as a sheet (01 5.3 M11): Space, Agent and "mine
/// only" as chips at the touch size, each change applies at once; "Done" closes the sheet.
/// [all] are the unfiltered rows, the source of the agents.
class RunningFilterSheet extends ConsumerWidget {
  /// Creates the sheet body.
  const RunningFilterSheet({super.key, required this.all});

  /// Every row before the filters.
  final List<RunningRow> all;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final filter = ref.watch(runningFilterProvider);
    final notifier = ref.read(runningFilterProvider.notifier);
    final spaces = ref.watch(storedSpacesProvider).value ?? const <Space>[];
    final agents = <PrincipalId, String>{
      for (final row in all) row.run.agentId: row.agentName,
    };
    Widget section(String title, List<Widget> chips) => Padding(
      padding: const EdgeInsets.only(top: DsSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Semantics(
            header: true,
            child: Text(
              title,
              style: dsTextStyle(DsType.label).copyWith(color: c.textPrimary),
            ),
          ),
          const SizedBox(height: DsSpace.s6),
          Wrap(spacing: DsSpace.s6, runSpacing: DsSpace.s6, children: chips),
        ],
      ),
    );
    return SingleChildScrollView(
      key: const ValueKey('running.filterSheet'),
      padding: const EdgeInsets.all(DsSpace.s16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            header: true,
            child: Text(
              l.runningFilters,
              style: dsTextStyle(DsType.heading).copyWith(color: c.textPrimary),
            ),
          ),
          section(l.runningFilterSpace, [
            FilterChipButton(
              key: const ValueKey('running.space.all'),
              label: l.runningFilterAll,
              selected: filter.spaceId == null,
              onTap: () => notifier.setSpace(null),
            ),
            for (final s in spaces)
              FilterChipButton(
                key: ValueKey('running.space.${s.id.value}'),
                label: dsIsolate(s.name),
                selected: filter.spaceId == s.id,
                onTap: () => notifier.setSpace(s.id),
              ),
          ]),
          section(l.runningFilterAgent, [
            FilterChipButton(
              key: const ValueKey('running.agent.all'),
              label: l.runningFilterAll,
              selected: filter.agentId == null,
              onTap: () => notifier.setAgent(null),
            ),
            for (final MapEntry(key: id, value: name) in agents.entries)
              FilterChipButton(
                key: ValueKey('running.agent.${id.value}'),
                label: dsIsolate(name),
                selected: filter.agentId == id,
                onTap: () => notifier.setAgent(id),
              ),
          ]),
          section(l.runningMineOnly, [
            FilterChipButton(
              key: const ValueKey('running.filter.mine'),
              label: l.runningMineOnly,
              selected: filter.mineOnly,
              onTap: () => notifier.setMineOnly(!filter.mineOnly),
            ),
          ]),
          const SizedBox(height: DsSpace.s16),
          DsButton(
            key: const ValueKey('running.filtersDone'),
            label: l.runningFiltersDone,
            expand: true,
            onPressed: () => unawaited(Navigator.of(context).maybePop()),
          ),
        ],
      ),
    );
  }
}

/// One row of D10: state icon and word, agent, path, duration, context, costs and the buttons
/// Stop (only with [RunningRow.canStop]) and Open.
class RunningRowTile extends ConsumerWidget {
  /// Creates the row of [row].
  const RunningRowTile({super.key, required this.row});

  /// The row.
  final RunningRow row;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final run = row.run;
    final state = DsRunState.fromCode(run.state.code);
    final stateWord = switch (state) {
      DsRunState.queued => l.liveBoxStateQueued,
      DsRunState.waitingInput => l.liveBoxStateWaitingForYou,
      _ => l.liveBoxStateRunning,
    };
    final now = ref.watch(runningClockProvider).value;
    final started = run.startedAt;
    // A queued run has no duration yet (01 D10: "Queued" without a time).
    final duration = started == null || now == null
        ? null
        : dsFormat(context).elapsed(now.difference(started));
    final context0 = ref.watch(runContextProvider(run.id.value)).value;
    final mode = costModeOf(run);
    final cost = DsCost(
      DsMoney(run.cost.amountMicros / 1000000, run.cost.currency),
      mode: switch (mode) {
        BillingMode.organization => DsBillingMode.organization,
        BillingMode.personal => DsBillingMode.personal,
        BillingMode.unknown => DsBillingMode.personal,
      },
      isEstimate: run.cost.isEstimate,
    );
    final path = row.path.map(dsIsolate).join(' > ');
    void open() => context.go('/t/${run.threadId.value}');
    return Semantics(
      container: true,
      label: l.runningRowLabel(
        dsIsolate(row.agentName),
        path,
        stateWord,
        duration ?? '',
      ),
      child: Container(
        key: ValueKey('running.row.${run.id.value}'),
        constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s16,
          vertical: DsSpace.s6,
        ),
        decoration: BoxDecoration(
          border: Border(
            bottom: BorderSide(
              color: c.borderSubtle,
              width: DsBorderWidth.hairline,
            ),
          ),
        ),
        child: Wrap(
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: DsSpace.s12,
          runSpacing: DsSpace.s4,
          children: [
            ExcludeSemantics(
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DsIcon(
                    dsRunStateGlyph(state),
                    size: DsIconSizeRole.sm,
                    color: dsRunStateColor(context, state),
                    decorative: true,
                  ),
                  const SizedBox(width: DsSpace.s6),
                  Text(
                    dsIsolate(row.agentName),
                    style: dsTextStyle(DsType.bodyStrong)
                        .copyWith(color: c.textPrimary),
                  ),
                ],
              ),
            ),
            ExcludeSemantics(
              child: Text(
                path,
                style: dsTextStyle(DsType.body).copyWith(color: c.textPrimary),
              ),
            ),
            ExcludeSemantics(
              child: Text(
                state == DsRunState.running
                    ? (duration ?? stateWord)
                    : stateWord,
                key: ValueKey('running.state.${run.id.value}'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: c.textSecondary),
              ),
            ),
            if (state == DsRunState.waitingInput && duration != null)
              ExcludeSemantics(
                child: Text(
                  duration,
                  style: dsTextStyle(DsType.codeSm)
                      .copyWith(color: c.textSecondary),
                ),
              ),
            if (context0 != null)
              Text(
                l.runningContext(dsFormat(context).percent(context0.round())),
                key: ValueKey('running.context.${run.id.value}'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: c.textSecondary),
              ),
            if (state != DsRunState.queued)
              DsCostText(cost, key: ValueKey('running.cost.${run.id.value}')),
            if (row.canStop)
              DsButton(
                key: ValueKey('running.stop.${run.id.value}'),
                label: l.runningStop,
                style: DsButtonStyle.secondary,
                onPressed: () {
                  final engine = ref.read(syncEngineProvider);
                  if (engine != null) stopRun(engine, run);
                },
              ),
            DsButton(
              key: ValueKey('running.open.${run.id.value}'),
              label: l.runningOpen,
              style: DsButtonStyle.secondary,
              onPressed: open,
            ),
          ],
        ),
      ),
    );
  }
}

class _Footer extends StatelessWidget {
  const _Footer({required this.sum});

  final RunningSum sum;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final format = dsFormat(context);
    final amount = [
      for (final MapEntry(key: currency, value: micros)
          in sum.organizationMicros.entries)
        format.money(micros / 1000000, currency),
    ].join(', ');
    return Container(
      key: const ValueKey('running.footer'),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s16,
        vertical: DsSpace.s8,
      ),
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(color: c.borderSubtle, width: DsBorderWidth.hairline),
        ),
      ),
      child: Text(
        [
          if (sum.hasOrganization)
            l.runningSumOrganization(amount, l.costEstimate),
          if (sum.hasPersonal) l.runningPersonalNote,
        ].join(' '),
        key: const ValueKey('running.sum'),
        style: dsTextStyle(DsType.bodySm).copyWith(color: c.textSecondary),
      ),
    );
  }
}

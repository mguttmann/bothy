// SPDX-License-Identifier: Apache-2.0

/// The search assistant popup (docs/design/search-assistant.md 4, docs/spec/01-product.md D6):
/// a short-lived conversation over the shell. The person asks in their own words; each answer
/// lists sessions with their place, time and the assistant's reason, or a note that the hits are
/// full text. A tap closes the popup and opens the thread at the matching message. Closing the
/// popup forgets the conversation; nothing is stored.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../search/segments_text.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;
import 'assist_conversation.dart';
import 'assist_port.dart';
import 'providers.dart';

/// Opens the search assistant over [context] with [initialQuestion] as the first question (none
/// when empty) and goes to the session the person picks.
///
/// On the phone the popup fills the screen. Completes when the popup is closed.
///
/// Example: `onTap: () => showSearchAssistant(context, initialQuestion: text)`.
Future<void> showSearchAssistant(
  BuildContext context, {
  String initialQuestion = '',
}) async {
  final router = GoRouter.maybeOf(context);
  final phone = ProviderScope.containerOf(
    context,
    listen: false,
  ).read(appPortsProvider).formFactor.isPhone;
  final path = await showDialog<String>(
    context: context,
    barrierColor: context.dsColors.scrim,
    useSafeArea: !phone,
    builder: (_) => SearchAssistantDialog(
      initialQuestion: initialQuestion,
      fullScreen: phone,
    ),
  );
  if (path != null) router?.go(path);
}

/// The popup; pops with the path of the chosen session, or null.
///
/// Keys: `assist.dialog`, `assist.field`, `assist.send`, `assist.close`, `assist.question.<i>`,
/// `assist.pending`, `assist.failure.<i>`, `assist.fallback.<i>`, `assist.cost.<i>`,
/// `assist.nothing.<i>`, `assist.hit.<i>.<thread>`.
class SearchAssistantDialog extends ConsumerStatefulWidget {
  /// Creates the popup; a non-empty [initialQuestion] is asked when it opens.
  const SearchAssistantDialog({
    super.key,
    this.initialQuestion = '',
    this.fullScreen = false,
  });

  /// The first question.
  final String initialQuestion;

  /// Whether the popup fills the screen (the phone).
  final bool fullScreen;

  @override
  ConsumerState<SearchAssistantDialog> createState() =>
      _SearchAssistantDialogState();
}

class _SearchAssistantDialogState extends ConsumerState<SearchAssistantDialog> {
  late final AssistConversation _conversation = AssistConversation(
    ref.read(assistPortProvider),
  );
  final TextEditingController _field = TextEditingController();
  final FocusNode _fieldFocus = FocusNode();
  final ScrollController _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    _conversation.addListener(_changed);
    if (widget.initialQuestion.trim().isNotEmpty) {
      unawaited(_conversation.ask(widget.initialQuestion));
    }
  }

  void _changed() {
    if (!mounted) return;
    setState(() {});
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && _scroll.hasClients) {
        _scroll.jumpTo(_scroll.position.maxScrollExtent);
      }
    });
  }

  @override
  void dispose() {
    _conversation
      ..removeListener(_changed)
      ..dispose();
    _field.dispose();
    _fieldFocus.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _send() {
    final text = _field.text;
    if (text.trim().isEmpty || _conversation.busy) return;
    _field.clear();
    unawaited(_conversation.ask(text));
    _fieldFocus.requestFocus();
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent &&
        event.logicalKey == LogicalKeyboardKey.escape) {
      Navigator.of(context).pop();
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final exchanges = _conversation.exchanges;
    final body = Focus(
      canRequestFocus: false,
      skipTraversal: true,
      onKeyEvent: _onKey,
      child: Semantics(
        scopesRoute: true,
        namesRoute: true,
        explicitChildNodes: true,
        label: l.assistTitle,
        child: Column(
          mainAxisSize: widget.fullScreen ? MainAxisSize.max : MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _Header(onClose: () => Navigator.of(context).pop()),
            Divider(height: 1, color: c.borderSubtle),
            Flexible(
              fit: widget.fullScreen ? FlexFit.tight : FlexFit.loose,
              child: exchanges.isEmpty
                  ? Padding(
                      padding: const EdgeInsets.all(DsSpace.s16),
                      child: Text(
                        l.assistIntro,
                        key: const ValueKey('assist.intro'),
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: c.textMuted),
                      ),
                    )
                  : ListView(
                      controller: _scroll,
                      shrinkWrap: !widget.fullScreen,
                      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
                      children: [
                        for (final (i, e) in exchanges.indexed)
                          _ExchangeView(index: i, exchange: e),
                      ],
                    ),
            ),
            Divider(height: 1, color: c.borderSubtle),
            _Field(
              controller: _field,
              focusNode: _fieldFocus,
              busy: _conversation.busy,
              onSend: _send,
            ),
          ],
        ),
      ),
    );
    if (widget.fullScreen) {
      return Dialog.fullscreen(
        key: const ValueKey('assist.dialog'),
        backgroundColor: c.surfaceOverlay,
        child: SafeArea(child: body),
      );
    }
    return Dialog(
      key: const ValueKey('assist.dialog'),
      alignment: Alignment.topCenter,
      insetPadding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s24,
        vertical: DsSpace.s48,
      ),
      backgroundColor: c.surfaceOverlay,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(DsRadius.lg),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: body,
      ),
    );
  }
}

/// The title and the close button.
class _Header extends StatelessWidget {
  const _Header({required this.onClose});

  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s8,
      ),
      child: Row(
        children: [
          DsIcon(
            DsIcons.actionSearch,
            size: DsIconSizeRole.sm,
            color: c.textSecondary,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s8),
          Expanded(
            child: Text(
              l.assistTitle,
              style: dsTextStyle(DsType.bodyStrong)
                  .copyWith(color: c.textPrimary),
            ),
          ),
          IconButton(
            key: const ValueKey('assist.close'),
            tooltip: l.switcherClose,
            onPressed: onClose,
            icon: DsIcon(
              DsIcons.actionDismiss,
              size: DsIconSizeRole.sm,
              color: c.textSecondary,
              decorative: true,
            ),
          ),
        ],
      ),
    );
  }
}

/// The question field with its button; Enter sends.
class _Field extends StatelessWidget {
  const _Field({
    required this.controller,
    required this.focusNode,
    required this.busy,
    required this.onSend,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final bool busy;
  final VoidCallback onSend;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s12),
      child: Row(
        children: [
          Expanded(
            child: Semantics(
              container: true,
              label: l.assistFieldLabel,
              child: TextField(
                key: const ValueKey('assist.field'),
                controller: controller,
                focusNode: focusNode,
                autofocus: true,
                textInputAction: TextInputAction.search,
                onSubmitted: (_) => onSend(),
                style: dsTextStyle(DsType.body).copyWith(color: c.textPrimary),
                decoration: InputDecoration(
                  hintText: l.assistFieldHint,
                  isDense: true,
                ),
              ),
            ),
          ),
          const SizedBox(width: DsSpace.s8),
          DsButton(
            key: const ValueKey('assist.send'),
            label: l.assistSend,
            style: DsButtonStyle.primary,
            loading: busy,
            onPressed: busy ? null : onSend,
          ),
        ],
      ),
    );
  }
}

/// One question with its answer.
class _ExchangeView extends ConsumerWidget {
  const _ExchangeView({required this.index, required this.exchange});

  final int index;
  final AssistExchange exchange;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final answer = exchange.answer;
    final failure = exchange.failure;
    final note = dsTextStyle(DsType.bodySm).copyWith(color: c.textMuted);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: DsSpace.s16,
            vertical: DsSpace.s8,
          ),
          child: Text(
            dsIsolate(exchange.question),
            key: ValueKey('assist.question.$index'),
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: c.textPrimary),
          ),
        ),
        if (exchange.pending)
          Padding(
            key: const ValueKey('assist.pending'),
            padding: const EdgeInsets.symmetric(horizontal: DsSpace.s16),
            child: Text(l.assistSearching, style: note),
          ),
        if (failure != null)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: DsSpace.s16),
            child: Text(
              _failureText(l, failure),
              key: ValueKey('assist.failure.$index'),
              style: dsTextStyle(DsType.bodySm).copyWith(color: c.dangerFg),
            ),
          ),
        if (answer != null) ...[
          if (answer.fallback case final fallback?)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s16),
              child: Text(
                _fallbackText(l, fallback),
                key: ValueKey('assist.fallback.$index'),
                style: note,
              ),
            ),
          if (answer.hits.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s16),
              child: Text(
                l.assistNothing,
                key: ValueKey('assist.nothing.$index'),
                style: note,
              ),
            ),
          for (final hit in answer.hits) AssistHitTile(index: index, hit: hit),
          if (answer.costMicros case final micros?)
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: DsSpace.s16,
                vertical: DsSpace.s4,
              ),
              child: Text(
                l.assistCost(_amount(micros, answer.currency ?? '')),
                key: ValueKey('assist.cost.$index'),
                style: dsTextStyle(DsType.caption).copyWith(color: c.textMuted),
              ),
            ),
        ],
      ],
    );
  }
}

/// One session: title, place, time, the reason or "Full text", and the excerpt; a tap pops the
/// popup with the session's path.
class AssistHitTile extends ConsumerWidget {
  /// Creates the tile of [hit] in the answer of question [index].
  const AssistHitTile({super.key, required this.index, required this.hit});

  /// The question the hit answers.
  final int index;

  /// The hit.
  final AssistHit hit;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final zone = ref.watch(personTimeZoneProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final format = dsFormat(context);
    final time =
        '${format.relativeDay(hit.at, now: now, timeZone: zone)} '
        '${format.time(hit.at, timeZone: zone)}';
    final title = hit.threadTitle ?? hit.channelName;
    final place = [?hit.spaceName, hit.channelName].map(dsIsolate).join(' > ');
    final why = hit.reason ?? l.assistFulltext;
    return DsTappable(
      key: ValueKey('assist.hit.$index.${hit.threadId.value}'),
      label: l.assistHitLabel(dsIsolate(title), place, time, why),
      excludeChildSemantics: true,
      onTap: () => Navigator.of(context).pop(assistTargetOf(hit)),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s16,
          vertical: DsSpace.s8,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              dsIsolate(title),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.bodyStrong)
                  .copyWith(color: c.textPrimary),
            ),
            const SizedBox(height: DsSpace.s2),
            Text(
              '$place  $time',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: c.textSecondary),
            ),
            const SizedBox(height: DsSpace.s2),
            Text(
              hit.picked ? dsIsolate(why) : why,
              style: dsTextStyle(hit.picked ? DsType.body : DsType.caption)
                  .copyWith(color: hit.picked ? c.textPrimary : c.textMuted),
            ),
            if (hit.segments.isNotEmpty) ...[
              const SizedBox(height: DsSpace.s2),
              SegmentsText(segments: hit.segments, maxLines: 2),
            ],
          ],
        ),
      ),
    );
  }
}

String _failureText(AppLocalizations l, AssistFailureKind kind) =>
    switch (kind) {
      AssistFailureKind.queryEmpty => l.assistQueryEmpty,
      AssistFailureKind.queryTooLong => l.assistQueryTooLong,
      AssistFailureKind.offline => l.assistOffline,
      AssistFailureKind.failed => l.assistFailed,
    };

String _fallbackText(AppLocalizations l, AssistFallback fallback) =>
    switch (fallback) {
      AssistFallback.notConfigured => l.assistFallbackNotConfigured,
      AssistFallback.unavailable => l.assistFallbackUnavailable,
      AssistFallback.timeout => l.assistFallbackTimeout,
    };

/// [micros] millionths of [currency] with four decimals, for example `0.0012 USD`.
String _amount(int micros, String currency) {
  final value = (micros / 1000000).toStringAsFixed(4);
  return currency.isEmpty ? value : '$value $currency';
}

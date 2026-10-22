// SPDX-License-Identifier: Apache-2.0

/// The consent of 11 4.1 in plain words: what an app may read and write, the events it
/// receives, where data are sent, its commands and components, the grants it asks for and the
/// publisher's `data_egress` text (brief M6-08 Design notes 1 and 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/apps_repository.dart';
import 'app_labels.dart';

/// The consent of [manifest]: the scopes with their meaning (required ones checked and fixed,
/// optional ones a checkbox), the events as checkboxes, the destinations of outgoing data, the
/// commands, the components, the requested grants with their purpose and the picker the
/// caller builds per grant ([grantTarget]), and the `data_egress` text.
///
/// The view decides nothing: which scopes and events are checked is [scopes] and [events] of
/// the caller, and whether the server accepts them is its answer to the install (15 1
/// principle 1). With [choicesOnly] the view shows only the scopes and events (the new consent
/// of an update, 11 4.2: an update never adds a grant).
///
/// Keys: `<keyPrefix>.view`, `<keyPrefix>.scope.<scope>`, `<keyPrefix>.event.<type>`.
///
/// Example:
/// ```dart
/// ConsentView(
///   manifest: upload.info,
///   scopes: chosen,
///   events: events,
///   onScope: (scope, on) => setState(() => on ? chosen.add(scope) : chosen.remove(scope)),
///   onEvent: (type, on) => setState(() => on ? events.add(type) : events.remove(type)),
/// )
/// ```
class ConsentView extends StatelessWidget {
  /// Creates the consent of [manifest].
  const ConsentView({
    super.key,
    required this.manifest,
    required this.scopes,
    required this.events,
    required this.onScope,
    required this.onEvent,
    this.grantTarget,
    this.choicesOnly = false,
    this.enabled = true,
    this.keyPrefix = 'consent',
  });

  /// The manifest the owner consents to.
  final AppManifestInfo manifest;

  /// The checked scopes (every required one and the chosen optional ones).
  final Set<String> scopes;

  /// The checked events.
  final Set<String> events;

  /// Checks or unchecks an optional scope.
  final void Function(String scope, bool on) onScope;

  /// Checks or unchecks an event.
  final void Function(String type, bool on) onEvent;

  /// The target picker of the requested grant at the index, or null for none (an instance
  /// grant has no picker: its target is the instance).
  final Widget? Function(int index, AppGrantAsk ask)? grantTarget;

  /// Whether only the scopes and events are shown.
  final bool choicesOnly;

  /// Whether the checkboxes can change (false while a call runs).
  final bool enabled;

  /// The prefix of the keys.
  final String keyPrefix;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final heading = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final code = dsTextStyle(DsType.code).copyWith(color: colors.textSecondary);
    final m = manifest;

    Widget section(String title, List<Widget> children) => Padding(
      padding: const EdgeInsets.only(top: DsSpace.s16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(header: true, child: Text(title, style: heading)),
          const SizedBox(height: DsSpace.s4),
          if (children.isEmpty)
            Text(l.adminAppConsentNone, style: secondary)
          else
            ...children,
        ],
      ),
    );

    Widget scope(String s, {required bool required}) => CheckboxListTile(
      key: ValueKey('$keyPrefix.scope.$s'),
      contentPadding: EdgeInsets.zero,
      controlAffinity: ListTileControlAffinity.leading,
      dense: true,
      value: required || scopes.contains(s),
      onChanged: required || !enabled ? null : (on) => onScope(s, on ?? false),
      title: Text(appScopeLabel(l, s), style: body),
      subtitle: Text(
        '$s, ${required ? l.adminAppScopeRequired : l.adminAppScopeOptional}',
        style: code,
      ),
    );

    return Column(
      key: ValueKey('$keyPrefix.view'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (!choicesOnly)
          Text(
            l.adminAppConsentIntro(m.name, m.publisher, m.version),
            style: body,
          ),
        section(l.adminAppConsentScopes, [
          for (final s in m.requiredScopes) scope(s, required: true),
          for (final s in m.optionalScopes) scope(s, required: false),
        ]),
        section(l.adminAppConsentEvents, [
          for (final e in m.events)
            CheckboxListTile(
              key: ValueKey('$keyPrefix.event.$e'),
              contentPadding: EdgeInsets.zero,
              controlAffinity: ListTileControlAffinity.leading,
              dense: true,
              value: events.contains(e),
              onChanged: enabled ? (on) => onEvent(e, on ?? false) : null,
              title: Text(e, style: body),
            ),
        ]),
        if (!choicesOnly) ...[
          section(l.adminAppConsentDestinations, [
            for (final h in m.targetHosts) Text(h, style: body),
          ]),
          section(l.adminAppConsentCommands, [
            for (final c in m.commands)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s4),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('/${c.name}', style: code),
                    Text(c.description, style: body),
                  ],
                ),
              ),
          ]),
          section(l.adminAppConsentComponents, [
            if (m.components.isNotEmpty)
              Text(
                [for (final c in m.components) appComponentLabel(l, c)]
                    .join(', '),
                style: body,
              ),
          ]),
          section(l.adminAppConsentGrants, [
            for (final (i, g) in m.requestedGrants.indexed)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s8),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(appGrantAskLabel(l, g), style: body),
                    Text(l.adminAppGrantPurpose(g.purpose), style: secondary),
                    ?grantTarget?.call(i, g),
                  ],
                ),
              ),
          ]),
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                Semantics(
                  header: true,
                  child: Text(l.adminAppConsentEgress, style: heading),
                ),
                const SizedBox(height: DsSpace.s4),
                switch (m.dataEgress) {
                  final egress? when egress.trim().isNotEmpty => DsBanner(
                    tone: DsTone.warning,
                    text: egress,
                  ),
                  _ => Text(l.adminAppConsentNoEgress, style: secondary),
                },
              ],
            ),
          ),
        ],
      ],
    );
  }
}

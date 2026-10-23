// SPDX-License-Identifier: Apache-2.0

/// The public keys receivers verify outgoing webhooks with (11 7 "Key"; `getWebhookKeys`;
/// brief M6-08 Design note 3).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../self/device_tile.dart' show activityText;

/// The keys of `repository.webhookKeys()`: key ID, state and public key with a copy action,
/// explained as the keys receivers verify the `v1a` signature with.
///
/// The public keys are no secret (a leak at a receiver forges nothing, 11 7), so they are
/// shown in full and may be copied any time.
///
/// Keys: `keys.card`, `keys.copy.<kid>`.
///
/// Example: `WebhookKeysCard(repository: repository, timeZone: 'Europe/Berlin')`.
class WebhookKeysCard extends StatefulWidget {
  /// Creates the card.
  const WebhookKeysCard({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<WebhookKeysCard> createState() => _WebhookKeysCardState();
}

class _WebhookKeysCardState extends State<WebhookKeysCard> {
  List<WebhookKeyInfo>? _keys;
  ApiProblem? _problem;
  final Set<String> _copied = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final keys = await widget.repository.webhookKeys();
      if (mounted) {
        setState(() {
          _keys = keys;
          _problem = null;
        });
      }
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  Future<void> _copy(WebhookKeyInfo key) async {
    await Clipboard.setData(ClipboardData(text: key.publicKey));
    if (mounted) setState(() => _copied.add(key.kid));
  }

  String _state(AppLocalizations l, String state) => switch (state) {
    'active' => l.adminHookKeyActive,
    'next' => l.adminHookKeyNext,
    'retiring' => l.adminHookKeyRetiring,
    // A state of a newer server, shown as the server names it.
    _ => state,
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: const ValueKey('keys.card'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.adminHookKeysTitle,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s4),
        Text(
          l.adminHookKeysBody,
          style: dsTextStyle(DsType.body).copyWith(color: colors.textSecondary),
        ),
        const SizedBox(height: DsSpace.s8),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else
          for (final key in _keys ?? const <WebhookKeyInfo>[])
            Padding(
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Wrap(
                          spacing: DsSpace.s12,
                          children: [
                            Text(
                              key.kid,
                              style: dsTextStyle(DsType.bodyStrong)
                                  .copyWith(color: colors.textPrimary),
                            ),
                            Text(_state(l, key.state), style: secondary),
                            if (key.notAfter case final end?)
                              Text(
                                l.adminHookKeyUntil(
                                  activityText(
                                    context,
                                    end,
                                    now: widget.clock(),
                                    timeZone: widget.timeZone,
                                  ),
                                ),
                                style: secondary,
                              ),
                          ],
                        ),
                        Text(
                          key.publicKey,
                          style: dsTextStyle(DsType.code)
                              .copyWith(color: colors.textSecondary),
                        ),
                      ],
                    ),
                  ),
                  TextButton(
                    key: ValueKey('keys.copy.${key.kid}'),
                    onPressed: () => _copy(key),
                    child: Text(
                      _copied.contains(key.kid)
                          ? l.onboardingCopied
                          : l.onboardingCopy,
                    ),
                  ),
                ],
              ),
            ),
      ],
    );
  }
}

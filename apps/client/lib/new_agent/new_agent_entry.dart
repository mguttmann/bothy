// SPDX-License-Identifier: Apache-2.0

/// "New agent" in the desktop and the phone app (docs/spec/01-product.md 6.20;
/// docs/spec/26-builtin-agents.md 8; docs/decisions.d/A1-08.md).
///
/// The dialog, its preview, the steps and the port are the shared parts of `admin_ui`
/// ([NewAgentDialog], [NewAgentFlow], [ApiNewAgentPort]), so the console runs the same code; this
/// module wires them to the signed-in session: [newAgentOfferProvider] decides whether the entry
/// shows (an owner, a server that answers `listAgentTemplates` and a runner that reports
/// `hosts_agents`, 26 8.1), [newAgentHostOf] hands the port, the key maker and the announcement
/// interval to an entry, and [openNewAgent] opens the dialog from the space menu and then the new
/// channel or, on the desktop, the agent card (01 6.20 item 5).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../notifications_settings/providers.dart' show isOwnerProvider;

part 'new_agent_entry.g.dart';

/// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
/// out, not an owner, an older server or no runner that hosts agents (26 8.1).
@riverpod
Future<NewAgentOffer?> newAgentOffer(Ref ref) async {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  if (!await ref.watch(isOwnerProvider.future)) return null;
  // The offer reads only; no step-up, so the texts of the prompt are not needed here.
  return ApiNewAgentPort(
    services.auth.api,
    purpose: (name) => name,
    reason: (name) => name,
  ).offer();
}

/// The host's part of "New agent" over the signed-in session, or null while signed out;
/// [phone] fills the screen, [onCreated] opens what was created.
NewAgentHost? newAgentHostOf(
  BuildContext context,
  WidgetRef ref, {
  required bool phone,
  void Function(NewAgentResult result)? onCreated,
}) {
  final services = ref.read(signedInServicesProvider);
  if (services == null) return null;
  final l = AppLocalizations.of(context);
  return NewAgentHost(
    port: ApiNewAgentPort(
      services.auth.api,
      purpose: l.stepUpPurposeNewAgent,
      reason: l.newAgentAuditReason,
    ),
    newKey: services.ids.next,
    announceMinInterval: ref.read(appConfigProvider).liveAnnounceMinInterval,
    fullScreen: phone,
    onCreated: onCreated,
  );
}

/// Opens what the dialog created (01 6.20 item 5): the new channel, otherwise on the desktop the
/// agent card; the phone has no agent card and stays where it is.
void openNewAgentResult(
  BuildContext context,
  NewAgentResult result, {
  required bool phone,
}) {
  final channel = result.channelId;
  if (channel != null) {
    context.go('/c/$channel');
  } else if (!phone) {
    // The path of `adminPathOf` (admin/admin_route.dart), written here so the two modules do
    // not import each other.
    context.go(
      '/admin${AdminLocation(AdminSection.agents, id: result.agentId).path}',
    );
  }
}

/// Opens "New agent" from the space menu; does nothing without an offer or a session.
Future<void> openNewAgent(
  BuildContext context,
  WidgetRef ref, {
  required bool phone,
}) async {
  final offer = await ref.read(newAgentOfferProvider.future);
  if (offer == null || !context.mounted) return;
  final host = newAgentHostOf(context, ref, phone: phone);
  if (host == null) return;
  final result = await host.open(context, offer);
  if (result == null || !context.mounted) return;
  openNewAgentResult(context, result, phone: phone);
}

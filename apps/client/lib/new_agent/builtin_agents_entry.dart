// SPDX-License-Identifier: Apache-2.0

/// "Built-in agents" on the phone (docs/spec/01-product.md 6.20 item 6;
/// docs/spec/26-builtin-agents.md 8.4; brief A1-09 requirement 16; docs/decisions.d/A1-09.md).
///
/// The phone has no agent list of the administration (D-A1-08-2), so an owner reaches the
/// editing of the built-in agents from Me: [BuiltinAgentsEntry] opens [builtinAgentsPath], whose
/// [PhoneBuiltinAgentsPage] shows the shared [BuiltinAgentsView] of `admin_ui` over the
/// signed-in session, with the full-screen dialogs of the phone. The entry shows only for an
/// owner whose server offers "New agent" or has a built-in agent ([builtinAgentsAvailable]).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../settings/settings_providers.dart' show personTimeZoneProvider;
import 'new_agent_entry.dart';

part 'builtin_agents_entry.g.dart';

/// The path of the page of the built-in agents under Me.
const String builtinAgentsPath = '/admin/agents';

/// Whether the signed-in person gets "Built-in agents": an owner whose server offers "New
/// agent" (26 8.1) or lists a built-in agent, so a disabled runner does not hide the agents
/// that are already there. False while signed out and on any failure.
@riverpod
Future<bool> builtinAgentsAvailable(Ref ref) async {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return false;
  // Both are watched before the first gap, so the provider follows them.
  final owner = ref.watch(isOwnerProvider.future);
  final offer = ref.watch(newAgentOfferProvider.future);
  if (!await owner) return false;
  if (await offer != null) return true;
  try {
    // A read: no step-up, so the texts of the prompt are not needed.
    final agents = await ApiNewAgentPort(
      services.auth.api,
      purpose: (name) => name,
      reason: (name) => name,
    ).builtinAgents();
    return agents.isNotEmpty;
  } on Object {
    return false;
  }
}

/// The entry "Built-in agents" under Me; it opens [builtinAgentsPath].
class BuiltinAgentsEntry extends ConsumerWidget {
  /// Creates the entry.
  const BuiltinAgentsEntry({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!(ref.watch(builtinAgentsAvailableProvider).value ?? false)) {
      return const SizedBox.shrink();
    }
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return ListTile(
      key: const ValueKey('phone.me.builtinAgents'),
      title: Text(l.builtinAgentsEntry),
      subtitle: Text(l.builtinAgentsEntryHint),
      trailing: DsIcon(
        DsIcons.actionExpand,
        size: DsIconSizeRole.sm,
        color: colors.textSecondary,
        decorative: true,
      ),
      onTap: () => context.go(builtinAgentsPath),
    );
  }
}

/// The page [builtinAgentsPath]: the [BuiltinAgentsView] over the signed-in session; "Not
/// found or no access" ([notFound]) for a person the entry is not for.
class PhoneBuiltinAgentsPage extends ConsumerWidget {
  /// Creates the page; [notFound] is the page of a target that is not there.
  const PhoneBuiltinAgentsPage({super.key, required this.notFound});

  /// "Not found or no access" (01 6.7).
  final Widget notFound;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final available = ref.watch(builtinAgentsAvailableProvider);
    if (available.value == null && available.isLoading) {
      return const SizedBox.expand();
    }
    final host = (available.value ?? false)
        ? newAgentHostOf(context, ref, phone: true)
        : null;
    if (host == null) return notFound;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(DsSpace.s16),
      child: BuiltinAgentsView(
        host: host,
        timeZone: ref.watch(personTimeZoneProvider),
      ),
    );
  }
}

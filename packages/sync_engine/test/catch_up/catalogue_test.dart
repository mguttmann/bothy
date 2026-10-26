// SPDX-License-Identifier: Apache-2.0

// Every event type of the catalogue is either applied to a local table, handled in memory, or
// skipped on purpose (brief M4-14 Fixed requirement 10); the README lists the same sets.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

/// Durable types without a local table: they advance the cursor and are skipped.
const Set<String> skippedDurable = {
  'access.expiring',
  'agent.config_changed',
  'agent.followup',
  'agent.invocation',
  'agent_session.changed',
  'channel.settings_changed',
  'command.invoked',
  'device.pending',
  'gdpr.request_due',
  'gdpr.request_updated',
  'grant.changed',
  'group.changed',
  'import.progress',
  'interaction.created',
  'run.cancel_requested',
  'status.updated',
};

/// Ephemeral types the dispatcher hands to the ephemeral store, the run store hook or the
/// hint hook.
const Set<String> ephemeralHandled = {
  'typing.started',
  'agent.presence_changed',
  'run.updated',
  'notification.created',
  'host.presence_changed',
};

void main() {
  test(
    'every catalogue type is applied, held in memory or skipped on purpose',
    () {
      final durable = {
        for (final t in EventCatalog.types)
          if (EventCatalog.lookup(t)!.durable) t,
      };
      expect(appliedEventTypes.difference(durable), isEmpty);
      expect(durable.difference(appliedEventTypes), skippedDurable);
      final ephemeral = {
        for (final t in EventCatalog.types)
          if (!EventCatalog.lookup(t)!.durable) t,
      };
      expect(ephemeral.containsAll(ephemeralHandled), isTrue);
    },
  );
}

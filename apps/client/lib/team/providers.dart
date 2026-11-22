// SPDX-License-Identifier: Apache-2.0

/// The Riverpod providers of team channels (09 3.9; R-CL-03: widgets never call the API
/// themselves). Whether a channel is a team channel comes from `getChannelTeam` (`404` for
/// none): the stored channel and the sync events carry no team member, and only owners read a
/// team channel (readers rule, 09 3.9.1), so only an owner asks, once per text channel shown
/// (ruling D-TEAM-APP-1). The header reads the team again after every own change and when a
/// new message arrives in the team channel (a pause and a stop post a note there).
library;

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../sidebar/providers.dart' show storedChannelOfRouteProvider;
import 'team_port.dart';

part 'providers.g.dart';

/// The team operations of the signed-in session, or null before the login; tests override it.
@riverpod
TeamPort? teamPort(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services == null ? null : ApiTeamPort(services.auth.api);
}

/// Makes an idempotency key for a team operation; tests override it.
@riverpod
String Function() teamKeys(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services?.ids.next ?? () => '';
}

/// Whether [channel] may be a team channel at all (09 3.9.1): a text channel that is neither
/// archived nor read-only. A direct message, a forum and an unknown kind never are.
bool teamCapable(Channel channel) =>
    channel.kind == ChannelKind.text &&
    channel.archivedAt == null &&
    !channel.readOnly;

/// The team of [channelId], or null when it is no team channel, the person is no owner, the
/// channel is no text channel or the read failed (the header then stays hidden).
@riverpod
Future<TeamView?> channelTeam(Ref ref, String channelId) async {
  final port = ref.watch(teamPortProvider);
  if (port == null) return null;
  final channel = await ref.watch(
    storedChannelOfRouteProvider(channelId).future,
  );
  if (channel == null || channel.kind != ChannelKind.text) return null;
  if (!await ref.watch(isOwnerProvider.future)) return null;
  try {
    return await port.read(channel.id);
  } on TeamFailure {
    return null;
  }
}

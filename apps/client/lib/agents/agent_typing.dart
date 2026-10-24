// SPDX-License-Identifier: Apache-2.0

/// The typing line of an agent (D-L9; 04 12 item 1; brief M4-21 Design note 2): agents send
/// no typing; their line "`agent` is working" follows the run state.
library;

import 'package:domain/domain.dart';

/// The display names of the agents whose run in [runs] is `running`, sorted.
///
/// `queued` is not working yet and `waiting_input` waits for a person; both show in the live
/// box, not as typing. Typing events of an agent are never read here.
List<String> workingAgentNames(
  Iterable<RunSummary> runs,
  Map<PrincipalId, Principal> principals,
) {
  final names = <String>{
    for (final r in runs)
      if (r.state == RunState.running)
        principals[r.agentId]?.displayName ?? r.agentId.value,
  };
  return names.toList()..sort();
}

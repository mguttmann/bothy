// SPDX-License-Identifier: Apache-2.0

/// The built-in commands of 09 10.1 (brief M4-21 Design note 4): `/stop`, `/status`, `/new`,
/// `/resume`, `/title`, `/close`. Their names are fixed English identifiers, their descriptions
/// are translated, and they are control commands: they never go to a model and never through
/// `invokeCommand` (they have no `command_id`); each runs its own operation.
library;

import 'package:l10n/l10n.dart';

/// One built-in command of 09 10.1 with the action it runs.
enum BuiltinCommand {
  /// Stops the active run (`run.cancel`, `cancelRun`).
  stop('stop'),

  /// Opens the run and session details; reads only (`run.read`, `agent_session.read`).
  status('status'),

  /// Starts a new agent session (`agent_session.reset`, `resetAgentSession`).
  newSession('new'),

  /// Binds an earlier external session after a selection (`agent_session.bind`,
  /// `bindAgentSession`).
  resume('resume'),

  /// Sets the thread title; its source becomes `manual` (`thread.title.update`,
  /// `setThreadTitle`).
  title('title'),

  /// Marks the thread done (`thread.close`, `closeThread`).
  close('close');

  const BuiltinCommand(this.name);

  /// The identifier after the slash, never translated (09 10.1).
  final String name;

  /// The translated description of the suggestion list.
  String description(AppLocalizations l) => switch (this) {
    stop => l.commandBuiltinStop,
    status => l.commandBuiltinStatus,
    newSession => l.commandBuiltinNew,
    resume => l.commandBuiltinResume,
    title => l.commandBuiltinTitle,
    close => l.commandBuiltinClose,
  };

  /// The built-in named [name] (without the slash), or null.
  static BuiltinCommand? byName(String name) {
    for (final c in values) {
      if (c.name == name) return c;
    }
    return null;
  }
}

/// What the server's answers allow in the open thread, one flag per built-in (09 10.1:
/// "each only where the server's answers allow the action"); the caller derives them from
/// the stored access, the run and the thread like the buttons of M4-20 and M4-23 do.
typedef BuiltinAvailability = ({
  bool stop,
  bool session,
  bool title,
  bool close,
});

/// The built-ins that [allowed] permits, in the order of 09 10.1; `/status` only reads and is
/// always offered.
List<BuiltinCommand> availableBuiltins(BuiltinAvailability allowed) => [
  if (allowed.stop) BuiltinCommand.stop,
  BuiltinCommand.status,
  if (allowed.session) BuiltinCommand.newSession,
  if (allowed.session) BuiltinCommand.resume,
  if (allowed.title) BuiltinCommand.title,
  if (allowed.close) BuiltinCommand.close,
];

/// Runs the built-ins; the history column implements it with the outbox operations and the
/// dialogs of the thread view.
abstract interface class BuiltinCommandRunner {
  /// `/stop`: cancels the active run through the outbox (`cancelRun`), no confirmation.
  Future<void> stop();

  /// `/status`: opens the run and session details.
  Future<void> status();

  /// `/new`: resets the agent session (`resetAgentSession`).
  Future<void> newSession();

  /// `/resume`: shows the earlier sessions (`listAgentSessions`) and binds the chosen one
  /// (`bindAgentSession`).
  Future<void> resume();

  /// `/title <text>`: sets the title (`setThreadTitle`).
  Future<void> title(String text);

  /// `/close`: marks the thread done (`closeThread`).
  Future<void> close();
}

/// Runs [command] with the text [argument] after its name on [runner].
Future<void> runBuiltin(
  BuiltinCommandRunner runner,
  BuiltinCommand command,
  String argument,
) => switch (command) {
  BuiltinCommand.stop => runner.stop(),
  BuiltinCommand.status => runner.status(),
  BuiltinCommand.newSession => runner.newSession(),
  BuiltinCommand.resume => runner.resume(),
  BuiltinCommand.title => runner.title(argument.trim()),
  BuiltinCommand.close => runner.close(),
};

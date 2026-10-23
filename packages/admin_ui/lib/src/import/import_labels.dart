// SPDX-License-Identifier: Apache-2.0

/// The texts of the import screens: states, row phases, the field errors of the mapping file
/// and the `409 conflict` reasons of the import interface in plain language (docs/spec/
/// 22-import-migration.md 8; brief M8-08 Design notes 2 and 4).
library;

import 'package:api_client/api_client.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/self_repository.dart';
import '../self/self_feedback.dart';
import 'import_controller.dart';
import 'import_repository.dart';
import 'mapping_file.dart';

/// The label of the state of [source]; a cancelled source reads "Cancelled" (the enum has no
/// such state, M8-03 note 3).
String importStateLabel(AppLocalizations l, ImportSourceSnapshot source) =>
    switch (source.state) {
      ImportState.draft => l.importStateDraft,
      ImportState.running => l.importStateRunning,
      ImportState.paused => l.importStatePaused,
      ImportState.completed => l.importStateCompleted,
      ImportState.failed =>
        source.cancelled ? l.importStateCancelled : l.importStateFailed,
    };

/// The label of [phase] of a channel row (22 13.1).
String importPhaseLabel(AppLocalizations l, ImportChannelPhase phase) =>
    switch (phase) {
      ImportChannelPhase.notImported => l.importPhaseNotImported,
      ImportChannelPhase.mirror => l.importPhaseMirror,
      ImportChannelPhase.bothyOperatorOk => l.importPhaseOperatorOk,
      ImportChannelPhase.bothyNoOperator => l.importPhaseNoOperator,
      ImportChannelPhase.bothyOtherOperator => l.importPhaseOtherOperator,
    };

/// The number of items the server counts as `posted` over every item kind
/// (`stats.server.<item_kind>.posted`).
int importPostedCount(ImportSourceSnapshot source) => source.serverCounters
    .where((c) => c.path.endsWith('.posted'))
    .fold(0, (sum, c) => sum + c.value.toInt());

/// The problem text of the field error [code] of the mapping file (the codes of M8-03 note 2);
/// an unknown code is shown as it is.
String importFieldProblem(AppLocalizations l, String code) => switch (code) {
  'required' => l.importFieldRequired,
  'unknown_member' => l.importFieldUnknownMember,
  'invalid' => l.importFieldInvalid,
  'not_digits' => l.importFieldNotDigits,
  'duplicate' => l.importFieldDuplicate,
  'too_long' => l.importFieldTooLong,
  'unknown_handle' => l.importFieldUnknownHandle,
  'not_active' => l.importFieldNotActive,
  'unknown_space' || 'unknown_channel' => l.importFieldUnknownExisting,
  'unknown_category' => l.importFieldUnknownCategory,
  'not_enabled' => l.importFieldNotEnabled,
  _ => code,
};

/// One line per field error of [problem]: the place in the mapping file (`channels[2].target`,
/// from the JSON pointer) and what is wrong.
///
/// Example: a pointer `/config/channels/2/planned_operator` with `unknown_handle` reads
/// "channels[2].planned_operator: names no person or agent with this handle".
List<String> importFieldErrors(AppLocalizations l, ApiProblem problem) => [
  for (final e in problem.errors)
    l.importFieldError(switch (mappingPathOf(e.pointer)) {
      '' => l.importFieldWholeFile,
      final path => path,
    }, importFieldProblem(l, e.code)),
];

/// The text of [error] of a mapping file that could not be read.
String importFileErrorText(AppLocalizations l, MappingFileError error) =>
    switch (error.problem) {
      MappingFileProblem.invalidYaml => switch ((error.line, error.column)) {
        (final int line, final int column) => l.importFileInvalidYaml(
          line,
          column,
        ),
        _ => l.importFileInvalid,
      },
      MappingFileProblem.notMapping => l.importFileNotMapping,
    };

/// The failure of an import call [error] as the screen shows it: the `409 conflict` reasons of
/// the import interface in plain language (`import_state`, `lead_not_bothy`,
/// `channel_not_imported`), everything else as `adminFailure`; null when the person cancelled
/// the step-up.
SelfFailure? importFailure(AppLocalizations l, Object error) {
  if (error is SelfActionFailed && error.problem?.code == 'conflict') {
    final text = switch (error.problem?.reason) {
      'import_state' => l.importConflictState,
      'lead_not_bothy' => l.importConflictLeadNotBothy,
      'channel_not_imported' => l.importConflictNotImported,
      _ => null,
    };
    if (text != null) return SelfFailure(text);
  }
  return adminFailure(l, error);
}

/// Whether the error code [error] of a failed item may be shown: a code of letters, digits,
/// dots and underscores. Any other text could carry content of the source and stays hidden
/// (22 11, 14).
String? importItemErrorCode(String? error) =>
    error != null && RegExp(r'^[a-z0-9_.]{1,64}$').hasMatch(error)
    ? error
    : null;

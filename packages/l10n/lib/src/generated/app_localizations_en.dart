// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get triggerStateReceived => 'Received';

  @override
  String get triggerStateQueued => 'Queued';

  @override
  String get triggerStateWorking => 'Working';

  @override
  String get triggerStateDone => 'Done';

  @override
  String get triggerStateFailed => 'Failed';

  @override
  String get triggerStateFollowupDelivered => 'Follow-up delivered';

  @override
  String triggerStateFailedWithReason(String reason) {
    return 'Failed: $reason';
  }

  @override
  String get liveBoxStateQueued => 'Queued';

  @override
  String get liveBoxStateRunning => 'Running';

  @override
  String get liveBoxStateWaitingForYou => 'Waiting for you';

  @override
  String get liveBoxStateCompleted => 'Done';

  @override
  String get liveBoxStateFailed => 'Failed';

  @override
  String get liveBoxStateCancelled => 'Canceled';

  @override
  String liveBoxStateWaitingFor(String name) {
    return 'Waiting for $name';
  }

  @override
  String get liveBoxReasonBillingModeMismatch => 'Backend does not match';

  @override
  String get liveBoxReasonAccessRevoked => 'Access revoked';

  @override
  String get liveBoxReasonStale => 'Canceled after stall';

  @override
  String get liveBoxReasonAdapterError => 'Error in the adapter';

  @override
  String get liveBoxReasonCancelTimeout =>
      'Adapter did not confirm the cancellation';

  @override
  String get liveBoxReasonProjectNotReady => 'Project not ready';

  @override
  String liveBoxReasonUserCancelled(String name) {
    return 'Canceled by $name';
  }

  @override
  String get liveBoxStop => 'Stop';

  @override
  String liveBoxEarlierSteps(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count earlier steps',
      one: '1 earlier step',
    );
    return '$_temp0';
  }

  @override
  String liveBoxStall(int minutes) {
    String _temp0 = intl.Intl.pluralLogic(
      minutes,
      locale: localeName,
      other: 'No progress for $minutes minutes',
      one: 'No progress for 1 minute',
    );
    return '$_temp0';
  }

  @override
  String get accessNotFoundOrNoAccess => 'Not found or no access';

  @override
  String get accessNoAccessAnymore => 'No access anymore';

  @override
  String get messageSending => 'Sending';

  @override
  String get messageSendingDelayed => 'Sending delayed';

  @override
  String get connectionOffline => 'Offline';

  @override
  String get relativeDayToday => 'Today';

  @override
  String get relativeDayYesterday => 'Yesterday';

  @override
  String get problemValidationFailed => 'Some input is not valid';

  @override
  String get problemMalformedRequest => 'The request could not be read';

  @override
  String get problemCursorInvalid =>
      'The list position is no longer valid. Reload the list';

  @override
  String get problemWebhookHostNotAllowed =>
      'The webhook target is not on the allow list';

  @override
  String get problemUnauthenticated => 'Please sign in';

  @override
  String get problemTokenExpired => 'Your sign-in has expired';

  @override
  String get problemSessionRevoked => 'This session was ended';

  @override
  String get problemStepUpRequired => 'Please confirm with your device';

  @override
  String get problemUseDpopNonce => 'Please try again';

  @override
  String get problemInvalidDpopProof => 'The device proof was not accepted';

  @override
  String get problemForbidden => 'You are not allowed to do this';

  @override
  String get problemScopeMissing => 'The app lacks a required scope';

  @override
  String get problemDevicePending => 'This device is waiting for approval';

  @override
  String get problemChannelReadOnly => 'This channel is read-only';

  @override
  String get problemFeatureDisabled => 'This feature is turned off';

  @override
  String get problemAgentInvocationDenied => 'The agent cannot be started here';

  @override
  String get problemTerminalDenied => 'The terminal cannot be opened here';

  @override
  String get problemInvocationTokenInvalid =>
      'The call token of the agent is not valid';

  @override
  String get problemNotFound => 'Not found or no access';

  @override
  String get problemMethodNotAllowed => 'This request is not supported';

  @override
  String get problemConflict => 'This conflicts with the current state';

  @override
  String get problemRequestInProgress => 'The request is still in progress';

  @override
  String get problemI1Violation =>
      'This change would let others use a personal agent';

  @override
  String get problemI6Violation =>
      'Only the holder of the personal LLM login may open a terminal on this host';

  @override
  String get problemLastOwner => 'At least one active owner must remain';

  @override
  String get problemAlreadyStarted => 'The run has already started';

  @override
  String get problemBillingModeMismatch =>
      'The backend does not match the billing mode';

  @override
  String get problemToolClassMismatch =>
      'The reported tool class is above the allowed one';

  @override
  String get problemHostUnavailable => 'The host is not reachable';

  @override
  String get problemFingerprintMismatch =>
      'The fingerprint does not match the host';

  @override
  String get problemInvalidStateTransition =>
      'This change is not possible in the current state';

  @override
  String get problemBaseVersionConflict => 'The run changed in the meantime';

  @override
  String get problemComponentUsed =>
      'This element was already used, has expired or is turned off';

  @override
  String get problemOperatorMismatch =>
      'The target channel has a different operator';

  @override
  String get problemFileNotReady => 'The file is not ready yet';

  @override
  String get problemFileBlocked => 'The file is blocked and is never delivered';

  @override
  String get problemTitleManual => 'The title was set by hand';

  @override
  String get problemResyncRequired => 'A full reload is needed';

  @override
  String get problemMessageDeleted => 'The message was deleted';

  @override
  String get problemGone => 'This has expired';

  @override
  String get problemVersionConflict =>
      'Someone else changed this in the meantime';

  @override
  String get problemPayloadTooLarge => 'The content is too large';

  @override
  String get problemUnsupportedMediaType => 'This file type is not allowed';

  @override
  String get problemRangeNotSatisfiable =>
      'The requested part lies outside the file';

  @override
  String get problemIdempotencyKeyReuse =>
      'The request was repeated with different content';

  @override
  String get problemPreconditionRequired => 'The version check is missing';

  @override
  String get problemRateLimited => 'Too many requests. Please wait a moment';

  @override
  String get problemInternalError => 'Something went wrong on the server';

  @override
  String get problemUnavailable => 'The server is not available right now';

  @override
  String get problemMaintenance =>
      'Maintenance: changes are paused, reading continues';

  @override
  String get problemSetupRequired => 'The instance is not set up yet';

  @override
  String get problemForbiddenCapabilityMissing =>
      'You lack an extra permission for this';

  @override
  String get problemForbiddenLevelInsufficient =>
      'Your access level is too low for this';

  @override
  String get problemForbiddenOwnerOnly => 'Only an owner can do this';

  @override
  String get problemForbiddenViewAsReadOnly =>
      'Read only while you view as another person';

  @override
  String get problemChannelReadOnlyChannelArchived =>
      'This channel is archived';

  @override
  String get problemAgentInvocationDeniedProjectNotReady =>
      'The project of the agent is not ready';

  @override
  String problemFallback(String code) {
    return 'Something went wrong ($code)';
  }

  @override
  String get messageShowMore => 'Show more';

  @override
  String get messageEdited => '(edited)';

  @override
  String get messageDeleted => 'Message deleted';

  @override
  String get messageResend => 'Resend';

  @override
  String get messageEdit => 'Edit';

  @override
  String get messageDiscard => 'Discard';

  @override
  String get messageActionReply => 'Reply';

  @override
  String get messageActionReact => 'React';

  @override
  String get messageActionPin => 'Pin';

  @override
  String get messageActionMore => 'More';

  @override
  String get messageMentionsYou => 'Mentions you';

  @override
  String messageSemanticsLabel(String name, String time, String text) {
    return '$name, $time, $text';
  }

  @override
  String get messageOriginAgent => 'AGENT';

  @override
  String get messageOriginApp => 'APP';

  @override
  String get messageOriginImport => 'IMPORT';

  @override
  String get messageOriginSystem => 'SYSTEM';

  @override
  String messageReplyTo(String name) {
    return 'Reply to $name';
  }

  @override
  String messageAttachment(String name) {
    return 'Attachment $name';
  }

  @override
  String messageReaction(int count, String emoji) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count reactions $emoji',
      one: '1 reaction $emoji',
    );
    return '$_temp0';
  }

  @override
  String get messageVoicePlay => 'Play voice message';

  @override
  String get messageVoicePause => 'Pause voice message';

  @override
  String get messageVoiceTranscriptPending => 'Transcript pending';

  @override
  String messageSystemThreadReopened(String name) {
    return 'Reopened by message from $name';
  }

  @override
  String get messageSystemResumeFailed =>
      'The agent session could not be resumed';

  @override
  String messageSystemUnknown(String code) {
    return 'System message ($code)';
  }

  @override
  String get triggerStateCancelled => 'Canceled';

  @override
  String triggerStateCancelledWithReason(String reason) {
    return 'Canceled: $reason';
  }

  @override
  String triggerStateUnknown(String code) {
    return 'State $code';
  }

  @override
  String liveBoxStateUnknown(String code) {
    return 'State $code';
  }

  @override
  String liveBoxStopRun(String agent) {
    return 'Stop run of $agent';
  }

  @override
  String liveBoxSemanticsLabel(
    String agent,
    String state,
    String duration,
    int percent,
  ) {
    return 'Run of $agent: $state, for $duration, context $percent percent';
  }

  @override
  String get liveBoxFollowupHint =>
      'Messages in this thread reach the agent as a follow-up';

  @override
  String liveBoxTokens(String count) {
    return '$count tokens';
  }

  @override
  String liveBoxContext(String percent) {
    return 'Context $percent';
  }

  @override
  String liveBoxRan(String duration) {
    return 'Ran $duration';
  }

  @override
  String liveBoxTools(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count tools',
      one: '1 tool',
    );
    return '$_temp0';
  }

  @override
  String liveBoxSubagents(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count subagents',
      one: '1 subagent',
    );
    return '$_temp0';
  }

  @override
  String get liveBoxExpand => 'Show run details';

  @override
  String get liveBoxCollapse => 'Hide run details';

  @override
  String get liveBoxOpen => 'Open live box';

  @override
  String liveBoxAnnounceQuestion(String agent) {
    return 'Question from $agent';
  }

  @override
  String liveBoxAnnounceState(String agent, String state) {
    return 'Run of $agent: $state';
  }

  @override
  String get toolChipWaiting => 'waiting';

  @override
  String toolChipRunning(String duration) {
    return 'running $duration';
  }

  @override
  String get toolChipFailed => 'failed';

  @override
  String get toolChipCancelled => 'canceled';

  @override
  String get toolChipCompleted => 'completed';

  @override
  String toolChipUnknownStatus(String code) {
    return 'status $code';
  }

  @override
  String get toolChipKindToolCall => 'Tool';

  @override
  String get toolChipKindStatus => 'Status';

  @override
  String get toolChipKindSubagent => 'Subagent';

  @override
  String get toolChipKindWorkflow => 'Workflow';

  @override
  String get toolChipKindBackgroundJob => 'Background job';

  @override
  String get toolChipKindUnknown => 'Step';

  @override
  String toolChipLabel(String kind, String name, String status) {
    return '$kind $name, $status';
  }

  @override
  String get toolChipDetails => 'Show details';

  @override
  String get threadRowWaitingForYou => 'Waiting for you';

  @override
  String get threadRowStateOpen => 'Open';

  @override
  String get threadRowStateDone => 'Done';

  @override
  String threadRowUnread(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count unread',
      one: '1 unread',
      zero: 'no unread',
    );
    return '$_temp0';
  }

  @override
  String threadRowMentions(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count mentions',
      one: '1 mention',
    );
    return '$_temp0';
  }

  @override
  String threadRowLabel(
    String title,
    String state,
    String time,
    String unread,
  ) {
    return '$title, $state, $time, $unread';
  }

  @override
  String threadRowOperator(String name) {
    return 'Operator $name';
  }

  @override
  String get channelRowRunActive => 'run active';

  @override
  String get channelRowMuted => 'muted';

  @override
  String get spaceRowUnread => 'Unread';

  @override
  String spaceRowExpand(String name) {
    return 'Expand $name';
  }

  @override
  String spaceRowCollapse(String name) {
    return 'Collapse $name';
  }

  @override
  String componentsTextFrom(String agent) {
    return 'Text from $agent';
  }

  @override
  String componentsHeaderRequest(String agent) {
    return 'Request from $agent';
  }

  @override
  String componentsHeaderRun(String run) {
    return 'Run $run';
  }

  @override
  String componentsHeaderCapability(String capability) {
    return 'Requires the permission \'$capability\'';
  }

  @override
  String componentsApproverRunStarter(String name) {
    return 'Only $name or an owner can approve this';
  }

  @override
  String get componentsApproverOwner => 'Only an owner can approve this';

  @override
  String get componentsApproverAny =>
      'Anyone with access to this channel can answer';

  @override
  String componentsDeniedCapability(String capability) {
    return 'You do not have the permission \'$capability\' for this';
  }

  @override
  String get capabilityApproveActions => 'Approve actions';

  @override
  String componentsUsed(String name, String choice, String time) {
    return '$name chose \'$choice\', $time';
  }

  @override
  String get componentsExpired => 'Expired';

  @override
  String get componentsDisabled => 'No longer available';

  @override
  String get componentsSending => 'Sending your choice';

  @override
  String get componentsSubmit => 'Submit';

  @override
  String get componentsSelectPlaceholder => 'Choose';

  @override
  String get componentsFieldRequired => 'Required';

  @override
  String componentsFieldTooLong(int max) {
    return 'At most $max characters';
  }

  @override
  String get componentsFieldNumber => 'Enter a number';

  @override
  String get componentsUnsupported => 'This element needs a newer app';

  @override
  String composerMessageTo(String target) {
    return 'Message to $target';
  }

  @override
  String composerFollowupTo(String agent) {
    return 'Follow-up to $agent';
  }

  @override
  String composerTyping(String name) {
    return '$name is typing';
  }

  @override
  String get composerTypingSeveral => 'Several people are typing';

  @override
  String get composerSend => 'Send';

  @override
  String get composerAttach => 'Attach file';

  @override
  String get composerVoice => 'Record voice message';

  @override
  String composerOptionMissing(String option) {
    return 'Missing option: $option';
  }

  @override
  String get composerOffline => 'Offline: your message waits in the queue';

  @override
  String get composerReadOnly => 'Read only in this channel';

  @override
  String composerSuggestions(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count suggestions',
      one: '1 suggestion',
      zero: 'No suggestions',
    );
    return '$_temp0';
  }

  @override
  String get composerControlCommand => 'Control command';

  @override
  String get billingPersonal => 'Personal';

  @override
  String get billingOrganization => 'Organization';

  @override
  String billingUnknown(String code) {
    return 'Backend $code';
  }

  @override
  String billingReported(String mode) {
    return 'Reported: $mode';
  }

  @override
  String billingLabelPersonal(String name) {
    return 'Backend: Personal, runs on the subscription of $name';
  }

  @override
  String get billingLabelOrganization => 'Backend: Organization';

  @override
  String get costEstimate => 'Estimate';

  @override
  String get costInformationOnly => 'for information only';

  @override
  String markdownImage(String alt, String domain) {
    return 'Image: $alt ($domain)';
  }

  @override
  String markdownImageNoDomain(String alt) {
    return 'Image: $alt';
  }

  @override
  String get markdownCopyCode => 'Copy code';
}

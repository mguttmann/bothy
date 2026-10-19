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
  String get vizMetricMessagesCreated => 'Messages';

  @override
  String get vizMetricThreadsCreated => 'New threads';

  @override
  String get vizMetricRunsStarted => 'Runs started';

  @override
  String get vizMetricRunsFinished => 'Runs finished';

  @override
  String get vizMetricToolCalls => 'Tool calls';

  @override
  String get vizMetricApprovalCards => 'Approval cards';

  @override
  String get vizMetricCost => 'Costs';

  @override
  String get vizMetricInvocationsDenied => 'Denied invocations';

  @override
  String get vizMetricBillingModeSwitches => 'Billing mode switches';

  @override
  String get vizMetricCatchups => 'Catch-ups';

  @override
  String get vizMetricResyncs => 'Resyncs';

  @override
  String get vizMetricSlowConsumerCloses => 'Closes of slow clients';

  @override
  String vizMetricUnknown(String code) {
    return 'Metric $code';
  }

  @override
  String get vizSliApiAvailability => 'API availability';

  @override
  String get vizSliDeliveryLatency => 'Delivery latency';

  @override
  String get vizSliReconnect => 'Reconnect';

  @override
  String get vizSliPushAcceptance => 'Push acceptance';

  @override
  String get vizSliRunStart => 'Run start';

  @override
  String get vizSliTurnDuration => 'Turn duration';

  @override
  String get vizSloOnTarget => 'on target';

  @override
  String get vizSloBudgetTight => 'budget tight';

  @override
  String get vizSloTargetMissed => 'target missed';

  @override
  String get vizSloNotEnoughData => 'not enough data';

  @override
  String get vizOwnMeasurement => 'Own measurement';

  @override
  String vizTrafficLightLabel(String slo, String state) {
    return '$slo: $state, own measurement';
  }

  @override
  String vizTrafficLightChanged(String slo, String state) {
    return '$slo is now $state';
  }

  @override
  String get vizCostEstimate => 'Estimate';

  @override
  String get vizCostInformationOnly => 'information only';

  @override
  String get vizBadgePersonal => 'Personal';

  @override
  String get vizBadgeOrganization => 'Organization';

  @override
  String get vizApprovalWaiting => 'Approval waiting';

  @override
  String get vizMoreRunsThanShown => 'More runs than shown';

  @override
  String get vizRunStateWaitingInput => 'Waiting for input';

  @override
  String get vizRunStateUnknown => 'Unknown state';

  @override
  String vizStepToolCall(String name) {
    return 'Tool $name';
  }

  @override
  String vizStepStatus(String name) {
    return 'Status $name';
  }

  @override
  String vizStepSubagent(String name) {
    return 'Subagent $name';
  }

  @override
  String vizStepWorkflow(String name) {
    return 'Workflow $name';
  }

  @override
  String vizStepBackgroundJob(String name) {
    return 'Background job $name';
  }

  @override
  String vizLaneLabel(String space, String channel, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count active runs',
      one: '1 active run',
      zero: 'no active runs',
    );
    return '$space, $channel, $_temp0';
  }

  @override
  String vizBarRowLabel(String row, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count runs',
      one: '1 run',
    );
    return '$row, $_temp0';
  }

  @override
  String get vizChartRates => 'Rates';

  @override
  String get vizChartRuns => 'Runs';

  @override
  String vizTileUp(String delta) {
    return '$delta more than the previous period';
  }

  @override
  String vizTileDown(String delta) {
    return '$delta less than the previous period';
  }

  @override
  String get vizTileSame => 'same as the previous period';

  @override
  String get vizNotShown => 'not shown';

  @override
  String get vizStale => 'No new data';

  @override
  String get vizHarnessTitle => 'Visualization harness';

  @override
  String get vizHarnessTheme => 'Theme';

  @override
  String get vizHarnessThemeLight => 'Light';

  @override
  String get vizHarnessThemeDark => 'Dark';

  @override
  String get vizHarnessDensity => 'Density';

  @override
  String get vizHarnessDensityComfortable => 'Comfortable';

  @override
  String get vizHarnessDensityCompact => 'Compact';

  @override
  String get vizHarnessDensityTouch => 'Touch';

  @override
  String get vizHarnessTextScale => 'Text size';

  @override
  String get vizHarnessLanguage => 'Language';

  @override
  String get vizHarnessFixtureState => 'Fixture state';

  @override
  String get vizHarnessTimeLapse => 'Time-lapse';

  @override
  String get vizMarkLane => 'Lane';

  @override
  String get vizMarkRunCard => 'Run card';

  @override
  String get vizMarkArea => 'Area';

  @override
  String get vizMarkBar => 'Bar';

  @override
  String get vizMarkTrafficLight => 'Traffic light';

  @override
  String get vizMarkMetricTile => 'Metric tile';

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

  @override
  String get triggerReasonMultipleAgentsMentioned =>
      'Please mention only one agent.';

  @override
  String get triggerReasonAgentNotAllowedHere =>
      'This agent does not answer here.';

  @override
  String get triggerReasonNoTriggerRight => 'You may not trigger agents here.';

  @override
  String get triggerReasonRunActiveOtherAgent =>
      'Another agent is currently working in this thread.';

  @override
  String get triggerReasonAgentDisabled => 'Agent is switched off.';

  @override
  String get triggerReasonBillingModePersonalNotHolder =>
      'This agent uses a personal subscription.';

  @override
  String get triggerReasonVoiceUnsupported =>
      'Voice messages are not understood here.';

  @override
  String get triggerReasonRunActive =>
      'The agent is currently working; send the command again after the run.';

  @override
  String get liveBoxReasonUserCancelledNoName => 'Canceled by a person';

  @override
  String get threadRowStateWaiting => 'Waiting for an answer';

  @override
  String get vizViewLiveTopology => 'Live topology';

  @override
  String get vizViewBackend => 'Backend view';

  @override
  String get vizViewDelivery => 'Delivery and sync live';

  @override
  String get vizViewExecutive => 'Executive page';

  @override
  String get vizViewTimeline => 'Daily timeline';

  @override
  String get vizViewTv => 'TV mode';

  @override
  String get vizAsTable => 'As table';

  @override
  String get vizOpenGrafana => 'Open in Grafana';

  @override
  String get vizLoading => 'Loading';

  @override
  String get vizOnlyForOwner => 'Only for the owner';

  @override
  String vizSortBy(String column) {
    return 'Sort by $column';
  }

  @override
  String vizFrameTime(String time) {
    return 'As of $time';
  }

  @override
  String get vizNoActiveRuns => 'No active runs';

  @override
  String get vizRatesLastMinute => 'Last minute';

  @override
  String get vizNone => 'none';

  @override
  String get vizColSpace => 'Space';

  @override
  String get vizColChannel => 'Channel';

  @override
  String get vizColAgent => 'Agent';

  @override
  String get vizColState => 'State';

  @override
  String get vizColStep => 'Current step';

  @override
  String get vizColMode => 'Billing mode';

  @override
  String get vizColApproval => 'Approval';

  @override
  String get vizColCost => 'Cost so far';

  @override
  String get vizColMetric => 'Metric';

  @override
  String get vizColLastMinute => 'Last minute';

  @override
  String get vizColValue => 'Value';

  @override
  String get vizColNote => 'Note';

  @override
  String get vizColPresence => 'Presence';

  @override
  String get vizColConfiguredMode => 'Configured mode';

  @override
  String get vizColReportedMode => 'Reported mode';

  @override
  String get vizColSupportedModes => 'Supported modes';

  @override
  String get vizColSwitches => 'Mode switches';

  @override
  String get vizColI2Denied => 'Denied before delivery (I2)';

  @override
  String get vizColI3Denied => 'Failed on mode mismatch (I3)';

  @override
  String get vizColCosts => 'Costs per mode';

  @override
  String get vizColRateLimit => 'Rate limit';

  @override
  String get vizPresenceOnline => 'Online';

  @override
  String get vizPresenceDegraded => 'Degraded';

  @override
  String get vizPresenceOffline => 'Offline';

  @override
  String get vizPresenceUnknown => 'Unknown';

  @override
  String get vizFollowsRun => 'per run';

  @override
  String vizReportedMismatch(String mode) {
    return 'Reported: $mode';
  }

  @override
  String get vizRateLimitOk => 'within the limit';

  @override
  String get vizRateLimitNearLimit => 'near the limit';

  @override
  String get vizRateLimitLimited => 'limited';

  @override
  String get vizRateLimitUnknown => 'unknown';

  @override
  String vizRateLimitResets(String time) {
    return 'resets at $time';
  }

  @override
  String get vizSliNoEvents => 'no events';

  @override
  String vizSliValue(int count, String share) {
    final intl.NumberFormat countNumberFormat =
        intl.NumberFormat.decimalPattern(localeName);
    final String countString = countNumberFormat.format(count);

    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$countString events',
      one: '1 event',
    );
    return '$_temp0, $share within the threshold';
  }

  @override
  String vizCatchupValue(String count, String millis) {
    return '$count, together $millis ms';
  }

  @override
  String get vizConnectionsUser => 'Connections of people';

  @override
  String get vizConnectionsAgent => 'Connections of agents';

  @override
  String get vizConnectionsApp => 'Connections of apps';

  @override
  String get vizChartHistory => 'History';

  @override
  String get vizTileUsage => 'Usage';

  @override
  String get vizTileRuns => 'Runs and error rate';

  @override
  String get vizTileCosts => 'Costs per mode';

  @override
  String get vizTileSlo => 'SLO traffic light';

  @override
  String get vizColSlo => 'SLO';

  @override
  String get vizOwnMeasurementNote =>
      'Values can differ from Prometheus: the own measurement counts in the process and can lose one flush interval in a crash.';

  @override
  String get vizColRun => 'Agent and channel';

  @override
  String get vizColStart => 'Start';

  @override
  String get vizColEnd => 'End';

  @override
  String vizPlayhead(String time) {
    return 'Playhead $time';
  }

  @override
  String get vizPlay => 'Play';

  @override
  String get vizPause => 'Pause';

  @override
  String vizSpeed(int speed) {
    return '${speed}x';
  }

  @override
  String get vizJump => 'Jump to a minute';

  @override
  String vizTvPosition(String title, int index, int count) {
    return '$title ($index of $count)';
  }

  @override
  String get consoleNavLive => 'Live';

  @override
  String get consoleNavStats => 'Statistics';

  @override
  String get consoleNavHosts => 'Hosts and terminal';

  @override
  String get consoleNavPeople => 'People';

  @override
  String get consoleNavGroups => 'Groups and access grants';

  @override
  String get consoleNavDevices => 'Devices';

  @override
  String get consoleNavAgents => 'Agents and costs';

  @override
  String get consoleNavApps => 'Apps and webhooks';

  @override
  String get consoleNavAudit => 'Audit';

  @override
  String get consoleNavImport => 'Import';

  @override
  String get consoleNavInstance => 'Instance and branding';

  @override
  String get consoleNavStatus => 'Status';

  @override
  String get consoleNavigationLabel => 'Console navigation';

  @override
  String consoleTitle(String product) {
    return '$product console';
  }

  @override
  String get consolePlaceholderPage =>
      'This page comes with a later version of the console.';

  @override
  String consoleSignedInAs(String name) {
    return 'Signed in: $name';
  }

  @override
  String get consoleLogout => 'Log out';

  @override
  String consoleThreadInApp(String id) {
    return 'Thread $id: there is no chat in the browser. Open it in the app.';
  }

  @override
  String get consoleCopyThreadId => 'Copy thread ID';

  @override
  String get consoleCopied => 'Copied';

  @override
  String get consoleWorking => 'One moment';

  @override
  String consoleLoginTitle(String product) {
    return 'Log in to $product';
  }

  @override
  String consoleLoginWithProvider(String provider) {
    return 'Log in with $provider';
  }

  @override
  String get consoleLoginOr => 'or';

  @override
  String get consoleLoginName => 'Login name';

  @override
  String get consoleLoginWithPasskey => 'Log in with passkey';

  @override
  String get consoleLoginPasswordAndCode => 'Password and one-time code';

  @override
  String get consoleLoginPassword => 'Password';

  @override
  String get consoleLoginOneTimeCode => 'One-time code';

  @override
  String get consoleLoginSubmit => 'Log in';

  @override
  String get consoleLoginNoMethod =>
      'No login is set up for this instance. Ask the owner to set one up.';

  @override
  String get consoleLoginFailed =>
      'The login failed. Check the login name, password and one-time code and try again.';

  @override
  String get consoleLoginPasskeyFailed =>
      'The passkey was not accepted. Try again or choose another way.';

  @override
  String get consoleLoginOidcFailed =>
      'The login service did not confirm the login. Start the login again.';

  @override
  String get consoleLoginOidcCancelled =>
      'The login was cancelled at the login service. Start it again or choose another way.';

  @override
  String get consoleLoginOidcExpired =>
      'The login took too long. Start it again.';

  @override
  String get consoleLoginUnavailable =>
      'The server cannot be reached. Check the connection and try again.';

  @override
  String get consoleCallbackWorking => 'Completing the login';

  @override
  String get consoleStepUpTitle => 'Confirm that it is you';

  @override
  String get consoleStepUpExplanation =>
      'The console asks for a fresh confirmation before any administration.';

  @override
  String get consoleStepUpPasskey => 'Confirm with passkey';

  @override
  String consoleStepUpOidc(String provider) {
    return 'Log in again with $provider';
  }

  @override
  String get consoleStepUpPassword => 'Confirm with password and one-time code';

  @override
  String get consoleStepUpConfirm => 'Confirm';

  @override
  String get consoleStepUpFailed =>
      'The confirmation failed. Try again or choose another way.';

  @override
  String get consoleStepUpNoMethod =>
      'No way to confirm is set up for you. Ask the owner to set up a passkey for you.';

  @override
  String get consoleLoggedOutTitle => 'You are logged out.';

  @override
  String consoleLoggedOutReason(String reason) {
    return 'Reason: $reason';
  }

  @override
  String get consoleLoggedOutByYou => 'You logged out of the console.';

  @override
  String get consoleLogInAgain => 'Log in again';

  @override
  String get consoleUpdatedReload =>
      'The console was updated. Reload the page to continue.';

  @override
  String get consoleReload => 'Reload';

  @override
  String get onboardingConnectTitle => 'Connect to your instance';

  @override
  String get onboardingServerAddress => 'Server address';

  @override
  String get onboardingConnect => 'Connect';

  @override
  String get onboardingConnecting => 'Connecting';

  @override
  String get onboardingAddressInvalid =>
      'Enter the full address of the instance, for example https://chat.example.com.';

  @override
  String get onboardingNoInstance =>
      'No instance at this address. Check the address and try again.';

  @override
  String get onboardingCertificateInvalid =>
      'Certificate invalid. The connection is not secure, so the app does not connect. Ask the owner of the instance to renew the certificate.';

  @override
  String get onboardingNotSetUp =>
      'Instance not set up yet. The owner sets it up on the host with the setup wizard (install.sh).';

  @override
  String onboardingAppTooOld(String version) {
    return 'App too old. This instance needs version $version or newer. Install the update, then connect again.';
  }

  @override
  String onboardingLoginTitle(String productName) {
    return 'Log in to $productName';
  }

  @override
  String onboardingLoginWithProvider(String name) {
    return 'Log in with $name';
  }

  @override
  String get onboardingOr => 'or';

  @override
  String get onboardingLoginName => 'Login name';

  @override
  String get onboardingLoginWithPasskey => 'Log in with passkey';

  @override
  String get onboardingPasswordAndCode => 'Password and one-time code';

  @override
  String get onboardingPassword => 'Password';

  @override
  String get onboardingOneTimeCode => 'One-time code';

  @override
  String get onboardingLogIn => 'Log in';

  @override
  String get onboardingEnrollmentCodeLink => 'I have an enrollment code';

  @override
  String get onboardingPairLink => 'Pair with another device';

  @override
  String get onboardingNoMethod =>
      'No login is set up for this instance. Ask the owner of the instance.';

  @override
  String onboardingServerHost(String address) {
    return 'Server: $address';
  }

  @override
  String get onboardingChangeServer => 'Change';

  @override
  String get onboardingBack => 'Back';

  @override
  String get onboardingLoginFailed =>
      'Login failed. Check your details and try again.';

  @override
  String get onboardingUnreachable =>
      'The instance is not reachable. Check the network and try again.';

  @override
  String get onboardingBrowserCancelled =>
      'The login in the browser was cancelled. Start it again.';

  @override
  String get onboardingBrowserExpired =>
      'The login in the browser took too long. Start it again.';

  @override
  String get onboardingEnrollTitle => 'Enrollment code';

  @override
  String get onboardingEnrollCode => 'Code from the owner';

  @override
  String get onboardingEnrollHow => 'How do you want to log in?';

  @override
  String get onboardingEnrollThisDevice => 'Use this device';

  @override
  String get onboardingEnrollCreatePasskey => 'Create passkey';

  @override
  String onboardingEnrollLink(String name) {
    return 'Link with $name';
  }

  @override
  String get onboardingContinue => 'Continue';

  @override
  String get onboardingEnrollOnce => 'The code is valid once.';

  @override
  String get onboardingPendingTitle => 'Device waiting for confirmation';

  @override
  String get onboardingPendingExplain =>
      'Type this code on one of your active devices: Settings, Devices and sessions, Confirm. No active device: ask the owner.';

  @override
  String get onboardingCancel => 'Cancel';

  @override
  String onboardingValidFor(int minutes) {
    String _temp0 = intl.Intl.pluralLogic(
      minutes,
      locale: localeName,
      other: 'Valid for $minutes more minutes',
      one: 'Valid for 1 more minute',
      zero: 'Valid for less than a minute',
    );
    return '$_temp0';
  }

  @override
  String get onboardingPairTitle => 'Pair device';

  @override
  String get onboardingPairExplain =>
      'On an active device: Settings, Devices, Add device, then enter:';

  @override
  String get onboardingPairNewCode => 'New code';

  @override
  String get onboardingPairExpired => 'The code has expired. Get a new code.';

  @override
  String onboardingLockedTitle(String productName) {
    return '$productName is locked';
  }

  @override
  String get onboardingUnlock => 'Unlock';

  @override
  String get onboardingUnlockFailed => 'Unlocking failed.';

  @override
  String get onboardingTryAgain => 'Try again';

  @override
  String get onboardingLogOut => 'Log out';

  @override
  String get onboardingSignedOutTitle => 'You are logged out.';

  @override
  String onboardingSignedOutReason(String reason) {
    return 'Reason: $reason';
  }

  @override
  String get onboardingReasonLoggedOut => 'You logged out.';

  @override
  String get onboardingReasonRevokedBySelf =>
      'This device was revoked by another device.';

  @override
  String get onboardingReasonRevokedByOwner => 'The owner revoked this device.';

  @override
  String get onboardingReasonUserDisabled => 'Your account has been disabled.';

  @override
  String get onboardingReasonInactive =>
      'This device was not used for a long time.';

  @override
  String get onboardingReasonRejected =>
      'The confirmation of this device was rejected.';

  @override
  String get onboardingReasonPendingExpired =>
      'This device was not confirmed in time.';

  @override
  String get onboardingReasonSessionRevoked =>
      'Your session was ended on the server.';

  @override
  String get onboardingReasonSessionExpired => 'Your session has expired.';

  @override
  String get onboardingDataDeleted =>
      'The data of this device has been deleted.';

  @override
  String get onboardingLogInAgain => 'Log in again';

  @override
  String get onboardingContactOwner => 'Contact the owner of the instance.';

  @override
  String get onboardingCopy => 'Copy';

  @override
  String get onboardingCopied => 'Copied';

  @override
  String onboardingCodeLabel(String characters) {
    return 'Code: $characters';
  }

  @override
  String get onboardingLoopbackDone =>
      'The login is complete. You can close this window.';

  @override
  String get shellColumnSidebar => 'Spaces and channels';

  @override
  String get shellColumnPostList => 'Post list';

  @override
  String get shellColumnHistory => 'History';

  @override
  String get shellColumnSession => 'Session';

  @override
  String get shellResizeColumn => 'Resize column';

  @override
  String get shellUpdateRequired =>
      'Update required. Install the new version to connect again.';

  @override
  String get shellStepUpTitle => 'Confirm it is you';

  @override
  String get stepUpGenericPurpose => 'This action needs your confirmation.';

  @override
  String get stepUpNext =>
      'Next, your device asks for your fingerprint, face or device password.';

  @override
  String get stepUpContinue => 'Continue';

  @override
  String get selfCancel => 'Cancel';

  @override
  String get selfOffline => 'No connection. Try again when you are online.';

  @override
  String get selfLoadFailed => 'The list could not be loaded.';

  @override
  String get selfRetry => 'Try again';

  @override
  String stepUpPurposeApproveDevice(String name) {
    return 'Confirm the waiting device $name';
  }

  @override
  String get stepUpPurposeAddDevice => 'Add a new device to your account';

  @override
  String stepUpPurposeRevokeDevice(String name) {
    return 'Revoke the device $name';
  }

  @override
  String get devicesTitle => 'Devices';

  @override
  String get devicesAdd => 'Add device';

  @override
  String get devicesThisDevice => 'This device';

  @override
  String get devicesStatePending => 'Waiting for confirmation';

  @override
  String get devicesStateRevoked => 'Revoked';

  @override
  String devicesAppVersion(String version) {
    return 'Version $version';
  }

  @override
  String devicesLastActive(String when) {
    return 'Last active $when';
  }

  @override
  String get devicesPlatformIos => 'iPhone or iPad';

  @override
  String get devicesPlatformAndroid => 'Android';

  @override
  String get devicesPlatformMacos => 'Mac';

  @override
  String get devicesPlatformWindows => 'Windows';

  @override
  String get devicesPlatformWeb => 'Web console';

  @override
  String get devicesPlatformCli => 'Command line';

  @override
  String get devicesPlatformOther => 'Other platform';

  @override
  String get devicesEnterCode => 'Enter code';

  @override
  String get devicesRevoke => 'Revoke';

  @override
  String devicesRevokeTitle(String name) {
    return 'Revoke $name?';
  }

  @override
  String get devicesRevokeBody =>
      'The device is signed out and deletes its local data at its next contact.';

  @override
  String get devicesRevokeThisBody =>
      'You are signed out on this device and its local data is deleted.';

  @override
  String get devicesRevokeReason => 'Reason (optional)';

  @override
  String get devicesRevokeDefaultReason => 'Revoked in the settings';

  @override
  String devicesRevoked(String name) {
    return '$name was revoked.';
  }

  @override
  String devicesConfirmTitle(String name) {
    return 'Confirm $name';
  }

  @override
  String get devicesConfirmHint =>
      'Type the code that the waiting device shows.';

  @override
  String get devicesConfirmCode => 'Confirmation code';

  @override
  String get devicesConfirmSubmit => 'Confirm';

  @override
  String devicesConfirmed(String name) {
    return '$name is now active.';
  }

  @override
  String get devicesConfirmRejected =>
      'The waiting device was rejected because the code did not match.';

  @override
  String get devicesAddTitle => 'Add device';

  @override
  String get devicesAddHint =>
      'Type the pairing code that the new device shows.';

  @override
  String get devicesAddCode => 'Pairing code';

  @override
  String get devicesAddSubmit => 'Add';

  @override
  String devicesAdded(String name) {
    return '$name was added.';
  }

  @override
  String get devicesPairingNotFound => 'No open pairing with this code.';

  @override
  String get sessionsTitle => 'Sessions';

  @override
  String get sessionsCurrent => 'This session';

  @override
  String get sessionsUnknownDevice => 'Unknown device';

  @override
  String sessionsLastUsed(String when) {
    return 'Last used $when';
  }

  @override
  String get sessionsMethodOidc => 'Login service';

  @override
  String get sessionsMethodTrustedHeader => 'Login through the network';

  @override
  String get sessionsMethodPasskey => 'Passkey';

  @override
  String get sessionsMethodPasswordTotp => 'Password and one-time code';

  @override
  String get sessionsMethodEnrollmentCode => 'Enrollment code';

  @override
  String get sessionsMethodDevicePairing => 'Pairing';

  @override
  String get sessionsMethodOther => 'Other method';

  @override
  String get sessionsEnd => 'End session';

  @override
  String get sessionsEndTitle => 'End this session?';

  @override
  String get sessionsEndBody => 'The device of this session is signed out.';

  @override
  String get sessionsEndCurrentBody =>
      'You are signed out on this device and its local data is deleted.';

  @override
  String get sessionsEnded => 'The session was ended.';

  @override
  String get settingsTitle => 'Settings';

  @override
  String get settingsBack => 'Back';

  @override
  String get settingsSectionAppearance => 'Appearance';

  @override
  String get settingsSectionNotifications => 'Notifications';

  @override
  String get settingsSectionLanguage => 'Language';

  @override
  String get settingsSectionKeyboard => 'Keyboard';

  @override
  String get settingsSectionDevices => 'Devices and sessions';

  @override
  String get settingsSectionPrivacy => 'Privacy';

  @override
  String get settingsSectionAbout => 'About';

  @override
  String get settingsTheme => 'Theme';

  @override
  String get settingsThemeSystem => 'System';

  @override
  String get settingsThemeLight => 'Light';

  @override
  String get settingsThemeDark => 'Dark';

  @override
  String get settingsDensity => 'Density';

  @override
  String get settingsDensityComfortable => 'Normal';

  @override
  String get settingsDensityCompact => 'Compact';

  @override
  String get settingsMotion => 'Motion';

  @override
  String get settingsMotionSystem => 'System';

  @override
  String get settingsMotionFull => 'Full';

  @override
  String get settingsMotionReduced => 'Reduced';

  @override
  String get settingsEnterSends => 'Enter sends';

  @override
  String get settingsEnterSendsHint => 'Shift+Enter: new line';

  @override
  String get settingsColumnWidths => 'Column widths';

  @override
  String get settingsColumnWidthsReset => 'Reset';

  @override
  String get settingsLanguage => 'Language';

  @override
  String get settingsLanguageGerman => 'German';

  @override
  String get settingsLanguageEnglish => 'English';

  @override
  String get settingsTimeZone => 'Time zone';

  @override
  String settingsProfileFailed(String reason) {
    return 'The profile could not be saved: $reason';
  }

  @override
  String settingsShortcuts(String shortcut) {
    return 'All keyboard shortcuts: $shortcut';
  }

  @override
  String get settingsCrashReports => 'Send crash reports';

  @override
  String get settingsCrashReportsHint =>
      'Without content: app version, platform and device type only.';

  @override
  String get settingsClientMetrics => 'Send performance data';

  @override
  String get settingsClientMetricsHint => 'Without content, IDs or names.';

  @override
  String get settingsAboutAppVersion => 'App version';

  @override
  String get settingsAboutServerVersion => 'Server specification';

  @override
  String get settingsAboutLicenses => 'Licenses';

  @override
  String get sidebarJumpTo => 'Jump to';

  @override
  String get sidebarRunningNow => 'Running now';

  @override
  String get sidebarSettings => 'Settings';

  @override
  String sidebarOwnAvatar(String name) {
    return '$name, signed in';
  }

  @override
  String get sidebarMoveUp => 'Move up';

  @override
  String get sidebarMoveDown => 'Move down';

  @override
  String get sidebarNoSpaces => 'No spaces yet';

  @override
  String get postListNewPost => 'New post';

  @override
  String get postListFilterOpen => 'Open';

  @override
  String get postListFilterWaitingForMe => 'Waiting for me';

  @override
  String get postListFilterTags => 'Tags';

  @override
  String postListFilterTagsCount(int count) {
    return 'Tags ($count)';
  }

  @override
  String get postListSortLastActivity => 'Last activity';

  @override
  String get postListSortCreated => 'Newest first';

  @override
  String postListDone(int count) {
    return 'Done ($count)';
  }

  @override
  String postListDoneMore(int count) {
    return 'Done ($count+)';
  }

  @override
  String get postListEmpty => 'No open post here';

  @override
  String postListCreateFailed(String title) {
    return '\"$title\" could not be posted';
  }

  @override
  String get postListTryAgain => 'Try again';

  @override
  String get postListDiscard => 'Discard';

  @override
  String get postListLoadFailed => 'More posts could not be loaded';

  @override
  String get postListEdit => 'Edit';

  @override
  String postListFailureReason(String reason) {
    return 'Reason: $reason';
  }

  @override
  String postListLabel(String forum) {
    return 'Post list of $forum';
  }

  @override
  String get newPostTitle => 'Title';

  @override
  String get newPostMessage => 'First message';

  @override
  String get newPostTags => 'Tags';

  @override
  String get newPostCreate => 'Create post';

  @override
  String get newPostCancel => 'Cancel';

  @override
  String get adminTitle => 'Administration';

  @override
  String get adminBack => 'Back to the chat';

  @override
  String get adminNavPeople => 'People';

  @override
  String get adminNavGroups => 'Groups';

  @override
  String get adminNavDevices => 'Devices';

  @override
  String adminNavDevicesWaiting(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Devices ($count waiting)',
      one: 'Devices (1 waiting)',
      zero: 'Devices',
    );
    return '$_temp0';
  }

  @override
  String get adminReason => 'Reason (stored in the audit log)';

  @override
  String get adminDefaultReason => 'Changed in the administration';

  @override
  String get adminLoadMore => 'Load more';

  @override
  String get adminOneTimeSecret =>
      'The code was shown once and cannot be shown again. Issue a new code.';

  @override
  String get adminI1Explained =>
      'Change the mode of the affected agent under Agents first, then enable the person again.';

  @override
  String get adminPickPerson => 'Choose a person';

  @override
  String get adminRevoke => 'Revoke';

  @override
  String get adminOwnerBadge => 'Owner';

  @override
  String get adminStatusActive => 'Active';

  @override
  String get adminStatusPending => 'Waiting for login';

  @override
  String get adminStatusDisabled => 'Disabled';

  @override
  String get adminStatusDeleted => 'Deleted';

  @override
  String get adminPeopleTitle => 'People';

  @override
  String get adminPeopleAdd => 'Add person';

  @override
  String get adminPeopleSearch => 'Search';

  @override
  String get adminPeopleStatusAll => 'All states';

  @override
  String get adminPeopleGroupAll => 'All groups';

  @override
  String get adminPeopleGroupNote =>
      'The group filter applies only to the people loaded on this page.';

  @override
  String get adminPeopleEmpty => 'No person matches.';

  @override
  String adminPersonCreated(String name) {
    return '$name was created.';
  }

  @override
  String get adminCreatePersonTitle => 'Add person';

  @override
  String get adminCreatePersonBody =>
      'The person gets an account. With a local account, the enrollment code is shown once in the next step.';

  @override
  String get adminCreatePersonName => 'Display name';

  @override
  String get adminCreatePersonHandle => 'Handle';

  @override
  String get adminCreatePersonHandleInvalid =>
      'Use lower case letters, digits, dots, hyphens and underscores.';

  @override
  String get adminCreatePersonLocal => 'Local account with an enrollment code';

  @override
  String get adminCreatePersonSubmit => 'Create';

  @override
  String adminCodeTitle(String name) {
    return 'Enrollment code for $name';
  }

  @override
  String get adminCodeOnce =>
      'This code is shown only once. Hand it over in person or by phone.';

  @override
  String get adminCodePurposeFirstLogin => 'For the first login';

  @override
  String get adminCodePurposeRecovery => 'For recovery';

  @override
  String get adminCodePurposeIdentityLink => 'For linking a login service';

  @override
  String adminCodeExpires(String when) {
    return 'Valid until $when';
  }

  @override
  String get adminCodeClose => 'Close';

  @override
  String get adminPersonBack => 'All people';

  @override
  String get adminPersonDisable => 'Disable';

  @override
  String get adminPersonEnable => 'Enable';

  @override
  String get adminPersonNewCode => 'New enrollment code';

  @override
  String adminDisableTitle(String name) {
    return 'Disable $name?';
  }

  @override
  String adminDisableBody(String name) {
    return '$name is disabled. All sessions of $name end now and all devices are revoked.';
  }

  @override
  String adminPersonDisabled(String name) {
    return '$name is disabled. All sessions ended.';
  }

  @override
  String adminEnableTitle(String name) {
    return 'Enable $name?';
  }

  @override
  String adminEnableBody(String name) {
    return '$name can sign in again. Earlier devices stay revoked.';
  }

  @override
  String adminPersonEnabled(String name) {
    return '$name is enabled.';
  }

  @override
  String adminNewCodeTitle(String name) {
    return 'New enrollment code for $name?';
  }

  @override
  String adminNewCodeBody(String name) {
    return 'A new code is issued and shown once. Every older code of $name stops working.';
  }

  @override
  String get adminCredentialsTitle => 'Credentials';

  @override
  String get adminCredentialsPasswordSet => 'Local password set';

  @override
  String get adminCredentialsPasswordNone => 'No local password';

  @override
  String get adminCredentialsTotpSet => 'One-time code app set up';

  @override
  String get adminCredentialsTotpNone => 'No one-time code app';

  @override
  String get adminCredentialsIdentities => 'Linked login services';

  @override
  String get adminCredentialsPasskeys => 'Passkeys';

  @override
  String get adminCredentialsNone => 'None';

  @override
  String adminPasskeyAdded(String when) {
    return 'Added $when';
  }

  @override
  String get adminUnlink => 'Unlink';

  @override
  String adminUnlinkTitle(String subject) {
    return 'Unlink $subject?';
  }

  @override
  String adminUnlinkBody(String name) {
    return '$name can no longer sign in with this login service.';
  }

  @override
  String adminUnlinked(String subject) {
    return '$subject was unlinked.';
  }

  @override
  String adminRevokePasskeyTitle(String passkey) {
    return 'Revoke the passkey $passkey?';
  }

  @override
  String adminRevokePasskeyBody(String name) {
    return '$name can no longer sign in with this passkey.';
  }

  @override
  String adminPasskeyRevoked(String passkey) {
    return 'The passkey $passkey was revoked.';
  }

  @override
  String get adminEndSessionBody =>
      'The device of this session is signed out at once.';

  @override
  String get adminSessionEnded => 'The session was ended.';

  @override
  String get adminOwnersTitle => 'Owners';

  @override
  String get adminOwnersAdd => 'Add owner';

  @override
  String get adminOwnersAddTitle => 'Add an owner';

  @override
  String get adminOwnersAddBody =>
      'The person gets every permission of the instance.';

  @override
  String adminOwnersAdded(String name) {
    return '$name is now an owner.';
  }

  @override
  String get adminOwnersRevoke => 'Revoke';

  @override
  String adminOwnersRevokeTitle(String name) {
    return 'Revoke the owner $name?';
  }

  @override
  String adminOwnersRevokeBody(String name) {
    return '$name keeps the account but loses the owner permissions.';
  }

  @override
  String adminOwnersRevoked(String name) {
    return '$name is no longer an owner.';
  }

  @override
  String adminOwnerSince(String when, String reason) {
    return 'Since $when: $reason';
  }

  @override
  String get adminGroupsTitle => 'Groups';

  @override
  String get adminGroupsCreate => 'Create group';

  @override
  String get adminGroupsEmpty => 'No groups yet.';

  @override
  String adminGroupMembers(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count members',
      one: '1 member',
      zero: 'No members',
    );
    return '$_temp0';
  }

  @override
  String adminGroupCreated(String name) {
    return 'The group $name was created.';
  }

  @override
  String get adminGroupCreateTitle => 'Create group';

  @override
  String get adminGroupCreateBody => 'A new group without members is created.';

  @override
  String get adminGroupName => 'Name';

  @override
  String get adminGroupDescription => 'Description (optional)';

  @override
  String get adminGroupBack => 'All groups';

  @override
  String get adminGroupFromProvider =>
      'The login service manages the members of this group.';

  @override
  String get adminGroupRename => 'Rename';

  @override
  String adminGroupRenameTitle(String name) {
    return 'Rename $name';
  }

  @override
  String get adminGroupRenameBody =>
      'The group keeps its members and its permissions.';

  @override
  String adminGroupRenamed(String name) {
    return 'The group is now called $name.';
  }

  @override
  String get adminGroupDelete => 'Delete group';

  @override
  String adminGroupDeleteTitle(String name) {
    return 'Delete $name?';
  }

  @override
  String get adminGroupDeleteBody =>
      'The group is removed and the permissions it gave end. The members keep their accounts.';

  @override
  String get adminMembersTitle => 'Members';

  @override
  String get adminMembersEmpty => 'No members yet.';

  @override
  String get adminMemberAdd => 'Add member';

  @override
  String adminMemberAddTitle(String group) {
    return 'Add a member to $group';
  }

  @override
  String get adminMemberAddBody =>
      'The person gets the permissions of the group, until the given day when one is set.';

  @override
  String get adminMemberUntil => 'Until (optional, YYYY-MM-DD)';

  @override
  String get adminMemberUntilInvalid => 'Enter a date as YYYY-MM-DD.';

  @override
  String adminMemberAdded(String name, String group) {
    return '$name was added to $group.';
  }

  @override
  String adminMemberExpires(String when) {
    return 'Until $when';
  }

  @override
  String get adminMemberRemove => 'Remove';

  @override
  String adminMemberRemoveTitle(String name, String group) {
    return 'Remove $name from $group?';
  }

  @override
  String adminMemberRemoveBody(String name) {
    return '$name loses the permissions of the group.';
  }

  @override
  String adminMemberRemoved(String name, String group) {
    return '$name was removed from $group.';
  }

  @override
  String get adminWaitingTitle => 'Waiting devices';

  @override
  String get adminWaitingEmpty => 'No device is waiting.';

  @override
  String adminWaitingDevice(String person, String device) {
    return '$person: $device';
  }

  @override
  String adminWaitingLoggedIn(String when) {
    return 'Logged in $when';
  }

  @override
  String adminWaitingVia(String method) {
    return 'via $method';
  }

  @override
  String get adminWaitingNoOrigin => 'Origin unknown';

  @override
  String get adminApprove => 'Approve';

  @override
  String get adminReject => 'Reject';

  @override
  String adminApproveTitle(String device, String person) {
    return 'Approve $device of $person';
  }

  @override
  String adminApproveHint(String person) {
    return 'Ask $person for the code that the waiting device shows and type it here.';
  }

  @override
  String adminApproved(String device, String person) {
    return '$device of $person is now active.';
  }

  @override
  String adminRejectTitle(String device) {
    return 'Reject $device?';
  }

  @override
  String adminRejectBody(String person) {
    return 'The waiting device of $person is revoked and cannot sign in.';
  }

  @override
  String adminRejected(String device) {
    return '$device was rejected.';
  }

  @override
  String stepUpPurposeCreatePerson(String name) {
    return 'Create the person $name';
  }

  @override
  String stepUpPurposeDisablePerson(String name) {
    return 'Disable $name';
  }

  @override
  String stepUpPurposeEnablePerson(String name) {
    return 'Enable $name';
  }

  @override
  String stepUpPurposeNewCode(String name) {
    return 'Issue an enrollment code for $name';
  }

  @override
  String stepUpPurposeUnlinkIdentity(String name) {
    return 'Unlink a login service of $name';
  }

  @override
  String stepUpPurposeRevokePasskey(String name) {
    return 'Revoke a passkey of $name';
  }

  @override
  String stepUpPurposeAddOwner(String name) {
    return 'Make $name an owner';
  }

  @override
  String stepUpPurposeRevokeOwner(String name) {
    return 'Revoke the owner $name';
  }

  @override
  String stepUpPurposeCreateGroup(String name) {
    return 'Create the group $name';
  }

  @override
  String stepUpPurposeRenameGroup(String name) {
    return 'Rename the group $name';
  }

  @override
  String stepUpPurposeDeleteGroup(String name) {
    return 'Delete the group $name';
  }

  @override
  String stepUpPurposeAddMember(String name, String group) {
    return 'Add $name to $group';
  }

  @override
  String stepUpPurposeRemoveMember(String name, String group) {
    return 'Remove $name from $group';
  }

  @override
  String stepUpPurposeApproveDeviceOf(String device, String person) {
    return 'Approve the device $device of $person';
  }

  @override
  String stepUpPurposeRejectDevice(String device, String person) {
    return 'Reject the device $device of $person';
  }

  @override
  String get threadNoOperator => 'No agent answers here';

  @override
  String threadOperator(String name) {
    return 'Operator: $name';
  }

  @override
  String get threadTitleAutomatic => 'automatic';

  @override
  String get threadDone => 'Done';

  @override
  String get threadReopen => 'Reopen';

  @override
  String get threadMove => 'Move';

  @override
  String get threadPin => 'Pin';

  @override
  String get threadColumn => 'Column';

  @override
  String get threadEditTitle => 'Edit title';

  @override
  String get threadTitleLabel => 'Title';

  @override
  String get threadSave => 'Save';

  @override
  String get threadCancel => 'Cancel';

  @override
  String threadStarter(String title, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count replies',
      one: '1 reply',
    );
    return 'Thread: $title, $_temp0';
  }

  @override
  String get threadLoadingOlder => 'Loading earlier messages';

  @override
  String get threadLoadingNewer => 'Loading missing messages';

  @override
  String get threadEmpty => 'No messages yet';

  @override
  String get threadNoAccess => 'No access anymore';

  @override
  String get threadNoAccessWaiting => 'Messages that were not sent, to copy:';

  @override
  String get messageActionStartThread => 'Start thread';

  @override
  String get messageActionUnpin => 'Unpin';

  @override
  String get messageActionVersions => 'View versions';

  @override
  String get messageActionDelete => 'Delete';

  @override
  String get messageActionMarkUnread => 'Mark as unread';

  @override
  String get messageActionCopyText => 'Copy text';

  @override
  String get messageActionCopyLink => 'Copy link';

  @override
  String get messageMenuLabel => 'Actions of the message';

  @override
  String get messageDeleteTitle => 'Delete message?';

  @override
  String get messageDeleteBody =>
      'The message is replaced by \"Message deleted\" for everyone.';

  @override
  String get messageVersionsTitle => 'Versions';

  @override
  String get messageVersionsCurrent => 'Current version';

  @override
  String messageVersionsRevision(int number) {
    return 'Version $number';
  }

  @override
  String get messageVersionsEmpty => 'No earlier versions';

  @override
  String messageVersionsFailed(String reason) {
    return 'The versions could not be loaded: $reason';
  }

  @override
  String get messageConflictTitle => 'The message was changed elsewhere';

  @override
  String get messageConflictMine => 'Your version';

  @override
  String get messageConflictTheirs => 'Current version';

  @override
  String get messageConflictKeepMine => 'Keep mine';

  @override
  String get messageConflictTakeTheirs => 'Take theirs';

  @override
  String get messageEditLabel => 'Edit message';

  @override
  String get messageReactionTitle => 'React';

  @override
  String get messageReactionInput => 'Any character';

  @override
  String get messageReactionAdd => 'Add';

  @override
  String get messageCopied => 'Copied';

  @override
  String outboxFailedAction(String reason) {
    return 'Not saved: $reason';
  }

  @override
  String get outboxCompare => 'Compare';

  @override
  String get outboxCopy => 'Copy';

  @override
  String composerTooLong(int limit) {
    return 'The message is too long: at most $limit bytes';
  }

  @override
  String get composerDraftOverwritten =>
      'This draft replaced a newer version from another device';

  @override
  String composerReplyTo(String name) {
    return 'Reply to $name';
  }

  @override
  String get composerCancelReply => 'Cancel reply';

  @override
  String get composerTemplates => 'Templates';

  @override
  String get composerTemplatesPersonal => 'Your templates';

  @override
  String get composerTemplatesChannel => 'Templates of this channel';

  @override
  String get composerTemplatesEmpty => 'No templates';

  @override
  String get composerTemplatesSave => 'Save as template';

  @override
  String get composerTemplatesName => 'Name';

  @override
  String sessionPinned(int count) {
    return 'Pinned ($count)';
  }

  @override
  String get sessionPinnedEmpty => 'No pinned messages';

  @override
  String get sessionNoThread => 'Open a thread to see its session';

  @override
  String get threadMoveNoTarget => 'No channel with the same operator';

  @override
  String get threadMoveReason => 'Reason';

  @override
  String get adminNavGrants => 'Access grants';

  @override
  String get adminGrantsTitle => 'Access grants';

  @override
  String get adminGrantsTarget => 'Target';

  @override
  String get adminGrantsPickTarget => 'Choose a target';

  @override
  String get adminTargetInstance => 'Instance';

  @override
  String adminTargetSpace(String name) {
    return 'Space $name';
  }

  @override
  String adminTargetChannel(String path) {
    return 'Channel $path';
  }

  @override
  String get adminGrantsCreate => 'Create access grant';

  @override
  String get adminGrantsWhoHasAccess => 'Who has access?';

  @override
  String get adminGrantsEmpty => 'No access grant on this target.';

  @override
  String adminGrantsUntil(String day) {
    return 'until $day';
  }

  @override
  String adminGrantsReason(String reason) {
    return 'Reason: $reason';
  }

  @override
  String adminGrantCreated(String name) {
    return '$name received the access grant.';
  }

  @override
  String adminGrantRevoked(String name) {
    return 'The access grant for $name was revoked.';
  }

  @override
  String get adminLevelRead => 'Read';

  @override
  String get adminLevelWrite => 'Write';

  @override
  String get adminLevelTrigger => 'Trigger agents';

  @override
  String get adminLevelNone => 'No level';

  @override
  String get adminCapabilityUpload => 'Upload files';

  @override
  String get adminCapabilityExport => 'Export';

  @override
  String get adminCapabilityManage => 'Manage';

  @override
  String get adminCapabilityTerminal => 'Terminal';

  @override
  String adminWizardStep(int step, int total) {
    return 'Step $step of $total';
  }

  @override
  String get adminWizardWho => 'Who';

  @override
  String get adminWizardWhere => 'Where';

  @override
  String get adminWizardLevel => 'Level and capabilities';

  @override
  String get adminWizardKindPerson => 'Person';

  @override
  String get adminWizardKindGroup => 'Group';

  @override
  String get adminWizardKindAgent => 'Agent';

  @override
  String get adminWizardSearch => 'Search by name';

  @override
  String get adminWizardNoMatch => 'Nothing matches.';

  @override
  String get adminWizardCapabilities => 'Capabilities';

  @override
  String get adminWizardUntil => 'Valid until (YYYY-MM-DD, optional)';

  @override
  String get adminReasonRequired =>
      'Reason (required, stored in the audit log)';

  @override
  String get adminWizardBack => 'Back';

  @override
  String get adminWizardNext => 'Next';

  @override
  String get adminWizardConfirm => 'Confirm';

  @override
  String get adminPreviewTitle => 'Plain-language preview';

  @override
  String get adminPreviewLoading => 'Checking the effect';

  @override
  String get adminPreviewEffects => 'Who is affected';

  @override
  String adminPreviewAdded(String level) {
    return 'Gets $level';
  }

  @override
  String adminPreviewRemoved(String level) {
    return 'Loses $level';
  }

  @override
  String adminPreviewChanged(String before, String after) {
    return '$before becomes $after';
  }

  @override
  String adminPreviewCapabilities(String list) {
    return 'Adds: $list';
  }

  @override
  String get adminPreviewRejected => 'Would be rejected';

  @override
  String adminPreviewAgent(String agent) {
    return 'Agent: $agent';
  }

  @override
  String adminPreviewAffected(String people) {
    return 'Affected: $people';
  }

  @override
  String get adminPreviewI1Hint => 'Switch the agent to organization first.';

  @override
  String adminPreviewDenied(String reason) {
    return 'The access grant would be refused: $reason';
  }

  @override
  String get adminPreviewHints => 'Hints';

  @override
  String adminHintReadHorizon(String person, String channel, String agent) {
    return 'Read horizon (H1): $person can learn content of $channel through agent $agent';
  }

  @override
  String adminHintTriggerBlocked(String agent) {
    return 'Triggering blocked: $agent cannot be triggered by this principal (SR2, SR3)';
  }

  @override
  String adminHintGroupReach(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'This access grant reaches $count people',
      one: 'This access grant reaches 1 person',
    );
    return '$_temp0';
  }

  @override
  String get adminHintExpiryMissing =>
      'No expiry date: the access grant for a person outside of groups has no end';

  @override
  String get adminHintTerminalNotEffective =>
      'The access grant does not take effect on this host (SR10, SR12)';

  @override
  String adminHintUnknown(String code) {
    return 'Hint $code';
  }

  @override
  String adminRevokeGrantTitle(String name) {
    return 'Revoke the access grant for $name?';
  }

  @override
  String get adminRevokeGrantBody =>
      'Revoking takes effect immediately: the access ends now.';

  @override
  String adminAccessTitle(String target) {
    return 'Who has access? $target';
  }

  @override
  String get adminAccessEmpty => 'Nobody has access.';

  @override
  String get adminSourceDirect => 'Direct access grant';

  @override
  String adminSourceGroup(String group) {
    return 'Via group $group';
  }

  @override
  String adminSourceInheritedSpace(String space) {
    return 'Inherited from space $space';
  }

  @override
  String get adminSourceInheritedInstance => 'Inherited from the instance';

  @override
  String get adminSourceDm => 'Participant';

  @override
  String get adminSourceOperator => 'Operator of this channel';

  @override
  String adminSourceViaAgent(String agent) {
    return 'Can learn content here through agent $agent (H1)';
  }

  @override
  String adminSourceGuardBlocked(String rule) {
    return 'Trigger agents, but triggering is blocked ($rule)';
  }

  @override
  String adminSourceGuardAgent(String agent) {
    return 'Agent $agent';
  }

  @override
  String get adminSourceUnknown => 'Other source';

  @override
  String adminSourceMemberUntil(String day) {
    return 'member until $day';
  }

  @override
  String get adminAccessViewAs => 'View as person';

  @override
  String adminViewAsBanner(String name) {
    return 'Viewing as $name (read-only). Nothing is sent. Logged.';
  }

  @override
  String adminViewAsUntil(String when) {
    return 'The view ends $when';
  }

  @override
  String get adminViewAsExit => 'Exit';

  @override
  String get adminViewAsSpaces => 'Space list';

  @override
  String get adminViewAsChannels => 'Channel list';

  @override
  String get adminViewAsNothing => 'Nothing visible.';

  @override
  String stepUpPurposeCreateGrant(String name, String target) {
    return 'Create the access grant for $name on $target';
  }

  @override
  String stepUpPurposeRevokeGrant(String name) {
    return 'Revoke the access grant for $name';
  }

  @override
  String stepUpPurposeViewAs(String name) {
    return 'View as $name';
  }

  @override
  String get problemForbiddenDelegationLevel =>
      'You cannot give more than your own level';

  @override
  String get problemForbiddenDelegationCapabilities =>
      'You cannot give capabilities you do not have';

  @override
  String get problemForbiddenSubjectKind => 'Not allowed for agents or apps';

  @override
  String get adminNavAudit => 'Audit';

  @override
  String get auditTitle => 'Audit log';

  @override
  String get auditFilterAction => 'Action';

  @override
  String get auditFilterActorAll => 'All people';

  @override
  String get auditResultAll => 'All results';

  @override
  String get auditResultAllow => 'allowed';

  @override
  String get auditResultDeny => 'denied';

  @override
  String get auditResultError => 'error';

  @override
  String get auditFilterResourceKind => 'Resource kind';

  @override
  String get auditFilterResourceId => 'Resource ID';

  @override
  String get auditFilterFrom => 'From (YYYY-MM-DD)';

  @override
  String get auditFilterTo => 'To (YYYY-MM-DD)';

  @override
  String get auditFilterApply => 'Apply filters';

  @override
  String get auditFilterInvalidDate => 'Enter a date as YYYY-MM-DD.';

  @override
  String get auditColTime => 'Time';

  @override
  String get auditColWho => 'Who';

  @override
  String get auditColResult => 'Result';

  @override
  String get auditColResource => 'Resource';

  @override
  String get auditColReason => 'Reason';

  @override
  String get auditEmpty => 'No entries match these filters.';

  @override
  String get auditStepUpNeeded =>
      'Reading the audit log needs your confirmation.';

  @override
  String get auditCheckChain => 'Check chain';

  @override
  String auditChainOk(int seq) {
    return 'Chain checked up to no. $seq: unchanged.';
  }

  @override
  String auditChainBroken(int seq, String reason) {
    return 'Chain broken at no. $seq: $reason';
  }

  @override
  String auditChainRange(int from, int to, int count) {
    return 'Checked no. $from to $to; checkpoints compared: $count.';
  }

  @override
  String get auditChainReasonHash => 'the entry does not match its hash.';

  @override
  String get auditChainReasonLink =>
      'the entry does not link the entry before.';

  @override
  String get auditChainReasonGap => 'an entry is missing.';

  @override
  String get auditChainReasonCheckpoint => 'a checkpoint does not match.';

  @override
  String get auditChainReasonCheckpointInvalid => 'a checkpoint is invalid.';

  @override
  String auditChainReasonOther(String code) {
    return 'reason $code.';
  }

  @override
  String get auditChainWarningNoSink =>
      'Warning: no checkpoint target is configured, so checkpoints exist only in the database.';

  @override
  String auditChainWarning(String code) {
    return 'Warning: $code';
  }

  @override
  String get auditExport => 'Export';

  @override
  String get auditExportCsv => 'Export as CSV';

  @override
  String get auditExportNeedsRange =>
      'Choose a time range (from and to) to export.';

  @override
  String get auditExportDone => 'Export saved.';

  @override
  String get auditExportFailed =>
      'The export broke off; the file is incomplete.';

  @override
  String get stepUpPurposeAuditRead => 'Read the audit log';

  @override
  String get stepUpPurposeAuditVerify =>
      'Check the hash chain of the audit log';

  @override
  String get stepUpPurposeAuditExport => 'Export the audit log';

  @override
  String auditDetailTitle(int seq) {
    return 'Entry no. $seq';
  }

  @override
  String get auditDetailCorrelation => 'Correlation ID';

  @override
  String get auditDetailDenyReason => 'Deny reason';

  @override
  String get auditDetailDetails => 'Details';

  @override
  String get auditDetailHash => 'Hash';

  @override
  String get auditDetailPrevHash => 'Previous hash';

  @override
  String get auditDetailClose => 'Close';

  @override
  String get auditActionAuditRead => 'Read the audit log';

  @override
  String get auditActionAuditVerify => 'Check the audit chain';

  @override
  String get auditActionGrantCreate => 'Create a grant';

  @override
  String get auditActionGrantRevoke => 'Revoke a grant';

  @override
  String get auditActionOwnerAdd => 'Add an owner';

  @override
  String get auditActionOwnerRemove => 'Remove an owner';

  @override
  String get auditActionUserCreate => 'Create a person';

  @override
  String get auditActionUserDisable => 'Disable a person';

  @override
  String get auditActionUserEnable => 'Enable a person';

  @override
  String get auditActionDeviceApprove => 'Approve a device';

  @override
  String get auditActionDeviceRevoke => 'Revoke a device';

  @override
  String get auditActionGroupCreate => 'Create a group';

  @override
  String get auditActionStepUp => 'Confirm with step-up';
}

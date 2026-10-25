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
  String get adminNavHosts => 'Hosts';

  @override
  String get hostsAdd => 'Add host';

  @override
  String get hostsEmpty => 'No hosts yet.';

  @override
  String get hostOwnersOnly => 'Owners only';

  @override
  String get hostStatePairing => 'Pairing';

  @override
  String get hostStateConfirming => 'Waiting for confirmation';

  @override
  String get hostStateActive => 'active';

  @override
  String get hostStateDisabled => 'disabled';

  @override
  String get hostStateDuplicate =>
      'disabled: a second connection used the same keys, only a new pairing helps';

  @override
  String get hostStateRetired => 'retired';

  @override
  String get hostStateUnknown => 'unknown';

  @override
  String get hostConnected => 'connected';

  @override
  String get hostDisconnected => 'disconnected';

  @override
  String hostLastSeen(String when) {
    return 'last seen $when';
  }

  @override
  String get hostBack => 'Back to the hosts';

  @override
  String get hostFieldAssignee => 'Assignee';

  @override
  String get hostFieldPlatform => 'Platform';

  @override
  String get hostFieldConnection => 'Connection';

  @override
  String get hostFieldFingerprint => 'Fingerprint';

  @override
  String get hostFieldLlmLogin => 'LLM login';

  @override
  String get hostFieldServerHost => 'Server host';

  @override
  String get hostFieldChecks => 'Checks';

  @override
  String get hostFieldProfiles => 'Profiles';

  @override
  String get hostFieldGrants => 'Access grants';

  @override
  String get hostFieldSessionUsers => 'Session users';

  @override
  String get hostFieldSessions => 'Terminal sessions';

  @override
  String hostPlatformValue(String platform, String version) {
    return '$platform, version $version';
  }

  @override
  String get hostNotReported => 'Not reported yet';

  @override
  String hostFingerprintValue(String value, int generation) {
    return '$value   Key generation $generation';
  }

  @override
  String get hostNoKeys => 'No keys';

  @override
  String get hostLlmNone => 'None';

  @override
  String get hostLlmPersonal => 'Personal';

  @override
  String hostLlmPersonalOf(String name) {
    return 'Personal of $name';
  }

  @override
  String get hostLlmOrganization => 'Organization';

  @override
  String get hostLlmI6Explained =>
      'Personal: only this person may open a terminal here, not even other owners.';

  @override
  String get hostLlmHolder => 'Holder of the login';

  @override
  String hostLlmTitle(String name) {
    return 'LLM login on $name';
  }

  @override
  String get hostLlmBody =>
      'A change to Personal or a new holder ends the terminal sessions of other people on this host.';

  @override
  String hostLlmChanged(String name) {
    return 'The LLM login of $name is changed.';
  }

  @override
  String get hostServerHostQuestion =>
      'Is this host the server of the instance?';

  @override
  String get hostYes => 'Yes';

  @override
  String get hostNo => 'No';

  @override
  String hostServerHostDeclared(String answer) {
    return 'Declared: $answer';
  }

  @override
  String hostServerHostReported(String answer) {
    return 'Report of the host service: $answer';
  }

  @override
  String hostDoctorOk(String check) {
    return '$check ok';
  }

  @override
  String hostDoctorFailed(String check, String code) {
    return '$check failed: $code';
  }

  @override
  String hostDoctorAssigned(String osUser, String name) {
    return '$osUser (assigned to $name)';
  }

  @override
  String hostDoctorFormer(String osUser, String name) {
    return '$osUser (formerly assigned to $name)';
  }

  @override
  String hostDoctorUnknown(String osUser) {
    return '$osUser (no assignment known)';
  }

  @override
  String get hostDoctorNone => 'No check results yet.';

  @override
  String get hostDoctorAuditFailed =>
      'The audit of this host could not be read, so former session users show without their person.';

  @override
  String hostWorkdirs(String folders) {
    return 'Work folders: $folders';
  }

  @override
  String get hostWorkdirsNone => 'Work folders: none';

  @override
  String hostSessionUsersUnassigned(String osUser) {
    return 'Reported, not assigned: $osUser';
  }

  @override
  String get hostSessionUsersSr12 =>
      'Without a session user no terminal (SR12).';

  @override
  String get hostSessionUsersOnlyReported =>
      'Only names that the host reported during pairing or in its last report.';

  @override
  String get hostSessionUserAssign => 'Assign';

  @override
  String get hostSessionUserAssignTitle => 'Assign a session user';

  @override
  String get hostSessionUserAssignBody =>
      'A name once given to a person stays bound to that person on this host.';

  @override
  String get hostSessionUserPerson => 'Person';

  @override
  String get hostSessionUserName => 'Reported name';

  @override
  String hostSessionUserAssigned(String name, String osUser) {
    return '$name now has the session user $osUser.';
  }

  @override
  String get hostSessionUserEnd => 'End';

  @override
  String hostSessionUserEndTitle(String name) {
    return 'End the assignment of $name?';
  }

  @override
  String hostSessionUserEndBody(String name, String osUser) {
    return '$name can no longer open a terminal on this host. The name $osUser stays taken on this host and is never given to another person.';
  }

  @override
  String hostSessionUserEnded(String name, String osUser) {
    return 'The assignment of $name ended; $osUser stays taken.';
  }

  @override
  String get hostSessionUserTaken =>
      'This name is taken: a name once given to a person stays bound to that person on this host, also after the assignment ends, and each person has one session user per host.';

  @override
  String get hostSessionUserNotOnHost =>
      'The host has not reported this name. The host must report the name before it can be assigned.';

  @override
  String get hostSessionRowsTitle => 'Session user per person';

  @override
  String get hostSessionRowsAdd => 'Add a session user';

  @override
  String get hostSessionRowRemove => 'Remove';

  @override
  String get hostChange => 'Change';

  @override
  String hostAssigneeTitle(String name) {
    return 'Assignee of $name';
  }

  @override
  String get hostAssigneeBody =>
      'A person who is not an owner gets a terminal only on the host assigned to them (SR10).';

  @override
  String hostAssigneeChanged(String name) {
    return 'The assignee of $name is changed.';
  }

  @override
  String get hostGrantsAdd => 'Grant terminal';

  @override
  String get hostGrantsNone => 'No access grants on this host.';

  @override
  String get hostNotEffective => 'Not effective';

  @override
  String hostSessionsCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count open terminal sessions',
      one: '1 open terminal session',
      zero: 'No open terminal sessions',
    );
    return '$_temp0';
  }

  @override
  String get hostRepair => 'Re-pair';

  @override
  String hostRepairTitle(String name) {
    return 'Re-pair $name?';
  }

  @override
  String get hostRepairBody =>
      'The keys of the host are dropped now, its connections close and its terminal sessions end. A new pairing code follows; the key generation rises with the next pairing.';

  @override
  String get hostPair => 'Continue pairing';

  @override
  String get hostDisable => 'Disable';

  @override
  String hostDisableTitle(String name) {
    return 'Disable $name?';
  }

  @override
  String get hostDisableBody =>
      'All terminal sessions on this host end now. No one can open a terminal here until it is enabled again.';

  @override
  String get hostEnable => 'Enable';

  @override
  String hostEnableTitle(String name) {
    return 'Enable $name?';
  }

  @override
  String get hostEnableBody =>
      'Terminal sessions are possible again for the people who may open them.';

  @override
  String get hostRetire => 'Retire';

  @override
  String hostRetireTitle(String name) {
    return 'Retire $name?';
  }

  @override
  String get hostRetireBody =>
      'All terminal sessions end now and the host is retired for good. It cannot be paired or enabled again.';

  @override
  String hostStateChanged(String name, String state) {
    return '$name is now $state.';
  }

  @override
  String get hostPairingTitle => 'Pair host';

  @override
  String get hostCreateName => 'Name';

  @override
  String get hostCreateAssignee => 'Assignee (optional)';

  @override
  String get hostCreateSubmit => 'Create and get a code';

  @override
  String get hostPairingCommandIntro =>
      'Enter this command on the host as the user of the host service (a one-time code that expires soon):';

  @override
  String hostPairingExpiresIn(String time) {
    return 'Expires in $time';
  }

  @override
  String get hostPairingExpired =>
      'The pairing code has expired. Issue a new code.';

  @override
  String get hostPairingNewCode => 'New code';

  @override
  String get hostPairingNotYet =>
      'The host has not reported yet. Enter the command on the host, then try again.';

  @override
  String get hostPairingNeedsCode =>
      'This host waits for a pairing code. Issue a new code to continue.';

  @override
  String get hostPairingClose => 'Close';

  @override
  String hostFingerprintLabel(int count) {
    return 'Type the fingerprint from the host (first $count characters)';
  }

  @override
  String get hostFingerprintCompare =>
      'Compare with the output on the host and type it again.';

  @override
  String hostAttemptsLeft(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count attempts left',
      one: '1 attempt left',
    );
    return '$_temp0';
  }

  @override
  String get hostFingerprintFallback =>
      'No attempts left: the host is back in pairing and its keys are dropped. Issue a new pairing code.';

  @override
  String get hostLlmQuestion => 'LLM login on this host';

  @override
  String get hostConfirmSubmit => 'Confirm with passkey / device';

  @override
  String hostConfirmed(String name) {
    return '$name is active.';
  }

  @override
  String stepUpPurposeCreateHost(String name) {
    return 'Create the host $name';
  }

  @override
  String stepUpPurposePairingCode(String name) {
    return 'Issue a pairing code for $name';
  }

  @override
  String stepUpPurposeConfirmHost(String name) {
    return 'Confirm the host $name';
  }

  @override
  String stepUpPurposeUpdateHost(String name) {
    return 'Change the host $name';
  }

  @override
  String stepUpPurposeHostLlmLogin(String name) {
    return 'Change the LLM login of $name';
  }

  @override
  String stepUpPurposeHostState(String name) {
    return 'Change the state of $name';
  }

  @override
  String stepUpPurposeHostAudit(String name) {
    return 'Read the audit of $name';
  }

  @override
  String adminTargetHost(String name) {
    return 'Host $name';
  }

  @override
  String get adminTargetsHosts => 'Hosts';

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
  String get adminNavAgents => 'Agents';

  @override
  String get adminAgentsTitle => 'Agents';

  @override
  String get adminAgentsEmpty => 'No agents.';

  @override
  String get adminAgentsSwitchAll => 'Switch all agents';

  @override
  String get adminAgentDisabled => 'Disabled';

  @override
  String adminAgentWarningCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count warnings',
      one: '1 warning',
    );
    return '$_temp0';
  }

  @override
  String get adminAgentBack => 'Back to the agents';

  @override
  String get adminAgentStateOnline => 'Online';

  @override
  String get adminAgentStateDegraded => 'Degraded';

  @override
  String get adminAgentStateOffline => 'Offline';

  @override
  String get adminAgentStateUnknown => 'No heartbeat yet';

  @override
  String adminAgentAdapter(String kind) {
    return 'Adapter $kind';
  }

  @override
  String adminAgentRuntimeVersion(String version) {
    return 'Runtime version $version';
  }

  @override
  String adminAgentLastHeartbeat(String when) {
    return 'Last heartbeat $when';
  }

  @override
  String get adminAgentConfigured => 'Configured mode';

  @override
  String get adminAgentReported => 'Reported mode';

  @override
  String get adminAgentReportedPerRun => 'Follows the configured mode per run';

  @override
  String adminAgentEvidence(String method) {
    return 'Evidence: $method';
  }

  @override
  String adminAgentSupported(String modes) {
    return 'Can prove: $modes';
  }

  @override
  String get adminAgentSupportedNone => 'Can prove: not reported';

  @override
  String adminAgentHolder(String name) {
    return 'Holder: $name';
  }

  @override
  String adminAgentWarnMismatch(String reported, String configured) {
    return 'Adapter reports $reported, $configured is configured';
  }

  @override
  String get adminAgentWarnUnsupported => 'Adapter cannot prove this mode';

  @override
  String get adminAgentWarnEvidenceUnknown =>
      'Login type of the runtime unknown';

  @override
  String get adminAgentWarnToolClass =>
      'Adapter reports a different tool class';

  @override
  String adminAgentWarnUnknown(String code) {
    return 'Warning $code';
  }

  @override
  String get adminAgentWarnFailUntilSwitched =>
      'Invocations fail until the adapter is switched.';

  @override
  String get adminAgentWarnFailAll => 'All invocations fail.';

  @override
  String adminAgentSwitchHint(String mode) {
    return 'To switch the adapter, whoever runs the instance changes the login of its runtime to $mode. The warning disappears with the next heartbeat that reports $mode.';
  }

  @override
  String get adminAgentTargets => 'Operates';

  @override
  String get adminAgentTargetsNone => 'No space and no channel';

  @override
  String adminAgentTargetDefault(String target) {
    return '$target, by default';
  }

  @override
  String get adminAgentBackend => 'Backend';

  @override
  String get adminAgentSwitch => 'Switch backend';

  @override
  String get adminAgentHolderChange => 'Change holder';

  @override
  String get adminAgentSwitchExplain =>
      'Switching needs a reason and confirmation. Personal works only while exactly one person may trigger the agent (I1).';

  @override
  String adminSwitchTitle(String agent) {
    return 'Switch $agent';
  }

  @override
  String adminSwitchToOrgBody(String agent) {
    return '$agent then uses the organization key: the organization bears the costs, and everyone with Trigger agents may trigger it. The holder is cleared.';
  }

  @override
  String adminSwitchToPersonalBody(String agent) {
    return '$agent then uses the personal subscription of the holder: the holder pays, and only the holder may trigger it.';
  }

  @override
  String get adminSwitchHolderPick => 'Pick the holder';

  @override
  String get adminSwitchConfirm => 'Switch';

  @override
  String adminSwitched(String agent, String mode) {
    return '$agent now uses $mode';
  }

  @override
  String stepUpPurposeSwitchAgent(String agent) {
    return 'Switch the backend of $agent';
  }

  @override
  String get stepUpPurposeSwitchAll => 'Switch the backend of several agents';

  @override
  String stepUpPurposeSetHolder(String agent) {
    return 'Change the holder of $agent';
  }

  @override
  String stepUpPurposeCreateToken(String agent) {
    return 'Create a token for $agent';
  }

  @override
  String adminHolderTitle(String agent) {
    return 'Change the holder of $agent';
  }

  @override
  String adminHolderBody(String agent) {
    return '$agent keeps running on Personal, on the subscription of the new holder. Only the new holder may trigger it.';
  }

  @override
  String adminHolderChanged(String name, String agent) {
    return '$name is now the holder of $agent';
  }

  @override
  String get adminI1Title => 'Personal is not possible right now';

  @override
  String adminI1Agent(String agent, String holder, String names) {
    return '$agent would use the subscription of $holder, but these may trigger it too: $names';
  }

  @override
  String adminI1AgentNoHolder(String agent, String names) {
    return '$agent may also be triggered by: $names';
  }

  @override
  String adminI1Hidden(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count more you cannot see',
      one: '1 more you cannot see',
    );
    return '$_temp0';
  }

  @override
  String get adminI1Way =>
      'Revoke their access grants, group memberships or owner rights first, or keep Organization.';

  @override
  String get adminSwitchAllTitle => 'Switch several agents';

  @override
  String get adminSwitchAllOrgBody =>
      'Every agent then runs on the organization key; holders are cleared. Use this when there are doubts about a personal operation.';

  @override
  String get adminSwitchAllPersonalBody =>
      'The checked agents then run on the personal subscription of the holder. If one of them would break I1, nothing changes.';

  @override
  String get adminSwitchAllProposed =>
      'Proposed: the agents the last switch of several agents to Organization changed';

  @override
  String get adminSwitchAllNoProposal =>
      'No earlier switch to propose from. Check the agents yourself.';

  @override
  String adminSwitchAllDone(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count agents changed',
      one: '1 agent changed',
      zero: 'No agent changed',
    );
    return '$_temp0';
  }

  @override
  String get adminTokensTitle => 'API tokens';

  @override
  String get adminTokensEmpty => 'No tokens.';

  @override
  String get adminTokenCreate => 'Create token';

  @override
  String adminTokenCreateTitle(String agent) {
    return 'Create a token for $agent';
  }

  @override
  String adminTokenCreateBody(String agent) {
    return 'The adapter acts as $agent with this token. Its secret is shown only once.';
  }

  @override
  String get adminTokenName => 'Name';

  @override
  String get adminTokenLastDay => 'Last day (YYYY-MM-DD)';

  @override
  String get adminTokenLastDayInvalid => 'Enter a day as YYYY-MM-DD.';

  @override
  String get adminTokenDpop => 'DPoP key thumbprint of the adapter';

  @override
  String adminTokenValidUntil(String when) {
    return 'valid until $when';
  }

  @override
  String adminTokenLastUsed(String when) {
    return 'last used $when';
  }

  @override
  String get adminTokenBound => 'DPoP-bound';

  @override
  String get adminTokenRevokedLabel => 'Revoked';

  @override
  String get adminTokenRevoke => 'Revoke';

  @override
  String adminTokenRevokeTitle(String name) {
    return 'Revoke the token $name?';
  }

  @override
  String get adminTokenRevokeBody =>
      'An adapter that uses this token loses its access at once.';

  @override
  String adminTokenRevokedNotice(String name) {
    return 'The token $name is revoked.';
  }

  @override
  String adminTokenSecretTitle(String name) {
    return 'Token $name';
  }

  @override
  String get adminTokenSecretOnce =>
      'This secret is shown only once. Copy it into the adapter\'s secret store now.';

  @override
  String get adminTokenSecretLabel => 'Token secret';

  @override
  String adminGuidedExplainGrant(String person, String agent, String holder) {
    return 'This grant allows $person to trigger the agent $agent. $agent uses the personal subscription of $holder. $agent must first be switched to Organization; the organization key then bears the costs.';
  }

  @override
  String adminGuidedExplainChange(String person, String agent, String holder) {
    return 'This change allows $person to trigger the agent $agent. $agent uses the personal subscription of $holder. $agent must first be switched to Organization; the organization key then bears the costs.';
  }

  @override
  String adminGuidedNotReady(String agent) {
    return 'The adapter of $agent does not report an organization key yet. Invocations fail until it is switched.';
  }

  @override
  String adminGuidedGrant(String agent) {
    return 'Switch $agent to Organization and grant';
  }

  @override
  String adminGuidedContinue(String agent) {
    return 'Switch $agent to Organization and continue';
  }

  @override
  String get adminGuidedTitle => 'Switch to Organization';

  @override
  String adminGuidedDone(String agent) {
    return '$agent now uses Organization, and the change is saved.';
  }

  @override
  String adminGuidedStillReports(String agent, String mode) {
    return 'The adapter of $agent still reports $mode.';
  }

  @override
  String get adminGuidedDoneClose => 'Close';

  @override
  String get adminPersonViewAs => 'View as this person';

  @override
  String adminViewAsRemaining(String time) {
    return '$time left';
  }

  @override
  String get adminViewAsEnd => 'End view';

  @override
  String adminViewAsSidebarOf(String name) {
    return 'Sidebar as $name sees it';
  }

  @override
  String adminViewAsNotVisible(String name, String names) {
    return 'Not visible for $name: $names';
  }

  @override
  String get adminViewAsStructureOnly =>
      'The console shows structure and access only, never the content of a channel.';

  @override
  String get adminViewAsStructureLater =>
      'The structure appears when a view starts. End this view and start it again to see it.';

  @override
  String get consoleStepUpPasskeyNext =>
      'Next, your browser asks for your passkey.';

  @override
  String get consoleStepUpOidcAction =>
      'This change needs a new login at your login service. Confirm there, then make the change again.';

  @override
  String get terminalHeaderConnected => 'Connected';

  @override
  String get terminalHeaderConnecting => 'Connecting';

  @override
  String get terminalHeaderOffline => 'Not connected';

  @override
  String terminalHeaderFingerprint(String value) {
    return 'Fp $value';
  }

  @override
  String terminalHeaderFingerprintLabel(String fingerprint) {
    return 'Fingerprint of the host: $fingerprint';
  }

  @override
  String terminalHeaderRemaining(String time) {
    return 'Left $time';
  }

  @override
  String get terminalLlmLoginNone => 'No LLM login';

  @override
  String terminalTabLabel(String profile, int number) {
    return '$profile $number';
  }

  @override
  String get terminalDetach => 'Detach';

  @override
  String get terminalEnd => 'End';

  @override
  String get terminalReattach => 'Reattach';

  @override
  String get terminalStateConnecting =>
      'Opening the encrypted connection to the host';

  @override
  String get terminalStateDetached =>
      'Detached. The terminal session keeps going on the host.';

  @override
  String get terminalStateDetachedLost =>
      'The connection broke. The terminal session keeps going on the host.';

  @override
  String get terminalStateDetachedHostUnavailable =>
      'The host is not connected. Try to reattach later.';

  @override
  String get terminalStateDetachedTokenStale =>
      'The session token expired or was already used. Reattach confirms again.';

  @override
  String get terminalStateDetachedReplaced =>
      'Another device attached to this terminal session.';

  @override
  String get terminalStateDetachedServer =>
      'The server restarted. The terminal session keeps going on the host.';

  @override
  String get terminalStateDetachedSlow =>
      'The connection could not keep up. The terminal session keeps going on the host.';

  @override
  String get terminalStateDetachedAuth =>
      'The login of the connection failed. Sign in again, then reattach.';

  @override
  String get terminalStateDetachedRateLimited =>
      'Too many attempts. Wait a moment, then reattach.';

  @override
  String terminalStateExited(int code) {
    return 'The process exited with code $code.';
  }

  @override
  String terminalStateEnded(String reason) {
    return 'The terminal session ended: $reason';
  }

  @override
  String terminalStateBlockedRevoked(String reason) {
    return 'Access ended: $reason';
  }

  @override
  String get terminalStateBlockedHandshake =>
      'The encrypted connection failed its check. Someone may have tampered with it. No new attempt is made.';

  @override
  String get terminalStateBlockedKeyMismatch =>
      'The server sent a different host key than the checked one. The terminal session was closed.';

  @override
  String get terminalStateBlockedProtocol =>
      'The connection broke the terminal protocol. No new attempt is made.';

  @override
  String terminalStateFailed(String problem) {
    return 'The terminal could not be opened: $problem';
  }

  @override
  String terminalCloseReason(String reason) {
    String _temp0 = intl.Intl.selectLogic(reason, {
      'client_closed': 'ended by you',
      'closed_by_owner': 'ended by an owner',
      'process_exited': 'the process exited',
      'idle_timeout': 'no input for too long',
      'max_duration': 'the maximum duration was reached',
      'detach_expired': 'it was detached for too long',
      'access_revoked': 'access was revoked',
      'i6_violation': 'the LLM login of the host changed',
      'host_disabled': 'the host was disabled',
      'host_retired': 'the host was retired',
      'host_rekeyed': 'the host was paired again',
      'host_restarted': 'the host service restarted',
      'user_disabled': 'the account was disabled',
      'device_revoked': 'this device was removed',
      'session_revoked': 'the login was ended',
      'logout': 'you signed out',
      'open_failed': 'the opening failed',
      'host_policy_violation': 'a check on the host failed',
      'other': 'no reason given',
    });
    return '$_temp0';
  }

  @override
  String get terminalFooterEncrypted => 'End-to-end encrypted';

  @override
  String terminalFooterProfile(String profile) {
    return 'Profile $profile';
  }

  @override
  String get terminalFooterIdle => 'Idle timeout active';

  @override
  String get terminalOpenTitle => 'Open terminal';

  @override
  String get terminalOpenHost => 'Host';

  @override
  String get terminalOpenProfile => 'Profile';

  @override
  String get terminalOpenWorkdir => 'Working folder';

  @override
  String get terminalOpenLogin => 'LLM login';

  @override
  String get terminalOpenLogged => 'Every opening is logged.';

  @override
  String get terminalOpenConfirm => 'Confirm with passkey or device key';

  @override
  String get terminalCancel => 'Cancel';

  @override
  String get terminalKeyChangedTitle => 'The host\'s key has changed';

  @override
  String get terminalKeyChangedStored => 'Stored';

  @override
  String get terminalKeyChangedNow => 'Now';

  @override
  String get terminalKeyChangedBody =>
      'Without pairing again by an owner, this is a sign of an attack. Connect only after you have checked the new fingerprint on the host.';

  @override
  String get terminalKeyChangedReject => 'Do not connect';

  @override
  String get terminalKeyChangedAccept => 'Accept new key';

  @override
  String get terminalForeignTitle => 'This host belongs to another person';

  @override
  String terminalForeignBody(String host) {
    return '$host is assigned to another person. Your terminal session works in your own session user, but that person sets up the host and could read along through a local privilege escalation. Do not type secrets here unless you trust the host.';
  }

  @override
  String get terminalForeignConfirm => 'Connect anyway';

  @override
  String get terminalLinkTitle => 'Open link?';

  @override
  String get terminalLinkBody =>
      'The program in the terminal links to this address:';

  @override
  String get terminalLinkOpen => 'Open link';

  @override
  String get terminalLinkUnsupported =>
      'Only https and http links open. This address stays closed.';

  @override
  String terminalLinkHover(String target) {
    return 'Link: $target';
  }

  @override
  String get terminalPasteTitle => 'Paste with line breaks?';

  @override
  String terminalPasteBody(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count lines',
      one: '1 line',
    );
    return 'The text has $_temp0. Every line break can start a command.';
  }

  @override
  String get terminalPasteConfirm => 'Paste';

  @override
  String get terminalReadScreen => 'Read screen';

  @override
  String terminalScreenLabel(String line) {
    return 'Terminal, line at the cursor: $line';
  }

  @override
  String get terminalExtraKeys => 'Extra keys bar';

  @override
  String get terminalKeyEsc => 'Esc';

  @override
  String get terminalKeyTab => 'Tab';

  @override
  String get terminalKeyCtrl => 'Ctrl';

  @override
  String get terminalKeyAlt => 'Alt';

  @override
  String get terminalKeyUp => '↑';

  @override
  String get terminalKeyDown => '↓';

  @override
  String get terminalKeyLeft => '←';

  @override
  String get terminalKeyRight => '→';

  @override
  String get terminalKeyPipe => '|';

  @override
  String get terminalKeyTilde => '~';

  @override
  String get terminalKeySlash => '/';

  @override
  String get terminalKeyMinus => '-';

  @override
  String get terminalKeyEscName => 'Escape';

  @override
  String get terminalKeyTabName => 'Tab';

  @override
  String get terminalKeyCtrlName => 'Control';

  @override
  String get terminalKeyAltName => 'Alt';

  @override
  String get terminalKeyUpName => 'Arrow up';

  @override
  String get terminalKeyDownName => 'Arrow down';

  @override
  String get terminalKeyLeftName => 'Arrow left';

  @override
  String get terminalKeyRightName => 'Arrow right';

  @override
  String get terminalKeyPipeName => 'Vertical bar';

  @override
  String get terminalKeyTildeName => 'Tilde';

  @override
  String get terminalKeySlashName => 'Slash';

  @override
  String get terminalKeyMinusName => 'Minus';

  @override
  String terminalKeyLatched(String key) {
    return '$key, latched';
  }

  @override
  String terminalKeyHeld(String key) {
    return '$key, held';
  }

  @override
  String terminalStepUpOpen(String name) {
    return 'Open a terminal on $name';
  }

  @override
  String terminalStepUpAttach(String name) {
    return 'Reattach to the terminal on $name';
  }

  @override
  String terminalLinkHost(String name) {
    return 'Host: $name';
  }

  @override
  String fileStateUploading(int percent) {
    return 'Uploading $percent%';
  }

  @override
  String get fileStateWaiting => 'Waiting to upload';

  @override
  String get fileStateChecking => 'Being checked';

  @override
  String get fileStateUploadFailed => 'Upload failed';

  @override
  String get fileStateBlocked => 'Blocked';

  @override
  String get fileStateCanceled => 'Upload canceled';

  @override
  String get fileDeleted => 'File deleted';

  @override
  String fileSizeBytes(String size) {
    return '$size B';
  }

  @override
  String fileSizeKilobytes(String size) {
    return '$size KB';
  }

  @override
  String fileSizeMegabytes(String size) {
    return '$size MB';
  }

  @override
  String fileSizeGigabytes(String size) {
    return '$size GB';
  }

  @override
  String fileTileLabel(String name, String size, String state) {
    return '$name, $size, $state';
  }

  @override
  String get fileOpen => 'Open';

  @override
  String get fileSaveAs => 'Save as';

  @override
  String fileSaved(String name) {
    return 'Saved $name';
  }

  @override
  String fileDownloadFailed(String reason) {
    return 'Download failed: $reason';
  }

  @override
  String get fileDownloadDamaged =>
      'The download was damaged and has been deleted. Try again.';

  @override
  String fileImageOpen(String name) {
    return 'Open image $name';
  }

  @override
  String get fileImageClose => 'Close';

  @override
  String get fileImageLoading => 'Loading image';

  @override
  String fileMediaPlay(String name) {
    return 'Play $name';
  }

  @override
  String get fileMediaPause => 'Pause';

  @override
  String get fileMediaFailed => 'Playback failed';

  @override
  String get fileUploadAgain => 'Upload again';

  @override
  String fileRemove(String name) {
    return 'Remove $name';
  }

  @override
  String get fileFailPartConflict =>
      'A part was sent twice with different content';

  @override
  String get fileFailSessionEnded => 'The upload session has ended';

  @override
  String get fileFailQuota => 'The storage quota is used up';

  @override
  String get fileFailNotAttachable => 'This file cannot be attached';

  @override
  String get fileFailUnreadable => 'The file cannot be read anymore';

  @override
  String composerFileTooLarge(String name, String limit) {
    return '$name is larger than the limit of $limit';
  }

  @override
  String composerTooManyFiles(int count) {
    return 'At most $count files per message';
  }

  @override
  String composerFileUnreadable(String name) {
    return '$name cannot be read';
  }

  @override
  String get composerDropFiles => 'Drop files to attach them';

  @override
  String composerAttachments(int count) {
    return 'Attachments ($count)';
  }

  @override
  String voiceRecording(String elapsed) {
    return 'Recording $elapsed';
  }

  @override
  String voiceRecorded(String elapsed) {
    return 'Voice message $elapsed';
  }

  @override
  String get voiceSend => 'Send voice message';

  @override
  String get voiceStop => 'Stop recording';

  @override
  String get voiceCancel => 'Discard recording';

  @override
  String get voiceMicrophoneDenied =>
      'Microphone access is off. Allow it in the system settings under Privacy and Security, Microphone.';

  @override
  String voiceLimitReached(String limit) {
    return 'The recording stopped at the limit of $limit';
  }

  @override
  String get voiceFailed => 'The recording failed';

  @override
  String voiceSpeed(String speed) {
    return 'Playback speed $speed';
  }

  @override
  String voiceSpeedValue(String speed) {
    return '${speed}x';
  }

  @override
  String get transcriptLabel => 'Transcript';

  @override
  String get transcriptFailed => 'No transcript';

  @override
  String get transcriptRequest => 'Transcribe';

  @override
  String get transcriptUnavailable => 'Transcription is not available here';

  @override
  String sessionFiles(int count) {
    return 'Files ($count)';
  }

  @override
  String get sessionFilesEmpty => 'No files';

  @override
  String fileVideoTitle(String name) {
    return 'Video $name';
  }

  @override
  String get adminNavApps => 'Apps';

  @override
  String get adminNavIncomingWebhooks => 'Incoming webhooks';

  @override
  String get adminAppListTitle => 'Apps';

  @override
  String get adminAppInstall => 'Install app';

  @override
  String get adminAppInstallNoPicker =>
      'Installing needs a file dialog; use the web console.';

  @override
  String get adminAppListEmpty => 'No apps installed.';

  @override
  String adminAppVersion(String version) {
    return 'Version $version';
  }

  @override
  String adminAppPublisher(String publisher) {
    return 'by $publisher';
  }

  @override
  String get adminAppBack => 'Back to the apps';

  @override
  String get adminAppStateActive => 'Active';

  @override
  String get adminAppStateSuspended => 'Suspended';

  @override
  String get adminAppStateUninstalled => 'Uninstalled';

  @override
  String get adminAppStateUnknown => 'Unknown state';

  @override
  String get adminAppScopeMessagesRead =>
      'Read messages and threads and search in them';

  @override
  String get adminAppScopeMessagesWrite =>
      'Write, edit and delete messages, react and start threads';

  @override
  String get adminAppScopeFilesRead => 'Read files';

  @override
  String get adminAppScopeFilesWrite => 'Upload files';

  @override
  String get adminAppScopeCommandsWrite =>
      'Register and remove commands and answer their suggestions';

  @override
  String get adminAppScopeComponentsWrite =>
      'Change its buttons and forms and answer interactions';

  @override
  String get adminAppScopeUsersRead => 'Read the profiles of people';

  @override
  String get adminAppScopeEventsSubscribe =>
      'Receive events and catch up on missed ones';

  @override
  String get adminAppScopeCostsReconcile =>
      'Read and submit cost reconciliations';

  @override
  String get adminAppScopeUnknown => 'No description in this version';

  @override
  String get adminAppScopeRequired => 'required';

  @override
  String get adminAppScopeOptional => 'optional';

  @override
  String get adminAppFileNotText => 'The file is not text (UTF-8).';

  @override
  String get adminAppFileNotJsonOrYaml => 'The file is neither JSON nor YAML.';

  @override
  String get adminAppFileNotObject => 'The file holds no manifest object.';

  @override
  String get adminAppUpdate => 'Update';

  @override
  String get adminAppSuspend => 'Suspend';

  @override
  String get adminAppResume => 'Resume';

  @override
  String get adminAppUninstall => 'Uninstall';

  @override
  String adminAppSuspendTitle(String name) {
    return 'Suspend $name?';
  }

  @override
  String get adminAppSuspendBody =>
      'An emergency stop: nothing is deleted, and resuming needs no new consent.';

  @override
  String adminAppResumeTitle(String name) {
    return 'Resume $name?';
  }

  @override
  String get adminAppResumeBody => 'This comes back when the app is resumed:';

  @override
  String adminAppUninstallTitle(String name) {
    return 'Uninstall $name?';
  }

  @override
  String get adminAppUninstallBody =>
      'This cannot be undone; a new installation needs a new consent.';

  @override
  String get adminAppEffectsPart => 'Part';

  @override
  String get adminAppEffectsResult => 'Effect';

  @override
  String get adminAppEffectRowTokens => 'Tokens';

  @override
  String get adminAppEffectRowDeliveries => 'Deliveries';

  @override
  String get adminAppEffectRowCommands => 'Commands';

  @override
  String get adminAppEffectRowComponents => 'Components';

  @override
  String get adminAppEffectRowGrants => 'Access grants and incoming webhooks';

  @override
  String get adminAppEffectRowMessages => 'Messages of the app';

  @override
  String get adminAppEffectBlocked => 'Blocked, not revoked';

  @override
  String get adminAppEffectPaused => 'Paused, the position is kept';

  @override
  String get adminAppEffectHidden => 'Hidden';

  @override
  String get adminAppEffectDisabled => 'Disabled';

  @override
  String get adminAppEffectSuspendGrants =>
      'Remain; incoming webhooks answer 404 until the app is resumed';

  @override
  String get adminAppEffectRemain => 'Remain';

  @override
  String get adminAppEffectRevoked => 'Revoked';

  @override
  String get adminAppEffectSubscriptionsDisabled =>
      'Subscriptions are disabled';

  @override
  String get adminAppEffectRemoved => 'Removed';

  @override
  String get adminAppEffectUninstallMessages =>
      'Remain; the author is kept and the app is disabled';

  @override
  String get adminAppEffectWorkAgain => 'Work again';

  @override
  String get adminAppEffectDeliveriesContinue =>
      'Continue from the kept position';

  @override
  String get adminAppEffectShownAgain => 'Shown again';

  @override
  String get adminAppEffectEnabledAgain => 'Enabled again';

  @override
  String get adminAppEffectResumeGrants =>
      'Unchanged; incoming webhooks accept calls again';

  @override
  String get adminAppEffectUnchanged => 'Unchanged';

  @override
  String adminAppSuspended(String name) {
    return '$name is suspended.';
  }

  @override
  String adminAppResumed(String name) {
    return '$name is active again.';
  }

  @override
  String adminAppUninstalled(String name) {
    return '$name is uninstalled.';
  }

  @override
  String adminAppUpdated(String name, String version) {
    return '$name now has version $version.';
  }

  @override
  String adminAppConsentSaved(String name) {
    return 'The new consent for $name is saved.';
  }

  @override
  String adminAppPurposeInstall(String name) {
    return 'Install the app $name';
  }

  @override
  String adminAppPurposeUpdate(String name) {
    return 'Update the app $name';
  }

  @override
  String adminAppPurposeScopes(String name) {
    return 'Change the consent for $name';
  }

  @override
  String adminAppPurposeSuspend(String name) {
    return 'Suspend the app $name';
  }

  @override
  String adminAppPurposeResume(String name) {
    return 'Resume the app $name';
  }

  @override
  String adminAppPurposeUninstall(String name) {
    return 'Uninstall the app $name';
  }

  @override
  String adminAppPurposeGrant(String name, String target) {
    return 'Create the access grant for $name on $target';
  }

  @override
  String adminAppPurposeToken(String name) {
    return 'Create a token for $name';
  }

  @override
  String get adminAppInstallTitle => 'Install an app';

  @override
  String get adminAppInstallPickBody =>
      'Choose the manifest file of the app (JSON or YAML). The server checks it before anything is installed.';

  @override
  String get adminAppPick => 'Choose file';

  @override
  String get adminAppChecking => 'Checking the manifest';

  @override
  String adminAppFileChosen(String name) {
    return 'File: $name';
  }

  @override
  String get adminAppInvalid => 'The server refused the manifest:';

  @override
  String adminAppErrorLine(String pointer, String rule) {
    return '$pointer: rule \"$rule\"';
  }

  @override
  String get adminAppManifestRoot => 'the whole manifest';

  @override
  String adminAppConsentIntro(String name, String publisher, String version) {
    return '$name by $publisher, version $version, asks for the following. Read it before you agree.';
  }

  @override
  String get adminAppConsentScopes => 'What the app may read and write';

  @override
  String get adminAppConsentEvents => 'Events the app receives';

  @override
  String get adminAppConsentDestinations => 'Where data are sent';

  @override
  String get adminAppConsentCommands => 'Commands';

  @override
  String get adminAppConsentComponents => 'Components';

  @override
  String get adminAppConsentGrants => 'Access grants the app asks for';

  @override
  String get adminAppConsentEgress =>
      'Data that leave the instance, in the publisher\'s words';

  @override
  String get adminAppConsentNoEgress =>
      'The manifest names no data that leave the instance.';

  @override
  String get adminAppConsentNone => 'None.';

  @override
  String get adminAppComponentButton => 'Buttons';

  @override
  String get adminAppComponentSelect => 'Selection lists';

  @override
  String get adminAppComponentForm => 'Forms';

  @override
  String adminAppGrantAskChannel(String level) {
    return '$level on one channel';
  }

  @override
  String adminAppGrantAskSpace(String level) {
    return '$level in one space';
  }

  @override
  String adminAppGrantAskInstance(String level) {
    return '$level on the whole instance';
  }

  @override
  String adminAppGrantAskOther(String level) {
    return '$level on a target this version does not know';
  }

  @override
  String adminAppGrantPurpose(String purpose) {
    return 'Purpose: $purpose';
  }

  @override
  String get adminAppGrantTarget => 'Target';

  @override
  String get adminAppGrantNone => 'Do not create';

  @override
  String get adminAppConsentToken => 'Token of the app';

  @override
  String get adminAppTokenBody =>
      'The app signs in with this token. Its secret is shown only once.';

  @override
  String get adminAppTokenDpop => 'DPoP key thumbprint (optional)';

  @override
  String get adminAppInstallSubmit => 'Install';

  @override
  String get adminAppGrantsStepTitle => 'Access grants and token';

  @override
  String adminAppGrantsStepBody(String name) {
    return '$name is installed. Check what each access grant changes, then create the access grants and the token.';
  }

  @override
  String get adminAppNoGrants =>
      'No access grant chosen; only the token is created.';

  @override
  String get adminAppCreateGrants => 'Create access grants and token';

  @override
  String adminAppStepInstall(String name) {
    return 'Install $name';
  }

  @override
  String adminAppStepGrant(String target) {
    return 'Access grant on $target';
  }

  @override
  String adminAppStepToken(String name) {
    return 'Token $name';
  }

  @override
  String adminAppStepLine(String step, String status) {
    return '$step: $status';
  }

  @override
  String get adminAppStepFinished => 'finished';

  @override
  String adminAppStepFailed(String reason) {
    return 'failed: $reason';
  }

  @override
  String get adminAppStepWaiting => 'waiting';

  @override
  String adminAppTokenSecretTitle(String name) {
    return 'Token of $name';
  }

  @override
  String get adminAppTokenSecretOnce =>
      'This secret is shown only once. Copy it into the app\'s secret store now.';

  @override
  String adminAppKeyId(String kid) {
    return 'Key ID of the webhook signatures: $kid';
  }

  @override
  String adminAppHomepage(String url) {
    return 'Homepage: $url';
  }

  @override
  String get adminAppPageEvents => 'Subscribed events';

  @override
  String get adminAppTokenNew => 'New token';

  @override
  String adminAppTokenCreateTitle(String name) {
    return 'Create a token for $name';
  }

  @override
  String get adminAppTokenRevokeBody =>
      'The app loses the access of this token at once.';

  @override
  String adminAppUpdateTitle(String name) {
    return 'Update $name';
  }

  @override
  String adminAppUpdateBody(String slug) {
    return 'Choose the new manifest file with the same slug ($slug).';
  }

  @override
  String adminAppDiffTitle(String from, String to) {
    return 'Changes from version $from to $to';
  }

  @override
  String get adminAppDiffScopes => 'What the app may read and write';

  @override
  String get adminAppDiffEvents => 'Events';

  @override
  String get adminAppDiffGrants => 'Requested access grants';

  @override
  String adminAppDiffAdded(String items) {
    return 'Added: $items';
  }

  @override
  String adminAppDiffRemoved(String items) {
    return 'Removed: $items';
  }

  @override
  String get adminAppDiffNone =>
      'Nothing changes in scopes, events, destinations, access grants or commands.';

  @override
  String get adminAppDiffRemovedNow => 'Removed items take effect at once.';

  @override
  String get adminAppDiffHostsConfirmed =>
      'You confirm new destinations with this update.';

  @override
  String get adminAppDiffGrantsLater =>
      'An update never adds an access grant: create added ones under Access grants.';

  @override
  String get adminAppDiffNewConsent =>
      'Added scopes and events need a new consent after the update; until then the app keeps its current ones.';

  @override
  String adminAppNewConsentTitle(String name) {
    return 'New consent for $name';
  }

  @override
  String adminAppNewConsentBody(String version) {
    return 'Version $version asks for more. Choose what the app may use; required items stay on.';
  }

  @override
  String get adminAppNewConsentSubmit => 'Save consent';

  @override
  String get adminAppNewConsentLater => 'Later';

  @override
  String get adminHookTitle => 'Outgoing webhooks';

  @override
  String get adminHookEmpty => 'No subscriptions.';

  @override
  String get adminHookCreate => 'New subscription';

  @override
  String get adminHookStateActive => 'Active';

  @override
  String get adminHookStatePaused => 'Paused';

  @override
  String get adminHookStateDisabled => 'Disabled';

  @override
  String get adminHookStateUnknown => 'Unknown state';

  @override
  String adminHookEvents(String events) {
    return 'Events: $events';
  }

  @override
  String adminHookFailures(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count failures in a row',
      one: '1 failure in a row',
      zero: 'No failures in a row',
    );
    return '$_temp0';
  }

  @override
  String adminHookSigned(String schemes, String kid) {
    return 'Signed with $schemes, key $kid';
  }

  @override
  String adminHookDisabledAt(String when) {
    return 'Disabled $when';
  }

  @override
  String get adminHookReasonGap =>
      'A gap: events left the event log before they were delivered.';

  @override
  String get adminHookReasonGone => 'The receiver answered 410 Gone.';

  @override
  String adminHookReasonFailures(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Disabled after $count failures in a row.',
      one: 'Disabled after 1 failure in a row.',
    );
    return '$_temp0';
  }

  @override
  String get adminHookReasonNone => 'The server names no reason.';

  @override
  String get adminHookPause => 'Pause';

  @override
  String get adminHookResume => 'Resume';

  @override
  String get adminHookDelete => 'Delete';

  @override
  String get adminHookDeliveries => 'Deliveries';

  @override
  String adminHookPauseTitle(String url) {
    return 'Pause the subscription to $url?';
  }

  @override
  String get adminHookPauseBody =>
      'Nothing goes to this address until you resume. Matching events keep being recorded and go out in order after resuming.';

  @override
  String adminHookResumeTitle(String url) {
    return 'Resume the subscription to $url?';
  }

  @override
  String get adminHookResumeBody =>
      'Waiting deliveries go out to this address again, in order.';

  @override
  String adminHookDeleteTitle(String url) {
    return 'Delete the subscription to $url?';
  }

  @override
  String get adminHookDeleteBody => 'No further events go to this address.';

  @override
  String adminHookPausedNotice(String url) {
    return 'The subscription to $url is paused.';
  }

  @override
  String adminHookResumedNotice(String url) {
    return 'The subscription to $url is active.';
  }

  @override
  String adminHookDeletedNotice(String url) {
    return 'The subscription to $url is deleted.';
  }

  @override
  String adminHookCreatedNotice(String url) {
    return 'The subscription to $url is created.';
  }

  @override
  String adminHookFormTitle(String app) {
    return 'New subscription for $app';
  }

  @override
  String get adminHookFormBody =>
      'Events of the chosen types go to this address, signed with v1a. The server accepts only https addresses its webhook settings allow.';

  @override
  String get adminHookUrl => 'Address';

  @override
  String get adminHookUrlHttps => 'The address must start with https://.';

  @override
  String get adminHookEventTypes => 'Event types';

  @override
  String get adminHookNoEvents =>
      'The installation grants no events to subscribe to.';

  @override
  String get adminHookHmac => 'Also sign with HMAC (v1)';

  @override
  String get adminHookSubmit => 'Create subscription';

  @override
  String adminHookPurposeCreate(String app) {
    return 'Create a webhook subscription for $app';
  }

  @override
  String adminHookPurposeChange(String url) {
    return 'Change the webhook subscription to $url';
  }

  @override
  String adminHookPurposeDelete(String url) {
    return 'Delete the webhook subscription to $url';
  }

  @override
  String get adminHookPurposeRedeliver => 'Deliver the event again';

  @override
  String adminHookDeliveriesTitle(String url) {
    return 'Deliveries to $url';
  }

  @override
  String get adminHookDeliveriesEmpty => 'No deliveries yet.';

  @override
  String get adminHookReload => 'Reload';

  @override
  String get adminHookMore => 'Load more';

  @override
  String get adminHookDeliveryPending => 'Waiting';

  @override
  String get adminHookDeliverySucceeded => 'Delivered';

  @override
  String get adminHookDeliveryFailed => 'Failed';

  @override
  String get adminHookDeliveryDead => 'Given up';

  @override
  String get adminHookDeliveryUnknown => 'Unknown status';

  @override
  String adminHookAttempts(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count attempts',
      one: '1 attempt',
    );
    return '$_temp0';
  }

  @override
  String adminHookNextAttempt(String when) {
    return 'next attempt $when';
  }

  @override
  String adminHookLastStatus(String code) {
    return 'last status $code';
  }

  @override
  String adminHookLastError(String error) {
    return 'last error: $error';
  }

  @override
  String adminHookEventSeq(String type, String seq) {
    return '$type, sequence $seq';
  }

  @override
  String adminHookEventGone(String seq) {
    return 'Event left the log, sequence $seq';
  }

  @override
  String adminHookCreatedAt(String when) {
    return 'created $when';
  }

  @override
  String adminHookDeliveredAt(String when) {
    return 'delivered $when';
  }

  @override
  String get adminHookErrorTimeout => 'timed out';

  @override
  String get adminHookErrorConnectionFailed => 'connection failed';

  @override
  String get adminHookErrorTlsFailed => 'TLS handshake or certificate failed';

  @override
  String get adminHookErrorAddressBlocked =>
      'the address resolves to a blocked network';

  @override
  String get adminHookErrorHostNotAllowed =>
      'the target address is not allowed';

  @override
  String get adminHookErrorHttpStatus => 'unexpected HTTP status';

  @override
  String get adminHookErrorThrottled => 'throttled by the receiver';

  @override
  String get adminHookErrorGone => 'the receiver answered 410 Gone';

  @override
  String get adminHookErrorNotAuthorized =>
      'the app may no longer receive this event';

  @override
  String get adminHookErrorNotSubscribed =>
      'the subscription no longer covers this event type';

  @override
  String get adminHookErrorEventExpired => 'the event left the event log';

  @override
  String get adminHookRedeliver => 'Redeliver';

  @override
  String get adminHookRedelivered => 'The delivery is due again.';

  @override
  String get adminHookRedeliverExpired =>
      'The event left the event log; it cannot be delivered again.';

  @override
  String get adminHookKeysTitle => 'Signing keys';

  @override
  String get adminHookKeysBody =>
      'Receivers verify the v1a signature of every delivery with one of these public keys.';

  @override
  String get adminHookKeyActive => 'Active';

  @override
  String get adminHookKeyNext => 'Next';

  @override
  String get adminHookKeyRetiring => 'Retiring';

  @override
  String adminHookKeyUntil(String when) {
    return 'valid until $when';
  }

  @override
  String adminSecretHmacTitle(String url) {
    return 'HMAC secret for $url';
  }

  @override
  String get adminSecretHmacLabel => 'HMAC secret (v1)';

  @override
  String get adminSecretHmacOnce =>
      'This secret is shown only now. Store it at the receiver, which checks the v1 signature with it.';

  @override
  String get adminManifestFileType => 'App manifest';

  @override
  String get adminIncomingTitle => 'Incoming webhooks';

  @override
  String get adminIncomingChannel => 'Channel';

  @override
  String get adminIncomingNoChannels => 'You see no channel.';

  @override
  String get adminIncomingEmpty => 'This channel has no incoming webhooks.';

  @override
  String get adminIncomingCreate => 'Create incoming webhook';

  @override
  String adminIncomingWritesAs(String app) {
    return 'writes as $app';
  }

  @override
  String adminIncomingCreatedAt(String when) {
    return 'created $when';
  }

  @override
  String adminIncomingLastUsed(String when) {
    return 'last used $when';
  }

  @override
  String get adminIncomingNeverUsed => 'never used';

  @override
  String get adminIncomingRevoke => 'Revoke';

  @override
  String adminIncomingRevokeTitle(String name) {
    return 'Revoke the incoming webhook $name?';
  }

  @override
  String get adminIncomingRevokeBody =>
      'Calls of its address are answered with 404 from now on.';

  @override
  String adminIncomingRevokedNotice(String name) {
    return 'The incoming webhook $name is revoked.';
  }

  @override
  String adminIncomingCreatedNotice(String name) {
    return 'The incoming webhook $name is created.';
  }

  @override
  String adminIncomingDialogTitle(String path) {
    return 'New incoming webhook in $path';
  }

  @override
  String get adminIncomingDialogBody =>
      'A service sends messages into this channel through the webhook\'s address. The webhook writes as an app.';

  @override
  String get adminIncomingName => 'Name';

  @override
  String get adminIncomingModeExisting => 'Write as an existing app';

  @override
  String get adminIncomingModeNewApp => 'New incoming app';

  @override
  String get adminIncomingNewAppBody =>
      'Installs a minimal app that may only write messages, with write access on exactly this channel.';

  @override
  String get adminIncomingApp => 'App';

  @override
  String get adminIncomingNoApps => 'No active app is installed.';

  @override
  String get adminIncomingSubmit => 'Create';

  @override
  String get adminIncomingStepInstall => 'Install the incoming app';

  @override
  String adminIncomingStepGrant(String path) {
    return 'Grant write on $path';
  }

  @override
  String get adminIncomingStepWebhook => 'Create the webhook';

  @override
  String adminIncomingStepLine(String step, String state) {
    return '$step: $state';
  }

  @override
  String get adminIncomingStepDone => 'done';

  @override
  String get adminIncomingStepWaiting => 'waiting';

  @override
  String get adminIncomingRetry => 'Retry';

  @override
  String get adminIncomingUninstall => 'Uninstall the incoming app';

  @override
  String get adminIncomingUninstalled => 'The incoming app is uninstalled.';

  @override
  String adminIncomingPurposeCreate(String path) {
    return 'Create an incoming webhook in $path';
  }

  @override
  String adminIncomingPurposeRevoke(String name) {
    return 'Revoke the incoming webhook $name';
  }

  @override
  String adminIncomingPurposeUninstall(String name) {
    return 'Uninstall the incoming app $name';
  }

  @override
  String adminSecretIncomingTitle(String name) {
    return 'Address of $name';
  }

  @override
  String get adminSecretIncomingLabel => 'Webhook address with token';

  @override
  String get adminSecretIncomingOnce =>
      'This address holds the token and is shown only now. Copy it into the sending service.';

  @override
  String get adminSecretIncomingBearer =>
      'Senders that can set a header drop the last segment of the address and send it as Authorization: Bearer <token>.';

  @override
  String get adminNavHints => 'Permission hints';

  @override
  String get adminNavExpiring => 'Expiring soon';

  @override
  String get hintsTitle => 'Permission hints';

  @override
  String get hintsIntro =>
      'Hints from the current permissions: what people can learn through agents and which access grants have no effect. A hint is not a block.';

  @override
  String get hintsScopeLabel => 'Show for';

  @override
  String get hintsScopeInstance => 'Whole instance';

  @override
  String get hintsKindLabel => 'Kind';

  @override
  String get hintsKindAll => 'All kinds';

  @override
  String get hintsKindH1 => 'Read horizon (H1)';

  @override
  String get hintsKindGrantToDisabled =>
      'Access grant to a disabled or deleted account';

  @override
  String get hintsKindTriggerBlocked => 'Trigger agents without effect';

  @override
  String get hintsKindAgentPersonal => 'Agent in Personal mode';

  @override
  String get hintsKindUnknown => 'Other hint';

  @override
  String hintsH1Text(String person, String channelPath, String agent) {
    return '$person can learn content of $channelPath through agent $agent.';
  }

  @override
  String hintsGrantToDisabledText(String subject, String target, String level) {
    return '$subject is disabled or deleted and still holds an access grant on $target: $level.';
  }

  @override
  String hintsTriggerBlockedText(String subject, String target) {
    return '$subject has the level Trigger agents on $target but cannot trigger an agent there.';
  }

  @override
  String get hintsRuleSr2 =>
      'SR2: an agent there has a tool class that only owners and people with a ready dev environment of their own may trigger.';

  @override
  String get hintsRuleSr3 =>
      'SR3: a channel there is bound to a project; a run of a person who is not an owner needs a ready dev environment of their own.';

  @override
  String hintsAgentPersonalText(String agent, String holder) {
    return 'Agent $agent uses Personal mode on the subscription of $holder.';
  }

  @override
  String hintsAgentPersonalNoHolder(String agent) {
    return 'Agent $agent uses Personal mode and has no holder.';
  }

  @override
  String hintsAgentPersonalSet(String people) {
    return 'I1 asks that only the holder can trigger it. These people can trigger it now: $people.';
  }

  @override
  String get hintsAgentPersonalNobody =>
      'I1 asks that only the holder can trigger it. Nobody can trigger it now.';

  @override
  String get hintsUnknownText => 'This version does not know this hint yet.';

  @override
  String get hintsEmpty => 'No permission hints.';

  @override
  String get hintsExplain => 'Explain';

  @override
  String get hintsExtend => 'Extend';

  @override
  String get hintsOpenAgent => 'Open agent';

  @override
  String get hintsClose => 'Close';

  @override
  String hintsExtendTitle(String name) {
    return 'Extend the access grant for $name';
  }

  @override
  String get hintsExtendNote =>
      'Extending creates a new access grant with a new reason and the last day you enter, then revokes the old one.';

  @override
  String hintsExtended(String name) {
    return 'The access grant for $name was extended: the new one applies, the old one is revoked.';
  }

  @override
  String hintsExtendRevokeFailed(String name, String reason) {
    return 'The new access grant for $name applies, but the old one could not be revoked: $reason';
  }

  @override
  String get expiringTitle => 'Expiring soon';

  @override
  String get expiringIntro =>
      'Access grants, group memberships and tokens of agents and apps that end soon. Extending replaces an access grant with a new one; a token is renewed by creating a new one and revoking the old one.';

  @override
  String get expiringEmpty => 'Nothing expires soon.';

  @override
  String get expiringKindGrant => 'Access grant';

  @override
  String get expiringKindMembership => 'Group membership';

  @override
  String get expiringKindToken => 'Token';

  @override
  String get expiringKindUnknown => 'Other access';

  @override
  String expiringGrantText(String subject, String target, String level) {
    return '$subject on $target: $level';
  }

  @override
  String expiringMembershipText(String person, String group) {
    return '$person in the group $group';
  }

  @override
  String expiringAgentTokenText(String agent) {
    return 'Token of the agent $agent';
  }

  @override
  String expiringAppTokenText(String app) {
    return 'Token of the app $app';
  }

  @override
  String get expiringAppTokenNote =>
      'There is no token screen for apps here yet: create a new token for the app, switch the app over, then revoke the old one.';

  @override
  String get expiringUnknownText =>
      'This version does not know this kind of access yet.';

  @override
  String get expiringOpenGroup => 'Open group';

  @override
  String get expiringRenewToken => 'Renew token';

  @override
  String get adminNavInstance => 'Instance and branding';

  @override
  String get instanceTitle => 'Instance and branding';

  @override
  String get instanceTabSettings => 'Settings';

  @override
  String get instanceTabBranding => 'Branding';

  @override
  String get instanceTabProviders => 'Login providers';

  @override
  String get instanceTabFlags => 'Feature flags';

  @override
  String get instanceSave => 'Save';

  @override
  String get instanceValueNone => 'no value';

  @override
  String get instanceValueEmpty => 'empty';

  @override
  String get instanceNeverChanged => 'Default, never changed';

  @override
  String instanceChangedBy(int version, String name, String when) {
    return 'Version $version, changed by $name, $when';
  }

  @override
  String instanceChangedAt(int version, String when) {
    return 'Version $version, changed $when';
  }

  @override
  String instanceChangeFromTo(String from, String to) {
    return 'From: $from\nTo: $to';
  }

  @override
  String instanceConflict(String current, String input) {
    return 'Another owner changed this setting in the meantime. Now stored: $current. Your input: $input. Save again to replace the stored value with your input.';
  }

  @override
  String get instanceFieldRequired => 'This field is required.';

  @override
  String get instanceFieldTooLong => 'This is too long for the server.';

  @override
  String get instanceFieldInvalidText =>
      'Contains a character that is not allowed, for example a control or text direction character.';

  @override
  String get instanceFieldUnknownColor =>
      'Only one of the palette colors can be chosen.';

  @override
  String get instanceFieldInvalidEnum =>
      'This value is not one of the allowed values.';

  @override
  String get instanceFieldInvalidFormat =>
      'This value does not have the expected form.';

  @override
  String get instanceFieldInvalidType => 'This value has the wrong type.';

  @override
  String get instanceFieldNotAllowed =>
      'This field is not allowed with these settings.';

  @override
  String get instanceFieldDuplicate => 'An entry appears twice.';

  @override
  String get instanceFieldUnknown => 'The server does not know this field.';

  @override
  String instanceFieldOther(String code) {
    return 'The server refused this value ($code).';
  }

  @override
  String get instanceSettingsIntro =>
      'These settings apply to the whole instance and take effect immediately.';

  @override
  String get instanceKindText => 'Text';

  @override
  String get instanceKindUuid => 'File ID';

  @override
  String instanceKindPalette(String keys) {
    return 'One of the palette colors: $keys';
  }

  @override
  String instanceKindLocale(String tags) {
    return 'Language: $tags';
  }

  @override
  String get instanceSettingChange => 'Change';

  @override
  String get instanceSettingOnBranding => 'Changed under Branding.';

  @override
  String get instanceSettingUnknownKey =>
      'This version has no editor for this key; change it through the API.';

  @override
  String get instanceSettingValue => 'Value';

  @override
  String instanceSettingConfirmTitle(String key) {
    return 'Change $key?';
  }

  @override
  String instanceSettingSaved(String key, String value) {
    return '$key is now $value.';
  }

  @override
  String get instanceBrandingName => 'Product name';

  @override
  String get instanceBrandingNameHelp =>
      'Shown to everyone in the apps and the console. The server checks the length and refuses control and text direction characters.';

  @override
  String get instanceAccentLabel => 'Accent';

  @override
  String get instanceAccentHelp =>
      'The accent comes only from the checked palette; there is no free color value.';

  @override
  String get instanceAccentSpruce => 'Spruce';

  @override
  String get instanceAccentLoch => 'Loch';

  @override
  String get instanceAccentHeather => 'Heather';

  @override
  String get instanceAccentRowan => 'Rowan';

  @override
  String get instanceAccentAmber => 'Amber';

  @override
  String get instanceAccentGorse => 'Gorse';

  @override
  String get instanceAccentMoss => 'Moss';

  @override
  String get instancePreviewLight => 'Light';

  @override
  String get instancePreviewDark => 'Dark';

  @override
  String get instancePreviewButton => 'Send';

  @override
  String get instancePreviewChip => 'Selected';

  @override
  String get instancePreviewLink => 'A link';

  @override
  String get instanceBrandingConfirmTitle => 'Change the branding?';

  @override
  String instanceBrandingNameChange(String from, String to) {
    return 'Product name: $from becomes $to';
  }

  @override
  String instanceBrandingAccentChange(String from, String to) {
    return 'Accent: $from becomes $to';
  }

  @override
  String get instanceBrandingEveryone =>
      'Everyone sees the change immediately.';

  @override
  String get instanceBrandingSaved => 'The branding is saved.';

  @override
  String instanceBrandingConflict(String name, String accent) {
    return 'Another owner changed the branding in the meantime: now $name with the accent $accent. Your input stays; save again to replace it.';
  }

  @override
  String get instanceLogoTitle => 'Logo';

  @override
  String get instanceLogoNone => 'No logo is set.';

  @override
  String get instanceLogoPending =>
      'The logo is being checked and appears when it is ready.';

  @override
  String get instanceLogoRefresh => 'Check again';

  @override
  String get instanceLogoUpload => 'Upload logo';

  @override
  String get instanceLogoRemove => 'Remove logo';

  @override
  String get instanceLogoConfirmTitle => 'Upload the logo?';

  @override
  String instanceLogoConfirmBody(String file) {
    return '$file replaces the current logo for everyone. The server checks it and removes metadata before it is shown.';
  }

  @override
  String get instanceLogoUploaded => 'The logo is uploaded.';

  @override
  String get instanceLogoRemoveTitle => 'Remove the logo?';

  @override
  String get instanceLogoRemoveBody =>
      'Everyone sees the product name without a logo.';

  @override
  String get instanceLogoRemoved => 'The logo is removed.';

  @override
  String get instanceProvidersIntro =>
      'People sign in through these providers. Changes take effect immediately.';

  @override
  String get instanceProvidersEmpty => 'No login provider is set up.';

  @override
  String get instanceProviderAdd => 'Add provider';

  @override
  String get instanceProviderKindOidc => 'OpenID Connect';

  @override
  String get instanceProviderKindHeader => 'Proxy header';

  @override
  String get instanceProviderKindLocal => 'Local accounts';

  @override
  String get instanceProviderKindUnknown => 'Unknown driver';

  @override
  String get instanceProviderStateEnabled => 'Enabled';

  @override
  String get instanceProviderStateDisabled => 'Disabled';

  @override
  String get instanceProviderProvisioningShort => 'Creates accounts';

  @override
  String instanceProviderHeaderLine(String header) {
    return 'Login header: $header';
  }

  @override
  String instanceProviderIssuerLine(String issuer) {
    return 'Issuer: $issuer';
  }

  @override
  String get instanceProviderChange => 'Change';

  @override
  String get instanceProviderEnable => 'Enable';

  @override
  String get instanceProviderDisable => 'Disable';

  @override
  String get instanceProviderDelete => 'Delete';

  @override
  String instanceProviderDisableTitle(String name) {
    return 'Disable $name?';
  }

  @override
  String instanceProviderDisableBody(String name) {
    return 'All sessions that started with $name end immediately. Nobody can sign in with it until it is enabled again.';
  }

  @override
  String instanceProviderEnableTitle(String name) {
    return 'Enable $name?';
  }

  @override
  String instanceProviderEnableBody(String name) {
    return 'People can sign in with $name again.';
  }

  @override
  String instanceProviderDeleteTitle(String name) {
    return 'Delete $name?';
  }

  @override
  String instanceProviderDeleteBody(String name) {
    return 'All sessions that started with $name end immediately. The server refuses while identities are linked to it; disable it then.';
  }

  @override
  String instanceProviderDisabled(String name) {
    return '$name is disabled; its sessions ended.';
  }

  @override
  String instanceProviderEnabled(String name) {
    return '$name is enabled.';
  }

  @override
  String instanceProviderDeleted(String name) {
    return '$name is deleted.';
  }

  @override
  String instanceProviderAdded(String name) {
    return '$name is added.';
  }

  @override
  String instanceProviderSaved(String name) {
    return '$name is saved.';
  }

  @override
  String get instanceProviderLoopbackRule =>
      'The proxy header provider works only while the server listens on a loopback address (127.0.0.1 or ::1); otherwise the server refuses to enable it.';

  @override
  String get instanceProviderLoopbackRefused =>
      'The server refused: the proxy header provider needs a server that listens only on a loopback address.';

  @override
  String get instanceProviderRedirectLabel =>
      'Redirect URI to register at the OIDC provider';

  @override
  String get instanceProviderRedirectCopy => 'Copy';

  @override
  String get instanceProviderRedirectCopied => 'Copied';

  @override
  String instanceProviderAddTitle(String kind) {
    return 'Add $kind provider';
  }

  @override
  String instanceProviderEditTitle(String name) {
    return 'Change $name';
  }

  @override
  String get instanceProviderName => 'Name';

  @override
  String get instanceProviderNameHelp => 'Shown on the login button.';

  @override
  String get instanceProviderIssuer => 'Issuer';

  @override
  String get instanceProviderIssuerHelp =>
      'Exact https address of the issuer, for example https://login.example.com';

  @override
  String get instanceProviderClientId => 'Client ID';

  @override
  String get instanceProviderClientAuth => 'Client authentication';

  @override
  String get instanceProviderSecretRef => 'Client secret reference';

  @override
  String get instanceProviderKeyRef => 'Private key reference';

  @override
  String get instanceProviderSecretRefHelp =>
      'A reference such as file:/srv/secrets/idp/client or env:BOTHY_IDP_CLIENT, never the secret itself. It is never shown again.';

  @override
  String get instanceProviderSecretKeep =>
      'Leave empty to keep the stored reference.';

  @override
  String get instanceProviderKeyAlg => 'Key algorithm';

  @override
  String get instanceProviderKeyId => 'Key ID';

  @override
  String get instanceProviderX5t => 'Certificate thumbprint (x5t#S256)';

  @override
  String get instanceProviderTenants => 'Allowed tenant IDs';

  @override
  String instanceProviderTenantsHelp(String tenantid) {
    return 'Separated by commas; required when the issuer contains $tenantid.';
  }

  @override
  String get instanceProviderAppRedirects => 'App redirect URIs';

  @override
  String get instanceProviderAppRedirectsHelp =>
      'One per line: private schemes or claimed https addresses of the apps.';

  @override
  String get instanceProviderGroupsClaim => 'Group claim';

  @override
  String get instanceProviderGroupsClaimHelp =>
      'Without a group claim, groups are not synchronized.';

  @override
  String get instanceProviderLoginHeader => 'Login header';

  @override
  String get instanceProviderLoginHeaderHelp =>
      'For example Tailscale-User-Login';

  @override
  String get instanceProviderNameHeader => 'Name header (optional)';

  @override
  String get instanceProviderLocalNote =>
      'Local accounts sign in with password and TOTP or a passkey; there is nothing to configure.';

  @override
  String get instanceProviderEnableNow => 'Enable at once';

  @override
  String get instanceProviderProvisioning =>
      'Create accounts for unknown people';

  @override
  String get instanceProviderProvisioningHelp =>
      'Otherwise a person needs an enrollment code for the first login.';

  @override
  String instanceProviderCreateSummary(String kind, String name) {
    return 'A new $kind provider named $name is added.';
  }

  @override
  String get instanceProviderStartsEnabled => 'It is enabled at once.';

  @override
  String get instanceProviderStartsDisabled => 'It starts disabled.';

  @override
  String get instanceProviderProvisioningOn =>
      'Unknown people get an account at their first login.';

  @override
  String get instanceProviderProvisioningOff =>
      'Unknown people need an enrollment code.';

  @override
  String instanceProviderNameChange(String from, String to) {
    return 'Name: $from becomes $to';
  }

  @override
  String instanceProviderMembersChange(String members) {
    return 'Changed settings: $members';
  }

  @override
  String stepUpPurposeInstanceSetting(String key) {
    return 'Change the instance setting $key';
  }

  @override
  String get stepUpPurposeBranding => 'Change the branding of the instance';

  @override
  String get stepUpPurposeLogo => 'Upload the logo of the instance';

  @override
  String get stepUpPurposeLogoRemove => 'Remove the logo of the instance';

  @override
  String stepUpPurposeProviderAdd(String name) {
    return 'Add the login provider $name';
  }

  @override
  String stepUpPurposeProviderEdit(String name) {
    return 'Change the login provider $name';
  }

  @override
  String stepUpPurposeProviderDisable(String name) {
    return 'Disable the login provider $name';
  }

  @override
  String stepUpPurposeProviderEnable(String name) {
    return 'Enable the login provider $name';
  }

  @override
  String stepUpPurposeProviderDelete(String name) {
    return 'Delete the login provider $name';
  }

  @override
  String stepUpPurposeFlagOn(String key) {
    return 'Switch on the feature flag $key';
  }

  @override
  String stepUpPurposeFlagOff(String key) {
    return 'Switch off the feature flag $key';
  }

  @override
  String get instanceFlagsIntro =>
      'Feature flags switch functions for the whole instance. The state for you includes rollouts to people and groups.';

  @override
  String get instanceFlagOn => 'on';

  @override
  String get instanceFlagOff => 'off';

  @override
  String get instanceFlagKindRelease => 'Release switch';

  @override
  String get instanceFlagKindOption => 'Option';

  @override
  String get instanceFlagKindUnknown => 'Unknown kind';

  @override
  String get instanceFlagOrphaned => 'No longer registered';

  @override
  String instanceFlagForYou(String state) {
    return 'For you: $state';
  }

  @override
  String get instanceFlagForYouUnknown => 'For you: not evaluated';

  @override
  String instanceFlagStored(String state) {
    return 'Stored: $state';
  }

  @override
  String instanceFlagDefault(String state) {
    return 'Default: $state';
  }

  @override
  String instanceFlagRollout(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Inverted for $count people or groups',
      one: 'Inverted for 1 person or group',
    );
    return '$_temp0';
  }

  @override
  String instanceFlagRolloutStays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'The rollout to $count people or groups stays.',
      one: 'The rollout to 1 person or group stays.',
    );
    return '$_temp0';
  }

  @override
  String instanceFlagOnTitle(String key) {
    return 'Switch on $key?';
  }

  @override
  String instanceFlagOffTitle(String key) {
    return 'Switch off $key?';
  }

  @override
  String get instanceFlagEveryone =>
      'The global value changes for everyone immediately.';

  @override
  String get instanceFlagReason => 'Reason (required)';

  @override
  String get instanceFlagSwitchOn => 'Switch on';

  @override
  String get instanceFlagSwitchOff => 'Switch off';

  @override
  String instanceFlagSwitchedOn(String key) {
    return '$key is switched on.';
  }

  @override
  String instanceFlagSwitchedOff(String key) {
    return '$key is switched off.';
  }

  @override
  String instanceFlagConflict(String key, String state) {
    return 'Another owner changed $key in the meantime; it is now $state. Nothing was changed; switch again if needed.';
  }

  @override
  String get adminLogoFileType => 'Image';

  @override
  String get notificationPlaceholder => 'New message';

  @override
  String notificationTitle(String name, String title) {
    return '$name · $title';
  }

  @override
  String notificationText(String author, String excerpt) {
    return '$author: $excerpt';
  }

  @override
  String notificationBundled(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count new messages',
      one: '1 new message',
    );
    return '$_temp0';
  }

  @override
  String get notificationSecurityTitle => 'New device';

  @override
  String get notificationSecurityText =>
      'A new device is waiting for your approval.';

  @override
  String get notificationAdminTitle => 'Administration';

  @override
  String get notificationAdminText =>
      'A reminder from the administration is waiting.';

  @override
  String get pushTitle => 'New notification';

  @override
  String get pushBodyMention => 'You were mentioned';

  @override
  String get pushBodyDm => 'New direct message';

  @override
  String get pushBodyRunDone => 'A run has finished';

  @override
  String get pushBodyRunFailed => 'A run has failed';

  @override
  String get pushBodyInputNeeded => 'An agent is waiting for your answer';

  @override
  String get pushMessageNotAvailable => 'Message not available';

  @override
  String get trayOpen => 'Open';

  @override
  String get trayQuit => 'Quit';

  @override
  String get settingsNotificationsWhileRunning =>
      'Notifications arrive only while the app is running.';

  @override
  String get settingsStartAtLogin => 'Start at login';

  @override
  String get settingsStartAtLoginUnavailable =>
      'This system does not offer starting at login for the app.';

  @override
  String get importTitle => 'Import from Discord';

  @override
  String get importAdd => 'Add source';

  @override
  String get importRefresh => 'Refresh';

  @override
  String get importEmpty =>
      'No import source yet. Add one with the mapping file of the import tool.';

  @override
  String get importStateDraft => 'Draft';

  @override
  String get importStateRunning => 'Started';

  @override
  String get importStatePaused => 'Paused';

  @override
  String get importStateCompleted => 'Completed';

  @override
  String get importStateFailed => 'Failed';

  @override
  String get importStateCancelled => 'Cancelled';

  @override
  String importItemsImported(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count items imported',
      one: '1 item imported',
    );
    return '$_temp0';
  }

  @override
  String get importBack => 'All import sources';

  @override
  String get importServerCounters => 'Counters of the server';

  @override
  String get importToolCounters => 'Counters of the import tool';

  @override
  String get importNoCounters => 'No counters yet.';

  @override
  String get importFailedItems => 'Failed items';

  @override
  String get importNoFailedItems => 'No failed items.';

  @override
  String get importChannelMapping => 'Channel mapping';

  @override
  String get importNoChannels => 'The mapping names no channel to import.';

  @override
  String importChannelDiscord(String id) {
    return 'Discord ID $id';
  }

  @override
  String get importChannelUnknownName => 'Name not readable';

  @override
  String get importPhaseNotImported => 'Not imported';

  @override
  String get importPhaseMirror => 'Mirror, read-only';

  @override
  String get importPhaseOperatorOk => 'Led here, operator ok';

  @override
  String get importPhaseNoOperator => 'Led here, no operator';

  @override
  String get importPhaseOtherOperator =>
      'Led here, other operator than planned';

  @override
  String get importReadOnly => 'Read-only';

  @override
  String get importWritable => 'Writable';

  @override
  String importPlanned(String agent) {
    return 'Planned operator: $agent';
  }

  @override
  String get importPlannedNone => 'No operator planned';

  @override
  String get importChannelUnreadable => 'The channel could not be read.';

  @override
  String get importLeadHereAction => 'Lead here';

  @override
  String get importLeadDiscordAction => 'Discord leads';

  @override
  String get importSetOperatorAction => 'Set operator';

  @override
  String get importStart => 'Start';

  @override
  String get importPause => 'Pause';

  @override
  String get importCancel => 'Cancel import';

  @override
  String get importComplete => 'Complete';

  @override
  String get importCompleteHint =>
      'Completing is possible once every imported channel is led here.';

  @override
  String importStartTitle(String name) {
    return 'Start $name?';
  }

  @override
  String get importStartBody =>
      'The import tool may then write into this source with --apply. Nothing is imported until you use the tool.';

  @override
  String importPauseTitle(String name) {
    return 'Pause $name?';
  }

  @override
  String get importPauseBody =>
      'The import tool cannot write into this source until you start it again. The leads stay as they are.';

  @override
  String importCancelTitle(String name) {
    return 'Cancel $name?';
  }

  @override
  String get importCancelBody =>
      'The source ends as cancelled and cannot be started again. What was imported stays; the import tool can no longer write into this source.';

  @override
  String importCompleteTitle(String name) {
    return 'Complete $name?';
  }

  @override
  String get importCompleteBody =>
      'The mirror ends and the source is completed. The mapping in import_items stays until the retention class import is deleted. After that deletion a repeated import is no longer idempotent: it would import everything again.';

  @override
  String importLeadHereTitle(String name) {
    return 'Lead $name here?';
  }

  @override
  String get importLeadHereOrder =>
      'Keep this order, so two places never lead at the same time:';

  @override
  String get importLeadHereStepDiscord =>
      '1. In Discord you set the channel read-only and removed the agent there.';

  @override
  String importLeadHereStepSwitch(String command) {
    return '2. $command succeeded.';
  }

  @override
  String get importLeadHereStepLead =>
      '3. Now this server leads: the channel here becomes writable.';

  @override
  String importLeadHereStepOperator(String agent) {
    return '4. Then you set the operator $agent.';
  }

  @override
  String get importLeadHereStepNoOperator =>
      '4. The mapping plans no operator for this channel.';

  @override
  String importLeadHereOtherOperator(String agent) {
    return 'The channel has another operator than the planned $agent. Imported threads keep $agent; with another operator they stay without an agent.';
  }

  @override
  String importLeadDiscordTitle(String name) {
    return 'Discord leads in $name?';
  }

  @override
  String importLeadDiscordBody(String command) {
    return 'The channel here becomes read-only for everyone, owners too, and answers of agents in progress are cancelled. Messages written here stay here; $command counts them. Afterwards you can remove the operator.';
  }

  @override
  String importLeadHereDone(String name) {
    return '$name is led here.';
  }

  @override
  String importLeadDiscordDone(String name) {
    return 'Discord leads in $name; it is read-only here.';
  }

  @override
  String importStarted(String name) {
    return '$name is started.';
  }

  @override
  String importPausedDone(String name) {
    return '$name is paused.';
  }

  @override
  String importCancelledDone(String name) {
    return '$name is cancelled.';
  }

  @override
  String importCompletedDone(String name) {
    return '$name is completed.';
  }

  @override
  String get importConflictState =>
      'The source is in another state now. The page shows the current state.';

  @override
  String get importConflictLeadNotHere =>
      'Not every imported channel is led here yet. Switch each channel over first.';

  @override
  String get importConflictNotImported => 'This channel is not imported yet.';

  @override
  String get importCreateTitle => 'Add import source';

  @override
  String get importCreateName => 'Name of the source';

  @override
  String get importCreateHint =>
      'The mapping file is the YAML of the import tool (bothy import discord init-map writes a skeleton). It holds no secret and is not stored in the browser.';

  @override
  String get importCreatePick => 'Choose mapping file';

  @override
  String get importCreateFileRead => 'Mapping file read.';

  @override
  String get importCreateSubmit => 'Add';

  @override
  String importFileInvalidYaml(int line, int column) {
    return 'The file is no valid YAML (line $line, column $column). A key named twice is refused too.';
  }

  @override
  String get importFileInvalid => 'The file is no valid YAML.';

  @override
  String get importFileNotMapping =>
      'The file must be one YAML mapping with text keys.';

  @override
  String get importFileUnreadable => 'The file is no UTF-8 text.';

  @override
  String get importServerErrors => 'The server refused the mapping:';

  @override
  String importFieldError(String path, String problem) {
    return '$path: $problem';
  }

  @override
  String get importFieldWholeFile => 'the file';

  @override
  String get importFieldRequired => 'is required';

  @override
  String get importFieldUnknownMember => 'is not a member of the mapping file';

  @override
  String get importFieldInvalid => 'has a wrong type or value';

  @override
  String get importFieldNotDigits =>
      'must be a Discord ID in quotes, digits only';

  @override
  String get importFieldDuplicate => 'names a Discord ID a second time';

  @override
  String get importFieldTooLong => 'is too long';

  @override
  String get importFieldUnknownHandle =>
      'names no person or agent with this handle';

  @override
  String get importFieldNotActive =>
      'names an agent or person that is not active';

  @override
  String get importFieldUnknownExisting => 'names nothing that exists here';

  @override
  String get importFieldUnknownCategory =>
      'names no category of the list spaces';

  @override
  String get importFieldNotEnabled => 'is not available yet';

  @override
  String importCreateConfirmTitle(String name) {
    return 'Add $name?';
  }

  @override
  String get importCreateConfirmBody =>
      'The server stores the mapping without secrets. The source starts as a draft; nothing is imported yet.';

  @override
  String get importNextSteps => 'Next steps:';

  @override
  String importNextCheck(String command) {
    return '1. Check the mapping with $command; it only reads.';
  }

  @override
  String get importNextStart => '2. Start the source here.';

  @override
  String importNextApply(String command) {
    return '3. Import with $command.';
  }

  @override
  String importOperatorTitle(String name) {
    return 'Operator of $name';
  }

  @override
  String get importOperatorBody =>
      'The channel is led here now. Set the planned operator, so the imported threads and the channel have the same agent.';

  @override
  String get importOperatorAgent => 'Agent';

  @override
  String importOperatorNotPlanned(String agent, String planned) {
    return '$agent is not the planned operator $planned. Imported threads keep $planned; they stay without an agent in this channel.';
  }

  @override
  String get importOperatorSet => 'Set operator';

  @override
  String get importOperatorSkip => 'Not now';

  @override
  String importOperatorDone(String agent, String name) {
    return '$agent is the operator of $name.';
  }

  @override
  String importOperatorRemoveTitle(String name) {
    return 'Remove the operator of $name?';
  }

  @override
  String get importOperatorRemoveBody =>
      'Discord leads again. Without an operator no agent answers here.';

  @override
  String get importOperatorRemove => 'Remove operator';

  @override
  String importOperatorRemoved(String name) {
    return '$name has no operator now.';
  }

  @override
  String importFailedItem(String kind, String origin, String error) {
    return '$kind $origin: $error';
  }

  @override
  String stepUpPurposeImportCreate(String name) {
    return 'Add the import source $name';
  }

  @override
  String stepUpPurposeImportStart(String name) {
    return 'Start the import $name';
  }

  @override
  String stepUpPurposeImportPause(String name) {
    return 'Pause the import $name';
  }

  @override
  String stepUpPurposeImportCancel(String name) {
    return 'Cancel the import $name';
  }

  @override
  String stepUpPurposeImportComplete(String name) {
    return 'Complete the import $name';
  }

  @override
  String stepUpPurposeImportLead(String name) {
    return 'Change the lead of $name';
  }

  @override
  String stepUpPurposeImportOperator(String name) {
    return 'Set the operator of $name';
  }

  @override
  String get notifyLevelAll => 'All';

  @override
  String get notifyLevelMentions => 'Mentions';

  @override
  String get notifyLevelMute => 'Mute';

  @override
  String notifyInherits(String level) {
    return 'inherits: $level';
  }

  @override
  String notifyInheritFrom(String level, String source) {
    return 'Inherit ($level from $source)';
  }

  @override
  String get notifySourceThread => 'the thread';

  @override
  String get notifySourceChannel => 'the channel';

  @override
  String get notifySourceSpace => 'the space';

  @override
  String get notifySourceChannelDefault => 'the default of the channel';

  @override
  String get notifySourceDm => 'the direct message';

  @override
  String get notifySourcePersonalDefault => 'your default';

  @override
  String get notifySourceInstanceDefault => 'the default of the instance';

  @override
  String get notifyFollowing => 'Following';

  @override
  String notifyMutedUntil(String when) {
    return 'muted until $when';
  }

  @override
  String get notifyMenuNotifications => 'Notifications';

  @override
  String get notifyMenuMuteUntil => 'Mute until';

  @override
  String get notifyMuteOneHour => 'For 1 hour';

  @override
  String notifyMuteTomorrow(String time) {
    return 'Tomorrow $time';
  }

  @override
  String get notifyMuteChooseDate => 'Choose date and time';

  @override
  String notifyMenuOf(String name) {
    return 'Menu of $name';
  }

  @override
  String get notifyMenuOpen => 'More';

  @override
  String get channelMenuExport => 'Export history';

  @override
  String exportTitle(String name) {
    return 'Export $name';
  }

  @override
  String get exportFrom => 'From';

  @override
  String get exportTo => 'To';

  @override
  String get exportRangeInvalid =>
      'The first day must not be after the last day.';

  @override
  String get exportStart => 'Choose file and export';

  @override
  String exportProgress(String size) {
    return '$size written';
  }

  @override
  String exportDone(String size) {
    return 'Export saved: $size.';
  }

  @override
  String get exportCancelled =>
      'Export cancelled. The incomplete file was deleted.';

  @override
  String get exportBrokeOff =>
      'The export broke off. The incomplete file was deleted.';

  @override
  String get exportCancel => 'Cancel';

  @override
  String get exportClose => 'Close';

  @override
  String stepUpPurposeChannelExport(String name) {
    return 'Export the history of $name';
  }

  @override
  String get notifySettingsDefault => 'Default for each new channel';

  @override
  String get notifySettingsDefaultHint => 'Applies where nothing else is set.';

  @override
  String get notifySettingsRuns => 'Runs I triggered';

  @override
  String get notifySettingsRunsExplain =>
      'A question of your run always notifies you, also when muted. Its end or failure follows the level.';

  @override
  String get notifySettingsPreview => 'Content in notifications';

  @override
  String get notifyPreviewFull => 'Text and names';

  @override
  String get notifyPreviewNames => 'Names only';

  @override
  String get notifyPreviewNone => 'Only New message';

  @override
  String get notifySettingsQuietHours => 'Quiet hours';

  @override
  String get notifySettingsInputBreaks => 'Questions break quiet hours';

  @override
  String get notifySettingsInputBreaksHint =>
      'A question of your own run stays audible.';

  @override
  String get notifySettingsAdmin => 'Administration reminders';

  @override
  String get notifySettingsAdminHint =>
      'For example an access grant that expires.';

  @override
  String get notifySettingsTree => 'Per space, channel and thread';

  @override
  String get notifySettingsTreeEmpty => 'Nothing to show yet.';

  @override
  String get notifyChoiceInherit => 'Inherit';

  @override
  String get quietHoursNone => 'No quiet hours.';

  @override
  String get quietHoursAdd => 'Add quiet hours';

  @override
  String get quietHoursEdit => 'Edit';

  @override
  String get quietHoursDelete => 'Delete';

  @override
  String get quietHoursAllDay => 'All day';

  @override
  String quietHoursRange(String start, String end) {
    return '$start to $end';
  }

  @override
  String get quietHoursOff => 'off';

  @override
  String get quietHoursDayMon => 'Mon';

  @override
  String get quietHoursDayTue => 'Tue';

  @override
  String get quietHoursDayWed => 'Wed';

  @override
  String get quietHoursDayThu => 'Thu';

  @override
  String get quietHoursDayFri => 'Fri';

  @override
  String get quietHoursDaySat => 'Sat';

  @override
  String get quietHoursDaySun => 'Sun';

  @override
  String get quietHoursEditorTitle => 'Quiet hours';

  @override
  String get quietHoursDays => 'Days';

  @override
  String get quietHoursStart => 'Start';

  @override
  String get quietHoursEnd => 'End';

  @override
  String get quietHoursTimeZone => 'Time zone';

  @override
  String quietHoursTimeZoneMine(String zone) {
    return 'Mine ($zone)';
  }

  @override
  String get quietHoursEnabled => 'On';

  @override
  String get quietHoursSave => 'Save';

  @override
  String get quietHoursCancel => 'Cancel';

  @override
  String get quietHoursTimeInvalid => 'Use HH:MM, for example 07:30.';

  @override
  String get quietHoursNoDays => 'Choose at least one day.';

  @override
  String get searchTitle => 'Search';

  @override
  String get searchFieldLabel => 'Search text';

  @override
  String get searchFieldHint => 'Search messages, threads and files';

  @override
  String get searchSubmit => 'Search';

  @override
  String get searchFilters => 'Filter';

  @override
  String get searchFilterSpace => 'Space';

  @override
  String get searchFilterChannel => 'Channel';

  @override
  String get searchFilterFrom => 'From';

  @override
  String get searchFilterAll => 'all';

  @override
  String get searchFilterType => 'Type';

  @override
  String get searchTypeMessages => 'Messages';

  @override
  String get searchTypeThreads => 'Threads';

  @override
  String get searchTypeFiles => 'Files';

  @override
  String get searchFilterTime => 'Time';

  @override
  String get searchTimeAny => 'any time';

  @override
  String get searchTimeDay => 'last 24 hours';

  @override
  String get searchTimeWeek => 'last 7 days';

  @override
  String get searchTimeMonth => 'last 30 days';

  @override
  String get searchFilterStatus => 'Status';

  @override
  String get searchStatusAll => 'open and done';

  @override
  String get searchStatusOpen => 'open';

  @override
  String get searchStatusDone => 'done';

  @override
  String get searchHasFile => 'With files only';

  @override
  String get searchSort => 'Sort';

  @override
  String get searchSortRelevance => 'Relevance';

  @override
  String get searchSortNewest => 'Newest first';

  @override
  String searchResultCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count results, only from channels you may see',
      one: '1 result, only from channels you may see',
    );
    return '$_temp0';
  }

  @override
  String get searchNoResults => 'No results';

  @override
  String get searchLocalOnly => 'Messages stored only on this device';

  @override
  String get searchCandidatesCapped =>
      'There are too many matches. Narrow the search to see the best ones.';

  @override
  String get searchLoadMore => 'Load more';

  @override
  String get searchQueryEmpty => 'Enter at least one word.';

  @override
  String get searchQueryTooLong => 'The search text is too long.';

  @override
  String get searchFailed => 'The search did not work. Try again.';

  @override
  String get searchHitThread => 'Thread';

  @override
  String get searchHitFile => 'File';

  @override
  String get searchHitTranscript => 'Transcript';

  @override
  String searchHitLabel(String path, String author, String time, String text) {
    return '$path, $author, $time: $text';
  }

  @override
  String get switcherLabel => 'Quick switcher';

  @override
  String get switcherHint =>
      'Jump to a channel, thread, person, agent or command';

  @override
  String get switcherFilterAll => 'All';

  @override
  String get switcherFilterChannels => 'Channels';

  @override
  String get switcherFilterThreads => 'Threads';

  @override
  String get switcherFilterPeople => 'People';

  @override
  String get switcherFilterAgents => 'Agents';

  @override
  String get switcherFilterCommands => 'Commands';

  @override
  String get switcherTabHint => 'Tab: Filter';

  @override
  String get switcherKindSpace => 'Space';

  @override
  String get switcherKindChannel => 'Channel';

  @override
  String get switcherKindThread => 'Thread';

  @override
  String get switcherKindPerson => 'Person';

  @override
  String get switcherKindAgent => 'Agent';

  @override
  String get switcherKindCommand => 'Command';

  @override
  String get switcherNothing => 'Nothing found';

  @override
  String get switcherClose => 'Close';

  @override
  String get mentionsTitle => 'Mentions';

  @override
  String get mentionsEmpty => 'Nobody has mentioned you yet.';

  @override
  String get mentionsFailed => 'The mentions could not be loaded. Try again.';

  @override
  String get mentionsRetry => 'Try again';

  @override
  String runningTitle(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Running now ($count)',
      zero: 'Running now',
    );
    return '$_temp0';
  }

  @override
  String get runningFilterSpace => 'Space';

  @override
  String get runningFilterAgent => 'Agent';

  @override
  String get runningFilterAll => 'all';

  @override
  String get runningMineOnly => 'mine only';

  @override
  String get runningStop => 'Stop';

  @override
  String get runningOpen => 'Open';

  @override
  String get runningEmpty => 'No run is active.';

  @override
  String runningSumOrganization(String amount, String estimate) {
    return 'Organization: sum of active runs $amount ($estimate).';
  }

  @override
  String get runningPersonalNote => 'Personal runs: information only.';

  @override
  String runningContext(String percent) {
    return '$percent context';
  }

  @override
  String runningRowLabel(
    String agent,
    String path,
    String state,
    String duration,
  ) {
    return '$agent, $path, $state, $duration';
  }

  @override
  String get runningFailed =>
      'The active runs could not be loaded; the list shows what this device knows.';

  @override
  String sidebarRunningNowCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Running now, $count active runs',
      one: 'Running now, 1 active run',
    );
    return '$_temp0';
  }

  @override
  String get consoleTerminalPinningNotice =>
      'In the browser this check only notices a key change that the server reports; it does not protect against the server itself.';

  @override
  String get consoleTerminalPasskeyTitle => 'A passkey is needed';

  @override
  String get consoleTerminalPasskeySetup =>
      'Opening a terminal in the console needs a passkey with user verification, and your account has none yet. Set up a passkey for your account first, then open the terminal again. In the desktop and mobile apps the terminal opens with the device key instead.';

  @override
  String get consoleTerminalPasskeyClose => 'Close';

  @override
  String get consoleTerminalOpen => 'Open terminal';

  @override
  String get consoleTerminalBack => 'Back to host';

  @override
  String get componentsDeniedLevel =>
      'You may not answer agents in this channel';

  @override
  String get liveBoxReasonInterrupted => 'Run interrupted';

  @override
  String get liveBoxSendAgain => 'Send again';

  @override
  String agentWorking(String agent) {
    return '$agent is working';
  }

  @override
  String get commandBuiltinStop => 'Stop the active run';

  @override
  String get commandBuiltinStatus => 'Show the run and session details';

  @override
  String get commandBuiltinNew => 'Start a new agent session';

  @override
  String get commandBuiltinResume => 'Continue an earlier session';

  @override
  String get commandBuiltinTitle => 'Set the thread title';

  @override
  String get commandBuiltinClose => 'Mark the thread done';

  @override
  String get commandStatusTitle => 'Run and session';

  @override
  String get commandStatusNoRun => 'No run is active';

  @override
  String get commandResumeTitle => 'Continue a session';

  @override
  String get commandResumeEmpty => 'No earlier sessions';

  @override
  String get sessionAgentBackend => 'Backend';

  @override
  String get sessionAgentModel => 'Model';

  @override
  String get sessionAgentSession => 'Session';

  @override
  String get sessionAgentProject => 'Project';

  @override
  String get sessionAgentContext => 'Context';

  @override
  String get sessionAgentThreadCost => 'Thread cost';

  @override
  String get sessionAgentRuns => 'Runs';

  @override
  String sessionAgentRunsMore(int count) {
    return '$count+';
  }

  @override
  String sessionAgentEarlier(int count) {
    return 'Earlier sessions ($count)';
  }

  @override
  String commandNotAvailable(String command) {
    return '$command is not available here; nothing was sent';
  }

  @override
  String get shortcutListTitle => 'Keyboard shortcuts';

  @override
  String get shortcutListClose => 'Close';

  @override
  String get shortcutListDisabled => 'Not available yet';

  @override
  String get shortcutQuickSwitcher => 'Quick switcher';

  @override
  String get shortcutSearch => 'Search';

  @override
  String get shortcutSearchInThread => 'Search in thread';

  @override
  String get shortcutNewPost => 'New post or new chat in the current forum';

  @override
  String get shortcutFocusInput => 'Focus the input';

  @override
  String get shortcutChannel => 'Next or previous channel';

  @override
  String get shortcutUnreadChannel => 'Next or previous unread channel';

  @override
  String get shortcutColumn =>
      'Switch column (sidebar, list, history, session)';

  @override
  String get shortcutMarkRead => 'Mark thread as read';

  @override
  String get shortcutMarkUnread => 'Mark as unread (from a message)';

  @override
  String get shortcutDone => 'Mark thread as done or reopen it';

  @override
  String get shortcutStopRun => 'Stop the run in the current thread';

  @override
  String get shortcutLiveBox => 'Expand and collapse the live box';

  @override
  String get shortcutSessionColumn => 'Show and hide the session column';

  @override
  String get shortcutEditLast => 'Edit the last own message';

  @override
  String get shortcutSend => 'Send / new line';

  @override
  String get shortcutRunningNow => 'Running now';

  @override
  String get shortcutMentions => 'Mentions';

  @override
  String get shortcutSettings => 'Settings';

  @override
  String get shortcutList => 'List of shortcuts';

  @override
  String get shortcutHostsTerminal => 'Open hosts and terminal';

  @override
  String get shortcutKeyCmd => 'Cmd';

  @override
  String get shortcutKeyCtrl => 'Ctrl';

  @override
  String get shortcutKeyOption => 'Option';

  @override
  String get shortcutKeyAlt => 'Alt';

  @override
  String get shortcutKeyShift => 'Shift';

  @override
  String get shortcutKeyArrow => 'Arrow';

  @override
  String get shortcutKeyUp => 'up';

  @override
  String get shortcutKeyDown => 'down';

  @override
  String get shortcutKeyEscape => 'Esc';

  @override
  String get shortcutKeyTab => 'Tab';

  @override
  String get shortcutKeyEnter => 'Enter';

  @override
  String get shortcutKeyClick => 'Click';

  @override
  String get shortcutKeyThen => 'then';

  @override
  String get shortcutKeyOr => 'or';

  @override
  String get shortcutInEmptyField => 'in the empty field';

  @override
  String get shortcutReversible => '(reversible)';

  @override
  String get menuGo => 'Go';

  @override
  String get menuThread => 'Thread';

  @override
  String get menuWindow => 'Window';

  @override
  String get menuHelp => 'Help';

  @override
  String get threadSearchTitle => 'Search in this thread';

  @override
  String get phoneTabSpaces => 'Spaces';

  @override
  String get phoneTabMentions => 'Mentions';

  @override
  String get phoneTabRunning => 'Running';

  @override
  String get phoneTabMe => 'Me';

  @override
  String get phoneBack => 'Back';

  @override
  String phoneBackTo(String name) {
    return 'Back to $name';
  }

  @override
  String get phoneMoreActions => 'More actions';

  @override
  String phoneOfflineWaiting(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Offline. $count changes waiting',
      one: 'Offline. 1 change waiting',
      zero: 'Offline',
    );
    return '$_temp0';
  }

  @override
  String get phoneSearchFilters => 'Filters';

  @override
  String get componentsOpenForm => 'Fill in';

  @override
  String get pushChannelMentions => 'Mentions';

  @override
  String get pushChannelMentionsDescription =>
      'When someone mentions you or writes to you directly';

  @override
  String get pushChannelQuestions => 'Questions';

  @override
  String get pushChannelQuestionsDescription =>
      'When an agent asks you, a new device of yours waits for approval, or the administration needs you';

  @override
  String get pushChannelMessages => 'All messages';

  @override
  String get pushChannelMessagesDescription =>
      'New messages in channels and threads you follow with All';

  @override
  String get pushChannelRuns => 'Runs';

  @override
  String get pushChannelRunsDescription =>
      'When a run you started finishes or fails';

  @override
  String get adminNavStatistics => 'Statistics';

  @override
  String get statsTitle => 'Statistics';

  @override
  String get statsRangeLabel => 'Time range';

  @override
  String get statsRangeDay => 'Last 24 hours';

  @override
  String get statsRangeWeek => 'Last 7 days';

  @override
  String get statsRangeMonth => 'Last 30 days';

  @override
  String get statsRangeQuarter => 'Last 90 days';

  @override
  String get statsRangeCustom => 'Custom range';

  @override
  String statsRangeCustomValue(String from, String to) {
    return '$from to $to';
  }

  @override
  String get statsScopeLabel => 'Scope';

  @override
  String get statsScopeAll => 'All spaces';

  @override
  String statsScopeRemoved(String name, String reason) {
    return '$name is not offered: $reason';
  }

  @override
  String get statsOpenTelemetry => 'Open telemetry dashboard';

  @override
  String get statsLoading => 'Loading statistics';

  @override
  String get statsEmpty => 'No data in this range';

  @override
  String get statsCurrencyLabel => 'Currency';

  @override
  String get statsCurrencyRequired =>
      'The range has costs in several currencies. The costs show one currency at a time.';

  @override
  String get statsSectionRuns => 'Runs by result';

  @override
  String get statsSectionDuration => 'Run duration per agent';

  @override
  String get statsDurationNote =>
      'The values of the latest period per agent; the table lists every period.';

  @override
  String get statsSectionCostsAgent => 'Costs per agent';

  @override
  String get statsSectionCostsSpace => 'Costs per space';

  @override
  String get statsSectionFailures => 'Failure reasons';

  @override
  String get statsSectionTools => 'Tool calls by tool';

  @override
  String get statsSectionErrors => 'Error rates';

  @override
  String get statsSectionUsage => 'Usage per space';

  @override
  String get statsColumnPeriod => 'Period';

  @override
  String get statsColumnAgent => 'Agent';

  @override
  String get statsColumnSpace => 'Space';

  @override
  String get statsColumnRuns => 'Runs';

  @override
  String get statsColumnMedian => 'Median (p50)';

  @override
  String get statsColumnP95 => 'p95';

  @override
  String get statsColumnMode => 'Mode';

  @override
  String get statsColumnAmount => 'Amount';

  @override
  String get statsColumnReason => 'Reason';

  @override
  String get statsColumnCount => 'Count';

  @override
  String get statsColumnTool => 'Tool';

  @override
  String get statsColumnCalls => 'Calls';

  @override
  String get statsColumnFailed => 'Failed';

  @override
  String get statsColumnMetric => 'Metric';

  @override
  String get statsColumnValue => 'Value';

  @override
  String get statsColumnMessages => 'Messages';

  @override
  String get statsColumnThreadsCreated => 'New threads';

  @override
  String get statsColumnThreadsDone => 'Done threads';

  @override
  String get statsColumnActivePeople => 'Active people';

  @override
  String get statsColumnStorage => 'Storage';

  @override
  String get statsFewerThanMinimum => 'fewer than the minimum group';

  @override
  String get statsErrorRunFailure => 'Failed runs';

  @override
  String get statsErrorInvocationDenials => 'Refused invocations';

  @override
  String get statsErrorWebhookFailure => 'Failed webhook deliveries';

  @override
  String statsErrorPushFailure(String provider) {
    return 'Disabled push registrations ($provider)';
  }

  @override
  String get statsErrorScanErrors => 'Scan errors';

  @override
  String get statsErrorScanInfected => 'Infected files';

  @override
  String statsChartSummary(String title, String rows) {
    return '$title: $rows';
  }
}

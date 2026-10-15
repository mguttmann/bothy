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
}

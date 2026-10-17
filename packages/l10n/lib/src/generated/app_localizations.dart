import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_de.dart';
import 'app_localizations_en.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'generated/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations)!;
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('de'),
    Locale('en'),
  ];

  /// Label of the trigger state `received` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Received'**
  String get triggerStateReceived;

  /// Label of the trigger state `queued` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Queued'**
  String get triggerStateQueued;

  /// Label of the trigger state `working` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Working'**
  String get triggerStateWorking;

  /// Label of the trigger state `done` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Done'**
  String get triggerStateDone;

  /// Label of the trigger state `failed` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Failed'**
  String get triggerStateFailed;

  /// Label of the trigger state `followup_delivered` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Follow-up delivered'**
  String get triggerStateFollowupDelivered;

  /// Tooltip and semantics label of the trigger state `failed` with the translated reason (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Failed: {reason}'**
  String triggerStateFailedWithReason(String reason);

  /// State word of the live box for the run state `queued` (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Queued'**
  String get liveBoxStateQueued;

  /// State word of the live box for the run state `running` (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Running'**
  String get liveBoxStateRunning;

  /// State word of the live box for the run state `waiting_input` when the question waits for the reader (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Waiting for you'**
  String get liveBoxStateWaitingForYou;

  /// State word of the live box for the run state `completed` (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Done'**
  String get liveBoxStateCompleted;

  /// State word of the live box for the run state `failed` (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Failed'**
  String get liveBoxStateFailed;

  /// State word of the live box for the run state `cancelled` (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Canceled'**
  String get liveBoxStateCancelled;

  /// State word of the live box for `waiting_input` when the question waits for another person (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Waiting for {name}'**
  String liveBoxStateWaitingFor(String name);

  /// Translated state reason `billing_mode_mismatch` of a run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Backend does not match'**
  String get liveBoxReasonBillingModeMismatch;

  /// Translated state reason `access_revoked` of a run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Access revoked'**
  String get liveBoxReasonAccessRevoked;

  /// Translated state reason `stale` of a run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Canceled after stall'**
  String get liveBoxReasonStale;

  /// Translated state reason `adapter_error` of a run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Error in the adapter'**
  String get liveBoxReasonAdapterError;

  /// Translated state reason `cancel_timeout` of a run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Adapter did not confirm the cancellation'**
  String get liveBoxReasonCancelTimeout;

  /// Translated state reason `project_not_ready` of a run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Project not ready'**
  String get liveBoxReasonProjectNotReady;

  /// Translated state reason `user_cancelled` of a run with the name of the person who stopped it (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Canceled by {name}'**
  String liveBoxReasonUserCancelled(String name);

  /// Button in the footer of the live box that stops the run (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Stop'**
  String get liveBoxStop;

  /// Link to the steps of the live box above the visible ones (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 earlier step} other{{count} earlier steps}}'**
  String liveBoxEarlierSteps(int count);

  /// Banner of the live box after `run.stall_after` without progress; the caller passes the minutes (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'{minutes, plural, =1{No progress for 1 minute} other{No progress for {minutes} minutes}}'**
  String liveBoxStall(int minutes);

  /// Shown for a link to something invisible, whether it exists or not (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Not found or no access'**
  String get accessNotFoundOrNoAccess;

  /// Shown when an open view closes because the person lost access (01 section 6.6).
  ///
  /// In en, this message translates to:
  /// **'No access anymore'**
  String get accessNoAccessAnymore;

  /// State of a sent message that the server has not confirmed yet (01 section 6.6).
  ///
  /// In en, this message translates to:
  /// **'Sending'**
  String get messageSending;

  /// State of a waiting message whose sending takes longer than usual (01 section 6.6).
  ///
  /// In en, this message translates to:
  /// **'Sending delayed'**
  String get messageSendingDelayed;

  /// Banner at the top of the content while the app has no connection (01 section 6.6).
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get connectionOffline;

  /// A time stamp of today, in the time zone of the person.
  ///
  /// In en, this message translates to:
  /// **'Today'**
  String get relativeDayToday;

  /// A time stamp of yesterday, in the time zone of the person.
  ///
  /// In en, this message translates to:
  /// **'Yesterday'**
  String get relativeDayYesterday;

  /// Text of the problem code `validation_failed` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Some input is not valid'**
  String get problemValidationFailed;

  /// Text of the problem code `malformed_request` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The request could not be read'**
  String get problemMalformedRequest;

  /// Text of the problem code `cursor_invalid` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The list position is no longer valid. Reload the list'**
  String get problemCursorInvalid;

  /// Text of the problem code `webhook_host_not_allowed` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The webhook target is not on the allow list'**
  String get problemWebhookHostNotAllowed;

  /// Text of the problem code `unauthenticated` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Please sign in'**
  String get problemUnauthenticated;

  /// Text of the problem code `token_expired` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Your sign-in has expired'**
  String get problemTokenExpired;

  /// Text of the problem code `session_revoked` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This session was ended'**
  String get problemSessionRevoked;

  /// Text of the problem code `step_up_required` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Please confirm with your device'**
  String get problemStepUpRequired;

  /// Text of the problem code `use_dpop_nonce` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Please try again'**
  String get problemUseDpopNonce;

  /// Text of the problem code `invalid_dpop_proof` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The device proof was not accepted'**
  String get problemInvalidDpopProof;

  /// Text of the problem code `forbidden` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'You are not allowed to do this'**
  String get problemForbidden;

  /// Text of the problem code `scope_missing` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The app lacks a required scope'**
  String get problemScopeMissing;

  /// Text of the problem code `device_pending` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This device is waiting for approval'**
  String get problemDevicePending;

  /// Text of the problem code `channel_read_only` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This channel is read-only'**
  String get problemChannelReadOnly;

  /// Text of the problem code `feature_disabled` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This feature is turned off'**
  String get problemFeatureDisabled;

  /// Text of the problem code `agent_invocation_denied` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The agent cannot be started here'**
  String get problemAgentInvocationDenied;

  /// Text of the problem code `terminal_denied` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The terminal cannot be opened here'**
  String get problemTerminalDenied;

  /// Text of the problem code `invocation_token_invalid` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The call token of the agent is not valid'**
  String get problemInvocationTokenInvalid;

  /// Text of the problem code `not_found` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Not found or no access'**
  String get problemNotFound;

  /// Text of the problem code `method_not_allowed` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This request is not supported'**
  String get problemMethodNotAllowed;

  /// Text of the problem code `conflict` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This conflicts with the current state'**
  String get problemConflict;

  /// Text of the problem code `request_in_progress` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The request is still in progress'**
  String get problemRequestInProgress;

  /// Text of the problem code `i1_violation` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This change would let others use a personal agent'**
  String get problemI1Violation;

  /// Text of the problem code `i6_violation` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Only the holder of the personal LLM login may open a terminal on this host'**
  String get problemI6Violation;

  /// Text of the problem code `last_owner` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'At least one active owner must remain'**
  String get problemLastOwner;

  /// Text of the problem code `already_started` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The run has already started'**
  String get problemAlreadyStarted;

  /// Text of the problem code `billing_mode_mismatch` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The backend does not match the billing mode'**
  String get problemBillingModeMismatch;

  /// Text of the problem code `tool_class_mismatch` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The reported tool class is above the allowed one'**
  String get problemToolClassMismatch;

  /// Text of the problem code `host_unavailable` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The host is not reachable'**
  String get problemHostUnavailable;

  /// Text of the problem code `fingerprint_mismatch` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The fingerprint does not match the host'**
  String get problemFingerprintMismatch;

  /// Text of the problem code `invalid_state_transition` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This change is not possible in the current state'**
  String get problemInvalidStateTransition;

  /// Text of the problem code `base_version_conflict` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The run changed in the meantime'**
  String get problemBaseVersionConflict;

  /// Text of the problem code `component_used` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This element was already used, has expired or is turned off'**
  String get problemComponentUsed;

  /// Text of the problem code `operator_mismatch` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The target channel has a different operator'**
  String get problemOperatorMismatch;

  /// Text of the problem code `file_not_ready` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The file is not ready yet'**
  String get problemFileNotReady;

  /// Text of the problem code `file_blocked` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The file is blocked and is never delivered'**
  String get problemFileBlocked;

  /// Text of the problem code `title_manual` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The title was set by hand'**
  String get problemTitleManual;

  /// Text of the problem code `resync_required` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'A full reload is needed'**
  String get problemResyncRequired;

  /// Text of the problem code `message_deleted` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The message was deleted'**
  String get problemMessageDeleted;

  /// Text of the problem code `gone` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This has expired'**
  String get problemGone;

  /// Text of the problem code `version_conflict` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Someone else changed this in the meantime'**
  String get problemVersionConflict;

  /// Text of the problem code `payload_too_large` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The content is too large'**
  String get problemPayloadTooLarge;

  /// Text of the problem code `unsupported_media_type` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'This file type is not allowed'**
  String get problemUnsupportedMediaType;

  /// Text of the problem code `range_not_satisfiable` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The requested part lies outside the file'**
  String get problemRangeNotSatisfiable;

  /// Text of the problem code `idempotency_key_reuse` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The request was repeated with different content'**
  String get problemIdempotencyKeyReuse;

  /// Text of the problem code `precondition_required` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The version check is missing'**
  String get problemPreconditionRequired;

  /// Text of the problem code `rate_limited` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Too many requests. Please wait a moment'**
  String get problemRateLimited;

  /// Text of the problem code `internal_error` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Something went wrong on the server'**
  String get problemInternalError;

  /// Text of the problem code `unavailable` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The server is not available right now'**
  String get problemUnavailable;

  /// Text of the problem code `maintenance` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'Maintenance: changes are paused, reading continues'**
  String get problemMaintenance;

  /// Text of the problem code `setup_required` (docs/api/openapi.yaml, x-problem-codes).
  ///
  /// In en, this message translates to:
  /// **'The instance is not set up yet'**
  String get problemSetupRequired;

  /// Text of the problem code `forbidden` with the reason `capability_missing` (07 section 4.1, docs/api/openapi.yaml).
  ///
  /// In en, this message translates to:
  /// **'You lack an extra permission for this'**
  String get problemForbiddenCapabilityMissing;

  /// Text of the problem code `forbidden` with the reason `level_insufficient` (07 section 4.1, docs/api/openapi.yaml).
  ///
  /// In en, this message translates to:
  /// **'Your access level is too low for this'**
  String get problemForbiddenLevelInsufficient;

  /// Text of the problem code `forbidden` with the reason `owner_only` (07 section 4.1, docs/api/openapi.yaml).
  ///
  /// In en, this message translates to:
  /// **'Only an owner can do this'**
  String get problemForbiddenOwnerOnly;

  /// Text of the problem code `forbidden` with the reason `view_as_read_only` (07 section 4.1, docs/api/openapi.yaml).
  ///
  /// In en, this message translates to:
  /// **'Read only while you view as another person'**
  String get problemForbiddenViewAsReadOnly;

  /// Text of the problem code `channel_read_only` with the reason `channel_archived` (07 section 4.1, docs/api/openapi.yaml).
  ///
  /// In en, this message translates to:
  /// **'This channel is archived'**
  String get problemChannelReadOnlyChannelArchived;

  /// Text of the problem code `agent_invocation_denied` with the reason `project_not_ready` (07 section 4.1, docs/api/openapi.yaml).
  ///
  /// In en, this message translates to:
  /// **'The project of the agent is not ready'**
  String get problemAgentInvocationDeniedProjectNotReady;

  /// Text of a problem code the app does not know; shows the code so support can find it (01 section 9).
  ///
  /// In en, this message translates to:
  /// **'Something went wrong ({code})'**
  String problemFallback(String code);

  /// Label of the metric `messages_created` in charts and tiles (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Messages'**
  String get vizMetricMessagesCreated;

  /// Label of the metric `threads_created` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'New threads'**
  String get vizMetricThreadsCreated;

  /// Label of the metric `runs_started` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Runs started'**
  String get vizMetricRunsStarted;

  /// Label of the metric `runs_finished` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Runs finished'**
  String get vizMetricRunsFinished;

  /// Label of the metric `tool_calls` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Tool calls'**
  String get vizMetricToolCalls;

  /// Label of the metric `approval_cards` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Approval cards'**
  String get vizMetricApprovalCards;

  /// Label of the metric `cost` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Costs'**
  String get vizMetricCost;

  /// Label of the metric `invocations_denied` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Denied invocations'**
  String get vizMetricInvocationsDenied;

  /// Label of the metric `billing_mode_switches` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Billing mode switches'**
  String get vizMetricBillingModeSwitches;

  /// Label of the metric `catchups` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Catch-ups'**
  String get vizMetricCatchups;

  /// Label of the metric `resyncs` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Resyncs'**
  String get vizMetricResyncs;

  /// Label of the metric `slow_consumer_closes` (25 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Closes of slow clients'**
  String get vizMetricSlowConsumerCloses;

  /// Label of a metric the app does not know yet; shows its code.
  ///
  /// In en, this message translates to:
  /// **'Metric {code}'**
  String vizMetricUnknown(String code);

  /// Display name of the SLO of `sli.api_availability` (slos.md section 2).
  ///
  /// In en, this message translates to:
  /// **'API availability'**
  String get vizSliApiAvailability;

  /// Display name of the SLO of `sli.delivery_latency` (slos.md section 2).
  ///
  /// In en, this message translates to:
  /// **'Delivery latency'**
  String get vizSliDeliveryLatency;

  /// Display name of the SLO of `sli.reconnect` (slos.md section 2).
  ///
  /// In en, this message translates to:
  /// **'Reconnect'**
  String get vizSliReconnect;

  /// Display name of the SLO of `sli.push_acceptance` (slos.md section 2).
  ///
  /// In en, this message translates to:
  /// **'Push acceptance'**
  String get vizSliPushAcceptance;

  /// Display name of the SLO of `sli.run_start` (slos.md section 2).
  ///
  /// In en, this message translates to:
  /// **'Run start'**
  String get vizSliRunStart;

  /// Display name of the SLO of `sli.turn_duration` (slos.md section 2).
  ///
  /// In en, this message translates to:
  /// **'Turn duration'**
  String get vizSliTurnDuration;

  /// Text of the SLO traffic light in state `green` (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'on target'**
  String get vizSloOnTarget;

  /// Text of the SLO traffic light in state `amber` (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'budget tight'**
  String get vizSloBudgetTight;

  /// Text of the SLO traffic light in state `red` (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'target missed'**
  String get vizSloTargetMissed;

  /// Text of the SLO traffic light in state `insufficient_data` (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'not enough data'**
  String get vizSloNotEnoughData;

  /// Label of the SLO traffic light: the values come from the own measurement (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'Own measurement'**
  String get vizOwnMeasurement;

  /// Screen reader label of one SLO traffic light (25 section 7.4).
  ///
  /// In en, this message translates to:
  /// **'{slo}: {state}, own measurement'**
  String vizTrafficLightLabel(String slo, String state);

  /// Polite announcement when the state of an SLO traffic light changes; never on an unchanged frame (25 section 7.4).
  ///
  /// In en, this message translates to:
  /// **'{slo} is now {state}'**
  String vizTrafficLightChanged(String slo, String state);

  /// Note on a cost in mode `organization` while `cost_is_estimate` is true (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Estimate'**
  String get vizCostEstimate;

  /// Note on every cost in mode `personal` (25 section 4 rule 2).
  ///
  /// In en, this message translates to:
  /// **'information only'**
  String get vizCostInformationOnly;

  /// Backend policy badge for mode `personal` (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Personal'**
  String get vizBadgePersonal;

  /// Backend policy badge for mode `organization` (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Organization'**
  String get vizBadgeOrganization;

  /// Note on a run card while an approval card of the run waits for a decision (25 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Approval waiting'**
  String get vizApprovalWaiting;

  /// Marker when the live frame or the timeline has `runs_truncated` (25 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'More runs than shown'**
  String get vizMoreRunsThanShown;

  /// State word of a run card for `waiting_input` (components.md section 2).
  ///
  /// In en, this message translates to:
  /// **'Waiting for input'**
  String get vizRunStateWaitingInput;

  /// State word of a run card for a state the app does not know yet.
  ///
  /// In en, this message translates to:
  /// **'Unknown state'**
  String get vizRunStateUnknown;

  /// Tool chip of a run card: current step of kind `tool_call` with the tool name (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Tool {name}'**
  String vizStepToolCall(String name);

  /// Tool chip of a run card: current step of kind `status`.
  ///
  /// In en, this message translates to:
  /// **'Status {name}'**
  String vizStepStatus(String name);

  /// Tool chip of a run card: current step of kind `subagent`.
  ///
  /// In en, this message translates to:
  /// **'Subagent {name}'**
  String vizStepSubagent(String name);

  /// Tool chip of a run card: current step of kind `workflow`.
  ///
  /// In en, this message translates to:
  /// **'Workflow {name}'**
  String vizStepWorkflow(String name);

  /// Tool chip of a run card: current step of kind `background_job`.
  ///
  /// In en, this message translates to:
  /// **'Background job {name}'**
  String vizStepBackgroundJob(String name);

  /// Screen reader label of a lane of the live topology (25 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'{space}, {channel}, {count, plural, =0{no active runs} =1{1 active run} other{{count} active runs}}'**
  String vizLaneLabel(String space, String channel, int count);

  /// Screen reader label of one row of the bar mark of the timeline (25 section 6.5).
  ///
  /// In en, this message translates to:
  /// **'{row}, {count, plural, =1{1 run} other{{count} runs}}'**
  String vizBarRowLabel(String row, int count);

  /// Title of the area chart of rates (25 section 6.5).
  ///
  /// In en, this message translates to:
  /// **'Rates'**
  String get vizChartRates;

  /// Title of the bar chart of runs (25 section 6.5).
  ///
  /// In en, this message translates to:
  /// **'Runs'**
  String get vizChartRuns;

  /// Comparison of a metric tile with the previous period when the value rose (components.md section 11).
  ///
  /// In en, this message translates to:
  /// **'{delta} more than the previous period'**
  String vizTileUp(String delta);

  /// Comparison of a metric tile with the previous period when the value fell (components.md section 11).
  ///
  /// In en, this message translates to:
  /// **'{delta} less than the previous period'**
  String vizTileDown(String delta);

  /// Comparison of a metric tile with the previous period when the value did not change (components.md section 11).
  ///
  /// In en, this message translates to:
  /// **'same as the previous period'**
  String get vizTileSame;

  /// Screen reader text for a value the server did not deliver (no permission); never read as zero (25 section 4 rule 5).
  ///
  /// In en, this message translates to:
  /// **'not shown'**
  String get vizNotShown;

  /// Marker of a live view when no frame arrived for viz.stale_after (25 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'No new data'**
  String get vizStale;

  /// Title of the harness app of bothy_viz (25 section 7.1).
  ///
  /// In en, this message translates to:
  /// **'Visualization harness'**
  String get vizHarnessTitle;

  /// Toolbar label of the harness: light or dark theme.
  ///
  /// In en, this message translates to:
  /// **'Theme'**
  String get vizHarnessTheme;

  /// Harness choice: light theme.
  ///
  /// In en, this message translates to:
  /// **'Light'**
  String get vizHarnessThemeLight;

  /// Harness choice: dark theme.
  ///
  /// In en, this message translates to:
  /// **'Dark'**
  String get vizHarnessThemeDark;

  /// Toolbar label of the harness: density context.
  ///
  /// In en, this message translates to:
  /// **'Density'**
  String get vizHarnessDensity;

  /// Harness choice: density `comfortable`.
  ///
  /// In en, this message translates to:
  /// **'Comfortable'**
  String get vizHarnessDensityComfortable;

  /// Harness choice: density `compact`.
  ///
  /// In en, this message translates to:
  /// **'Compact'**
  String get vizHarnessDensityCompact;

  /// Harness choice: density `touch`.
  ///
  /// In en, this message translates to:
  /// **'Touch'**
  String get vizHarnessDensityTouch;

  /// Toolbar label of the harness: text scaling.
  ///
  /// In en, this message translates to:
  /// **'Text size'**
  String get vizHarnessTextScale;

  /// Toolbar label of the harness: language of the texts.
  ///
  /// In en, this message translates to:
  /// **'Language'**
  String get vizHarnessLanguage;

  /// Toolbar label of the harness: state of the fixtures (normal, empty, ...).
  ///
  /// In en, this message translates to:
  /// **'Fixture state'**
  String get vizHarnessFixtureState;

  /// Label of the slider that advances the fake clock of the fixture sources.
  ///
  /// In en, this message translates to:
  /// **'Time-lapse'**
  String get vizHarnessTimeLapse;

  /// Name of the lane mark in the harness list.
  ///
  /// In en, this message translates to:
  /// **'Lane'**
  String get vizMarkLane;

  /// Name of the run card mark in the harness list.
  ///
  /// In en, this message translates to:
  /// **'Run card'**
  String get vizMarkRunCard;

  /// Name of the area mark in the harness list.
  ///
  /// In en, this message translates to:
  /// **'Area'**
  String get vizMarkArea;

  /// Name of the bar mark in the harness list.
  ///
  /// In en, this message translates to:
  /// **'Bar'**
  String get vizMarkBar;

  /// Name of the traffic light mark in the harness list.
  ///
  /// In en, this message translates to:
  /// **'Traffic light'**
  String get vizMarkTrafficLight;

  /// Name of the metric tile mark in the harness list.
  ///
  /// In en, this message translates to:
  /// **'Metric tile'**
  String get vizMarkMetricTile;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['de', 'en'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'de':
      return AppLocalizationsDe();
    case 'en':
      return AppLocalizationsEn();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}

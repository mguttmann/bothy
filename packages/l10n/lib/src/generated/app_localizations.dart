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

  /// Text button under a collapsed long message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Show more'**
  String get messageShowMore;

  /// Suffix after a message whose `edited_at` is set (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'(edited)'**
  String get messageEdited;

  /// Line that replaces a message whose `deleted_at` is set (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Message deleted'**
  String get messageDeleted;

  /// Button of a message that failed permanently; sends it again with a new ID (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Resend'**
  String get messageResend;

  /// Button of a failed message and action of the hover bar (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Edit'**
  String get messageEdit;

  /// Button of a failed message that removes it from the queue (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Discard'**
  String get messageDiscard;

  /// Action of the hover bar of a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Reply'**
  String get messageActionReply;

  /// Action of the hover bar of a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'React'**
  String get messageActionReact;

  /// Action of the hover bar of a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Pin'**
  String get messageActionPin;

  /// Action of the hover bar that opens further actions (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'More'**
  String get messageActionMore;

  /// Start of the screen reader label of a message that mentions the reader (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Mentions you'**
  String get messageMentionsYou;

  /// Screen reader label of a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'{name}, {time}, {text}'**
  String messageSemanticsLabel(String name, String time, String text);

  /// Badge after the name of a message written by an agent (`origin = agent`, components.md section 1, SEC-OUT-3).
  ///
  /// In en, this message translates to:
  /// **'AGENT'**
  String get messageOriginAgent;

  /// Badge after the name of a message written by an app (`origin = app`, components.md section 1, SEC-OUT-3).
  ///
  /// In en, this message translates to:
  /// **'APP'**
  String get messageOriginApp;

  /// Badge after the name of an imported message (`origin = import`, components.md section 1, SEC-OUT-3).
  ///
  /// In en, this message translates to:
  /// **'IMPORT'**
  String get messageOriginImport;

  /// Badge of a message the server wrote (`origin = system`, components.md section 1, SEC-OUT-3).
  ///
  /// In en, this message translates to:
  /// **'SYSTEM'**
  String get messageOriginSystem;

  /// Screen reader label of the quote line above a reply (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Reply to {name}'**
  String messageReplyTo(String name);

  /// Screen reader label of an attachment of a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Attachment {name}'**
  String messageAttachment(String name);

  /// Screen reader label of a reaction chip under a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 reaction {emoji}} other{{count} reactions {emoji}}}'**
  String messageReaction(int count, String emoji);

  /// Play button of a voice message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Play voice message'**
  String get messageVoicePlay;

  /// Pause button of a playing voice message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Pause voice message'**
  String get messageVoicePause;

  /// Line under a voice message while its transcript is not done (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Transcript pending'**
  String get messageVoiceTranscriptPending;

  /// System line `thread.reopened` (01 section 6.1, components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Reopened by message from {name}'**
  String messageSystemThreadReopened(String name);

  /// System line `agent_session.resume_failed` (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'The agent session could not be resumed'**
  String get messageSystemResumeFailed;

  /// System line of a code the app does not know yet (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'System message ({code})'**
  String messageSystemUnknown(String code);

  /// Label of the trigger state `cancelled` on the triggering message (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Canceled'**
  String get triggerStateCancelled;

  /// Tooltip and screen reader label of the trigger state `cancelled` with the translated reason (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Canceled: {reason}'**
  String triggerStateCancelledWithReason(String reason);

  /// Label of a trigger state the app does not know yet (components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'State {code}'**
  String triggerStateUnknown(String code);

  /// State word of the live box for a run state the app does not know yet (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'State {code}'**
  String liveBoxStateUnknown(String code);

  /// Screen reader label of the stop button of the live box (components.md section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Stop run of {agent}'**
  String liveBoxStopRun(String agent);

  /// Screen reader label of the live box container (components.md section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Run of {agent}: {state}, for {duration}, context {percent} percent'**
  String liveBoxSemanticsLabel(
    String agent,
    String state,
    String duration,
    int percent,
  );

  /// Note in the footer of a running live box (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Messages in this thread reach the agent as a follow-up'**
  String get liveBoxFollowupHint;

  /// Token count in the header of the live box; the caller formats the number (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'{count} tokens'**
  String liveBoxTokens(String count);

  /// Context use in the header of the live box; the caller formats the percent (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Context {percent}'**
  String liveBoxContext(String percent);

  /// Start of the collapsed line of a finished run (components.md section 2.2).
  ///
  /// In en, this message translates to:
  /// **'Ran {duration}'**
  String liveBoxRan(String duration);

  /// Number of tools in the collapsed line of a run (components.md section 2.2).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 tool} other{{count} tools}}'**
  String liveBoxTools(int count);

  /// Number of subagents in the collapsed line of a run (components.md section 2.2).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 subagent} other{{count} subagents}}'**
  String liveBoxSubagents(int count);

  /// Screen reader label of the expand arrow of the live box (components.md section 2.1 and 2.2).
  ///
  /// In en, this message translates to:
  /// **'Show run details'**
  String get liveBoxExpand;

  /// Screen reader label of the collapse arrow of an expanded live box (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Hide run details'**
  String get liveBoxCollapse;

  /// Screen reader label of the mobile line of the live box; tapping opens the sheet (components.md section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Open live box'**
  String get liveBoxOpen;

  /// Screen reader announcement of a new question of a run (components.md section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Question from {agent}'**
  String liveBoxAnnounceQuestion(String agent);

  /// Screen reader announcement of a state change of a run (components.md section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Run of {agent}: {state}'**
  String liveBoxAnnounceState(String agent, String state);

  /// Suffix of a pending step of the live box (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'waiting'**
  String get toolChipWaiting;

  /// Suffix of a running step with its run time (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'running {duration}'**
  String toolChipRunning(String duration);

  /// Suffix of a failed step (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'failed'**
  String get toolChipFailed;

  /// Suffix of a canceled step (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'canceled'**
  String get toolChipCancelled;

  /// Status word of a completed step in the screen reader label (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'completed'**
  String get toolChipCompleted;

  /// Suffix of a step status the app does not know yet (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'status {code}'**
  String toolChipUnknownStatus(String code);

  /// Kind `tool_call` of a step in the screen reader label (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Tool'**
  String get toolChipKindToolCall;

  /// Kind `status` of a step in the screen reader label (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Status'**
  String get toolChipKindStatus;

  /// Kind `subagent` of a step in the screen reader label (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Subagent'**
  String get toolChipKindSubagent;

  /// Kind `workflow` of a step in the screen reader label (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Workflow'**
  String get toolChipKindWorkflow;

  /// Kind `background_job` of a step in the screen reader label (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Background job'**
  String get toolChipKindBackgroundJob;

  /// Kind of a step the app does not know yet (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Step'**
  String get toolChipKindUnknown;

  /// Screen reader label of a tool chip; the status includes the duration where there is one (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'{kind} {name}, {status}'**
  String toolChipLabel(String kind, String name, String status);

  /// Hint of a tool chip that opens the step detail (components.md section 3).
  ///
  /// In en, this message translates to:
  /// **'Show details'**
  String get toolChipDetails;

  /// Badge on the second line of a forum post that waits for the reader (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Waiting for you'**
  String get threadRowWaitingForYou;

  /// State word of an open post in the screen reader label (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get threadRowStateOpen;

  /// State word of a done post in the screen reader label (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Done'**
  String get threadRowStateDone;

  /// Unread count in the screen reader label of a forum post or channel (components.md sections 6 and 7).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{no unread} =1{1 unread} other{{count} unread}}'**
  String threadRowUnread(int count);

  /// Mention count of a forum post or channel (components.md sections 6 and 7).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 mention} other{{count} mentions}}'**
  String threadRowMentions(int count);

  /// Screen reader label of a forum post row (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'{title}, {state}, {time}, {unread}'**
  String threadRowLabel(String title, String state, String time, String unread);

  /// Operator agent on the second line of a forum post (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Operator {name}'**
  String threadRowOperator(String name);

  /// Part of the screen reader label of a channel with an active run (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'run active'**
  String get channelRowRunActive;

  /// Part of the screen reader label of a muted channel (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'muted'**
  String get channelRowMuted;

  /// Screen reader label of the dot of a collapsed space with unread items (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Unread'**
  String get spaceRowUnread;

  /// Screen reader hint of a collapsed space row (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Expand {name}'**
  String spaceRowExpand(String name);

  /// Screen reader hint of an expanded space row (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Collapse {name}'**
  String spaceRowCollapse(String name);

  /// Label of the divider above the agent's own text on a card (components.md section 8, SEC-OUT-4).
  ///
  /// In en, this message translates to:
  /// **'Text from {agent}'**
  String componentsTextFrom(String agent);

  /// First line of the card header, built from server fields (components.md section 8, SEC-OUT-4).
  ///
  /// In en, this message translates to:
  /// **'Request from {agent}'**
  String componentsHeaderRequest(String agent);

  /// Line of the card header naming the run, from server fields (components.md section 8, SEC-OUT-4).
  ///
  /// In en, this message translates to:
  /// **'Run {run}'**
  String componentsHeaderRun(String run);

  /// Line of the card header naming the required capability (components.md section 8, SEC-OUT-4).
  ///
  /// In en, this message translates to:
  /// **'Requires the permission \'{capability}\''**
  String componentsHeaderCapability(String capability);

  /// Who may approve: the person who triggered the run, or an owner (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Only {name} or an owner can approve this'**
  String componentsApproverRunStarter(String name);

  /// Who may approve: an owner only (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Only an owner can approve this'**
  String get componentsApproverOwner;

  /// Who may approve: every person who may use the component (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Anyone with access to this channel can answer'**
  String get componentsApproverAny;

  /// Lock note of an active card the reader may not use for lack of a capability (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'You do not have the permission \'{capability}\' for this'**
  String componentsDeniedCapability(String capability);

  /// Name of the capability `approve_actions` (01 section 9 term table).
  ///
  /// In en, this message translates to:
  /// **'Approve actions'**
  String get capabilityApproveActions;

  /// Line of a used card (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'{name} chose \'{choice}\', {time}'**
  String componentsUsed(String name, String choice, String time);

  /// Line of an expired card (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Expired'**
  String get componentsExpired;

  /// Line of a disabled card (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'No longer available'**
  String get componentsDisabled;

  /// Screen reader label of the pressed button while the answer is pending (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Sending your choice'**
  String get componentsSending;

  /// Submit button of a form or a multiple selection without its own label (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Submit'**
  String get componentsSubmit;

  /// Placeholder of a select field without its own placeholder (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Choose'**
  String get componentsSelectPlaceholder;

  /// Validation message of an empty required form field (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Required'**
  String get componentsFieldRequired;

  /// Validation message of a too long form field; the limit comes from the definition (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'At most {max} characters'**
  String componentsFieldTooLong(int max);

  /// Validation message of a number field with other text (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'Enter a number'**
  String get componentsFieldNumber;

  /// Line for a component type the app does not know yet (components.md section 8).
  ///
  /// In en, this message translates to:
  /// **'This element needs a newer app'**
  String get componentsUnsupported;

  /// Placeholder and screen reader label of the input (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Message to {target}'**
  String composerMessageTo(String target);

  /// Placeholder of the input during an active run (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Follow-up to {agent}'**
  String composerFollowupTo(String agent);

  /// Typing line above the input (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'{name} is typing'**
  String composerTyping(String name);

  /// Typing line above the input when more than one person types (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Several people are typing'**
  String get composerTypingSeveral;

  /// Send button of the input (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Send'**
  String get composerSend;

  /// Attachment button of the input (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Attach file'**
  String get composerAttach;

  /// Microphone button of the input (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Record voice message'**
  String get composerVoice;

  /// Reason why sending is disabled while a required command option is empty (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Missing option: {option}'**
  String composerOptionMissing(String option);

  /// Hint in the input while offline (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Offline: your message waits in the queue'**
  String get composerOffline;

  /// Note that replaces the input without write permission (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Read only in this channel'**
  String get composerReadOnly;

  /// Announced number of suggestions of the suggestion list (components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{No suggestions} =1{1 suggestion} other{{count} suggestions}}'**
  String composerSuggestions(int count);

  /// Hint on a suggestion row of a control command (`control = true`, components.md section 9).
  ///
  /// In en, this message translates to:
  /// **'Control command'**
  String get composerControlCommand;

  /// Backend policy badge `personal` (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Personal'**
  String get billingPersonal;

  /// Backend policy badge `organization` (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Organization'**
  String get billingOrganization;

  /// Backend policy badge of a mode the app does not know yet (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Backend {code}'**
  String billingUnknown(String code);

  /// Warning badge when the reported mode differs from the configured one (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Reported: {mode}'**
  String billingReported(String mode);

  /// Screen reader label of the badge `personal` (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Backend: Personal, runs on the subscription of {name}'**
  String billingLabelPersonal(String name);

  /// Screen reader label of the badge `organization` (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Backend: Organization'**
  String get billingLabelOrganization;

  /// Suffix of a cost in Organization mode until reconciled (components.md section 10, 01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Estimate'**
  String get costEstimate;

  /// Suffix of a cost in Personal mode (components.md section 10, 01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'for information only'**
  String get costInformationOnly;

  /// Text that replaces an image of a message; images are never loaded from a URL (components.md section 1, SEC-OUT-2).
  ///
  /// In en, this message translates to:
  /// **'Image: {alt} ({domain})'**
  String markdownImage(String alt, String domain);

  /// Text that replaces an image without a domain (components.md section 1, SEC-OUT-2).
  ///
  /// In en, this message translates to:
  /// **'Image: {alt}'**
  String markdownImageNoDomain(String alt);

  /// Copy button of a code block in a message (components.md section 1).
  ///
  /// In en, this message translates to:
  /// **'Copy code'**
  String get markdownCopyCode;

  /// Notice for the rejection reason `multiple_agents_mentioned` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Please mention only one agent.'**
  String get triggerReasonMultipleAgentsMentioned;

  /// Notice for the rejection reason `agent_not_allowed_here` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'This agent does not answer here.'**
  String get triggerReasonAgentNotAllowedHere;

  /// Notice for the rejection reason `no_trigger_right` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'You may not trigger agents here.'**
  String get triggerReasonNoTriggerRight;

  /// Notice for the rejection reason `run_active_other_agent` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Another agent is currently working in this thread.'**
  String get triggerReasonRunActiveOtherAgent;

  /// Notice for the rejection reason `agent_disabled` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Agent is switched off.'**
  String get triggerReasonAgentDisabled;

  /// Notice for the rejection reason `billing_mode_personal_not_holder` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'This agent uses a personal subscription.'**
  String get triggerReasonBillingModePersonalNotHolder;

  /// Notice for the rejection reason `voice_unsupported` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'Voice messages are not understood here.'**
  String get triggerReasonVoiceUnsupported;

  /// Notice for the rejection reason `run_active` at acceptance of a triggering message (docs/spec/09 section 3.5; components.md section 5).
  ///
  /// In en, this message translates to:
  /// **'The agent is currently working; send the command again after the run.'**
  String get triggerReasonRunActive;

  /// Translated state reason `user_cancelled` of a run when the name of the person is not known (components.md section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Canceled by a person'**
  String get liveBoxReasonUserCancelledNoName;

  /// State word of a forum post whose run waits for an answer from someone other than the reader (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Waiting for an answer'**
  String get threadRowStateWaiting;

  /// Title and tab of the live topology view (25 section 6.1, screen D12).
  ///
  /// In en, this message translates to:
  /// **'Live topology'**
  String get vizViewLiveTopology;

  /// Title and tab of the backend view (25 section 6.2).
  ///
  /// In en, this message translates to:
  /// **'Backend view'**
  String get vizViewBackend;

  /// Title and tab of the delivery and sync view (25 section 6.3).
  ///
  /// In en, this message translates to:
  /// **'Delivery and sync live'**
  String get vizViewDelivery;

  /// Title and tab of the executive page (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'Executive page'**
  String get vizViewExecutive;

  /// Title and tab of the replayable daily timeline (25 section 6.5).
  ///
  /// In en, this message translates to:
  /// **'Daily timeline'**
  String get vizViewTimeline;

  /// Name of TV mode in the view registry (25 section 6.6, screen D13).
  ///
  /// In en, this message translates to:
  /// **'TV mode'**
  String get vizViewTv;

  /// Label of the switch in every view header that shows the same values as a table (25 section 7.4).
  ///
  /// In en, this message translates to:
  /// **'As table'**
  String get vizAsTable;

  /// Link of a view that opens the dashboard in the system browser; only when the instance sets a dashboard URL (25 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Open in Grafana'**
  String get vizOpenGrafana;

  /// Screen reader label of a view while its data loads.
  ///
  /// In en, this message translates to:
  /// **'Loading'**
  String get vizLoading;

  /// Note of a view or tile that only the owner of the instance sees, shown instead of its data in a space scope (25 sections 6.3 and 6.4).
  ///
  /// In en, this message translates to:
  /// **'Only for the owner'**
  String get vizOnlyForOwner;

  /// Screen reader label of a column header of a table view; a tap sorts by the column.
  ///
  /// In en, this message translates to:
  /// **'Sort by {column}'**
  String vizSortBy(String column);

  /// Time of the shown live frame under the view title.
  ///
  /// In en, this message translates to:
  /// **'As of {time}'**
  String vizFrameTime(String time);

  /// Live topology without an active run.
  ///
  /// In en, this message translates to:
  /// **'No active runs'**
  String get vizNoActiveRuns;

  /// Heading of the rates of the last full minute in the live topology (screen D12).
  ///
  /// In en, this message translates to:
  /// **'Last minute'**
  String get vizRatesLastMinute;

  /// Table cell of a value that does not exist for this row, for example no current step.
  ///
  /// In en, this message translates to:
  /// **'none'**
  String get vizNone;

  /// Column header: the space of a run.
  ///
  /// In en, this message translates to:
  /// **'Space'**
  String get vizColSpace;

  /// Column header: the channel of a run.
  ///
  /// In en, this message translates to:
  /// **'Channel'**
  String get vizColChannel;

  /// Column header: the agent.
  ///
  /// In en, this message translates to:
  /// **'Agent'**
  String get vizColAgent;

  /// Column header: the state of a run or a traffic light.
  ///
  /// In en, this message translates to:
  /// **'State'**
  String get vizColState;

  /// Column header: the current step of a run (tool chip).
  ///
  /// In en, this message translates to:
  /// **'Current step'**
  String get vizColStep;

  /// Column header: the billing mode badge of a run.
  ///
  /// In en, this message translates to:
  /// **'Billing mode'**
  String get vizColMode;

  /// Column header: whether an approval card of the run waits.
  ///
  /// In en, this message translates to:
  /// **'Approval'**
  String get vizColApproval;

  /// Column header: the cost of a run so far.
  ///
  /// In en, this message translates to:
  /// **'Cost so far'**
  String get vizColCost;

  /// Column header: the metric of a row.
  ///
  /// In en, this message translates to:
  /// **'Metric'**
  String get vizColMetric;

  /// Column header: the value of the last full minute.
  ///
  /// In en, this message translates to:
  /// **'Last minute'**
  String get vizColLastMinute;

  /// Column header: the value of a row.
  ///
  /// In en, this message translates to:
  /// **'Value'**
  String get vizColValue;

  /// Column header: comparison or cost label of a tile, or the label own measurement.
  ///
  /// In en, this message translates to:
  /// **'Note'**
  String get vizColNote;

  /// Column header: presence of an agent from its heartbeats.
  ///
  /// In en, this message translates to:
  /// **'Presence'**
  String get vizColPresence;

  /// Column header and label: billing mode set for the agent (25 section 6.2).
  ///
  /// In en, this message translates to:
  /// **'Configured mode'**
  String get vizColConfiguredMode;

  /// Column header and label: billing mode the adapter reports (25 section 6.2).
  ///
  /// In en, this message translates to:
  /// **'Reported mode'**
  String get vizColReportedMode;

  /// Column header and label: billing modes the agent supports.
  ///
  /// In en, this message translates to:
  /// **'Supported modes'**
  String get vizColSupportedModes;

  /// Column header and label: billing mode switches in the period.
  ///
  /// In en, this message translates to:
  /// **'Mode switches'**
  String get vizColSwitches;

  /// Column header and label: invocations denied before delivery by invariant I2.
  ///
  /// In en, this message translates to:
  /// **'Denied before delivery (I2)'**
  String get vizColI2Denied;

  /// Column header and label: runs failed with billing_mode_mismatch by invariant I3.
  ///
  /// In en, this message translates to:
  /// **'Failed on mode mismatch (I3)'**
  String get vizColI3Denied;

  /// Column header and label: costs of an agent per billing mode.
  ///
  /// In en, this message translates to:
  /// **'Costs per mode'**
  String get vizColCosts;

  /// Column header and label: rate-limit state the adapter reports.
  ///
  /// In en, this message translates to:
  /// **'Rate limit'**
  String get vizColRateLimit;

  /// Presence of an agent: online.
  ///
  /// In en, this message translates to:
  /// **'Online'**
  String get vizPresenceOnline;

  /// Presence of an agent: degraded.
  ///
  /// In en, this message translates to:
  /// **'Degraded'**
  String get vizPresenceDegraded;

  /// Presence of an agent: offline.
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get vizPresenceOffline;

  /// Presence of an agent without a live frame or with an unknown state.
  ///
  /// In en, this message translates to:
  /// **'Unknown'**
  String get vizPresenceUnknown;

  /// Reported mode of an adapter that follows the configured mode per run and reports none.
  ///
  /// In en, this message translates to:
  /// **'per run'**
  String get vizFollowsRun;

  /// Warning badge when the reported billing mode differs from the configured one (components.md section 10).
  ///
  /// In en, this message translates to:
  /// **'Reported: {mode}'**
  String vizReportedMismatch(String mode);

  /// Rate-limit state ok.
  ///
  /// In en, this message translates to:
  /// **'within the limit'**
  String get vizRateLimitOk;

  /// Rate-limit state near_limit.
  ///
  /// In en, this message translates to:
  /// **'near the limit'**
  String get vizRateLimitNearLimit;

  /// Rate-limit state limited.
  ///
  /// In en, this message translates to:
  /// **'limited'**
  String get vizRateLimitLimited;

  /// Rate-limit state unknown or not reported.
  ///
  /// In en, this message translates to:
  /// **'unknown'**
  String get vizRateLimitUnknown;

  /// Time at which the rate limit resets.
  ///
  /// In en, this message translates to:
  /// **'resets at {time}'**
  String vizRateLimitResets(String time);

  /// Value of an SLI without events in the window.
  ///
  /// In en, this message translates to:
  /// **'no events'**
  String get vizSliNoEvents;

  /// Value of an SLI of the live frame: number of events and the good share.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 event} other{{count} events}}, {share} within the threshold'**
  String vizSliValue(int count, String share);

  /// Catch-ups of the last minute with their summed duration in milliseconds.
  ///
  /// In en, this message translates to:
  /// **'{count}, together {millis} ms'**
  String vizCatchupValue(String count, String millis);

  /// Open gateway connections of kind user on this node.
  ///
  /// In en, this message translates to:
  /// **'Connections of people'**
  String get vizConnectionsUser;

  /// Open gateway connections of kind agent on this node.
  ///
  /// In en, this message translates to:
  /// **'Connections of agents'**
  String get vizConnectionsAgent;

  /// Open gateway connections of kind app on this node.
  ///
  /// In en, this message translates to:
  /// **'Connections of apps'**
  String get vizConnectionsApp;

  /// Heading of the history chart of the delivery view.
  ///
  /// In en, this message translates to:
  /// **'History'**
  String get vizChartHistory;

  /// Tile group of the executive page: messages, new threads, active people.
  ///
  /// In en, this message translates to:
  /// **'Usage'**
  String get vizTileUsage;

  /// Tile group of the executive page: runs and error rates.
  ///
  /// In en, this message translates to:
  /// **'Runs and error rate'**
  String get vizTileRuns;

  /// Tile group of the executive page: costs per billing mode.
  ///
  /// In en, this message translates to:
  /// **'Costs per mode'**
  String get vizTileCosts;

  /// Tile group of the executive page: the SLO traffic lights.
  ///
  /// In en, this message translates to:
  /// **'SLO traffic light'**
  String get vizTileSlo;

  /// Column header: the SLO of a traffic light.
  ///
  /// In en, this message translates to:
  /// **'SLO'**
  String get vizColSlo;

  /// Note under the SLO traffic lights (25 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'Values can differ from Prometheus: the own measurement counts in the process and can lose one flush interval in a crash.'**
  String get vizOwnMeasurementNote;

  /// Column header: row of a run bar of the timeline.
  ///
  /// In en, this message translates to:
  /// **'Agent and channel'**
  String get vizColRun;

  /// Column header: start of a run.
  ///
  /// In en, this message translates to:
  /// **'Start'**
  String get vizColStart;

  /// Column header: end of a run, or the playhead while it runs.
  ///
  /// In en, this message translates to:
  /// **'End'**
  String get vizColEnd;

  /// Position of the timeline playback under the view title.
  ///
  /// In en, this message translates to:
  /// **'Playhead {time}'**
  String vizPlayhead(String time);

  /// Button of the timeline that starts the playback.
  ///
  /// In en, this message translates to:
  /// **'Play'**
  String get vizPlay;

  /// Button of the timeline that stops the playback.
  ///
  /// In en, this message translates to:
  /// **'Pause'**
  String get vizPause;

  /// Speed of the timeline playback in seconds of time of day per second (viz.timeline_speeds).
  ///
  /// In en, this message translates to:
  /// **'{speed}x'**
  String vizSpeed(int speed);

  /// Label of the slider that moves the playhead of the timeline.
  ///
  /// In en, this message translates to:
  /// **'Jump to a minute'**
  String get vizJump;

  /// Header of TV mode: the shown view and its place in the rotation (screen D13).
  ///
  /// In en, this message translates to:
  /// **'{title} ({index} of {count})'**
  String vizTvPosition(String title, int index, int count);

  /// Title of screen E1, the server address (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Connect to your instance'**
  String get onboardingConnectTitle;

  /// Label of the address field on E1 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Server address'**
  String get onboardingServerAddress;

  /// Main button of E1; calls getDiscovery (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Connect'**
  String get onboardingConnect;

  /// State of E1 while getDiscovery runs (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Connecting'**
  String get onboardingConnecting;

  /// E1 error for an address that is no http or https URL (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Enter the full address of the instance, for example https://chat.example.com.'**
  String get onboardingAddressInvalid;

  /// E1 result without a valid discovery answer (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'No instance at this address. Check the address and try again.'**
  String get onboardingNoInstance;

  /// E1 result for a TLS error; there is no way to continue (01 section 5.6, R-PR-14).
  ///
  /// In en, this message translates to:
  /// **'Certificate invalid. The connection is not secure, so the app does not connect. Ask the owner of the instance to renew the certificate.'**
  String get onboardingCertificateInvalid;

  /// E1 result on setup_required with the pointer to the setup wizard (01 section 5.6, 16 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Instance not set up yet. The owner sets it up on the host with the setup wizard (install.sh).'**
  String get onboardingNotSetUp;

  /// E1 result below min_client_versions with the update path (01 section 5.6, 18).
  ///
  /// In en, this message translates to:
  /// **'App too old. This instance needs version {version} or newer. Install the update, then connect again.'**
  String onboardingAppTooOld(String version);

  /// Title of E2 with branding.product_name of the instance (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Log in to {productName}'**
  String onboardingLoginTitle(String productName);

  /// Button of E2 for an OIDC or proxy provider of discovery (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Log in with {name}'**
  String onboardingLoginWithProvider(String name);

  /// Separator on E2 between the providers and the local login (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'or'**
  String get onboardingOr;

  /// Field of E2 for the local login name (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Login name'**
  String get onboardingLoginName;

  /// Button of E2 for the passkey login (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Log in with passkey'**
  String get onboardingLoginWithPasskey;

  /// Button of E2 and option of E3 for the password with TOTP (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Password and one-time code'**
  String get onboardingPasswordAndCode;

  /// Password field of E2 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Password'**
  String get onboardingPassword;

  /// TOTP field of E2 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'One-time code'**
  String get onboardingOneTimeCode;

  /// Button of E2 that sends the password and one-time code (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Log in'**
  String get onboardingLogIn;

  /// Link of E2 to E3 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'I have an enrollment code'**
  String get onboardingEnrollmentCodeLink;

  /// Link of E2 to E5 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Pair with another device'**
  String get onboardingPairLink;

  /// E2 text when discovery names no login method (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'No login is set up for this instance. Ask the owner of the instance.'**
  String get onboardingNoMethod;

  /// Line of E2 with the address of the instance (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Server: {address}'**
  String onboardingServerHost(String address);

  /// Button of E2 back to E1 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Change'**
  String get onboardingChangeServer;

  /// Back button of E3 and E5 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Back'**
  String get onboardingBack;

  /// E2 and E3 error after a rejected login or code; always the same text (01 section 5.6, 08 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Login failed. Check your details and try again.'**
  String get onboardingLoginFailed;

  /// E2 to E5 error without an answer of the server (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'The instance is not reachable. Check the network and try again.'**
  String get onboardingUnreachable;

  /// E2 error after an OIDC redirect with an error (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'The login in the browser was cancelled. Start it again.'**
  String get onboardingBrowserCancelled;

  /// E2 error after an OIDC attempt expired (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'The login in the browser took too long. Start it again.'**
  String get onboardingBrowserExpired;

  /// Title of E3 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Enrollment code'**
  String get onboardingEnrollTitle;

  /// Field of E3 for the enrollment code (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Code from the owner'**
  String get onboardingEnrollCode;

  /// Question of E3 above the methods (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'How do you want to log in?'**
  String get onboardingEnrollHow;

  /// E3 method that redeems the code with the device keys only (08 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Use this device'**
  String get onboardingEnrollThisDevice;

  /// E3 method that redeems the code with a new passkey (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Create passkey'**
  String get onboardingEnrollCreatePasskey;

  /// E3 method that links a login service with the code (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Link with {name}'**
  String onboardingEnrollLink(String name);

  /// Main button of E3 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Continue'**
  String get onboardingContinue;

  /// Note of E3 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'The code is valid once.'**
  String get onboardingEnrollOnce;

  /// Title of E4 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Device waiting for confirmation'**
  String get onboardingPendingTitle;

  /// Instructions of E4 below the verification code (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Type this code on one of your active devices: Settings, Devices and sessions, Confirm. No active device: ask the owner.'**
  String get onboardingPendingExplain;

  /// Button of E4 that logs out and deletes the keys (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get onboardingCancel;

  /// Remaining time of a code on E4 and E5 as text (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'{minutes, plural, =0{Valid for less than a minute} =1{Valid for 1 more minute} other{Valid for {minutes} more minutes}}'**
  String onboardingValidFor(int minutes);

  /// Title of E5 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Pair device'**
  String get onboardingPairTitle;

  /// Instructions of E5 above the pairing code (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'On an active device: Settings, Devices, Add device, then enter:'**
  String get onboardingPairExplain;

  /// Button of E5 that starts a new pairing (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'New code'**
  String get onboardingPairNewCode;

  /// E5 text after the pairing expired (08 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'The code has expired. Get a new code.'**
  String get onboardingPairExpired;

  /// Title of E6 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'{productName} is locked'**
  String onboardingLockedTitle(String productName);

  /// Button of E6 that asks for biometrics or the device password (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Unlock'**
  String get onboardingUnlock;

  /// E6 text after a failed unlock (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Unlocking failed.'**
  String get onboardingUnlockFailed;

  /// Button of E6 after a failed unlock (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Try again'**
  String get onboardingTryAgain;

  /// Button of E6 and of the desktop shell that logs out and deletes the local data (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Log out'**
  String get onboardingLogOut;

  /// Title of E7 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'You are logged out.'**
  String get onboardingSignedOutTitle;

  /// Line of E7 with the reason text (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Reason: {reason}'**
  String onboardingSignedOutReason(String reason);

  /// E7 reason after the person logged out or cancelled E4 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'You logged out.'**
  String get onboardingReasonLoggedOut;

  /// E7 reason for devices.revoke_reason revoked_by_self (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'This device was revoked by another device.'**
  String get onboardingReasonRevokedBySelf;

  /// E7 reason for devices.revoke_reason revoked_by_owner (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'The owner revoked this device.'**
  String get onboardingReasonRevokedByOwner;

  /// E7 reason for devices.revoke_reason user_disabled (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Your account has been disabled.'**
  String get onboardingReasonUserDisabled;

  /// E7 reason for devices.revoke_reason inactive (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'This device was not used for a long time.'**
  String get onboardingReasonInactive;

  /// E7 reason for devices.revoke_reason verification_failed (rejected in E4).
  ///
  /// In en, this message translates to:
  /// **'The confirmation of this device was rejected.'**
  String get onboardingReasonRejected;

  /// E7 reason for devices.revoke_reason pending_expired (E4 expired).
  ///
  /// In en, this message translates to:
  /// **'This device was not confirmed in time.'**
  String get onboardingReasonPendingExpired;

  /// E7 reason for a revoked session (4012, session.revoked, 401 session_revoked).
  ///
  /// In en, this message translates to:
  /// **'Your session was ended on the server.'**
  String get onboardingReasonSessionRevoked;

  /// E7 reason when the server no longer accepts the refresh token (auth.session_max_age).
  ///
  /// In en, this message translates to:
  /// **'Your session has expired.'**
  String get onboardingReasonSessionExpired;

  /// Sentence of E7 after the wipe (04 section 8.5, R-SY-14).
  ///
  /// In en, this message translates to:
  /// **'The data of this device has been deleted.'**
  String get onboardingDataDeleted;

  /// Button of E7 that leads to E2 (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Log in again'**
  String get onboardingLogInAgain;

  /// E7 text for a disabled person instead of the login button (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Contact the owner of the instance.'**
  String get onboardingContactOwner;

  /// Button that copies a code (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Copy'**
  String get onboardingCopy;

  /// Confirmation after a code was copied (01 section 5.6).
  ///
  /// In en, this message translates to:
  /// **'Copied'**
  String get onboardingCopied;

  /// Semantics label of a code, spelled character by character (01 section 5.6, 8).
  ///
  /// In en, this message translates to:
  /// **'Code: {characters}'**
  String onboardingCodeLabel(String characters);

  /// Text of the browser page the loopback redirect answers with (15 section 8).
  ///
  /// In en, this message translates to:
  /// **'The login is complete. You can close this window.'**
  String get onboardingLoopbackDone;

  /// Semantics label of the sidebar column frame (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'Spaces and channels'**
  String get shellColumnSidebar;

  /// Semantics label of the post list column frame (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'Post list'**
  String get shellColumnPostList;

  /// Semantics label of the history column frame (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'History'**
  String get shellColumnHistory;

  /// Semantics label of the session column frame (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'Session'**
  String get shellColumnSession;

  /// Semantics label of the drag handle between two columns (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'Resize column'**
  String get shellResizeColumn;

  /// Banner after the close code 4007 (15 section 15).
  ///
  /// In en, this message translates to:
  /// **'Update required. Install the new version to connect again.'**
  String get shellUpdateRequired;

  /// Title of the system dialog that confirms a step-up signature with biometrics or the device password (08 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Confirm it is you'**
  String get shellStepUpTitle;

  /// Purpose text of the step-up dialog when the action names none (08 section 6.2).
  ///
  /// In en, this message translates to:
  /// **'This action needs your confirmation.'**
  String get stepUpGenericPurpose;

  /// Second line of the step-up dialog before the system prompt (08 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Next, your device asks for your fingerprint, face or device password.'**
  String get stepUpNext;

  /// Button of the step-up dialog that continues to the system prompt.
  ///
  /// In en, this message translates to:
  /// **'Continue'**
  String get stepUpContinue;

  /// Cancel button of the device, session and step-up dialogs.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get selfCancel;

  /// Message when a device or session action got no answer from the server.
  ///
  /// In en, this message translates to:
  /// **'No connection. Try again when you are online.'**
  String get selfOffline;

  /// Banner when the device or session list could not be loaded.
  ///
  /// In en, this message translates to:
  /// **'The list could not be loaded.'**
  String get selfLoadFailed;

  /// Action of the banner when a list could not be loaded.
  ///
  /// In en, this message translates to:
  /// **'Try again'**
  String get selfRetry;

  /// Purpose text of the step-up when a waiting device is confirmed (08 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Confirm the waiting device {name}'**
  String stepUpPurposeApproveDevice(String name);

  /// Purpose text of the step-up when a device is added by its pairing code (08 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'Add a new device to your account'**
  String get stepUpPurposeAddDevice;

  /// Purpose text of the step-up when a device is revoked (08 section 10).
  ///
  /// In en, this message translates to:
  /// **'Revoke the device {name}'**
  String stepUpPurposeRevokeDevice(String name);

  /// Heading of the own device list (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Devices'**
  String get devicesTitle;

  /// Button that opens the field for the pairing code of a new device (01 section 5.6 E5).
  ///
  /// In en, this message translates to:
  /// **'Add device'**
  String get devicesAdd;

  /// Badge of the device the app runs on.
  ///
  /// In en, this message translates to:
  /// **'This device'**
  String get devicesThisDevice;

  /// Badge of a waiting device (08 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Waiting for confirmation'**
  String get devicesStatePending;

  /// Badge of a revoked device (08 section 10).
  ///
  /// In en, this message translates to:
  /// **'Revoked'**
  String get devicesStateRevoked;

  /// App version of a device in the device list.
  ///
  /// In en, this message translates to:
  /// **'Version {version}'**
  String devicesAppVersion(String version);

  /// Last activity of a device; when is a relative day and a clock time.
  ///
  /// In en, this message translates to:
  /// **'Last active {when}'**
  String devicesLastActive(String when);

  /// Platform name of the device platform ios.
  ///
  /// In en, this message translates to:
  /// **'iPhone or iPad'**
  String get devicesPlatformIos;

  /// Platform name of the device platform android.
  ///
  /// In en, this message translates to:
  /// **'Android'**
  String get devicesPlatformAndroid;

  /// Platform name of the device platform macos.
  ///
  /// In en, this message translates to:
  /// **'Mac'**
  String get devicesPlatformMacos;

  /// Platform name of the device platform windows.
  ///
  /// In en, this message translates to:
  /// **'Windows'**
  String get devicesPlatformWindows;

  /// Platform name of the device platform web.
  ///
  /// In en, this message translates to:
  /// **'Web console'**
  String get devicesPlatformWeb;

  /// Platform name of the device platform cli.
  ///
  /// In en, this message translates to:
  /// **'Command line'**
  String get devicesPlatformCli;

  /// Platform name of a platform this build does not know.
  ///
  /// In en, this message translates to:
  /// **'Other platform'**
  String get devicesPlatformOther;

  /// Button of a waiting device that opens the field for its confirmation code (01 section 5.6 E4).
  ///
  /// In en, this message translates to:
  /// **'Enter code'**
  String get devicesEnterCode;

  /// Button that revokes a device (08 section 10).
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get devicesRevoke;

  /// Title of the confirmation before a device is revoked (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Revoke {name}?'**
  String devicesRevokeTitle(String name);

  /// Text of the confirmation before another device is revoked (08 section 10).
  ///
  /// In en, this message translates to:
  /// **'The device is signed out and deletes its local data at its next contact.'**
  String get devicesRevokeBody;

  /// Text of the confirmation before this device is revoked (08 section 10).
  ///
  /// In en, this message translates to:
  /// **'You are signed out on this device and its local data is deleted.'**
  String get devicesRevokeThisBody;

  /// Label of the optional reason field when a device is revoked.
  ///
  /// In en, this message translates to:
  /// **'Reason (optional)'**
  String get devicesRevokeReason;

  /// Reason sent with a revocation when the person typed none.
  ///
  /// In en, this message translates to:
  /// **'Revoked in the settings'**
  String get devicesRevokeDefaultReason;

  /// Message after a device was revoked.
  ///
  /// In en, this message translates to:
  /// **'{name} was revoked.'**
  String devicesRevoked(String name);

  /// Title of the dialog that takes the confirmation code of a waiting device (08 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Confirm {name}'**
  String devicesConfirmTitle(String name);

  /// Hint of the dialog that takes the confirmation code of a waiting device.
  ///
  /// In en, this message translates to:
  /// **'Type the code that the waiting device shows.'**
  String get devicesConfirmHint;

  /// Label of the field for the confirmation code of a waiting device.
  ///
  /// In en, this message translates to:
  /// **'Confirmation code'**
  String get devicesConfirmCode;

  /// Button that sends the confirmation code of a waiting device.
  ///
  /// In en, this message translates to:
  /// **'Confirm'**
  String get devicesConfirmSubmit;

  /// Message after a waiting device was confirmed.
  ///
  /// In en, this message translates to:
  /// **'{name} is now active.'**
  String devicesConfirmed(String name);

  /// Message after a wrong confirmation code revoked the waiting device (08 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'The waiting device was rejected because the code did not match.'**
  String get devicesConfirmRejected;

  /// Title of the dialog that takes the pairing code of a new device (08 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'Add device'**
  String get devicesAddTitle;

  /// Hint of the dialog that takes the pairing code of a new device.
  ///
  /// In en, this message translates to:
  /// **'Type the pairing code that the new device shows.'**
  String get devicesAddHint;

  /// Label of the field for the pairing code of a new device.
  ///
  /// In en, this message translates to:
  /// **'Pairing code'**
  String get devicesAddCode;

  /// Button that sends the pairing code of a new device.
  ///
  /// In en, this message translates to:
  /// **'Add'**
  String get devicesAddSubmit;

  /// Message after a new device was added by its pairing code.
  ///
  /// In en, this message translates to:
  /// **'{name} was added.'**
  String devicesAdded(String name);

  /// Message when the pairing code matches no open pairing (08 section 5.3, 404).
  ///
  /// In en, this message translates to:
  /// **'No open pairing with this code.'**
  String get devicesPairingNotFound;

  /// Heading of the own session list.
  ///
  /// In en, this message translates to:
  /// **'Sessions'**
  String get sessionsTitle;

  /// Badge of the session of this app.
  ///
  /// In en, this message translates to:
  /// **'This session'**
  String get sessionsCurrent;

  /// Name of a session whose device is not in the device list.
  ///
  /// In en, this message translates to:
  /// **'Unknown device'**
  String get sessionsUnknownDevice;

  /// Last use of a session; when is a relative day and a clock time.
  ///
  /// In en, this message translates to:
  /// **'Last used {when}'**
  String sessionsLastUsed(String when);

  /// Login method oidc of a session.
  ///
  /// In en, this message translates to:
  /// **'Login service'**
  String get sessionsMethodOidc;

  /// Login method trusted_header of a session (08 section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Login through the network'**
  String get sessionsMethodTrustedHeader;

  /// Login method passkey of a session.
  ///
  /// In en, this message translates to:
  /// **'Passkey'**
  String get sessionsMethodPasskey;

  /// Login method password_totp of a session.
  ///
  /// In en, this message translates to:
  /// **'Password and one-time code'**
  String get sessionsMethodPasswordTotp;

  /// Login method enrollment_code of a session.
  ///
  /// In en, this message translates to:
  /// **'Enrollment code'**
  String get sessionsMethodEnrollmentCode;

  /// Login method device_pairing of a session.
  ///
  /// In en, this message translates to:
  /// **'Pairing'**
  String get sessionsMethodDevicePairing;

  /// Login method this build does not know.
  ///
  /// In en, this message translates to:
  /// **'Other method'**
  String get sessionsMethodOther;

  /// Button that ends a session.
  ///
  /// In en, this message translates to:
  /// **'End session'**
  String get sessionsEnd;

  /// Title of the confirmation before a session ends (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'End this session?'**
  String get sessionsEndTitle;

  /// Text of the confirmation before another session ends.
  ///
  /// In en, this message translates to:
  /// **'The device of this session is signed out.'**
  String get sessionsEndBody;

  /// Text of the confirmation before the session of this app ends (R-SY-14).
  ///
  /// In en, this message translates to:
  /// **'You are signed out on this device and its local data is deleted.'**
  String get sessionsEndCurrentBody;

  /// Message after a session was ended.
  ///
  /// In en, this message translates to:
  /// **'The session was ended.'**
  String get sessionsEnded;

  /// Title of the settings and the button that opens them (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get settingsTitle;

  /// Button that leaves the settings.
  ///
  /// In en, this message translates to:
  /// **'Back'**
  String get settingsBack;

  /// Settings section Appearance (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Appearance'**
  String get settingsSectionAppearance;

  /// Settings section Notifications (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Notifications'**
  String get settingsSectionNotifications;

  /// Settings section Language (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Language'**
  String get settingsSectionLanguage;

  /// Settings section Keyboard (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Keyboard'**
  String get settingsSectionKeyboard;

  /// Settings section Devices and sessions (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Devices and sessions'**
  String get settingsSectionDevices;

  /// Settings section Privacy (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'Privacy'**
  String get settingsSectionPrivacy;

  /// Settings section About (01 section 5.1 D8).
  ///
  /// In en, this message translates to:
  /// **'About'**
  String get settingsSectionAbout;

  /// Label of the setting appearance.theme (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Theme'**
  String get settingsTheme;

  /// Value system of appearance.theme.
  ///
  /// In en, this message translates to:
  /// **'System'**
  String get settingsThemeSystem;

  /// Value light of appearance.theme.
  ///
  /// In en, this message translates to:
  /// **'Light'**
  String get settingsThemeLight;

  /// Value dark of appearance.theme.
  ///
  /// In en, this message translates to:
  /// **'Dark'**
  String get settingsThemeDark;

  /// Label of the setting appearance.density (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Density'**
  String get settingsDensity;

  /// Value comfortable of appearance.density.
  ///
  /// In en, this message translates to:
  /// **'Normal'**
  String get settingsDensityComfortable;

  /// Value compact of appearance.density.
  ///
  /// In en, this message translates to:
  /// **'Compact'**
  String get settingsDensityCompact;

  /// Label of the setting appearance.reduce_motion (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Motion'**
  String get settingsMotion;

  /// Value system of appearance.reduce_motion.
  ///
  /// In en, this message translates to:
  /// **'System'**
  String get settingsMotionSystem;

  /// Value off of appearance.reduce_motion (motion is not reduced).
  ///
  /// In en, this message translates to:
  /// **'Full'**
  String get settingsMotionFull;

  /// Value on of appearance.reduce_motion (motion is reduced).
  ///
  /// In en, this message translates to:
  /// **'Reduced'**
  String get settingsMotionReduced;

  /// Label of the setting composer.enter_sends (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Enter sends'**
  String get settingsEnterSends;

  /// Hint of the setting composer.enter_sends.
  ///
  /// In en, this message translates to:
  /// **'Shift+Enter: new line'**
  String get settingsEnterSendsHint;

  /// Label of the reset of layout.column_widths (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Column widths'**
  String get settingsColumnWidths;

  /// Button that deletes layout.column_widths.
  ///
  /// In en, this message translates to:
  /// **'Reset'**
  String get settingsColumnWidthsReset;

  /// Label of the language of the profile (01 section 9).
  ///
  /// In en, this message translates to:
  /// **'Language'**
  String get settingsLanguage;

  /// Language choice German.
  ///
  /// In en, this message translates to:
  /// **'German'**
  String get settingsLanguageGerman;

  /// Language choice English.
  ///
  /// In en, this message translates to:
  /// **'English'**
  String get settingsLanguageEnglish;

  /// Label of the time zone of the profile.
  ///
  /// In en, this message translates to:
  /// **'Time zone'**
  String get settingsTimeZone;

  /// Message when updateMyProfile failed; reason is the translated problem.
  ///
  /// In en, this message translates to:
  /// **'The profile could not be saved: {reason}'**
  String settingsProfileFailed(String reason);

  /// Hint of the keyboard section that names the shortcut of the overlay (M4-24).
  ///
  /// In en, this message translates to:
  /// **'All keyboard shortcuts: {shortcut}'**
  String settingsShortcuts(String shortcut);

  /// Label of the setting privacy.crash_reports (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Send crash reports'**
  String get settingsCrashReports;

  /// Hint of the setting privacy.crash_reports (15 section 16).
  ///
  /// In en, this message translates to:
  /// **'Without content: app version, platform and device type only.'**
  String get settingsCrashReportsHint;

  /// Label of the setting privacy.client_metrics (01 section 11).
  ///
  /// In en, this message translates to:
  /// **'Send performance data'**
  String get settingsClientMetrics;

  /// Hint of the setting privacy.client_metrics (15 section 17).
  ///
  /// In en, this message translates to:
  /// **'Without content, IDs or names.'**
  String get settingsClientMetricsHint;

  /// Label of the version of this build.
  ///
  /// In en, this message translates to:
  /// **'App version'**
  String get settingsAboutAppVersion;

  /// Label of the spec_version of the instance.
  ///
  /// In en, this message translates to:
  /// **'Server specification'**
  String get settingsAboutServerVersion;

  /// Button that opens the licenses page.
  ///
  /// In en, this message translates to:
  /// **'Licenses'**
  String get settingsAboutLicenses;

  /// Placeholder of the quick switcher field at the top of the sidebar; M4-23 opens the switcher (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Jump to'**
  String get sidebarJumpTo;

  /// Entry of the sidebar that opens the cross-cutting view of active runs; M4-23 wires it (01 section 4).
  ///
  /// In en, this message translates to:
  /// **'Running now'**
  String get sidebarRunningNow;

  /// Button next to the own avatar at the bottom of the sidebar (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get sidebarSettings;

  /// Semantics label of the own avatar at the bottom of the sidebar.
  ///
  /// In en, this message translates to:
  /// **'{name}, signed in'**
  String sidebarOwnAvatar(String name);

  /// Accessibility action of a space row that moves the space one place up in the personal order (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Move up'**
  String get sidebarMoveUp;

  /// Accessibility action of a space row that moves the space one place down in the personal order (components.md section 7).
  ///
  /// In en, this message translates to:
  /// **'Move down'**
  String get sidebarMoveDown;

  /// Text of the sidebar while the person sees no space.
  ///
  /// In en, this message translates to:
  /// **'No spaces yet'**
  String get sidebarNoSpaces;

  /// Primary button in the header of the post list and title of the dialog that creates a forum post (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'New post'**
  String get postListNewPost;

  /// Filter chip of the post list: every open post (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get postListFilterOpen;

  /// Filter chip of the post list: posts with a question of a run the reader started, or an unread mention of the reader (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Waiting for me'**
  String get postListFilterWaitingForMe;

  /// Filter chip of the post list that opens the tag selection (components.md section 6).
  ///
  /// In en, this message translates to:
  /// **'Tags'**
  String get postListFilterTags;

  /// Filter chip of the post list with the number of selected tags.
  ///
  /// In en, this message translates to:
  /// **'Tags ({count})'**
  String postListFilterTagsCount(int count);

  /// Sort chip of the post list: newest activity first.
  ///
  /// In en, this message translates to:
  /// **'Last activity'**
  String get postListSortLastActivity;

  /// Sort chip of the post list: newest post first (sort by creation).
  ///
  /// In en, this message translates to:
  /// **'Newest first'**
  String get postListSortCreated;

  /// Row that collapses or expands the section of done posts at the bottom of the post list (01 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Done ({count})'**
  String postListDone(int count);

  /// Row of the done section while further pages of the forum are not loaded yet, so more done posts may exist (01 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Done ({count}+)'**
  String postListDoneMore(int count);

  /// Text of the post list when no open post matches the filters.
  ///
  /// In en, this message translates to:
  /// **'No open post here'**
  String get postListEmpty;

  /// Notice in the post list when the server refused a new post; the outbox keeps it until the person tries again or discards it.
  ///
  /// In en, this message translates to:
  /// **'\"{title}\" could not be posted'**
  String postListCreateFailed(String title);

  /// Button of a refused new post: sends it again.
  ///
  /// In en, this message translates to:
  /// **'Try again'**
  String get postListTryAgain;

  /// Button of a refused new post: deletes it.
  ///
  /// In en, this message translates to:
  /// **'Discard'**
  String get postListDiscard;

  /// Row at the end of the post list when a page failed; its button tries again.
  ///
  /// In en, this message translates to:
  /// **'More posts could not be loaded'**
  String get postListLoadFailed;

  /// Button of a refused new post: opens it in the New post dialog to change and send it again.
  ///
  /// In en, this message translates to:
  /// **'Edit'**
  String get postListEdit;

  /// Second line of a refused new post with the server's reason.
  ///
  /// In en, this message translates to:
  /// **'Reason: {reason}'**
  String postListFailureReason(String reason);

  /// Semantics label of the post list of a forum.
  ///
  /// In en, this message translates to:
  /// **'Post list of {forum}'**
  String postListLabel(String forum);

  /// Label of the title field of the dialog that creates a forum post.
  ///
  /// In en, this message translates to:
  /// **'Title'**
  String get newPostTitle;

  /// Label of the field for the first message of a new forum post.
  ///
  /// In en, this message translates to:
  /// **'First message'**
  String get newPostMessage;

  /// Label of the tag selection of the dialog that creates a forum post.
  ///
  /// In en, this message translates to:
  /// **'Tags'**
  String get newPostTags;

  /// Button of the dialog that creates the forum post.
  ///
  /// In en, this message translates to:
  /// **'Create post'**
  String get newPostCreate;

  /// Button of the dialog that closes it without creating a post.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get newPostCancel;

  /// Heading of the administration navigation (01 section 5.2 A1).
  ///
  /// In en, this message translates to:
  /// **'Administration'**
  String get adminTitle;

  /// Button above the administration navigation that leaves the administration.
  ///
  /// In en, this message translates to:
  /// **'Back to the chat'**
  String get adminBack;

  /// Entry of the administration navigation: people (01 section 5.2 A1).
  ///
  /// In en, this message translates to:
  /// **'People'**
  String get adminNavPeople;

  /// Entry of the administration navigation: groups (07 section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Groups'**
  String get adminNavGroups;

  /// Entry of the administration navigation: the queue of waiting devices.
  ///
  /// In en, this message translates to:
  /// **'Devices'**
  String get adminNavDevices;

  /// Entry of the administration navigation with the number of waiting devices (01 section 5.2 A1).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{Devices} =1{Devices (1 waiting)} other{Devices ({count} waiting)}}'**
  String adminNavDevicesWaiting(int count);

  /// Label of the optional reason field of a confirmation in the administration.
  ///
  /// In en, this message translates to:
  /// **'Reason (stored in the audit log)'**
  String get adminReason;

  /// Reason sent to the server when the person typed none.
  ///
  /// In en, this message translates to:
  /// **'Changed in the administration'**
  String get adminDefaultReason;

  /// Button below a list of the administration that loads its next page.
  ///
  /// In en, this message translates to:
  /// **'Load more'**
  String get adminLoadMore;

  /// Note when the server refuses to repeat an enrollment code (06 section 7, one_time_secret).
  ///
  /// In en, this message translates to:
  /// **'The code was shown once and cannot be shown again. Issue a new code.'**
  String get adminOneTimeSecret;

  /// Explanation after enabling a person failed with i1_violation (10 section 5.4).
  ///
  /// In en, this message translates to:
  /// **'Change the mode of the affected agent under Agents first, then enable the person again.'**
  String get adminI1Explained;

  /// Hint of the person selection in the dialogs of the administration.
  ///
  /// In en, this message translates to:
  /// **'Choose a person'**
  String get adminPickPerson;

  /// Button that revokes a passkey.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get adminRevoke;

  /// Badge of a person who is an owner (07 section 2.7).
  ///
  /// In en, this message translates to:
  /// **'Owner'**
  String get adminOwnerBadge;

  /// State of a person: active.
  ///
  /// In en, this message translates to:
  /// **'Active'**
  String get adminStatusActive;

  /// State of a person who has not signed in yet.
  ///
  /// In en, this message translates to:
  /// **'Waiting for login'**
  String get adminStatusPending;

  /// State of a disabled person (08 section 11).
  ///
  /// In en, this message translates to:
  /// **'Disabled'**
  String get adminStatusDisabled;

  /// State of a deleted person.
  ///
  /// In en, this message translates to:
  /// **'Deleted'**
  String get adminStatusDeleted;

  /// Heading of the people list (01 section 5.2 A1).
  ///
  /// In en, this message translates to:
  /// **'People'**
  String get adminPeopleTitle;

  /// Button that creates a person (no self-registration, no invitation links).
  ///
  /// In en, this message translates to:
  /// **'Add person'**
  String get adminPeopleAdd;

  /// Label of the search field of the people list.
  ///
  /// In en, this message translates to:
  /// **'Search'**
  String get adminPeopleSearch;

  /// Choice of the state filter that shows everybody.
  ///
  /// In en, this message translates to:
  /// **'All states'**
  String get adminPeopleStatusAll;

  /// Choice of the group filter that shows everybody.
  ///
  /// In en, this message translates to:
  /// **'All groups'**
  String get adminPeopleGroupAll;

  /// Note below the filters while a group filter is active; the server has no group filter for people.
  ///
  /// In en, this message translates to:
  /// **'The group filter applies only to the people loaded on this page.'**
  String get adminPeopleGroupNote;

  /// Text of the people list when nobody matches.
  ///
  /// In en, this message translates to:
  /// **'No person matches.'**
  String get adminPeopleEmpty;

  /// Notice after a person was created.
  ///
  /// In en, this message translates to:
  /// **'{name} was created.'**
  String adminPersonCreated(String name);

  /// Title of the dialog that creates a person.
  ///
  /// In en, this message translates to:
  /// **'Add person'**
  String get adminCreatePersonTitle;

  /// What creating a person does (08 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'The person gets an account. With a local account, the enrollment code is shown once in the next step.'**
  String get adminCreatePersonBody;

  /// Label of the display name field.
  ///
  /// In en, this message translates to:
  /// **'Display name'**
  String get adminCreatePersonName;

  /// Label of the handle field.
  ///
  /// In en, this message translates to:
  /// **'Handle'**
  String get adminCreatePersonHandle;

  /// Error below the handle field when it does not match the pattern.
  ///
  /// In en, this message translates to:
  /// **'Use lower case letters, digits, dots, hyphens and underscores.'**
  String get adminCreatePersonHandleInvalid;

  /// Checkbox: the person gets a local identity and a first login code.
  ///
  /// In en, this message translates to:
  /// **'Local account with an enrollment code'**
  String get adminCreatePersonLocal;

  /// Button that confirms creating a person.
  ///
  /// In en, this message translates to:
  /// **'Create'**
  String get adminCreatePersonSubmit;

  /// Title of the view that shows an enrollment code once.
  ///
  /// In en, this message translates to:
  /// **'Enrollment code for {name}'**
  String adminCodeTitle(String name);

  /// Warning of the enrollment code view (08 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'This code is shown only once. Hand it over in person or by phone.'**
  String get adminCodeOnce;

  /// Purpose of an enrollment code: first_login.
  ///
  /// In en, this message translates to:
  /// **'For the first login'**
  String get adminCodePurposeFirstLogin;

  /// Purpose of an enrollment code: recovery.
  ///
  /// In en, this message translates to:
  /// **'For recovery'**
  String get adminCodePurposeRecovery;

  /// Purpose of an enrollment code: identity_link.
  ///
  /// In en, this message translates to:
  /// **'For linking a login service'**
  String get adminCodePurposeIdentityLink;

  /// Expiry of an enrollment code; when is a relative day and a time.
  ///
  /// In en, this message translates to:
  /// **'Valid until {when}'**
  String adminCodeExpires(String when);

  /// Button that closes the enrollment code view and discards the code.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get adminCodeClose;

  /// Button above a person that returns to the list.
  ///
  /// In en, this message translates to:
  /// **'All people'**
  String get adminPersonBack;

  /// Button that disables a person (08 section 11).
  ///
  /// In en, this message translates to:
  /// **'Disable'**
  String get adminPersonDisable;

  /// Button that enables a disabled person.
  ///
  /// In en, this message translates to:
  /// **'Enable'**
  String get adminPersonEnable;

  /// Button that issues a new enrollment code (08 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'New enrollment code'**
  String get adminPersonNewCode;

  /// Title of the confirmation before a person is disabled.
  ///
  /// In en, this message translates to:
  /// **'Disable {name}?'**
  String adminDisableTitle(String name);

  /// What disabling does (08 section 11).
  ///
  /// In en, this message translates to:
  /// **'{name} is disabled. All sessions of {name} end now and all devices are revoked.'**
  String adminDisableBody(String name);

  /// Notice after a person was disabled.
  ///
  /// In en, this message translates to:
  /// **'{name} is disabled. All sessions ended.'**
  String adminPersonDisabled(String name);

  /// Title of the confirmation before a person is enabled.
  ///
  /// In en, this message translates to:
  /// **'Enable {name}?'**
  String adminEnableTitle(String name);

  /// What enabling does (08 section 11).
  ///
  /// In en, this message translates to:
  /// **'{name} can sign in again. Earlier devices stay revoked.'**
  String adminEnableBody(String name);

  /// Notice after a person was enabled.
  ///
  /// In en, this message translates to:
  /// **'{name} is enabled.'**
  String adminPersonEnabled(String name);

  /// Title of the confirmation before a new enrollment code is issued.
  ///
  /// In en, this message translates to:
  /// **'New enrollment code for {name}?'**
  String adminNewCodeTitle(String name);

  /// What issuing a code does (08 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'A new code is issued and shown once. Every older code of {name} stops working.'**
  String adminNewCodeBody(String name);

  /// Heading of the credentials of a person.
  ///
  /// In en, this message translates to:
  /// **'Credentials'**
  String get adminCredentialsTitle;

  /// The person has a local password.
  ///
  /// In en, this message translates to:
  /// **'Local password set'**
  String get adminCredentialsPasswordSet;

  /// The person has no local password.
  ///
  /// In en, this message translates to:
  /// **'No local password'**
  String get adminCredentialsPasswordNone;

  /// The person has a one-time code app.
  ///
  /// In en, this message translates to:
  /// **'One-time code app set up'**
  String get adminCredentialsTotpSet;

  /// The person has no one-time code app.
  ///
  /// In en, this message translates to:
  /// **'No one-time code app'**
  String get adminCredentialsTotpNone;

  /// Heading of the linked identities.
  ///
  /// In en, this message translates to:
  /// **'Linked login services'**
  String get adminCredentialsIdentities;

  /// Heading of the passkeys.
  ///
  /// In en, this message translates to:
  /// **'Passkeys'**
  String get adminCredentialsPasskeys;

  /// Text of an empty credential list.
  ///
  /// In en, this message translates to:
  /// **'None'**
  String get adminCredentialsNone;

  /// When a passkey was registered.
  ///
  /// In en, this message translates to:
  /// **'Added {when}'**
  String adminPasskeyAdded(String when);

  /// Button that unlinks an identity (08 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Unlink'**
  String get adminUnlink;

  /// Title of the confirmation before an identity is unlinked.
  ///
  /// In en, this message translates to:
  /// **'Unlink {subject}?'**
  String adminUnlinkTitle(String subject);

  /// What unlinking does.
  ///
  /// In en, this message translates to:
  /// **'{name} can no longer sign in with this login service.'**
  String adminUnlinkBody(String name);

  /// Notice after an identity was unlinked.
  ///
  /// In en, this message translates to:
  /// **'{subject} was unlinked.'**
  String adminUnlinked(String subject);

  /// Title of the confirmation before a passkey is revoked.
  ///
  /// In en, this message translates to:
  /// **'Revoke the passkey {passkey}?'**
  String adminRevokePasskeyTitle(String passkey);

  /// What revoking a passkey does.
  ///
  /// In en, this message translates to:
  /// **'{name} can no longer sign in with this passkey.'**
  String adminRevokePasskeyBody(String name);

  /// Notice after a passkey was revoked.
  ///
  /// In en, this message translates to:
  /// **'The passkey {passkey} was revoked.'**
  String adminPasskeyRevoked(String passkey);

  /// What ending the session of another person does.
  ///
  /// In en, this message translates to:
  /// **'The device of this session is signed out at once.'**
  String get adminEndSessionBody;

  /// Notice after a session of another person was ended.
  ///
  /// In en, this message translates to:
  /// **'The session was ended.'**
  String get adminSessionEnded;

  /// Heading of the owners (07 section 2.7).
  ///
  /// In en, this message translates to:
  /// **'Owners'**
  String get adminOwnersTitle;

  /// Button that makes a person an owner.
  ///
  /// In en, this message translates to:
  /// **'Add owner'**
  String get adminOwnersAdd;

  /// Title of the dialog that adds an owner.
  ///
  /// In en, this message translates to:
  /// **'Add an owner'**
  String get adminOwnersAddTitle;

  /// What adding an owner does (07 section 2.7).
  ///
  /// In en, this message translates to:
  /// **'The person gets every permission of the instance.'**
  String get adminOwnersAddBody;

  /// Notice after an owner was added.
  ///
  /// In en, this message translates to:
  /// **'{name} is now an owner.'**
  String adminOwnersAdded(String name);

  /// Button that revokes an owner.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get adminOwnersRevoke;

  /// Title of the confirmation before an owner is revoked.
  ///
  /// In en, this message translates to:
  /// **'Revoke the owner {name}?'**
  String adminOwnersRevokeTitle(String name);

  /// What revoking an owner does.
  ///
  /// In en, this message translates to:
  /// **'{name} keeps the account but loses the owner permissions.'**
  String adminOwnersRevokeBody(String name);

  /// Notice after an owner was revoked.
  ///
  /// In en, this message translates to:
  /// **'{name} is no longer an owner.'**
  String adminOwnersRevoked(String name);

  /// Since when and why a person is an owner.
  ///
  /// In en, this message translates to:
  /// **'Since {when}: {reason}'**
  String adminOwnerSince(String when, String reason);

  /// Heading of the group list (07 section 2.1).
  ///
  /// In en, this message translates to:
  /// **'Groups'**
  String get adminGroupsTitle;

  /// Button that creates a group.
  ///
  /// In en, this message translates to:
  /// **'Create group'**
  String get adminGroupsCreate;

  /// Text of an empty group list.
  ///
  /// In en, this message translates to:
  /// **'No groups yet.'**
  String get adminGroupsEmpty;

  /// Member count of a group.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{No members} =1{1 member} other{{count} members}}'**
  String adminGroupMembers(int count);

  /// Notice after a group was created.
  ///
  /// In en, this message translates to:
  /// **'The group {name} was created.'**
  String adminGroupCreated(String name);

  /// Title of the dialog that creates a group.
  ///
  /// In en, this message translates to:
  /// **'Create group'**
  String get adminGroupCreateTitle;

  /// What creating a group does.
  ///
  /// In en, this message translates to:
  /// **'A new group without members is created.'**
  String get adminGroupCreateBody;

  /// Label of the group name field.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get adminGroupName;

  /// Label of the group description field.
  ///
  /// In en, this message translates to:
  /// **'Description (optional)'**
  String get adminGroupDescription;

  /// Button above a group that returns to the list.
  ///
  /// In en, this message translates to:
  /// **'All groups'**
  String get adminGroupBack;

  /// Note of a group whose members come from the identity provider (08 section 3.4).
  ///
  /// In en, this message translates to:
  /// **'The login service manages the members of this group.'**
  String get adminGroupFromProvider;

  /// Button that renames a group.
  ///
  /// In en, this message translates to:
  /// **'Rename'**
  String get adminGroupRename;

  /// Title of the dialog that renames a group.
  ///
  /// In en, this message translates to:
  /// **'Rename {name}'**
  String adminGroupRenameTitle(String name);

  /// What renaming a group does.
  ///
  /// In en, this message translates to:
  /// **'The group keeps its members and its permissions.'**
  String get adminGroupRenameBody;

  /// Notice after a group was renamed.
  ///
  /// In en, this message translates to:
  /// **'The group is now called {name}.'**
  String adminGroupRenamed(String name);

  /// Button that deletes a group.
  ///
  /// In en, this message translates to:
  /// **'Delete group'**
  String get adminGroupDelete;

  /// Title of the confirmation before a group is deleted.
  ///
  /// In en, this message translates to:
  /// **'Delete {name}?'**
  String adminGroupDeleteTitle(String name);

  /// What deleting a group does.
  ///
  /// In en, this message translates to:
  /// **'The group is removed and the permissions it gave end. The members keep their accounts.'**
  String get adminGroupDeleteBody;

  /// Heading of the members of a group.
  ///
  /// In en, this message translates to:
  /// **'Members'**
  String get adminMembersTitle;

  /// Text of a group without members.
  ///
  /// In en, this message translates to:
  /// **'No members yet.'**
  String get adminMembersEmpty;

  /// Button that adds a member to a group.
  ///
  /// In en, this message translates to:
  /// **'Add member'**
  String get adminMemberAdd;

  /// Title of the dialog that adds a member.
  ///
  /// In en, this message translates to:
  /// **'Add a member to {group}'**
  String adminMemberAddTitle(String group);

  /// What adding a member does (07 section 2.1).
  ///
  /// In en, this message translates to:
  /// **'The person gets the permissions of the group, until the given day when one is set.'**
  String get adminMemberAddBody;

  /// Label of the expiry field of a membership.
  ///
  /// In en, this message translates to:
  /// **'Until (optional, YYYY-MM-DD)'**
  String get adminMemberUntil;

  /// Error below the expiry field.
  ///
  /// In en, this message translates to:
  /// **'Enter a date as YYYY-MM-DD.'**
  String get adminMemberUntilInvalid;

  /// Notice after a member was added.
  ///
  /// In en, this message translates to:
  /// **'{name} was added to {group}.'**
  String adminMemberAdded(String name, String group);

  /// When a membership ends.
  ///
  /// In en, this message translates to:
  /// **'Until {when}'**
  String adminMemberExpires(String when);

  /// Button that removes a member from a group.
  ///
  /// In en, this message translates to:
  /// **'Remove'**
  String get adminMemberRemove;

  /// Title of the confirmation before a member is removed.
  ///
  /// In en, this message translates to:
  /// **'Remove {name} from {group}?'**
  String adminMemberRemoveTitle(String name, String group);

  /// What removing a member does.
  ///
  /// In en, this message translates to:
  /// **'{name} loses the permissions of the group.'**
  String adminMemberRemoveBody(String name);

  /// Notice after a member was removed.
  ///
  /// In en, this message translates to:
  /// **'{name} was removed from {group}.'**
  String adminMemberRemoved(String name, String group);

  /// Heading of the queue of waiting devices (01 section 5.2 A1).
  ///
  /// In en, this message translates to:
  /// **'Waiting devices'**
  String get adminWaitingTitle;

  /// Text of an empty queue.
  ///
  /// In en, this message translates to:
  /// **'No device is waiting.'**
  String get adminWaitingEmpty;

  /// A waiting device with its person.
  ///
  /// In en, this message translates to:
  /// **'{person}: {device}'**
  String adminWaitingDevice(String person, String device);

  /// When the waiting device logged in.
  ///
  /// In en, this message translates to:
  /// **'Logged in {when}'**
  String adminWaitingLoggedIn(String when);

  /// The origin of the login of a waiting device.
  ///
  /// In en, this message translates to:
  /// **'via {method}'**
  String adminWaitingVia(String method);

  /// A waiting device without a session that names its login.
  ///
  /// In en, this message translates to:
  /// **'Origin unknown'**
  String get adminWaitingNoOrigin;

  /// Button that approves a waiting device (08 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Approve'**
  String get adminApprove;

  /// Button that rejects a waiting device.
  ///
  /// In en, this message translates to:
  /// **'Reject'**
  String get adminReject;

  /// Title of the dialog that approves a waiting device.
  ///
  /// In en, this message translates to:
  /// **'Approve {device} of {person}'**
  String adminApproveTitle(String device, String person);

  /// Hint of the approval dialog; the code never appears for owners (08 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Ask {person} for the code that the waiting device shows and type it here.'**
  String adminApproveHint(String person);

  /// Notice after a waiting device was approved.
  ///
  /// In en, this message translates to:
  /// **'{device} of {person} is now active.'**
  String adminApproved(String device, String person);

  /// Title of the confirmation before a waiting device is rejected.
  ///
  /// In en, this message translates to:
  /// **'Reject {device}?'**
  String adminRejectTitle(String device);

  /// What rejecting a waiting device does.
  ///
  /// In en, this message translates to:
  /// **'The waiting device of {person} is revoked and cannot sign in.'**
  String adminRejectBody(String person);

  /// Notice after a waiting device was rejected.
  ///
  /// In en, this message translates to:
  /// **'{device} was rejected.'**
  String adminRejected(String device);

  /// Purpose of the step-up of createUser.
  ///
  /// In en, this message translates to:
  /// **'Create the person {name}'**
  String stepUpPurposeCreatePerson(String name);

  /// Purpose of the step-up of disableUser.
  ///
  /// In en, this message translates to:
  /// **'Disable {name}'**
  String stepUpPurposeDisablePerson(String name);

  /// Purpose of the step-up of enableUser.
  ///
  /// In en, this message translates to:
  /// **'Enable {name}'**
  String stepUpPurposeEnablePerson(String name);

  /// Purpose of the step-up of createEnrollmentCode.
  ///
  /// In en, this message translates to:
  /// **'Issue an enrollment code for {name}'**
  String stepUpPurposeNewCode(String name);

  /// Purpose of the step-up of unlinkIdentity.
  ///
  /// In en, this message translates to:
  /// **'Unlink a login service of {name}'**
  String stepUpPurposeUnlinkIdentity(String name);

  /// Purpose of the step-up of revokePasskey.
  ///
  /// In en, this message translates to:
  /// **'Revoke a passkey of {name}'**
  String stepUpPurposeRevokePasskey(String name);

  /// Purpose of the step-up of addOwner.
  ///
  /// In en, this message translates to:
  /// **'Make {name} an owner'**
  String stepUpPurposeAddOwner(String name);

  /// Purpose of the step-up of revokeOwner.
  ///
  /// In en, this message translates to:
  /// **'Revoke the owner {name}'**
  String stepUpPurposeRevokeOwner(String name);

  /// Purpose of the step-up of createGroup.
  ///
  /// In en, this message translates to:
  /// **'Create the group {name}'**
  String stepUpPurposeCreateGroup(String name);

  /// Purpose of the step-up of updateGroup.
  ///
  /// In en, this message translates to:
  /// **'Rename the group {name}'**
  String stepUpPurposeRenameGroup(String name);

  /// Purpose of the step-up of deleteGroup.
  ///
  /// In en, this message translates to:
  /// **'Delete the group {name}'**
  String stepUpPurposeDeleteGroup(String name);

  /// Purpose of the step-up of addGroupMember.
  ///
  /// In en, this message translates to:
  /// **'Add {name} to {group}'**
  String stepUpPurposeAddMember(String name, String group);

  /// Purpose of the step-up of removeGroupMember.
  ///
  /// In en, this message translates to:
  /// **'Remove {name} from {group}'**
  String stepUpPurposeRemoveMember(String name, String group);

  /// Purpose of the step-up of approveDevice for the device of another person.
  ///
  /// In en, this message translates to:
  /// **'Approve the device {device} of {person}'**
  String stepUpPurposeApproveDeviceOf(String device, String person);

  /// Purpose of the step-up of revokeDevice that rejects a waiting device.
  ///
  /// In en, this message translates to:
  /// **'Reject the device {device} of {person}'**
  String stepUpPurposeRejectDevice(String device, String person);

  /// Entry of the administration navigation for the audit log (01 section 5.2 A6).
  ///
  /// In en, this message translates to:
  /// **'Audit'**
  String get adminNavAudit;

  /// Title of the audit log screen of the administration (01 5.2 A6).
  ///
  /// In en, this message translates to:
  /// **'Audit log'**
  String get auditTitle;

  /// Label of the action filter of the audit log; a list of the loaded action IDs or a typed action ID.
  ///
  /// In en, this message translates to:
  /// **'Action'**
  String get auditFilterAction;

  /// First choice of the person filter of the audit log: no filter.
  ///
  /// In en, this message translates to:
  /// **'All people'**
  String get auditFilterActorAll;

  /// First choice of the result filter of the audit log: no filter.
  ///
  /// In en, this message translates to:
  /// **'All results'**
  String get auditResultAll;

  /// Result of an audit entry whose decision was allow.
  ///
  /// In en, this message translates to:
  /// **'allowed'**
  String get auditResultAllow;

  /// Result of an audit entry whose decision was deny.
  ///
  /// In en, this message translates to:
  /// **'denied'**
  String get auditResultDeny;

  /// Result of an audit entry whose decision was error.
  ///
  /// In en, this message translates to:
  /// **'error'**
  String get auditResultError;

  /// Label of the resource kind filter of the audit log, for example space or grant.
  ///
  /// In en, this message translates to:
  /// **'Resource kind'**
  String get auditFilterResourceKind;

  /// Label of the resource ID filter of the audit log.
  ///
  /// In en, this message translates to:
  /// **'Resource ID'**
  String get auditFilterResourceId;

  /// Label of the first day of the time range filter of the audit log, read in the person's time zone.
  ///
  /// In en, this message translates to:
  /// **'From (YYYY-MM-DD)'**
  String get auditFilterFrom;

  /// Label of the last day (inclusive) of the time range filter of the audit log.
  ///
  /// In en, this message translates to:
  /// **'To (YYYY-MM-DD)'**
  String get auditFilterTo;

  /// Button that loads the audit log with the typed filters.
  ///
  /// In en, this message translates to:
  /// **'Apply filters'**
  String get auditFilterApply;

  /// Error under a day field of the audit log filter that holds no valid date.
  ///
  /// In en, this message translates to:
  /// **'Enter a date as YYYY-MM-DD.'**
  String get auditFilterInvalidDate;

  /// Column header of the audit log: when it happened.
  ///
  /// In en, this message translates to:
  /// **'Time'**
  String get auditColTime;

  /// Column header of the audit log: the acting person or agent.
  ///
  /// In en, this message translates to:
  /// **'Who'**
  String get auditColWho;

  /// Column header of the audit log: allowed, denied or error.
  ///
  /// In en, this message translates to:
  /// **'Result'**
  String get auditColResult;

  /// Column header of the audit log: the resource of the action.
  ///
  /// In en, this message translates to:
  /// **'Resource'**
  String get auditColResource;

  /// Column header of the audit log: the deny reason or the stated reason.
  ///
  /// In en, this message translates to:
  /// **'Reason'**
  String get auditColReason;

  /// Shown when the audit log has no entry for the filters.
  ///
  /// In en, this message translates to:
  /// **'No entries match these filters.'**
  String get auditEmpty;

  /// Shown when the person cancelled the step-up that reading the audit log needs.
  ///
  /// In en, this message translates to:
  /// **'Reading the audit log needs your confirmation.'**
  String get auditStepUpNeeded;

  /// Button that checks the hash chain of the audit log (verifyAuditChain).
  ///
  /// In en, this message translates to:
  /// **'Check chain'**
  String get auditCheckChain;

  /// Result of an intact hash chain check (01 5.2 A6, ADR 0021).
  ///
  /// In en, this message translates to:
  /// **'Chain checked up to no. {seq}: unchanged.'**
  String auditChainOk(int seq);

  /// Result of a hash chain check that found the first broken entry; reason is one of the auditChainReason texts.
  ///
  /// In en, this message translates to:
  /// **'Chain broken at no. {seq}: {reason}'**
  String auditChainBroken(int seq, String reason);

  /// Range and checkpoints of a hash chain check.
  ///
  /// In en, this message translates to:
  /// **'Checked no. {from} to {to}; checkpoints compared: {count}.'**
  String auditChainRange(int from, int to, int count);

  /// Failure reason hash_mismatch of a chain check.
  ///
  /// In en, this message translates to:
  /// **'the entry does not match its hash.'**
  String get auditChainReasonHash;

  /// Failure reason link_mismatch of a chain check.
  ///
  /// In en, this message translates to:
  /// **'the entry does not link the entry before.'**
  String get auditChainReasonLink;

  /// Failure reason seq_gap of a chain check.
  ///
  /// In en, this message translates to:
  /// **'an entry is missing.'**
  String get auditChainReasonGap;

  /// Failure reason checkpoint_mismatch of a chain check.
  ///
  /// In en, this message translates to:
  /// **'a checkpoint does not match.'**
  String get auditChainReasonCheckpoint;

  /// Failure reason checkpoint_invalid of a chain check.
  ///
  /// In en, this message translates to:
  /// **'a checkpoint is invalid.'**
  String get auditChainReasonCheckpointInvalid;

  /// Failure reason of a chain check that this app does not know.
  ///
  /// In en, this message translates to:
  /// **'reason {code}.'**
  String auditChainReasonOther(String code);

  /// Warning checkpoint_sink_not_configured of a chain check.
  ///
  /// In en, this message translates to:
  /// **'Warning: no checkpoint target is configured, so checkpoints exist only in the database.'**
  String get auditChainWarningNoSink;

  /// A warning of a chain check that this app does not know, by its code.
  ///
  /// In en, this message translates to:
  /// **'Warning: {code}'**
  String auditChainWarning(String code);

  /// Button that saves the audit log of the time range as NDJSON with hashes.
  ///
  /// In en, this message translates to:
  /// **'Export'**
  String get auditExport;

  /// Button that saves the audit log of the time range as a CSV converted on the device.
  ///
  /// In en, this message translates to:
  /// **'Export as CSV'**
  String get auditExportCsv;

  /// Shown when the person exports without a time range in the filter.
  ///
  /// In en, this message translates to:
  /// **'Choose a time range (from and to) to export.'**
  String get auditExportNeedsRange;

  /// Notice after the audit export was written to the chosen file.
  ///
  /// In en, this message translates to:
  /// **'Export saved.'**
  String get auditExportDone;

  /// Notice when the audit export stream ended with an error after the file was opened.
  ///
  /// In en, this message translates to:
  /// **'The export broke off; the file is incomplete.'**
  String get auditExportFailed;

  /// Purpose of the step-up dialog when reading the audit log (05 audit.read).
  ///
  /// In en, this message translates to:
  /// **'Read the audit log'**
  String get stepUpPurposeAuditRead;

  /// Purpose of the step-up dialog when checking the audit chain (05 audit.verify).
  ///
  /// In en, this message translates to:
  /// **'Check the hash chain of the audit log'**
  String get stepUpPurposeAuditVerify;

  /// Purpose of the step-up dialog when exporting the audit log (05 audit.read).
  ///
  /// In en, this message translates to:
  /// **'Export the audit log'**
  String get stepUpPurposeAuditExport;

  /// Title of the details of one audit entry.
  ///
  /// In en, this message translates to:
  /// **'Entry no. {seq}'**
  String auditDetailTitle(int seq);

  /// Label of the correlation ID in the details of an audit entry.
  ///
  /// In en, this message translates to:
  /// **'Correlation ID'**
  String get auditDetailCorrelation;

  /// Label of the deny reason in the details of an audit entry.
  ///
  /// In en, this message translates to:
  /// **'Deny reason'**
  String get auditDetailDenyReason;

  /// Label of the details object the server returns for an audit entry.
  ///
  /// In en, this message translates to:
  /// **'Details'**
  String get auditDetailDetails;

  /// Label of the hash of an audit entry.
  ///
  /// In en, this message translates to:
  /// **'Hash'**
  String get auditDetailHash;

  /// Label of the hash of the entry before, in the details of an audit entry.
  ///
  /// In en, this message translates to:
  /// **'Previous hash'**
  String get auditDetailPrevHash;

  /// Button that closes the details of an audit entry.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get auditDetailClose;

  /// Label of the action audit.read in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Read the audit log'**
  String get auditActionAuditRead;

  /// Label of the action audit.verify in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Check the audit chain'**
  String get auditActionAuditVerify;

  /// Label of the action grant.create in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Create a grant'**
  String get auditActionGrantCreate;

  /// Label of the action grant.revoke in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Revoke a grant'**
  String get auditActionGrantRevoke;

  /// Label of the action owner.add in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Add an owner'**
  String get auditActionOwnerAdd;

  /// Label of the action owner.remove in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Remove an owner'**
  String get auditActionOwnerRemove;

  /// Label of the action user.create in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Create a person'**
  String get auditActionUserCreate;

  /// Label of the action user.disable in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Disable a person'**
  String get auditActionUserDisable;

  /// Label of the action user.enable in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Enable a person'**
  String get auditActionUserEnable;

  /// Label of the action device.approve in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Approve a device'**
  String get auditActionDeviceApprove;

  /// Label of the action device.revoke in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Revoke a device'**
  String get auditActionDeviceRevoke;

  /// Label of the action group.create in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Create a group'**
  String get auditActionGroupCreate;

  /// Label of the action auth.step_up in the audit log.
  ///
  /// In en, this message translates to:
  /// **'Confirm with step-up'**
  String get auditActionStepUp;

  /// Entry of the grants in the administration navigation (01 section 5.2 A1).
  ///
  /// In en, this message translates to:
  /// **'Access grants'**
  String get adminNavGrants;

  /// Title of the grants screen (01 section 5.2 A2, A3).
  ///
  /// In en, this message translates to:
  /// **'Access grants'**
  String get adminGrantsTitle;

  /// Label of the target picker of the grants screen.
  ///
  /// In en, this message translates to:
  /// **'Target'**
  String get adminGrantsTarget;

  /// Hint of the target picker before a target is chosen.
  ///
  /// In en, this message translates to:
  /// **'Choose a target'**
  String get adminGrantsPickTarget;

  /// The instance as the target of an access grant (07 section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Instance'**
  String get adminTargetInstance;

  /// A space as the target of an access grant.
  ///
  /// In en, this message translates to:
  /// **'Space {name}'**
  String adminTargetSpace(String name);

  /// A channel as the target of an access grant, with its space.
  ///
  /// In en, this message translates to:
  /// **'Channel {path}'**
  String adminTargetChannel(String path);

  /// Button and title of the grant wizard (01 section 5.2 A2).
  ///
  /// In en, this message translates to:
  /// **'Create access grant'**
  String get adminGrantsCreate;

  /// Button and title of the access view of a space or channel (01 section 5.2 A3).
  ///
  /// In en, this message translates to:
  /// **'Who has access?'**
  String get adminGrantsWhoHasAccess;

  /// Empty grants list of a target.
  ///
  /// In en, this message translates to:
  /// **'No access grant on this target.'**
  String get adminGrantsEmpty;

  /// End of an access grant or source in a list.
  ///
  /// In en, this message translates to:
  /// **'until {day}'**
  String adminGrantsUntil(String day);

  /// Reason of an access grant in a list.
  ///
  /// In en, this message translates to:
  /// **'Reason: {reason}'**
  String adminGrantsReason(String reason);

  /// Notice after an access grant was created.
  ///
  /// In en, this message translates to:
  /// **'{name} received the access grant.'**
  String adminGrantCreated(String name);

  /// Notice after an access grant was revoked.
  ///
  /// In en, this message translates to:
  /// **'The access grant for {name} was revoked.'**
  String adminGrantRevoked(String name);

  /// Level read of an access grant (07 section 2.2).
  ///
  /// In en, this message translates to:
  /// **'Read'**
  String get adminLevelRead;

  /// Level write of an access grant (07 section 2.2).
  ///
  /// In en, this message translates to:
  /// **'Write'**
  String get adminLevelWrite;

  /// Level trigger of an access grant (01 section 9 term table).
  ///
  /// In en, this message translates to:
  /// **'Trigger agents'**
  String get adminLevelTrigger;

  /// Level none: no level before or after a change, or access only via an agent.
  ///
  /// In en, this message translates to:
  /// **'No level'**
  String get adminLevelNone;

  /// Capability upload (07 section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Upload files'**
  String get adminCapabilityUpload;

  /// Capability export (07 section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Export'**
  String get adminCapabilityExport;

  /// Capability manage (07 section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Manage'**
  String get adminCapabilityManage;

  /// Capability terminal on a host (07 section 2.12).
  ///
  /// In en, this message translates to:
  /// **'Terminal'**
  String get adminCapabilityTerminal;

  /// Progress of the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Step {step} of {total}'**
  String adminWizardStep(int step, int total);

  /// Step 1 of the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Who'**
  String get adminWizardWho;

  /// Step 2 of the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Where'**
  String get adminWizardWhere;

  /// Step 3 of the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Level and capabilities'**
  String get adminWizardLevel;

  /// Subject kind person in the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Person'**
  String get adminWizardKindPerson;

  /// Subject kind group in the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Group'**
  String get adminWizardKindGroup;

  /// Subject kind agent in the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Agent'**
  String get adminWizardKindAgent;

  /// Search field of the subject step.
  ///
  /// In en, this message translates to:
  /// **'Search by name'**
  String get adminWizardSearch;

  /// Empty result of a wizard list.
  ///
  /// In en, this message translates to:
  /// **'Nothing matches.'**
  String get adminWizardNoMatch;

  /// Label of the capability boxes.
  ///
  /// In en, this message translates to:
  /// **'Capabilities'**
  String get adminWizardCapabilities;

  /// Field of the expiry date of an access grant.
  ///
  /// In en, this message translates to:
  /// **'Valid until (YYYY-MM-DD, optional)'**
  String get adminWizardUntil;

  /// Required reason of an access grant or a revocation (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'Reason (required, stored in the audit log)'**
  String get adminReasonRequired;

  /// Previous step of the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Back'**
  String get adminWizardBack;

  /// Next step of the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Next'**
  String get adminWizardNext;

  /// Confirms the access grant after the preview; the step-up follows (07 section 9.3).
  ///
  /// In en, this message translates to:
  /// **'Confirm'**
  String get adminWizardConfirm;

  /// Title of the preview panel (07 section 9.3).
  ///
  /// In en, this message translates to:
  /// **'Plain-language preview'**
  String get adminPreviewTitle;

  /// Shown while the preview loads.
  ///
  /// In en, this message translates to:
  /// **'Checking the effect'**
  String get adminPreviewLoading;

  /// Title of the effects per principal in the preview.
  ///
  /// In en, this message translates to:
  /// **'Who is affected'**
  String get adminPreviewEffects;

  /// Effect label: a principal gains a level.
  ///
  /// In en, this message translates to:
  /// **'Gets {level}'**
  String adminPreviewAdded(String level);

  /// Effect label: a principal loses its level.
  ///
  /// In en, this message translates to:
  /// **'Loses {level}'**
  String adminPreviewRemoved(String level);

  /// Effect label: the level of a principal changes.
  ///
  /// In en, this message translates to:
  /// **'{before} becomes {after}'**
  String adminPreviewChanged(String before, String after);

  /// Effect label: capabilities that are added.
  ///
  /// In en, this message translates to:
  /// **'Adds: {list}'**
  String adminPreviewCapabilities(String list);

  /// Title of an invariant violation in the preview (07 section 9.3).
  ///
  /// In en, this message translates to:
  /// **'Would be rejected'**
  String get adminPreviewRejected;

  /// The agent an invariant violation names.
  ///
  /// In en, this message translates to:
  /// **'Agent: {agent}'**
  String adminPreviewAgent(String agent);

  /// The principals an invariant violation names.
  ///
  /// In en, this message translates to:
  /// **'Affected: {people}'**
  String adminPreviewAffected(String people);

  /// Guidance for an I1 violation in the preview (01 section 5.2 A2; 10 section 11).
  ///
  /// In en, this message translates to:
  /// **'Switch the agent to organization first.'**
  String get adminPreviewI1Hint;

  /// A delegation limit refuses the access grant (07 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'The access grant would be refused: {reason}'**
  String adminPreviewDenied(String reason);

  /// Title of the hints in the preview.
  ///
  /// In en, this message translates to:
  /// **'Hints'**
  String get adminPreviewHints;

  /// Hint read_horizon of the preview (07 section 6).
  ///
  /// In en, this message translates to:
  /// **'Read horizon (H1): {person} can learn content of {channel} through agent {agent}'**
  String adminHintReadHorizon(String person, String channel, String agent);

  /// Hint trigger_blocked_tool_class of the preview (07 section 5).
  ///
  /// In en, this message translates to:
  /// **'Triggering blocked: {agent} cannot be triggered by this principal (SR2, SR3)'**
  String adminHintTriggerBlocked(String agent);

  /// Hint group_reach of the preview.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{This access grant reaches 1 person} other{This access grant reaches {count} people}}'**
  String adminHintGroupReach(int count);

  /// Hint expiry_missing of the preview (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'No expiry date: the access grant for a person outside of groups has no end'**
  String get adminHintExpiryMissing;

  /// Hint terminal_not_effective of the preview (07 section 2.12).
  ///
  /// In en, this message translates to:
  /// **'The access grant does not take effect on this host (SR10, SR12)'**
  String get adminHintTerminalNotEffective;

  /// A hint code this app does not know.
  ///
  /// In en, this message translates to:
  /// **'Hint {code}'**
  String adminHintUnknown(String code);

  /// Title of the revoke confirmation.
  ///
  /// In en, this message translates to:
  /// **'Revoke the access grant for {name}?'**
  String adminRevokeGrantTitle(String name);

  /// Body of the revoke confirmation (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'Revoking takes effect immediately: the access ends now.'**
  String get adminRevokeGrantBody;

  /// Title of the access view of a target.
  ///
  /// In en, this message translates to:
  /// **'Who has access? {target}'**
  String adminAccessTitle(String target);

  /// Empty access view.
  ///
  /// In en, this message translates to:
  /// **'Nobody has access.'**
  String get adminAccessEmpty;

  /// Source direct (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Direct access grant'**
  String get adminSourceDirect;

  /// Source group (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Via group {group}'**
  String adminSourceGroup(String group);

  /// Source inherited from a space (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Inherited from space {space}'**
  String adminSourceInheritedSpace(String space);

  /// Source inherited from the instance (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Inherited from the instance'**
  String get adminSourceInheritedInstance;

  /// Source dm_participant (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Participant'**
  String get adminSourceDm;

  /// Source operator (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Operator of this channel'**
  String get adminSourceOperator;

  /// Source via_agent with hint H1 (07 sections 6 and 9.1).
  ///
  /// In en, this message translates to:
  /// **'Can learn content here through agent {agent} (H1)'**
  String adminSourceViaAgent(String agent);

  /// Source guard_blocked with its rule (07 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Trigger agents, but triggering is blocked ({rule})'**
  String adminSourceGuardBlocked(String rule);

  /// The agent of a guard_blocked source.
  ///
  /// In en, this message translates to:
  /// **'Agent {agent}'**
  String adminSourceGuardAgent(String agent);

  /// A source kind this app does not know.
  ///
  /// In en, this message translates to:
  /// **'Other source'**
  String get adminSourceUnknown;

  /// End of the group membership of a source.
  ///
  /// In en, this message translates to:
  /// **'member until {day}'**
  String adminSourceMemberUntil(String day);

  /// Picker of the view as a person (01 section 5.2 A4).
  ///
  /// In en, this message translates to:
  /// **'View as person'**
  String get adminAccessViewAs;

  /// Frame of the view as a person (01 section 5.2 A4).
  ///
  /// In en, this message translates to:
  /// **'Viewing as {name} (read-only). Nothing is sent. Logged.'**
  String adminViewAsBanner(String name);

  /// End of the view as a person (07 section 9.2).
  ///
  /// In en, this message translates to:
  /// **'The view ends {when}'**
  String adminViewAsUntil(String when);

  /// Leaves the view as a person.
  ///
  /// In en, this message translates to:
  /// **'Exit'**
  String get adminViewAsExit;

  /// Title of the spaces the person sees.
  ///
  /// In en, this message translates to:
  /// **'Space list'**
  String get adminViewAsSpaces;

  /// Title of the channels the person sees.
  ///
  /// In en, this message translates to:
  /// **'Channel list'**
  String get adminViewAsChannels;

  /// The person sees no space or channel.
  ///
  /// In en, this message translates to:
  /// **'Nothing visible.'**
  String get adminViewAsNothing;

  /// Purpose of the step-up of createGrant.
  ///
  /// In en, this message translates to:
  /// **'Create the access grant for {name} on {target}'**
  String stepUpPurposeCreateGrant(String name, String target);

  /// Purpose of the step-up of revokeGrant.
  ///
  /// In en, this message translates to:
  /// **'Revoke the access grant for {name}'**
  String stepUpPurposeRevokeGrant(String name);

  /// Purpose of the step-up of viewAsUser.
  ///
  /// In en, this message translates to:
  /// **'View as {name}'**
  String stepUpPurposeViewAs(String name);

  /// Text of forbidden with reason delegation_exceeds_own_level (07 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'You cannot give more than your own level'**
  String get problemForbiddenDelegationLevel;

  /// Text of forbidden with reason delegation_exceeds_own_capabilities (07 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'You cannot give capabilities you do not have'**
  String get problemForbiddenDelegationCapabilities;

  /// Text of forbidden with reason subject_kind_not_allowed (07 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'Not allowed for agents or apps'**
  String get problemForbiddenSubjectKind;

  /// Navigation entry of screen A8 (01 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Hosts'**
  String get adminNavHosts;

  /// Button of the hosts list that opens the pairing wizard.
  ///
  /// In en, this message translates to:
  /// **'Add host'**
  String get hostsAdd;

  /// Hosts list without entries.
  ///
  /// In en, this message translates to:
  /// **'No hosts yet.'**
  String get hostsEmpty;

  /// Host without an assignee: only owners get a terminal (SR10).
  ///
  /// In en, this message translates to:
  /// **'Owners only'**
  String get hostOwnersOnly;

  /// Host state pairing (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Pairing'**
  String get hostStatePairing;

  /// Host state confirming (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Waiting for confirmation'**
  String get hostStateConfirming;

  /// Host state active (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'active'**
  String get hostStateActive;

  /// Host state disabled by an owner (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'disabled'**
  String get hostStateDisabled;

  /// Host disabled with duplicate_connection (24 section 3.4).
  ///
  /// In en, this message translates to:
  /// **'disabled: a second connection used the same keys, only a new pairing helps'**
  String get hostStateDuplicate;

  /// Host state retired (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'retired'**
  String get hostStateRetired;

  /// A host state this build does not know.
  ///
  /// In en, this message translates to:
  /// **'unknown'**
  String get hostStateUnknown;

  /// The control connection of the host is open.
  ///
  /// In en, this message translates to:
  /// **'connected'**
  String get hostConnected;

  /// The control connection of the host is closed.
  ///
  /// In en, this message translates to:
  /// **'disconnected'**
  String get hostDisconnected;

  /// When the host service was last seen.
  ///
  /// In en, this message translates to:
  /// **'last seen {when}'**
  String hostLastSeen(String when);

  /// Back button of the host page.
  ///
  /// In en, this message translates to:
  /// **'Back to the hosts'**
  String get hostBack;

  /// Field label of the host page (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Assignee'**
  String get hostFieldAssignee;

  /// Field label of the host page (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Platform'**
  String get hostFieldPlatform;

  /// Field label of the host page (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Connection'**
  String get hostFieldConnection;

  /// Field label of the host page (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Fingerprint'**
  String get hostFieldFingerprint;

  /// Field label of the host page (01 A8).
  ///
  /// In en, this message translates to:
  /// **'LLM login'**
  String get hostFieldLlmLogin;

  /// Field label of the host page (01 A8; 24 section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Server host'**
  String get hostFieldServerHost;

  /// Field label of the check results (01 A8; 24 section 2.2).
  ///
  /// In en, this message translates to:
  /// **'Checks'**
  String get hostFieldChecks;

  /// Field label of the host page (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Profiles'**
  String get hostFieldProfiles;

  /// Field label of the grants on the host (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Access grants'**
  String get hostFieldGrants;

  /// Field label of the session users (01 A8; 24 section 2.2).
  ///
  /// In en, this message translates to:
  /// **'Session users'**
  String get hostFieldSessionUsers;

  /// Field label of the count of open sessions.
  ///
  /// In en, this message translates to:
  /// **'Terminal sessions'**
  String get hostFieldSessions;

  /// Platform and version of the host service.
  ///
  /// In en, this message translates to:
  /// **'{platform}, version {version}'**
  String hostPlatformValue(String platform, String version);

  /// A value the host did not report yet.
  ///
  /// In en, this message translates to:
  /// **'Not reported yet'**
  String get hostNotReported;

  /// Fingerprint in groups and the key generation (24 section 4.3).
  ///
  /// In en, this message translates to:
  /// **'{value}   Key generation {generation}'**
  String hostFingerprintValue(String value, int generation);

  /// The host has no keys (before pairing, after re-pairing).
  ///
  /// In en, this message translates to:
  /// **'No keys'**
  String get hostNoKeys;

  /// LLM login none (24 section 7.2).
  ///
  /// In en, this message translates to:
  /// **'None'**
  String get hostLlmNone;

  /// LLM login personal (24 section 7.2).
  ///
  /// In en, this message translates to:
  /// **'Personal'**
  String get hostLlmPersonal;

  /// LLM login personal with its holder.
  ///
  /// In en, this message translates to:
  /// **'Personal of {name}'**
  String hostLlmPersonalOf(String name);

  /// LLM login organization (24 section 7.2).
  ///
  /// In en, this message translates to:
  /// **'Organization'**
  String get hostLlmOrganization;

  /// Explanation of I6 under the choice personal (24 section 7.3).
  ///
  /// In en, this message translates to:
  /// **'Personal: only this person may open a terminal here, not even other owners.'**
  String get hostLlmI6Explained;

  /// Picker of the holder of a personal LLM login.
  ///
  /// In en, this message translates to:
  /// **'Holder of the login'**
  String get hostLlmHolder;

  /// Title of the dialog that changes the LLM login.
  ///
  /// In en, this message translates to:
  /// **'LLM login on {name}'**
  String hostLlmTitle(String name);

  /// Body of the dialog that changes the LLM login (24 section 7.3 point 3).
  ///
  /// In en, this message translates to:
  /// **'A change to Personal or a new holder ends the terminal sessions of other people on this host.'**
  String get hostLlmBody;

  /// Notice after the LLM login changed.
  ///
  /// In en, this message translates to:
  /// **'The LLM login of {name} is changed.'**
  String hostLlmChanged(String name);

  /// Mandatory question of the confirmation (24 section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Is this host the server of the instance?'**
  String get hostServerHostQuestion;

  /// Answer yes.
  ///
  /// In en, this message translates to:
  /// **'Yes'**
  String get hostYes;

  /// Answer no.
  ///
  /// In en, this message translates to:
  /// **'No'**
  String get hostNo;

  /// The owner answer to the server host question.
  ///
  /// In en, this message translates to:
  /// **'Declared: {answer}'**
  String hostServerHostDeclared(String answer);

  /// What the host service reported (24 section 2.3).
  ///
  /// In en, this message translates to:
  /// **'Report of the host service: {answer}'**
  String hostServerHostReported(String answer);

  /// A passed check, for example HD1 ok.
  ///
  /// In en, this message translates to:
  /// **'{check} ok'**
  String hostDoctorOk(String check);

  /// A failed check with its code.
  ///
  /// In en, this message translates to:
  /// **'{check} failed: {code}'**
  String hostDoctorFailed(String check, String code);

  /// A named session user with the person of its valid assignment (24 section 2.2).
  ///
  /// In en, this message translates to:
  /// **'{osUser} (assigned to {name})'**
  String hostDoctorAssigned(String osUser, String name);

  /// A named former session user with the person of its ended assignment from the audit.
  ///
  /// In en, this message translates to:
  /// **'{osUser} (formerly assigned to {name})'**
  String hostDoctorFormer(String osUser, String name);

  /// A named session user without a known assignment.
  ///
  /// In en, this message translates to:
  /// **'{osUser} (no assignment known)'**
  String hostDoctorUnknown(String osUser);

  /// The host reported no check results yet.
  ///
  /// In en, this message translates to:
  /// **'No check results yet.'**
  String get hostDoctorNone;

  /// The audit of the host was not readable.
  ///
  /// In en, this message translates to:
  /// **'The audit of this host could not be read, so former session users show without their person.'**
  String get hostDoctorAuditFailed;

  /// Work folders of a session user (ruling T13).
  ///
  /// In en, this message translates to:
  /// **'Work folders: {folders}'**
  String hostWorkdirs(String folders);

  /// A session user without work folders.
  ///
  /// In en, this message translates to:
  /// **'Work folders: none'**
  String get hostWorkdirsNone;

  /// A reported name without a valid assignment.
  ///
  /// In en, this message translates to:
  /// **'Reported, not assigned: {osUser}'**
  String hostSessionUsersUnassigned(String osUser);

  /// Note under the session users (07 SR12).
  ///
  /// In en, this message translates to:
  /// **'Without a session user no terminal (SR12).'**
  String get hostSessionUsersSr12;

  /// Note of the session user pickers (24 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Only names that the host reported during pairing or in its last report.'**
  String get hostSessionUsersOnlyReported;

  /// Button that assigns a session user.
  ///
  /// In en, this message translates to:
  /// **'Assign'**
  String get hostSessionUserAssign;

  /// Title of the dialog that assigns a session user.
  ///
  /// In en, this message translates to:
  /// **'Assign a session user'**
  String get hostSessionUserAssignTitle;

  /// Body of the dialog that assigns a session user (ruling T12).
  ///
  /// In en, this message translates to:
  /// **'A name once given to a person stays bound to that person on this host.'**
  String get hostSessionUserAssignBody;

  /// Person picker of a session user row.
  ///
  /// In en, this message translates to:
  /// **'Person'**
  String get hostSessionUserPerson;

  /// Name picker of a session user row.
  ///
  /// In en, this message translates to:
  /// **'Reported name'**
  String get hostSessionUserName;

  /// Notice after an assignment.
  ///
  /// In en, this message translates to:
  /// **'{name} now has the session user {osUser}.'**
  String hostSessionUserAssigned(String name, String osUser);

  /// Button that ends an assignment.
  ///
  /// In en, this message translates to:
  /// **'End'**
  String get hostSessionUserEnd;

  /// Title of the dialog that ends an assignment.
  ///
  /// In en, this message translates to:
  /// **'End the assignment of {name}?'**
  String hostSessionUserEndTitle(String name);

  /// Body of the dialog that ends an assignment (ruling T12).
  ///
  /// In en, this message translates to:
  /// **'{name} can no longer open a terminal on this host. The name {osUser} stays taken on this host and is never given to another person.'**
  String hostSessionUserEndBody(String name, String osUser);

  /// Notice after an assignment ended.
  ///
  /// In en, this message translates to:
  /// **'The assignment of {name} ended; {osUser} stays taken.'**
  String hostSessionUserEnded(String name, String osUser);

  /// Explanation of conflict with session_user_taken (ruling T12).
  ///
  /// In en, this message translates to:
  /// **'This name is taken: a name once given to a person stays bound to that person on this host, also after the assignment ends, and each person has one session user per host.'**
  String get hostSessionUserTaken;

  /// Explanation of validation_failed with session_user_not_on_host (24 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'The host has not reported this name. The host must report the name before it can be assigned.'**
  String get hostSessionUserNotOnHost;

  /// Heading of the session user rows of the confirmation.
  ///
  /// In en, this message translates to:
  /// **'Session user per person'**
  String get hostSessionRowsTitle;

  /// Adds a row of the confirmation.
  ///
  /// In en, this message translates to:
  /// **'Add a session user'**
  String get hostSessionRowsAdd;

  /// Removes a row of the confirmation.
  ///
  /// In en, this message translates to:
  /// **'Remove'**
  String get hostSessionRowRemove;

  /// Button that changes a field of the host page.
  ///
  /// In en, this message translates to:
  /// **'Change'**
  String get hostChange;

  /// Title of the dialog that changes the assignee.
  ///
  /// In en, this message translates to:
  /// **'Assignee of {name}'**
  String hostAssigneeTitle(String name);

  /// Body of the dialog that changes the assignee (07 SR10).
  ///
  /// In en, this message translates to:
  /// **'A person who is not an owner gets a terminal only on the host assigned to them (SR10).'**
  String get hostAssigneeBody;

  /// Notice after the assignee changed.
  ///
  /// In en, this message translates to:
  /// **'The assignee of {name} is changed.'**
  String hostAssigneeChanged(String name);

  /// Opens the grant wizard on this host.
  ///
  /// In en, this message translates to:
  /// **'Grant terminal'**
  String get hostGrantsAdd;

  /// A host without grants.
  ///
  /// In en, this message translates to:
  /// **'No access grants on this host.'**
  String get hostGrantsNone;

  /// Badge of a grant whose preview has the hint terminal_not_effective (07 section 9.3).
  ///
  /// In en, this message translates to:
  /// **'Not effective'**
  String get hostNotEffective;

  /// Count of open terminal sessions on the host.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{No open terminal sessions} =1{1 open terminal session} other{{count} open terminal sessions}}'**
  String hostSessionsCount(int count);

  /// Button that re-pairs the host (24 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Re-pair'**
  String get hostRepair;

  /// Title of the dialog that re-pairs.
  ///
  /// In en, this message translates to:
  /// **'Re-pair {name}?'**
  String hostRepairTitle(String name);

  /// Body of the dialog that re-pairs (24 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'The keys of the host are dropped now, its connections close and its terminal sessions end. A new pairing code follows; the key generation rises with the next pairing.'**
  String get hostRepairBody;

  /// Opens the pairing wizard of a host in pairing or confirming.
  ///
  /// In en, this message translates to:
  /// **'Continue pairing'**
  String get hostPair;

  /// Button that disables the host.
  ///
  /// In en, this message translates to:
  /// **'Disable'**
  String get hostDisable;

  /// Title of the dialog that disables.
  ///
  /// In en, this message translates to:
  /// **'Disable {name}?'**
  String hostDisableTitle(String name);

  /// Body of the dialog that disables (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'All terminal sessions on this host end now. No one can open a terminal here until it is enabled again.'**
  String get hostDisableBody;

  /// Button that enables the host again.
  ///
  /// In en, this message translates to:
  /// **'Enable'**
  String get hostEnable;

  /// Title of the dialog that enables.
  ///
  /// In en, this message translates to:
  /// **'Enable {name}?'**
  String hostEnableTitle(String name);

  /// Body of the dialog that enables.
  ///
  /// In en, this message translates to:
  /// **'Terminal sessions are possible again for the people who may open them.'**
  String get hostEnableBody;

  /// Button that retires the host.
  ///
  /// In en, this message translates to:
  /// **'Retire'**
  String get hostRetire;

  /// Title of the dialog that retires.
  ///
  /// In en, this message translates to:
  /// **'Retire {name}?'**
  String hostRetireTitle(String name);

  /// Body of the dialog that retires (24 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'All terminal sessions end now and the host is retired for good. It cannot be paired or enabled again.'**
  String get hostRetireBody;

  /// Notice after a state change.
  ///
  /// In en, this message translates to:
  /// **'{name} is now {state}.'**
  String hostStateChanged(String name, String state);

  /// Title of the pairing wizard (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Pair host'**
  String get hostPairingTitle;

  /// Name field of a new host.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get hostCreateName;

  /// Assignee picker of a new host.
  ///
  /// In en, this message translates to:
  /// **'Assignee (optional)'**
  String get hostCreateAssignee;

  /// Button of step 1 of the pairing wizard.
  ///
  /// In en, this message translates to:
  /// **'Create and get a code'**
  String get hostCreateSubmit;

  /// Text above the pairing command (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Enter this command on the host as the user of the host service (a one-time code that expires soon):'**
  String get hostPairingCommandIntro;

  /// Remaining time of the pairing code.
  ///
  /// In en, this message translates to:
  /// **'Expires in {time}'**
  String hostPairingExpiresIn(String time);

  /// The pairing code expired.
  ///
  /// In en, this message translates to:
  /// **'The pairing code has expired. Issue a new code.'**
  String get hostPairingExpired;

  /// Issues another pairing code.
  ///
  /// In en, this message translates to:
  /// **'New code'**
  String get hostPairingNewCode;

  /// Next pressed before the host reported its keys.
  ///
  /// In en, this message translates to:
  /// **'The host has not reported yet. Enter the command on the host, then try again.'**
  String get hostPairingNotYet;

  /// Step 2 without a code.
  ///
  /// In en, this message translates to:
  /// **'This host waits for a pairing code. Issue a new code to continue.'**
  String get hostPairingNeedsCode;

  /// Closes the pairing wizard.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get hostPairingClose;

  /// Fingerprint field of step 3 (24 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Type the fingerprint from the host (first {count} characters)'**
  String hostFingerprintLabel(int count);

  /// Advice after fingerprint_mismatch.
  ///
  /// In en, this message translates to:
  /// **'Compare with the output on the host and type it again.'**
  String get hostFingerprintCompare;

  /// Attempts left after fingerprint_mismatch.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 attempt left} other{{count} attempts left}}'**
  String hostAttemptsLeft(int count);

  /// Last failed attempt (24 section 3.2, TM-T19).
  ///
  /// In en, this message translates to:
  /// **'No attempts left: the host is back in pairing and its keys are dropped. Issue a new pairing code.'**
  String get hostFingerprintFallback;

  /// Mandatory question of the confirmation (24 section 7.2).
  ///
  /// In en, this message translates to:
  /// **'LLM login on this host'**
  String get hostLlmQuestion;

  /// Button of step 3 (01 A8).
  ///
  /// In en, this message translates to:
  /// **'Confirm with passkey / device'**
  String get hostConfirmSubmit;

  /// Notice after the confirmation.
  ///
  /// In en, this message translates to:
  /// **'{name} is active.'**
  String hostConfirmed(String name);

  /// Step-up purpose of createHost.
  ///
  /// In en, this message translates to:
  /// **'Create the host {name}'**
  String stepUpPurposeCreateHost(String name);

  /// Step-up purpose of issueHostPairingCode.
  ///
  /// In en, this message translates to:
  /// **'Issue a pairing code for {name}'**
  String stepUpPurposePairingCode(String name);

  /// Step-up purpose of confirmHost.
  ///
  /// In en, this message translates to:
  /// **'Confirm the host {name}'**
  String stepUpPurposeConfirmHost(String name);

  /// Step-up purpose of updateHost.
  ///
  /// In en, this message translates to:
  /// **'Change the host {name}'**
  String stepUpPurposeUpdateHost(String name);

  /// Step-up purpose of setHostLlmCredentials.
  ///
  /// In en, this message translates to:
  /// **'Change the LLM login of {name}'**
  String stepUpPurposeHostLlmLogin(String name);

  /// Step-up purpose of setHostState.
  ///
  /// In en, this message translates to:
  /// **'Change the state of {name}'**
  String stepUpPurposeHostState(String name);

  /// Step-up purpose of listAuditEvents for the former session users.
  ///
  /// In en, this message translates to:
  /// **'Read the audit of {name}'**
  String stepUpPurposeHostAudit(String name);

  /// Label of a host target of a grant.
  ///
  /// In en, this message translates to:
  /// **'Host {name}'**
  String adminTargetHost(String name);

  /// Heading of the host targets in the grant wizard.
  ///
  /// In en, this message translates to:
  /// **'Hosts'**
  String get adminTargetsHosts;

  /// Header of a thread or channel without an operator (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'No agent answers here'**
  String get threadNoOperator;

  /// Header of a thread naming its operator agent (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Operator: {name}'**
  String threadOperator(String name);

  /// Small mark after a thread title that the server set automatically (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'automatic'**
  String get threadTitleAutomatic;

  /// Header button that marks the thread done (01 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Done'**
  String get threadDone;

  /// Header button that opens a done thread again (01 section 6.1).
  ///
  /// In en, this message translates to:
  /// **'Reopen'**
  String get threadReopen;

  /// Header button that opens the dialog to move the thread (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Move'**
  String get threadMove;

  /// Header button that shows the pinned messages of the thread in the session column (01 section 5.1 D1).
  ///
  /// In en, this message translates to:
  /// **'Pin'**
  String get threadPin;

  /// Header button that shows or hides the session column (01 section 5.1 D1).
  ///
  /// In en, this message translates to:
  /// **'Column'**
  String get threadColumn;

  /// Header button and dialog title to change the thread title (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Edit title'**
  String get threadEditTitle;

  /// Label of the title field of the title dialog.
  ///
  /// In en, this message translates to:
  /// **'Title'**
  String get threadTitleLabel;

  /// Save button of the title dialog, the inline editor and the template dialog.
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get threadSave;

  /// Cancel button of the dialogs and the inline editor of the thread view.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get threadCancel;

  /// Line under a message of the main history that started a thread; opens it (01 section 5.1 D2).
  ///
  /// In en, this message translates to:
  /// **'Thread: {title}, {count, plural, =1{1 reply} other{{count} replies}}'**
  String threadStarter(String title, int count);

  /// Row at the top of the history while older messages or a gap load (04 section 4).
  ///
  /// In en, this message translates to:
  /// **'Loading earlier messages'**
  String get threadLoadingOlder;

  /// Row at the end of the history while a gap before newer messages loads (04 section 4).
  ///
  /// In en, this message translates to:
  /// **'Loading missing messages'**
  String get threadLoadingNewer;

  /// History of a thread without messages.
  ///
  /// In en, this message translates to:
  /// **'No messages yet'**
  String get threadEmpty;

  /// Text that replaces an open thread or channel whose access was removed (01 section 6.6).
  ///
  /// In en, this message translates to:
  /// **'No access anymore'**
  String get threadNoAccess;

  /// Heading above the waiting messages kept as text after the access was removed (01 section 6.6).
  ///
  /// In en, this message translates to:
  /// **'Messages that were not sent, to copy:'**
  String get threadNoAccessWaiting;

  /// Action of a message in the main history of a text channel (01 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Start thread'**
  String get messageActionStartThread;

  /// Action of a pinned message (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Unpin'**
  String get messageActionUnpin;

  /// Action of an edited message (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'View versions'**
  String get messageActionVersions;

  /// Action of a message and confirm button of the delete dialog (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get messageActionDelete;

  /// Action of a message; moves the read position before it (04 section 14).
  ///
  /// In en, this message translates to:
  /// **'Mark as unread'**
  String get messageActionMarkUnread;

  /// Action of a message that copies its text.
  ///
  /// In en, this message translates to:
  /// **'Copy text'**
  String get messageActionCopyText;

  /// Action of a message that copies its link.
  ///
  /// In en, this message translates to:
  /// **'Copy link'**
  String get messageActionCopyLink;

  /// Screen reader label of the action menu of a message.
  ///
  /// In en, this message translates to:
  /// **'Actions of the message'**
  String get messageMenuLabel;

  /// Title of the confirmation before a message is deleted (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Delete message?'**
  String get messageDeleteTitle;

  /// Text of the confirmation before a message is deleted (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'The message is replaced by \"Message deleted\" for everyone.'**
  String get messageDeleteBody;

  /// Title of the dialog with the earlier versions of a message (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Versions'**
  String get messageVersionsTitle;

  /// Heading of the current text in the versions dialog.
  ///
  /// In en, this message translates to:
  /// **'Current version'**
  String get messageVersionsCurrent;

  /// Heading of an earlier version in the versions dialog.
  ///
  /// In en, this message translates to:
  /// **'Version {number}'**
  String messageVersionsRevision(int number);

  /// Versions dialog of a message without earlier versions.
  ///
  /// In en, this message translates to:
  /// **'No earlier versions'**
  String get messageVersionsEmpty;

  /// Versions dialog after the request failed.
  ///
  /// In en, this message translates to:
  /// **'The versions could not be loaded: {reason}'**
  String messageVersionsFailed(String reason);

  /// Title of the dialog after an edit met a newer version (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'The message was changed elsewhere'**
  String get messageConflictTitle;

  /// Heading of the local text in the conflict dialog (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Your version'**
  String get messageConflictMine;

  /// Heading of the server text in the conflict dialog (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Current version'**
  String get messageConflictTheirs;

  /// Button of the conflict dialog: send the local text on the current version (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Keep mine'**
  String get messageConflictKeepMine;

  /// Button of the conflict dialog: keep the server text and drop the local edit (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Take theirs'**
  String get messageConflictTakeTheirs;

  /// Label of the inline editor of a message.
  ///
  /// In en, this message translates to:
  /// **'Edit message'**
  String get messageEditLabel;

  /// Title of the reaction picker (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'React'**
  String get messageReactionTitle;

  /// Label of the field of the reaction picker that takes any Unicode character (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Any character'**
  String get messageReactionInput;

  /// Button of the reaction picker that adds the typed character.
  ///
  /// In en, this message translates to:
  /// **'Add'**
  String get messageReactionAdd;

  /// Short note after text or a link was copied.
  ///
  /// In en, this message translates to:
  /// **'Copied'**
  String get messageCopied;

  /// Row of a write of the thread that the server refused for good (04 section 13.2 item 5).
  ///
  /// In en, this message translates to:
  /// **'Not saved: {reason}'**
  String outboxFailedAction(String reason);

  /// Button of a refused edit that opens both versions (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Compare'**
  String get outboxCompare;

  /// Button of a refused edit of a deleted message that copies its text (04 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Copy'**
  String get outboxCopy;

  /// Reason why a message is not sent; the limit comes from the instance (D-W-115).
  ///
  /// In en, this message translates to:
  /// **'The message is too long: at most {limit} bytes'**
  String composerTooLong(int limit);

  /// Note after a draft write reported overwrote_newer (04 section 14).
  ///
  /// In en, this message translates to:
  /// **'This draft replaced a newer version from another device'**
  String get composerDraftOverwritten;

  /// Line above the input while a reply is prepared.
  ///
  /// In en, this message translates to:
  /// **'Reply to {name}'**
  String composerReplyTo(String name);

  /// Button that drops the prepared reply.
  ///
  /// In en, this message translates to:
  /// **'Cancel reply'**
  String get composerCancelReply;

  /// Button that opens the templates menu of the input (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Templates'**
  String get composerTemplates;

  /// Group of the personal templates in the templates menu (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Your templates'**
  String get composerTemplatesPersonal;

  /// Group of the channel templates in the templates menu (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Templates of this channel'**
  String get composerTemplatesChannel;

  /// Templates menu without templates.
  ///
  /// In en, this message translates to:
  /// **'No templates'**
  String get composerTemplatesEmpty;

  /// Entry of the templates menu that saves the current text as a personal template (01 section 3.3).
  ///
  /// In en, this message translates to:
  /// **'Save as template'**
  String get composerTemplatesSave;

  /// Label of the name field when saving a template.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get composerTemplatesName;

  /// Section of the session column with the pinned messages of the thread (01 section 5.1 D1).
  ///
  /// In en, this message translates to:
  /// **'Pinned ({count})'**
  String sessionPinned(int count);

  /// Section "Pinned" of the session column without pinned messages.
  ///
  /// In en, this message translates to:
  /// **'No pinned messages'**
  String get sessionPinnedEmpty;

  /// Session column while no thread is open.
  ///
  /// In en, this message translates to:
  /// **'Open a thread to see its session'**
  String get sessionNoThread;

  /// Move dialog without a valid target (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'No channel with the same operator'**
  String get threadMoveNoTarget;

  /// Label of the reason field of the move dialog (ThreadMove.reason).
  ///
  /// In en, this message translates to:
  /// **'Reason'**
  String get threadMoveReason;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Live'**
  String get consoleNavLive;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Statistics'**
  String get consoleNavStats;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Hosts and terminal'**
  String get consoleNavHosts;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'People'**
  String get consoleNavPeople;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Groups and access grants'**
  String get consoleNavGroups;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Devices'**
  String get consoleNavDevices;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Agents and costs'**
  String get consoleNavAgents;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Apps and webhooks'**
  String get consoleNavApps;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Audit'**
  String get consoleNavAudit;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Import'**
  String get consoleNavImport;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Instance and branding'**
  String get consoleNavInstance;

  /// Web console screen 01 section 5.5: navigation entry.
  ///
  /// In en, this message translates to:
  /// **'Status'**
  String get consoleNavStatus;

  /// Web console screen 01 section 5.5: screen reader label of the navigation rail.
  ///
  /// In en, this message translates to:
  /// **'Console navigation'**
  String get consoleNavigationLabel;

  /// Web console screen 01 section 5.5: title of the console with the product name of the branding.
  ///
  /// In en, this message translates to:
  /// **'{product} console'**
  String consoleTitle(String product);

  /// Web console screen 01 section 5.5: body of a navigation entry whose screen is not built yet.
  ///
  /// In en, this message translates to:
  /// **'This page comes with a later version of the console.'**
  String get consolePlaceholderPage;

  /// Web console screen 01 section 5.5: header line with the signed-in person.
  ///
  /// In en, this message translates to:
  /// **'Signed in: {name}'**
  String consoleSignedInAs(String name);

  /// Web console screen 01 section 5.5: button that ends the console session.
  ///
  /// In en, this message translates to:
  /// **'Log out'**
  String get consoleLogout;

  /// Web console screen 01 section 5.5: a thread when the instance announces no deep link scheme.
  ///
  /// In en, this message translates to:
  /// **'Thread {id}: there is no chat in the browser. Open it in the app.'**
  String consoleThreadInApp(String id);

  /// Web console screen 01 section 5.5: action that copies the ID of a thread.
  ///
  /// In en, this message translates to:
  /// **'Copy thread ID'**
  String get consoleCopyThreadId;

  /// Web console screen 01 section 5.5: confirmation after a copy action.
  ///
  /// In en, this message translates to:
  /// **'Copied'**
  String get consoleCopied;

  /// Web console screen 01 section 5.5: progress while the console loads or sends.
  ///
  /// In en, this message translates to:
  /// **'One moment'**
  String get consoleWorking;

  /// Web console login E2 (01 section 5.6, brief M6-02): title with the product name of the branding.
  ///
  /// In en, this message translates to:
  /// **'Log in to {product}'**
  String consoleLoginTitle(String product);

  /// Web console login E2 (01 section 5.6, brief M6-02): button of a login provider.
  ///
  /// In en, this message translates to:
  /// **'Log in with {provider}'**
  String consoleLoginWithProvider(String provider);

  /// Web console login E2 (01 section 5.6, brief M6-02): divider between the provider buttons and the local login.
  ///
  /// In en, this message translates to:
  /// **'or'**
  String get consoleLoginOr;

  /// Web console login E2 (01 section 5.6, brief M6-02): field of the login name.
  ///
  /// In en, this message translates to:
  /// **'Login name'**
  String get consoleLoginName;

  /// Web console login E2 (01 section 5.6, brief M6-02): button of the passkey login.
  ///
  /// In en, this message translates to:
  /// **'Log in with passkey'**
  String get consoleLoginWithPasskey;

  /// Web console login E2 (01 section 5.6, brief M6-02): button that opens the password login.
  ///
  /// In en, this message translates to:
  /// **'Password and one-time code'**
  String get consoleLoginPasswordAndCode;

  /// Web console login E2 (01 section 5.6, brief M6-02): password field.
  ///
  /// In en, this message translates to:
  /// **'Password'**
  String get consoleLoginPassword;

  /// Web console login E2 (01 section 5.6, brief M6-02): field of the TOTP code.
  ///
  /// In en, this message translates to:
  /// **'One-time code'**
  String get consoleLoginOneTimeCode;

  /// Web console login E2 (01 section 5.6, brief M6-02): button that sends the password login.
  ///
  /// In en, this message translates to:
  /// **'Log in'**
  String get consoleLoginSubmit;

  /// Web console login E2 (01 section 5.6, brief M6-02): when discovery offers no method.
  ///
  /// In en, this message translates to:
  /// **'No login is set up for this instance. Ask the owner to set one up.'**
  String get consoleLoginNoMethod;

  /// Web console login E2 (01 section 5.6, brief M6-02): a refused password login.
  ///
  /// In en, this message translates to:
  /// **'The login failed. Check the login name, password and one-time code and try again.'**
  String get consoleLoginFailed;

  /// Web console login E2 (01 section 5.6, brief M6-02): a refused or cancelled passkey login.
  ///
  /// In en, this message translates to:
  /// **'The passkey was not accepted. Try again or choose another way.'**
  String get consoleLoginPasskeyFailed;

  /// Web console login E2 (01 section 5.6, brief M6-02): an OIDC callback that failed.
  ///
  /// In en, this message translates to:
  /// **'The login service did not confirm the login. Start the login again.'**
  String get consoleLoginOidcFailed;

  /// Web console login E2 (01 section 5.6, brief M6-02): an OIDC callback with access_denied.
  ///
  /// In en, this message translates to:
  /// **'The login was cancelled at the login service. Start it again or choose another way.'**
  String get consoleLoginOidcCancelled;

  /// Web console login E2 (01 section 5.6, brief M6-02): an OIDC callback with flow_expired or without its stored flow.
  ///
  /// In en, this message translates to:
  /// **'The login took too long. Start it again.'**
  String get consoleLoginOidcExpired;

  /// Web console login E2 (01 section 5.6, brief M6-02): a network error or a server error during login.
  ///
  /// In en, this message translates to:
  /// **'The server cannot be reached. Check the connection and try again.'**
  String get consoleLoginUnavailable;

  /// Web console login E2 (01 section 5.6, brief M6-02): progress on the callback page.
  ///
  /// In en, this message translates to:
  /// **'Completing the login'**
  String get consoleCallbackWorking;

  /// Web console step-up after login (08 section 7.5, brief M6-02): title.
  ///
  /// In en, this message translates to:
  /// **'Confirm that it is you'**
  String get consoleStepUpTitle;

  /// Web console step-up after login (08 section 7.5, brief M6-02): explanation below the title.
  ///
  /// In en, this message translates to:
  /// **'The console asks for a fresh confirmation before any administration.'**
  String get consoleStepUpExplanation;

  /// Web console step-up after login (08 section 7.5, brief M6-02): button of the passkey step-up.
  ///
  /// In en, this message translates to:
  /// **'Confirm with passkey'**
  String get consoleStepUpPasskey;

  /// Web console step-up after login (08 section 7.5, brief M6-02): button of the OIDC re-login.
  ///
  /// In en, this message translates to:
  /// **'Log in again with {provider}'**
  String consoleStepUpOidc(String provider);

  /// Web console step-up after login (08 section 7.5, brief M6-02): button that opens the password step-up.
  ///
  /// In en, this message translates to:
  /// **'Confirm with password and one-time code'**
  String get consoleStepUpPassword;

  /// Web console step-up after login (08 section 7.5, brief M6-02): button that sends the password step-up.
  ///
  /// In en, this message translates to:
  /// **'Confirm'**
  String get consoleStepUpConfirm;

  /// Web console step-up after login (08 section 7.5, brief M6-02): a refused confirmation.
  ///
  /// In en, this message translates to:
  /// **'The confirmation failed. Try again or choose another way.'**
  String get consoleStepUpFailed;

  /// Web console step-up after login (08 section 7.5, brief M6-02): a session without step-up methods.
  ///
  /// In en, this message translates to:
  /// **'No way to confirm is set up for you. Ask the owner to set up a passkey for you.'**
  String get consoleStepUpNoMethod;

  /// Web console E7 logged out (01 section 5.6, brief M6-02): title.
  ///
  /// In en, this message translates to:
  /// **'You are logged out.'**
  String get consoleLoggedOutTitle;

  /// Web console E7 logged out (01 section 5.6, brief M6-02): the reason, a problem text.
  ///
  /// In en, this message translates to:
  /// **'Reason: {reason}'**
  String consoleLoggedOutReason(String reason);

  /// Web console E7 logged out (01 section 5.6, brief M6-02): reason after the own logout.
  ///
  /// In en, this message translates to:
  /// **'You logged out of the console.'**
  String get consoleLoggedOutByYou;

  /// Web console E7 logged out (01 section 5.6, brief M6-02): button to E2.
  ///
  /// In en, this message translates to:
  /// **'Log in again'**
  String get consoleLogInAgain;

  /// Web console version check (brief M6-02 Design note 5): the console version differs from the API version of the server.
  ///
  /// In en, this message translates to:
  /// **'The console was updated. Reload the page to continue.'**
  String get consoleUpdatedReload;

  /// Web console version check (brief M6-02 Design note 5): button that reloads the page.
  ///
  /// In en, this message translates to:
  /// **'Reload'**
  String get consoleReload;

  /// Entry Agents of the administration (01 section 5.2 A5).
  ///
  /// In en, this message translates to:
  /// **'Agents'**
  String get adminNavAgents;

  /// Heading of the agent list (A5).
  ///
  /// In en, this message translates to:
  /// **'Agents'**
  String get adminAgentsTitle;

  /// The agent list has no entries.
  ///
  /// In en, this message translates to:
  /// **'No agents.'**
  String get adminAgentsEmpty;

  /// Button of the agent list: switch the backend of several agents (10 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Switch all agents'**
  String get adminAgentsSwitchAll;

  /// Label of a disabled agent in the list.
  ///
  /// In en, this message translates to:
  /// **'Disabled'**
  String get adminAgentDisabled;

  /// Number of server warnings of an agent in the list (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 warning} other{{count} warnings}}'**
  String adminAgentWarningCount(int count);

  /// Button of the agent page back to the list.
  ///
  /// In en, this message translates to:
  /// **'Back to the agents'**
  String get adminAgentBack;

  /// Presence of an agent: heartbeat received.
  ///
  /// In en, this message translates to:
  /// **'Online'**
  String get adminAgentStateOnline;

  /// Presence of an agent: online with problems.
  ///
  /// In en, this message translates to:
  /// **'Degraded'**
  String get adminAgentStateDegraded;

  /// Presence of an agent: no heartbeat within the timeout.
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get adminAgentStateOffline;

  /// Presence of an agent: no heartbeat so far.
  ///
  /// In en, this message translates to:
  /// **'No heartbeat yet'**
  String get adminAgentStateUnknown;

  /// Adapter of an agent on its page (A5).
  ///
  /// In en, this message translates to:
  /// **'Adapter {kind}'**
  String adminAgentAdapter(String kind);

  /// Version of the agent runtime from the heartbeat.
  ///
  /// In en, this message translates to:
  /// **'Runtime version {version}'**
  String adminAgentRuntimeVersion(String version);

  /// Time of the last heartbeat of the agent.
  ///
  /// In en, this message translates to:
  /// **'Last heartbeat {when}'**
  String adminAgentLastHeartbeat(String when);

  /// Label of the configured backend of the agent (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Configured mode'**
  String get adminAgentConfigured;

  /// Label of the backend the adapter reports in its heartbeat (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Reported mode'**
  String get adminAgentReported;

  /// Reported mode of an adapter that reports none because it follows the configured mode per run (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Follows the configured mode per run'**
  String get adminAgentReportedPerRun;

  /// Evidence source of the reported mode (billing_evidence.method, 10 section 13).
  ///
  /// In en, this message translates to:
  /// **'Evidence: {method}'**
  String adminAgentEvidence(String method);

  /// Modes the adapter can prove (billing_modes_supported, 10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Can prove: {modes}'**
  String adminAgentSupported(String modes);

  /// The adapter reported no billing_modes_supported.
  ///
  /// In en, this message translates to:
  /// **'Can prove: not reported'**
  String get adminAgentSupportedNone;

  /// Credential holder of an agent in mode personal (10 section 13).
  ///
  /// In en, this message translates to:
  /// **'Holder: {name}'**
  String adminAgentHolder(String name);

  /// Warning billing_mode_mismatch of the server (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Adapter reports {reported}, {configured} is configured'**
  String adminAgentWarnMismatch(String reported, String configured);

  /// Warning billing_mode_unsupported of the server (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Adapter cannot prove this mode'**
  String get adminAgentWarnUnsupported;

  /// Warning billing_evidence_unknown of the server (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Login type of the runtime unknown'**
  String get adminAgentWarnEvidenceUnknown;

  /// Warning tool_class_mismatch of the server (09 section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Adapter reports a different tool class'**
  String get adminAgentWarnToolClass;

  /// A warning code of the server this build does not know.
  ///
  /// In en, this message translates to:
  /// **'Warning {code}'**
  String adminAgentWarnUnknown(String code);

  /// Consequence of the warnings billing_mode_mismatch and billing_mode_unsupported (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'Invocations fail until the adapter is switched.'**
  String get adminAgentWarnFailUntilSwitched;

  /// Consequence of the warning billing_evidence_unknown (10 section 8).
  ///
  /// In en, this message translates to:
  /// **'All invocations fail.'**
  String get adminAgentWarnFailAll;

  /// Instructions on the agent page while the adapter still reports the old mode (10 sections 9.2, 10 and 11 step 5).
  ///
  /// In en, this message translates to:
  /// **'To switch the adapter, whoever runs the instance changes the login of its runtime to {mode}. The warning disappears with the next heartbeat that reports {mode}.'**
  String adminAgentSwitchHint(String mode);

  /// Heading of the spaces and channels the agent operates (10 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Operates'**
  String get adminAgentTargets;

  /// The agent operates no space and no channel.
  ///
  /// In en, this message translates to:
  /// **'No space and no channel'**
  String get adminAgentTargetsNone;

  /// A space whose default operator is the agent.
  ///
  /// In en, this message translates to:
  /// **'{target}, by default'**
  String adminAgentTargetDefault(String target);

  /// Heading of the backend section of the agent page (10 section 13).
  ///
  /// In en, this message translates to:
  /// **'Backend'**
  String get adminAgentBackend;

  /// Button that opens the switch of one agent (10 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Switch backend'**
  String get adminAgentSwitch;

  /// Button that changes the credential holder of a personal agent.
  ///
  /// In en, this message translates to:
  /// **'Change holder'**
  String get adminAgentHolderChange;

  /// Explanation of the backend switch on the agent page (01 section 5.2 A5).
  ///
  /// In en, this message translates to:
  /// **'Switching needs a reason and confirmation. Personal works only while exactly one person may trigger the agent (I1).'**
  String get adminAgentSwitchExplain;

  /// Title of the switch dialog of one agent.
  ///
  /// In en, this message translates to:
  /// **'Switch {agent}'**
  String adminSwitchTitle(String agent);

  /// What a switch to organization changes (10 sections 9.1 and 13).
  ///
  /// In en, this message translates to:
  /// **'{agent} then uses the organization key: the organization bears the costs, and everyone with Trigger agents may trigger it. The holder is cleared.'**
  String adminSwitchToOrgBody(String agent);

  /// What a switch to personal changes (10 sections 9.1 and 13).
  ///
  /// In en, this message translates to:
  /// **'{agent} then uses the personal subscription of the holder: the holder pays, and only the holder may trigger it.'**
  String adminSwitchToPersonalBody(String agent);

  /// Picker of the credential holder.
  ///
  /// In en, this message translates to:
  /// **'Pick the holder'**
  String get adminSwitchHolderPick;

  /// Confirm button of a switch.
  ///
  /// In en, this message translates to:
  /// **'Switch'**
  String get adminSwitchConfirm;

  /// Notice after a switch of one agent.
  ///
  /// In en, this message translates to:
  /// **'{agent} now uses {mode}'**
  String adminSwitched(String agent, String mode);

  /// Purpose of the step-up of agent.billing_mode.set.
  ///
  /// In en, this message translates to:
  /// **'Switch the backend of {agent}'**
  String stepUpPurposeSwitchAgent(String agent);

  /// Purpose of the step-up of agent.billing_mode.set_all.
  ///
  /// In en, this message translates to:
  /// **'Switch the backend of several agents'**
  String get stepUpPurposeSwitchAll;

  /// Purpose of the step-up of agent.credential_holder.set.
  ///
  /// In en, this message translates to:
  /// **'Change the holder of {agent}'**
  String stepUpPurposeSetHolder(String agent);

  /// Purpose of the step-up of token.create.
  ///
  /// In en, this message translates to:
  /// **'Create a token for {agent}'**
  String stepUpPurposeCreateToken(String agent);

  /// Title of the holder dialog.
  ///
  /// In en, this message translates to:
  /// **'Change the holder of {agent}'**
  String adminHolderTitle(String agent);

  /// What a change of the credential holder does (10 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'{agent} keeps running on Personal, on the subscription of the new holder. Only the new holder may trigger it.'**
  String adminHolderBody(String agent);

  /// Notice after a change of the holder.
  ///
  /// In en, this message translates to:
  /// **'{name} is now the holder of {agent}'**
  String adminHolderChanged(String name, String agent);

  /// Heading of the explanation of an I1 rejection (10 section 5.4).
  ///
  /// In en, this message translates to:
  /// **'Personal is not possible right now'**
  String get adminI1Title;

  /// One agent of an I1 rejection with the other principals the caller may read (10 section 5.4).
  ///
  /// In en, this message translates to:
  /// **'{agent} would use the subscription of {holder}, but these may trigger it too: {names}'**
  String adminI1Agent(String agent, String holder, String names);

  /// One agent of an I1 rejection without a holder in the answer.
  ///
  /// In en, this message translates to:
  /// **'{agent} may also be triggered by: {names}'**
  String adminI1AgentNoHolder(String agent, String names);

  /// Principals of an I1 rejection whose names the caller may not read (10 section 5.4).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 more you cannot see} other{{count} more you cannot see}}'**
  String adminI1Hidden(int count);

  /// The way back to personal is not a guided flow (10 section 11).
  ///
  /// In en, this message translates to:
  /// **'Revoke their access grants, group memberships or owner rights first, or keep Organization.'**
  String get adminI1Way;

  /// Title of the switch of several agents (10 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Switch several agents'**
  String get adminSwitchAllTitle;

  /// What all to organization does (10 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Every agent then runs on the organization key; holders are cleared. Use this when there are doubts about a personal operation.'**
  String get adminSwitchAllOrgBody;

  /// What all to personal does, all or nothing (10 section 9.1, BR-08).
  ///
  /// In en, this message translates to:
  /// **'The checked agents then run on the personal subscription of the holder. If one of them would break I1, nothing changes.'**
  String get adminSwitchAllPersonalBody;

  /// The list of all to personal is proposed from the audit (10 section 9.1).
  ///
  /// In en, this message translates to:
  /// **'Proposed: the agents the last switch of several agents to Organization changed'**
  String get adminSwitchAllProposed;

  /// The audit has no earlier switch of several agents.
  ///
  /// In en, this message translates to:
  /// **'No earlier switch to propose from. Check the agents yourself.'**
  String get adminSwitchAllNoProposal;

  /// Notice after the switch of several agents.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{No agent changed} =1{1 agent changed} other{{count} agents changed}}'**
  String adminSwitchAllDone(int count);

  /// Heading of the tokens of the agent (08 section 12).
  ///
  /// In en, this message translates to:
  /// **'API tokens'**
  String get adminTokensTitle;

  /// The agent has no tokens.
  ///
  /// In en, this message translates to:
  /// **'No tokens.'**
  String get adminTokensEmpty;

  /// Button that creates a token for the agent.
  ///
  /// In en, this message translates to:
  /// **'Create token'**
  String get adminTokenCreate;

  /// Title of the token form.
  ///
  /// In en, this message translates to:
  /// **'Create a token for {agent}'**
  String adminTokenCreateTitle(String agent);

  /// What a new token does (08 section 12).
  ///
  /// In en, this message translates to:
  /// **'The adapter acts as {agent} with this token. Its secret is shown only once.'**
  String adminTokenCreateBody(String agent);

  /// Field: name of the token.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get adminTokenName;

  /// Field: the last day the token is valid (expires_at, required).
  ///
  /// In en, this message translates to:
  /// **'Last day (YYYY-MM-DD)'**
  String get adminTokenLastDay;

  /// The last day of the token is no day.
  ///
  /// In en, this message translates to:
  /// **'Enter a day as YYYY-MM-DD.'**
  String get adminTokenLastDayInvalid;

  /// Field: the JWK thumbprint of the adapter key the token is bound to (08 section 12).
  ///
  /// In en, this message translates to:
  /// **'DPoP key thumbprint of the adapter'**
  String get adminTokenDpop;

  /// Expiry of a token in the list.
  ///
  /// In en, this message translates to:
  /// **'valid until {when}'**
  String adminTokenValidUntil(String when);

  /// Last use of a token in the list.
  ///
  /// In en, this message translates to:
  /// **'last used {when}'**
  String adminTokenLastUsed(String when);

  /// Label of a token bound to a DPoP key.
  ///
  /// In en, this message translates to:
  /// **'DPoP-bound'**
  String get adminTokenBound;

  /// Label of a revoked token in the list.
  ///
  /// In en, this message translates to:
  /// **'Revoked'**
  String get adminTokenRevokedLabel;

  /// Button that revokes a token.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get adminTokenRevoke;

  /// Title of the confirmation before a token is revoked.
  ///
  /// In en, this message translates to:
  /// **'Revoke the token {name}?'**
  String adminTokenRevokeTitle(String name);

  /// What revoking a token does.
  ///
  /// In en, this message translates to:
  /// **'An adapter that uses this token loses its access at once.'**
  String get adminTokenRevokeBody;

  /// Notice after a token was revoked.
  ///
  /// In en, this message translates to:
  /// **'The token {name} is revoked.'**
  String adminTokenRevokedNotice(String name);

  /// Title of the view of a new token secret.
  ///
  /// In en, this message translates to:
  /// **'Token {name}'**
  String adminTokenSecretTitle(String name);

  /// Warning of the one-time token secret (08 section 12).
  ///
  /// In en, this message translates to:
  /// **'This secret is shown only once. Copy it into the adapter\'s secret store now.'**
  String get adminTokenSecretOnce;

  /// Semantics label of the token secret.
  ///
  /// In en, this message translates to:
  /// **'Token secret'**
  String get adminTokenSecretLabel;

  /// Plain-language explanation of the guided flow (10 section 11 step 2).
  ///
  /// In en, this message translates to:
  /// **'This grant allows {person} to trigger the agent {agent}. {agent} uses the personal subscription of {holder}. {agent} must first be switched to Organization; the organization key then bears the costs.'**
  String adminGuidedExplainGrant(String person, String agent, String holder);

  /// Explanation of the guided flow when adding a group member or an owner (10 section 11 step 2).
  ///
  /// In en, this message translates to:
  /// **'This change allows {person} to trigger the agent {agent}. {agent} uses the personal subscription of {holder}. {agent} must first be switched to Organization; the organization key then bears the costs.'**
  String adminGuidedExplainChange(String person, String agent, String holder);

  /// Extra sentence when organization_ready is false (10 section 11 step 2).
  ///
  /// In en, this message translates to:
  /// **'The adapter of {agent} does not report an organization key yet. Invocations fail until it is switched.'**
  String adminGuidedNotReady(String agent);

  /// Button of the guided flow in the grant preview (10 section 11).
  ///
  /// In en, this message translates to:
  /// **'Switch {agent} to Organization and grant'**
  String adminGuidedGrant(String agent);

  /// Button of the guided flow when adding a group member or an owner (10 section 11).
  ///
  /// In en, this message translates to:
  /// **'Switch {agent} to Organization and continue'**
  String adminGuidedContinue(String agent);

  /// Title of the reason dialog of the guided flow (10 section 11 step 3).
  ///
  /// In en, this message translates to:
  /// **'Switch to Organization'**
  String get adminGuidedTitle;

  /// Result of the guided flow (10 section 11 step 5).
  ///
  /// In en, this message translates to:
  /// **'{agent} now uses Organization, and the change is saved.'**
  String adminGuidedDone(String agent);

  /// The adapter has not switched yet after the guided flow (10 section 11 step 5).
  ///
  /// In en, this message translates to:
  /// **'The adapter of {agent} still reports {mode}.'**
  String adminGuidedStillReports(String agent, String mode);

  /// Button that closes the result of the guided flow.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get adminGuidedDoneClose;

  /// Button on a person's detail that starts 'View as person' (01 section 5.2 A1 and A4).
  ///
  /// In en, this message translates to:
  /// **'View as this person'**
  String get adminPersonViewAs;

  /// Remaining time of the stored view in the banner of the console, minutes and seconds (07 section 9.2).
  ///
  /// In en, this message translates to:
  /// **'{time} left'**
  String adminViewAsRemaining(String time);

  /// Button of the banner that ends the stored view (endViewAs, 07 section 9.2).
  ///
  /// In en, this message translates to:
  /// **'End view'**
  String get adminViewAsEnd;

  /// Heading of the structure list of 'View as person' in the console (01 section 5.2 A4).
  ///
  /// In en, this message translates to:
  /// **'Sidebar as {name} sees it'**
  String adminViewAsSidebarOf(String name);

  /// Spaces the owner sees and the viewed person does not, their names joined with commas (01 section 5.2 A4).
  ///
  /// In en, this message translates to:
  /// **'Not visible for {name}: {names}'**
  String adminViewAsNotVisible(String name, String names);

  /// Note under the structure of 'View as person' in the console (ADR 0028; 01 section 5.2 A4).
  ///
  /// In en, this message translates to:
  /// **'The console shows structure and access only, never the content of a channel.'**
  String get adminViewAsStructureOnly;

  /// Shown when the console reloads during a stored view: the structure came only with the start (07 section 9.2).
  ///
  /// In en, this message translates to:
  /// **'The structure appears when a view starts. End this view and start it again to see it.'**
  String get adminViewAsStructureLater;

  /// Second line of the step-up of an action in the console when the person has a passkey (08 section 6.2).
  ///
  /// In en, this message translates to:
  /// **'Next, your browser asks for your passkey.'**
  String get consoleStepUpPasskeyNext;

  /// Step-up of an action in the console when only the OIDC login is available: the tab leaves, so the change is not repeated (08 section 6.2).
  ///
  /// In en, this message translates to:
  /// **'This change needs a new login at your login service. Confirm there, then make the change again.'**
  String get consoleStepUpOidcAction;

  /// Connection text in the header of the terminal view (components.md 12).
  ///
  /// In en, this message translates to:
  /// **'Connected'**
  String get terminalHeaderConnected;

  /// Connection text while the terminal stream and handshake run.
  ///
  /// In en, this message translates to:
  /// **'Connecting'**
  String get terminalHeaderConnecting;

  /// Connection text of the header when the terminal stream is closed.
  ///
  /// In en, this message translates to:
  /// **'Not connected'**
  String get terminalHeaderOffline;

  /// Start of the host key fingerprint in the terminal header (screen D11).
  ///
  /// In en, this message translates to:
  /// **'Fp {value}'**
  String terminalHeaderFingerprint(String value);

  /// Screen reader label of the fingerprint in the terminal header.
  ///
  /// In en, this message translates to:
  /// **'Fingerprint of the host: {fingerprint}'**
  String terminalHeaderFingerprintLabel(String fingerprint);

  /// Remaining time until terminal.max_session_duration in the terminal header (screen D11).
  ///
  /// In en, this message translates to:
  /// **'Left {time}'**
  String terminalHeaderRemaining(String time);

  /// Badge of the LLM login of a host with llm_credentials none (24 section 12).
  ///
  /// In en, this message translates to:
  /// **'No LLM login'**
  String get terminalLlmLoginNone;

  /// Label of one terminal session tab: profile and running number.
  ///
  /// In en, this message translates to:
  /// **'{profile} {number}'**
  String terminalTabLabel(String profile, int number);

  /// Button that detaches the terminal session; it keeps running on the host.
  ///
  /// In en, this message translates to:
  /// **'Detach'**
  String get terminalDetach;

  /// Button that ends the terminal session.
  ///
  /// In en, this message translates to:
  /// **'End'**
  String get terminalEnd;

  /// Button that attaches to a detached terminal session again.
  ///
  /// In en, this message translates to:
  /// **'Reattach'**
  String get terminalReattach;

  /// Banner while the terminal session opens.
  ///
  /// In en, this message translates to:
  /// **'Opening the encrypted connection to the host'**
  String get terminalStateConnecting;

  /// Banner of a terminal session the person detached.
  ///
  /// In en, this message translates to:
  /// **'Detached. The terminal session keeps going on the host.'**
  String get terminalStateDetached;

  /// Banner of a detached terminal session after the stream broke.
  ///
  /// In en, this message translates to:
  /// **'The connection broke. The terminal session keeps going on the host.'**
  String get terminalStateDetachedLost;

  /// Banner after close code 4102.
  ///
  /// In en, this message translates to:
  /// **'The host is not connected. Try to reattach later.'**
  String get terminalStateDetachedHostUnavailable;

  /// Banner after close code 4101; reattaching asks for a new step-up.
  ///
  /// In en, this message translates to:
  /// **'The session token expired or was already used. Reattach confirms again.'**
  String get terminalStateDetachedTokenStale;

  /// Banner after close code 4103; no automatic new attempt.
  ///
  /// In en, this message translates to:
  /// **'Another device attached to this terminal session.'**
  String get terminalStateDetachedReplaced;

  /// Banner after close codes 1001 and 4013.
  ///
  /// In en, this message translates to:
  /// **'The server restarted. The terminal session keeps going on the host.'**
  String get terminalStateDetachedServer;

  /// Banner after close code 4014.
  ///
  /// In en, this message translates to:
  /// **'The connection could not keep up. The terminal session keeps going on the host.'**
  String get terminalStateDetachedSlow;

  /// Banner after close code 4004.
  ///
  /// In en, this message translates to:
  /// **'The login of the connection failed. Sign in again, then reattach.'**
  String get terminalStateDetachedAuth;

  /// Banner after close code 4008.
  ///
  /// In en, this message translates to:
  /// **'Too many attempts. Wait a moment, then reattach.'**
  String get terminalStateDetachedRateLimited;

  /// Banner after the frame exit; the tab stays open.
  ///
  /// In en, this message translates to:
  /// **'The process exited with code {code}.'**
  String terminalStateExited(int code);

  /// Banner of an ended terminal session; the screen stays readable.
  ///
  /// In en, this message translates to:
  /// **'The terminal session ended: {reason}'**
  String terminalStateEnded(String reason);

  /// Danger banner of a revoked terminal session.
  ///
  /// In en, this message translates to:
  /// **'Access ended: {reason}'**
  String terminalStateBlockedRevoked(String reason);

  /// Hard warning after close code 4104 or a Noise or frame error.
  ///
  /// In en, this message translates to:
  /// **'The encrypted connection failed its check. Someone may have tampered with it. No new attempt is made.'**
  String get terminalStateBlockedHandshake;

  /// Hard warning when the key of the grant differs from the checked one (15 23.2 item 3).
  ///
  /// In en, this message translates to:
  /// **'The server sent a different host key than the checked one. The terminal session was closed.'**
  String get terminalStateBlockedKeyMismatch;

  /// Hard warning after close code 4002 or an unexpected message.
  ///
  /// In en, this message translates to:
  /// **'The connection broke the terminal protocol. No new attempt is made.'**
  String get terminalStateBlockedProtocol;

  /// Banner of a refused opening with the problem text.
  ///
  /// In en, this message translates to:
  /// **'The terminal could not be opened: {problem}'**
  String terminalStateFailed(String problem);

  /// Text of a close_reason of a terminal session (24 section 10.3).
  ///
  /// In en, this message translates to:
  /// **'{reason, select, client_closed{ended by you} closed_by_owner{ended by an owner} process_exited{the process exited} idle_timeout{no input for too long} max_duration{the maximum duration was reached} detach_expired{it was detached for too long} access_revoked{access was revoked} i6_violation{the LLM login of the host changed} host_disabled{the host was disabled} host_retired{the host was retired} host_rekeyed{the host was paired again} host_restarted{the host service restarted} user_disabled{the account was disabled} device_revoked{this device was removed} session_revoked{the login was ended} logout{you signed out} open_failed{the opening failed} host_policy_violation{a check on the host failed} other{no reason given}}'**
  String terminalCloseReason(String reason);

  /// Footer of the terminal view with a lock icon.
  ///
  /// In en, this message translates to:
  /// **'End-to-end encrypted'**
  String get terminalFooterEncrypted;

  /// Footer of the terminal view: profile of the session.
  ///
  /// In en, this message translates to:
  /// **'Profile {profile}'**
  String terminalFooterProfile(String profile);

  /// Footer of the terminal view: the idle limit of the host applies.
  ///
  /// In en, this message translates to:
  /// **'Idle timeout active'**
  String get terminalFooterIdle;

  /// Title of the dialog that confirms an opening (screen D11).
  ///
  /// In en, this message translates to:
  /// **'Open terminal'**
  String get terminalOpenTitle;

  /// Label of the host in the dialog Open terminal.
  ///
  /// In en, this message translates to:
  /// **'Host'**
  String get terminalOpenHost;

  /// Label of the profile choice in the dialog Open terminal.
  ///
  /// In en, this message translates to:
  /// **'Profile'**
  String get terminalOpenProfile;

  /// Label of the working folder choice for a profile of kind claude.
  ///
  /// In en, this message translates to:
  /// **'Working folder'**
  String get terminalOpenWorkdir;

  /// Label of the LLM login badge in the dialog Open terminal.
  ///
  /// In en, this message translates to:
  /// **'LLM login'**
  String get terminalOpenLogin;

  /// Note in the dialog Open terminal.
  ///
  /// In en, this message translates to:
  /// **'Every opening is logged.'**
  String get terminalOpenLogged;

  /// Main button of the dialog Open terminal; the step-up follows.
  ///
  /// In en, this message translates to:
  /// **'Confirm with passkey or device key'**
  String get terminalOpenConfirm;

  /// Cancel button of the terminal dialogs.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get terminalCancel;

  /// Title of the hard warning of a changed host key (24 section 5.5).
  ///
  /// In en, this message translates to:
  /// **'The host\'s key has changed'**
  String get terminalKeyChangedTitle;

  /// Label of the pinned fingerprint in the key change warning.
  ///
  /// In en, this message translates to:
  /// **'Stored'**
  String get terminalKeyChangedStored;

  /// Label of the current fingerprint in the key change warning.
  ///
  /// In en, this message translates to:
  /// **'Now'**
  String get terminalKeyChangedNow;

  /// Body of the key change warning.
  ///
  /// In en, this message translates to:
  /// **'Without pairing again by an owner, this is a sign of an attack. Connect only after you have checked the new fingerprint on the host.'**
  String get terminalKeyChangedBody;

  /// Main action of the key change warning; opens nothing.
  ///
  /// In en, this message translates to:
  /// **'Do not connect'**
  String get terminalKeyChangedReject;

  /// Secondary action of the key change warning; opens with pin_changed.
  ///
  /// In en, this message translates to:
  /// **'Accept new key'**
  String get terminalKeyChangedAccept;

  /// Title of the warning of 24 section 8.4 before the step-up.
  ///
  /// In en, this message translates to:
  /// **'This host belongs to another person'**
  String get terminalForeignTitle;

  /// Body of the warning of 24 section 8.4.
  ///
  /// In en, this message translates to:
  /// **'{host} is assigned to another person. Your terminal session works in your own session user, but that person sets up the host and could read along through a local privilege escalation. Do not type secrets here unless you trust the host.'**
  String terminalForeignBody(String host);

  /// Button of the warning of 24 section 8.4 that continues to the step-up.
  ///
  /// In en, this message translates to:
  /// **'Connect anyway'**
  String get terminalForeignConfirm;

  /// Title of the confirmation before a terminal hyperlink opens (24 section 11.2).
  ///
  /// In en, this message translates to:
  /// **'Open link?'**
  String get terminalLinkTitle;

  /// Body of the link confirmation; the full target follows.
  ///
  /// In en, this message translates to:
  /// **'The program in the terminal links to this address:'**
  String get terminalLinkBody;

  /// Button that opens the confirmed link.
  ///
  /// In en, this message translates to:
  /// **'Open link'**
  String get terminalLinkOpen;

  /// Note in the link confirmation for any other scheme.
  ///
  /// In en, this message translates to:
  /// **'Only https and http links open. This address stays closed.'**
  String get terminalLinkUnsupported;

  /// Hint with the target of a terminal hyperlink under the pointer.
  ///
  /// In en, this message translates to:
  /// **'Link: {target}'**
  String terminalLinkHover(String target);

  /// Title of the confirmation before pasting text with line breaks (15 23.4).
  ///
  /// In en, this message translates to:
  /// **'Paste with line breaks?'**
  String get terminalPasteTitle;

  /// Body of the paste confirmation.
  ///
  /// In en, this message translates to:
  /// **'The text has {count, plural, =1{1 line} other{{count} lines}}. Every line break can start a command.'**
  String terminalPasteBody(int count);

  /// Button that pastes the text.
  ///
  /// In en, this message translates to:
  /// **'Paste'**
  String get terminalPasteConfirm;

  /// Screen reader action that reads the visible terminal screen (01 section 8).
  ///
  /// In en, this message translates to:
  /// **'Read screen'**
  String get terminalReadScreen;

  /// Screen reader label of the terminal screen.
  ///
  /// In en, this message translates to:
  /// **'Terminal, line at the cursor: {line}'**
  String terminalScreenLabel(String line);

  /// Screen reader label of the extra keys bar (01 screen M8).
  ///
  /// In en, this message translates to:
  /// **'Extra keys bar'**
  String get terminalExtraKeys;

  /// Visible label of the extra key Escape.
  ///
  /// In en, this message translates to:
  /// **'Esc'**
  String get terminalKeyEsc;

  /// Visible label of the extra key Tab.
  ///
  /// In en, this message translates to:
  /// **'Tab'**
  String get terminalKeyTab;

  /// Visible label of the extra key Control.
  ///
  /// In en, this message translates to:
  /// **'Ctrl'**
  String get terminalKeyCtrl;

  /// Visible label of the extra key Alt.
  ///
  /// In en, this message translates to:
  /// **'Alt'**
  String get terminalKeyAlt;

  /// Visible label of the extra key Arrow up.
  ///
  /// In en, this message translates to:
  /// **'↑'**
  String get terminalKeyUp;

  /// Visible label of the extra key Arrow down.
  ///
  /// In en, this message translates to:
  /// **'↓'**
  String get terminalKeyDown;

  /// Visible label of the extra key Arrow left.
  ///
  /// In en, this message translates to:
  /// **'←'**
  String get terminalKeyLeft;

  /// Visible label of the extra key Arrow right.
  ///
  /// In en, this message translates to:
  /// **'→'**
  String get terminalKeyRight;

  /// Visible label of the extra key Vertical bar.
  ///
  /// In en, this message translates to:
  /// **'|'**
  String get terminalKeyPipe;

  /// Visible label of the extra key Tilde.
  ///
  /// In en, this message translates to:
  /// **'~'**
  String get terminalKeyTilde;

  /// Visible label of the extra key Slash.
  ///
  /// In en, this message translates to:
  /// **'/'**
  String get terminalKeySlash;

  /// Visible label of the extra key Minus.
  ///
  /// In en, this message translates to:
  /// **'-'**
  String get terminalKeyMinus;

  /// Screen reader label of the extra key Escape.
  ///
  /// In en, this message translates to:
  /// **'Escape'**
  String get terminalKeyEscName;

  /// Screen reader label of the extra key Tab.
  ///
  /// In en, this message translates to:
  /// **'Tab'**
  String get terminalKeyTabName;

  /// Screen reader label of the extra key Control.
  ///
  /// In en, this message translates to:
  /// **'Control'**
  String get terminalKeyCtrlName;

  /// Screen reader label of the extra key Alt.
  ///
  /// In en, this message translates to:
  /// **'Alt'**
  String get terminalKeyAltName;

  /// Screen reader label of the extra key Arrow up.
  ///
  /// In en, this message translates to:
  /// **'Arrow up'**
  String get terminalKeyUpName;

  /// Screen reader label of the extra key Arrow down.
  ///
  /// In en, this message translates to:
  /// **'Arrow down'**
  String get terminalKeyDownName;

  /// Screen reader label of the extra key Arrow left.
  ///
  /// In en, this message translates to:
  /// **'Arrow left'**
  String get terminalKeyLeftName;

  /// Screen reader label of the extra key Arrow right.
  ///
  /// In en, this message translates to:
  /// **'Arrow right'**
  String get terminalKeyRightName;

  /// Screen reader label of the extra key Vertical bar.
  ///
  /// In en, this message translates to:
  /// **'Vertical bar'**
  String get terminalKeyPipeName;

  /// Screen reader label of the extra key Tilde.
  ///
  /// In en, this message translates to:
  /// **'Tilde'**
  String get terminalKeyTildeName;

  /// Screen reader label of the extra key Slash.
  ///
  /// In en, this message translates to:
  /// **'Slash'**
  String get terminalKeySlashName;

  /// Screen reader label of the extra key Minus.
  ///
  /// In en, this message translates to:
  /// **'Minus'**
  String get terminalKeyMinusName;

  /// Screen reader label of a modifier that applies to the next key only.
  ///
  /// In en, this message translates to:
  /// **'{key}, latched'**
  String terminalKeyLatched(String key);

  /// Screen reader label of a modifier that stays on until tapped again.
  ///
  /// In en, this message translates to:
  /// **'{key}, held'**
  String terminalKeyHeld(String key);

  /// Step-up purpose of terminal.open.
  ///
  /// In en, this message translates to:
  /// **'Open a terminal on {name}'**
  String terminalStepUpOpen(String name);

  /// Step-up purpose of terminal.attach.
  ///
  /// In en, this message translates to:
  /// **'Reattach to the terminal on {name}'**
  String terminalStepUpAttach(String name);

  /// The host of a terminal link in the link confirmation, shown before the full target.
  ///
  /// In en, this message translates to:
  /// **'Host: {name}'**
  String terminalLinkHost(String name);

  /// State of an attachment while its parts are sent (12 2 'Upload running').
  ///
  /// In en, this message translates to:
  /// **'Uploading {percent}%'**
  String fileStateUploading(int percent);

  /// State of an attachment whose upload has not started yet (offline, 15 7.5).
  ///
  /// In en, this message translates to:
  /// **'Waiting to upload'**
  String get fileStateWaiting;

  /// State of a file after the upload while the server checks it (12 2).
  ///
  /// In en, this message translates to:
  /// **'Being checked'**
  String get fileStateChecking;

  /// State of a file whose upload or check failed (12 2).
  ///
  /// In en, this message translates to:
  /// **'Upload failed'**
  String get fileStateUploadFailed;

  /// State of a file the malware scan blocked (12 2).
  ///
  /// In en, this message translates to:
  /// **'Blocked'**
  String get fileStateBlocked;

  /// State of a file whose upload session was aborted or expired (12 2).
  ///
  /// In en, this message translates to:
  /// **'Upload canceled'**
  String get fileStateCanceled;

  /// Tombstone of a deleted file (04 2.2 rule 5).
  ///
  /// In en, this message translates to:
  /// **'File deleted'**
  String get fileDeleted;

  /// File size in bytes.
  ///
  /// In en, this message translates to:
  /// **'{size} B'**
  String fileSizeBytes(String size);

  /// File size in kilobytes.
  ///
  /// In en, this message translates to:
  /// **'{size} KB'**
  String fileSizeKilobytes(String size);

  /// File size in megabytes.
  ///
  /// In en, this message translates to:
  /// **'{size} MB'**
  String fileSizeMegabytes(String size);

  /// File size in gigabytes.
  ///
  /// In en, this message translates to:
  /// **'{size} GB'**
  String fileSizeGigabytes(String size);

  /// Screen reader label of an attachment: name, size and state.
  ///
  /// In en, this message translates to:
  /// **'{name}, {size}, {state}'**
  String fileTileLabel(String name, String size, String state);

  /// Opens a downloaded file with the system (12 9).
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get fileOpen;

  /// Saves a file at a place the person chooses (12 9).
  ///
  /// In en, this message translates to:
  /// **'Save as'**
  String get fileSaveAs;

  /// Notice after a file was saved.
  ///
  /// In en, this message translates to:
  /// **'Saved {name}'**
  String fileSaved(String name);

  /// Notice when a download failed, with the reason.
  ///
  /// In en, this message translates to:
  /// **'Download failed: {reason}'**
  String fileDownloadFailed(String reason);

  /// Notice when the received bytes do not match Repr-Digest (12 9.3).
  ///
  /// In en, this message translates to:
  /// **'The download was damaged and has been deleted. Try again.'**
  String get fileDownloadDamaged;

  /// Screen reader label of an image preview that opens the full view.
  ///
  /// In en, this message translates to:
  /// **'Open image {name}'**
  String fileImageOpen(String name);

  /// Closes the full view of an image.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get fileImageClose;

  /// Placeholder while a thumbnail loads from the server.
  ///
  /// In en, this message translates to:
  /// **'Loading image'**
  String get fileImageLoading;

  /// Plays an audio or video attachment through a file ticket (12 9.2).
  ///
  /// In en, this message translates to:
  /// **'Play {name}'**
  String fileMediaPlay(String name);

  /// Pauses an audio or video attachment.
  ///
  /// In en, this message translates to:
  /// **'Pause'**
  String get fileMediaPause;

  /// Notice when an audio or video attachment cannot be played.
  ///
  /// In en, this message translates to:
  /// **'Playback failed'**
  String get fileMediaFailed;

  /// Sends a failed upload again as a new upload (brief M4-22 Design note 1).
  ///
  /// In en, this message translates to:
  /// **'Upload again'**
  String get fileUploadAgain;

  /// Removes an attachment from the input before sending.
  ///
  /// In en, this message translates to:
  /// **'Remove {name}'**
  String fileRemove(String name);

  /// Reason of a failed upload: 409 conflict with reason part_conflict (12 3.2).
  ///
  /// In en, this message translates to:
  /// **'A part was sent twice with different content'**
  String get fileFailPartConflict;

  /// Reason of a failed upload whose session was aborted or expired (410 gone, 12 3.6).
  ///
  /// In en, this message translates to:
  /// **'The upload session has ended'**
  String get fileFailSessionEnded;

  /// Reason of a failed upload: 409 conflict with reason quota_exceeded (12 3.1).
  ///
  /// In en, this message translates to:
  /// **'The storage quota is used up'**
  String get fileFailQuota;

  /// Reason of a failed message: 409 conflict with reason file_not_attachable (12 10).
  ///
  /// In en, this message translates to:
  /// **'This file cannot be attached'**
  String get fileFailNotAttachable;

  /// Reason of a failed upload whose local file is gone.
  ///
  /// In en, this message translates to:
  /// **'The file cannot be read anymore'**
  String get fileFailUnreadable;

  /// Refusal of an attachment over file_max_upload_bytes (15 14).
  ///
  /// In en, this message translates to:
  /// **'{name} is larger than the limit of {limit}'**
  String composerFileTooLarge(String name, String limit);

  /// Refusal of attachments over message_max_attachments (12 10 item 2).
  ///
  /// In en, this message translates to:
  /// **'At most {count} files per message'**
  String composerTooManyFiles(int count);

  /// Refusal of an attachment whose file cannot be read.
  ///
  /// In en, this message translates to:
  /// **'{name} cannot be read'**
  String composerFileUnreadable(String name);

  /// Hint while files are dragged over the input.
  ///
  /// In en, this message translates to:
  /// **'Drop files to attach them'**
  String get composerDropFiles;

  /// Screen reader label of the attachments of the input.
  ///
  /// In en, this message translates to:
  /// **'Attachments ({count})'**
  String composerAttachments(int count);

  /// The recording bar while a voice message is recorded.
  ///
  /// In en, this message translates to:
  /// **'Recording {elapsed}'**
  String voiceRecording(String elapsed);

  /// The recording bar after a recording stopped.
  ///
  /// In en, this message translates to:
  /// **'Voice message {elapsed}'**
  String voiceRecorded(String elapsed);

  /// Sends the recorded voice message.
  ///
  /// In en, this message translates to:
  /// **'Send voice message'**
  String get voiceSend;

  /// Stops the recording.
  ///
  /// In en, this message translates to:
  /// **'Stop recording'**
  String get voiceStop;

  /// Discards the recording.
  ///
  /// In en, this message translates to:
  /// **'Discard recording'**
  String get voiceCancel;

  /// Notice when the microphone permission is denied (15 13).
  ///
  /// In en, this message translates to:
  /// **'Microphone access is off. Allow it in the system settings under Privacy and Security, Microphone.'**
  String get voiceMicrophoneDenied;

  /// Notice when a recording reached voice_max_duration_ms (15 13).
  ///
  /// In en, this message translates to:
  /// **'The recording stopped at the limit of {limit}'**
  String voiceLimitReached(String limit);

  /// Notice when the recorder failed.
  ///
  /// In en, this message translates to:
  /// **'The recording failed'**
  String get voiceFailed;

  /// Screen reader label of the speed button of a voice message (15 13).
  ///
  /// In en, this message translates to:
  /// **'Playback speed {speed}'**
  String voiceSpeed(String speed);

  /// The speed of a voice message on its button.
  ///
  /// In en, this message translates to:
  /// **'{speed}x'**
  String voiceSpeedValue(String speed);

  /// Heading of the transcript of a voice message (12 8.2).
  ///
  /// In en, this message translates to:
  /// **'Transcript'**
  String get transcriptLabel;

  /// Transcript state failed (12 8.2).
  ///
  /// In en, this message translates to:
  /// **'No transcript'**
  String get transcriptFailed;

  /// Asks the server for a transcript (file.transcribe, 12 8.2).
  ///
  /// In en, this message translates to:
  /// **'Transcribe'**
  String get transcriptRequest;

  /// Answer 409 transcription_unavailable (12 8.2).
  ///
  /// In en, this message translates to:
  /// **'Transcription is not available here'**
  String get transcriptUnavailable;

  /// Heading of the files section of the session column (01 5.1 D1).
  ///
  /// In en, this message translates to:
  /// **'Files ({count})'**
  String sessionFiles(int count);

  /// The files section without files.
  ///
  /// In en, this message translates to:
  /// **'No files'**
  String get sessionFilesEmpty;

  /// Title of the dialog that plays a video attachment through a file ticket (12 9.2).
  ///
  /// In en, this message translates to:
  /// **'Video {name}'**
  String fileVideoTitle(String name);

  /// Entry of the administration navigation: the installed apps (01 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'Apps'**
  String get adminNavApps;

  /// Entry of the administration navigation: the incoming webhooks per channel (11 section 8).
  ///
  /// In en, this message translates to:
  /// **'Incoming webhooks'**
  String get adminNavIncomingWebhooks;

  /// Heading of the apps list of the administration (11 4).
  ///
  /// In en, this message translates to:
  /// **'Apps'**
  String get adminAppListTitle;

  /// Button that starts the installation of an app from a manifest file.
  ///
  /// In en, this message translates to:
  /// **'Install app'**
  String get adminAppInstall;

  /// Hint next to the disabled install button when the host offers no file dialog.
  ///
  /// In en, this message translates to:
  /// **'Installing needs a file dialog; use the web console.'**
  String get adminAppInstallNoPicker;

  /// The apps list without apps.
  ///
  /// In en, this message translates to:
  /// **'No apps installed.'**
  String get adminAppListEmpty;

  /// The version of an app manifest.
  ///
  /// In en, this message translates to:
  /// **'Version {version}'**
  String adminAppVersion(String version);

  /// The publisher of an app.
  ///
  /// In en, this message translates to:
  /// **'by {publisher}'**
  String adminAppPublisher(String publisher);

  /// Button on the app page that returns to the apps list.
  ///
  /// In en, this message translates to:
  /// **'Back to the apps'**
  String get adminAppBack;

  /// State badge of an active app installation.
  ///
  /// In en, this message translates to:
  /// **'Active'**
  String get adminAppStateActive;

  /// State badge of a suspended app installation.
  ///
  /// In en, this message translates to:
  /// **'Suspended'**
  String get adminAppStateSuspended;

  /// State badge of an uninstalled app.
  ///
  /// In en, this message translates to:
  /// **'Uninstalled'**
  String get adminAppStateUninstalled;

  /// State badge of a state this version does not know.
  ///
  /// In en, this message translates to:
  /// **'Unknown state'**
  String get adminAppStateUnknown;

  /// Plain meaning of the app scope messages:read (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Read messages and threads and search in them'**
  String get adminAppScopeMessagesRead;

  /// Plain meaning of the app scope messages:write (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Write, edit and delete messages, react and start threads'**
  String get adminAppScopeMessagesWrite;

  /// Plain meaning of the app scope files:read (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Read files'**
  String get adminAppScopeFilesRead;

  /// Plain meaning of the app scope files:write (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Upload files'**
  String get adminAppScopeFilesWrite;

  /// Plain meaning of the app scope commands:write (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Register and remove commands and answer their suggestions'**
  String get adminAppScopeCommandsWrite;

  /// Plain meaning of the app scope components:write (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Change its buttons and forms and answer interactions'**
  String get adminAppScopeComponentsWrite;

  /// Plain meaning of the app scope users:read (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Read the profiles of people'**
  String get adminAppScopeUsersRead;

  /// Plain meaning of the app scope events:subscribe (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Receive events and catch up on missed ones'**
  String get adminAppScopeEventsSubscribe;

  /// Plain meaning of the app scope costs:reconcile (05 2.6).
  ///
  /// In en, this message translates to:
  /// **'Read and submit cost reconciliations'**
  String get adminAppScopeCostsReconcile;

  /// Meaning of a scope this version does not know.
  ///
  /// In en, this message translates to:
  /// **'No description in this version'**
  String get adminAppScopeUnknown;

  /// Badge of a scope the app requires.
  ///
  /// In en, this message translates to:
  /// **'required'**
  String get adminAppScopeRequired;

  /// Badge of a scope the owner may grant.
  ///
  /// In en, this message translates to:
  /// **'optional'**
  String get adminAppScopeOptional;

  /// A manifest file that is not UTF-8 text.
  ///
  /// In en, this message translates to:
  /// **'The file is not text (UTF-8).'**
  String get adminAppFileNotText;

  /// A manifest file that is neither JSON nor YAML.
  ///
  /// In en, this message translates to:
  /// **'The file is neither JSON nor YAML.'**
  String get adminAppFileNotJsonOrYaml;

  /// A manifest file whose document is a list or a single value.
  ///
  /// In en, this message translates to:
  /// **'The file holds no manifest object.'**
  String get adminAppFileNotObject;

  /// Button that updates an app with a new manifest.
  ///
  /// In en, this message translates to:
  /// **'Update'**
  String get adminAppUpdate;

  /// Button that suspends an app (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Suspend'**
  String get adminAppSuspend;

  /// Button that resumes a suspended app (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Resume'**
  String get adminAppResume;

  /// Button that uninstalls an app (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Uninstall'**
  String get adminAppUninstall;

  /// Title of the suspend dialog.
  ///
  /// In en, this message translates to:
  /// **'Suspend {name}?'**
  String adminAppSuspendTitle(String name);

  /// Body of the suspend dialog (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'An emergency stop: nothing is deleted, and resuming needs no new consent.'**
  String get adminAppSuspendBody;

  /// Title of the resume dialog.
  ///
  /// In en, this message translates to:
  /// **'Resume {name}?'**
  String adminAppResumeTitle(String name);

  /// Body of the resume dialog above the table of effects.
  ///
  /// In en, this message translates to:
  /// **'This comes back when the app is resumed:'**
  String get adminAppResumeBody;

  /// Title of the uninstall dialog.
  ///
  /// In en, this message translates to:
  /// **'Uninstall {name}?'**
  String adminAppUninstallTitle(String name);

  /// Body of the uninstall dialog (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'This cannot be undone; a new installation needs a new consent.'**
  String get adminAppUninstallBody;

  /// Column heading of the table of effects: what is affected.
  ///
  /// In en, this message translates to:
  /// **'Part'**
  String get adminAppEffectsPart;

  /// Column heading of the table of effects: what happens to it.
  ///
  /// In en, this message translates to:
  /// **'Effect'**
  String get adminAppEffectsResult;

  /// Row of the table of effects (11 4.3): the tokens of the app.
  ///
  /// In en, this message translates to:
  /// **'Tokens'**
  String get adminAppEffectRowTokens;

  /// Row of the table of effects (11 4.3): outgoing webhook deliveries.
  ///
  /// In en, this message translates to:
  /// **'Deliveries'**
  String get adminAppEffectRowDeliveries;

  /// Row of the table of effects (11 4.3): the commands of the app.
  ///
  /// In en, this message translates to:
  /// **'Commands'**
  String get adminAppEffectRowCommands;

  /// Row of the table of effects (11 4.3): buttons and forms of the app.
  ///
  /// In en, this message translates to:
  /// **'Components'**
  String get adminAppEffectRowComponents;

  /// Row of the table of effects (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Access grants and incoming webhooks'**
  String get adminAppEffectRowGrants;

  /// Row of the table of effects (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Messages of the app'**
  String get adminAppEffectRowMessages;

  /// Effect of suspend on tokens (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Blocked, not revoked'**
  String get adminAppEffectBlocked;

  /// Effect of suspend on deliveries (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Paused, the position is kept'**
  String get adminAppEffectPaused;

  /// Effect of suspend on commands (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Hidden'**
  String get adminAppEffectHidden;

  /// Effect of suspend and uninstall on components (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Disabled'**
  String get adminAppEffectDisabled;

  /// Effect of suspend on access grants and incoming webhooks (11 4.3, 8.4).
  ///
  /// In en, this message translates to:
  /// **'Remain; incoming webhooks answer 404 until the app is resumed'**
  String get adminAppEffectSuspendGrants;

  /// Effect of suspend on the messages of the app (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Remain'**
  String get adminAppEffectRemain;

  /// Effect of uninstall on tokens, access grants and incoming webhooks (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Revoked'**
  String get adminAppEffectRevoked;

  /// Effect of uninstall on deliveries (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Subscriptions are disabled'**
  String get adminAppEffectSubscriptionsDisabled;

  /// Effect of uninstall on commands (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Removed'**
  String get adminAppEffectRemoved;

  /// Effect of uninstall on the messages of the app (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Remain; the author is kept and the app is disabled'**
  String get adminAppEffectUninstallMessages;

  /// What resume brings back for tokens (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Work again'**
  String get adminAppEffectWorkAgain;

  /// What resume brings back for deliveries (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Continue from the kept position'**
  String get adminAppEffectDeliveriesContinue;

  /// What resume brings back for commands (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Shown again'**
  String get adminAppEffectShownAgain;

  /// What resume brings back for components (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Enabled again'**
  String get adminAppEffectEnabledAgain;

  /// What resume brings back for access grants and incoming webhooks (11 4.3, 8.4).
  ///
  /// In en, this message translates to:
  /// **'Unchanged; incoming webhooks accept calls again'**
  String get adminAppEffectResumeGrants;

  /// What resume changes for the messages of the app (11 4.3).
  ///
  /// In en, this message translates to:
  /// **'Unchanged'**
  String get adminAppEffectUnchanged;

  /// Notice after suspending an app.
  ///
  /// In en, this message translates to:
  /// **'{name} is suspended.'**
  String adminAppSuspended(String name);

  /// Notice after resuming an app.
  ///
  /// In en, this message translates to:
  /// **'{name} is active again.'**
  String adminAppResumed(String name);

  /// Notice after uninstalling an app.
  ///
  /// In en, this message translates to:
  /// **'{name} is uninstalled.'**
  String adminAppUninstalled(String name);

  /// Notice after updating an app.
  ///
  /// In en, this message translates to:
  /// **'{name} now has version {version}.'**
  String adminAppUpdated(String name, String version);

  /// Notice after the new consent of an update.
  ///
  /// In en, this message translates to:
  /// **'The new consent for {name} is saved.'**
  String adminAppConsentSaved(String name);

  /// Step-up purpose of app.install.
  ///
  /// In en, this message translates to:
  /// **'Install the app {name}'**
  String adminAppPurposeInstall(String name);

  /// Step-up purpose of an update of the manifest.
  ///
  /// In en, this message translates to:
  /// **'Update the app {name}'**
  String adminAppPurposeUpdate(String name);

  /// Step-up purpose of app.scopes.update.
  ///
  /// In en, this message translates to:
  /// **'Change the consent for {name}'**
  String adminAppPurposeScopes(String name);

  /// Step-up purpose of app.suspend.
  ///
  /// In en, this message translates to:
  /// **'Suspend the app {name}'**
  String adminAppPurposeSuspend(String name);

  /// Step-up purpose of app.resume.
  ///
  /// In en, this message translates to:
  /// **'Resume the app {name}'**
  String adminAppPurposeResume(String name);

  /// Step-up purpose of app.uninstall.
  ///
  /// In en, this message translates to:
  /// **'Uninstall the app {name}'**
  String adminAppPurposeUninstall(String name);

  /// Step-up purpose of grant.create for an app.
  ///
  /// In en, this message translates to:
  /// **'Create the access grant for {name} on {target}'**
  String adminAppPurposeGrant(String name, String target);

  /// Step-up purpose of token.create for an app.
  ///
  /// In en, this message translates to:
  /// **'Create a token for {name}'**
  String adminAppPurposeToken(String name);

  /// Title of the install dialog.
  ///
  /// In en, this message translates to:
  /// **'Install an app'**
  String get adminAppInstallTitle;

  /// First step of the install dialog (11 4.1: a file, never an address).
  ///
  /// In en, this message translates to:
  /// **'Choose the manifest file of the app (JSON or YAML). The server checks it before anything is installed.'**
  String get adminAppInstallPickBody;

  /// Button that opens the file dialog for a manifest.
  ///
  /// In en, this message translates to:
  /// **'Choose file'**
  String get adminAppPick;

  /// Shown while the server validates the manifest.
  ///
  /// In en, this message translates to:
  /// **'Checking the manifest'**
  String get adminAppChecking;

  /// The name of the chosen manifest file.
  ///
  /// In en, this message translates to:
  /// **'File: {name}'**
  String adminAppFileChosen(String name);

  /// Heading of the failed validation rules (11 3.2).
  ///
  /// In en, this message translates to:
  /// **'The server refused the manifest:'**
  String get adminAppInvalid;

  /// One failed validation rule with its place in the manifest.
  ///
  /// In en, this message translates to:
  /// **'{pointer}: rule \"{rule}\"'**
  String adminAppErrorLine(String pointer, String rule);

  /// Place of a failed rule that concerns the manifest as a whole.
  ///
  /// In en, this message translates to:
  /// **'the whole manifest'**
  String get adminAppManifestRoot;

  /// Introduction of the consent (11 4.1).
  ///
  /// In en, this message translates to:
  /// **'{name} by {publisher}, version {version}, asks for the following. Read it before you agree.'**
  String adminAppConsentIntro(String name, String publisher, String version);

  /// Heading of the scopes in the consent (11 4.1).
  ///
  /// In en, this message translates to:
  /// **'What the app may read and write'**
  String get adminAppConsentScopes;

  /// Heading of the events in the consent (11 4.1).
  ///
  /// In en, this message translates to:
  /// **'Events the app receives'**
  String get adminAppConsentEvents;

  /// Heading of the target hosts of outgoing data in the consent (11 4.1).
  ///
  /// In en, this message translates to:
  /// **'Where data are sent'**
  String get adminAppConsentDestinations;

  /// Heading of the commands in the consent.
  ///
  /// In en, this message translates to:
  /// **'Commands'**
  String get adminAppConsentCommands;

  /// Heading of the components in the consent.
  ///
  /// In en, this message translates to:
  /// **'Components'**
  String get adminAppConsentComponents;

  /// Heading of the requested grants in the consent (11 4.1).
  ///
  /// In en, this message translates to:
  /// **'Access grants the app asks for'**
  String get adminAppConsentGrants;

  /// Heading of data_egress in the consent (11 3.1).
  ///
  /// In en, this message translates to:
  /// **'Data that leave the instance, in the publisher\'s words'**
  String get adminAppConsentEgress;

  /// Consent of a manifest without data_egress.
  ///
  /// In en, this message translates to:
  /// **'The manifest names no data that leave the instance.'**
  String get adminAppConsentNoEgress;

  /// An empty part of the consent.
  ///
  /// In en, this message translates to:
  /// **'None.'**
  String get adminAppConsentNone;

  /// Component type button of a manifest.
  ///
  /// In en, this message translates to:
  /// **'Buttons'**
  String get adminAppComponentButton;

  /// Component type select of a manifest.
  ///
  /// In en, this message translates to:
  /// **'Selection lists'**
  String get adminAppComponentSelect;

  /// Component type form of a manifest.
  ///
  /// In en, this message translates to:
  /// **'Forms'**
  String get adminAppComponentForm;

  /// A requested grant on a channel the owner picks.
  ///
  /// In en, this message translates to:
  /// **'{level} on one channel'**
  String adminAppGrantAskChannel(String level);

  /// A requested grant on a space the owner picks.
  ///
  /// In en, this message translates to:
  /// **'{level} in one space'**
  String adminAppGrantAskSpace(String level);

  /// A requested grant on the instance.
  ///
  /// In en, this message translates to:
  /// **'{level} on the whole instance'**
  String adminAppGrantAskInstance(String level);

  /// A requested grant on an unknown target kind.
  ///
  /// In en, this message translates to:
  /// **'{level} on a target this version does not know'**
  String adminAppGrantAskOther(String level);

  /// The publisher's purpose of a requested grant.
  ///
  /// In en, this message translates to:
  /// **'Purpose: {purpose}'**
  String adminAppGrantPurpose(String purpose);

  /// Label of the target picker of a requested grant.
  ///
  /// In en, this message translates to:
  /// **'Target'**
  String get adminAppGrantTarget;

  /// Item of the target picker: no grant is created.
  ///
  /// In en, this message translates to:
  /// **'Do not create'**
  String get adminAppGrantNone;

  /// Heading of the token fields in the consent.
  ///
  /// In en, this message translates to:
  /// **'Token of the app'**
  String get adminAppConsentToken;

  /// Explanation of the token of an app.
  ///
  /// In en, this message translates to:
  /// **'The app signs in with this token. Its secret is shown only once.'**
  String get adminAppTokenBody;

  /// Optional DPoP thumbprint of an app token (08 12).
  ///
  /// In en, this message translates to:
  /// **'DPoP key thumbprint (optional)'**
  String get adminAppTokenDpop;

  /// Confirms the consent and installs the app.
  ///
  /// In en, this message translates to:
  /// **'Install'**
  String get adminAppInstallSubmit;

  /// Title of the grants step of the install dialog.
  ///
  /// In en, this message translates to:
  /// **'Access grants and token'**
  String get adminAppGrantsStepTitle;

  /// Body of the grants step.
  ///
  /// In en, this message translates to:
  /// **'{name} is installed. Check what each access grant changes, then create the access grants and the token.'**
  String adminAppGrantsStepBody(String name);

  /// Grants step without chosen grants.
  ///
  /// In en, this message translates to:
  /// **'No access grant chosen; only the token is created.'**
  String get adminAppNoGrants;

  /// Button that creates the chosen grants and the token.
  ///
  /// In en, this message translates to:
  /// **'Create access grants and token'**
  String get adminAppCreateGrants;

  /// Progress line of the installation.
  ///
  /// In en, this message translates to:
  /// **'Install {name}'**
  String adminAppStepInstall(String name);

  /// Progress line of one grant.
  ///
  /// In en, this message translates to:
  /// **'Access grant on {target}'**
  String adminAppStepGrant(String target);

  /// Progress line of the token.
  ///
  /// In en, this message translates to:
  /// **'Token {name}'**
  String adminAppStepToken(String name);

  /// A progress line: the step and its status.
  ///
  /// In en, this message translates to:
  /// **'{step}: {status}'**
  String adminAppStepLine(String step, String status);

  /// Status of a finished step.
  ///
  /// In en, this message translates to:
  /// **'finished'**
  String get adminAppStepFinished;

  /// Status of a failed step with the server's reason.
  ///
  /// In en, this message translates to:
  /// **'failed: {reason}'**
  String adminAppStepFailed(String reason);

  /// Status of a step that has not run yet.
  ///
  /// In en, this message translates to:
  /// **'waiting'**
  String get adminAppStepWaiting;

  /// Title of the dialog that shows an app token once.
  ///
  /// In en, this message translates to:
  /// **'Token of {name}'**
  String adminAppTokenSecretTitle(String name);

  /// Warning of the dialog that shows an app token once (08 12).
  ///
  /// In en, this message translates to:
  /// **'This secret is shown only once. Copy it into the app\'s secret store now.'**
  String get adminAppTokenSecretOnce;

  /// The ID of the active signing key shown with the app token (11 4.1).
  ///
  /// In en, this message translates to:
  /// **'Key ID of the webhook signatures: {kid}'**
  String adminAppKeyId(String kid);

  /// The homepage of an app, shown as text only.
  ///
  /// In en, this message translates to:
  /// **'Homepage: {url}'**
  String adminAppHomepage(String url);

  /// Heading of the events the installation subscribes to.
  ///
  /// In en, this message translates to:
  /// **'Subscribed events'**
  String get adminAppPageEvents;

  /// Button that creates a token for an app.
  ///
  /// In en, this message translates to:
  /// **'New token'**
  String get adminAppTokenNew;

  /// Title of the token form of an app.
  ///
  /// In en, this message translates to:
  /// **'Create a token for {name}'**
  String adminAppTokenCreateTitle(String name);

  /// Body of the revoke dialog of an app token.
  ///
  /// In en, this message translates to:
  /// **'The app loses the access of this token at once.'**
  String get adminAppTokenRevokeBody;

  /// Title of the update dialog.
  ///
  /// In en, this message translates to:
  /// **'Update {name}'**
  String adminAppUpdateTitle(String name);

  /// First step of the update dialog (11 4.2).
  ///
  /// In en, this message translates to:
  /// **'Choose the new manifest file with the same slug ({slug}).'**
  String adminAppUpdateBody(String slug);

  /// Heading of the diff of two manifests.
  ///
  /// In en, this message translates to:
  /// **'Changes from version {from} to {to}'**
  String adminAppDiffTitle(String from, String to);

  /// Part of the diff: scopes.
  ///
  /// In en, this message translates to:
  /// **'What the app may read and write'**
  String get adminAppDiffScopes;

  /// Part of the diff: events.
  ///
  /// In en, this message translates to:
  /// **'Events'**
  String get adminAppDiffEvents;

  /// Part of the diff: requested grants.
  ///
  /// In en, this message translates to:
  /// **'Requested access grants'**
  String get adminAppDiffGrants;

  /// Added items of one part of the diff.
  ///
  /// In en, this message translates to:
  /// **'Added: {items}'**
  String adminAppDiffAdded(String items);

  /// Removed items of one part of the diff.
  ///
  /// In en, this message translates to:
  /// **'Removed: {items}'**
  String adminAppDiffRemoved(String items);

  /// A diff without changes.
  ///
  /// In en, this message translates to:
  /// **'Nothing changes in scopes, events, destinations, access grants or commands.'**
  String get adminAppDiffNone;

  /// Notice of the diff (11 4.2).
  ///
  /// In en, this message translates to:
  /// **'Removed items take effect at once.'**
  String get adminAppDiffRemovedNow;

  /// Notice of the diff (11 4.2).
  ///
  /// In en, this message translates to:
  /// **'You confirm new destinations with this update.'**
  String get adminAppDiffHostsConfirmed;

  /// Notice of the diff (11 4.2).
  ///
  /// In en, this message translates to:
  /// **'An update never adds an access grant: create added ones under Access grants.'**
  String get adminAppDiffGrantsLater;

  /// Notice of the diff (11 4.2).
  ///
  /// In en, this message translates to:
  /// **'Added scopes and events need a new consent after the update; until then the app keeps its current ones.'**
  String get adminAppDiffNewConsent;

  /// Title of the new consent after an update.
  ///
  /// In en, this message translates to:
  /// **'New consent for {name}'**
  String adminAppNewConsentTitle(String name);

  /// Body of the new consent after an update (11 4.2).
  ///
  /// In en, this message translates to:
  /// **'Version {version} asks for more. Choose what the app may use; required items stay on.'**
  String adminAppNewConsentBody(String version);

  /// Button that saves the new consent.
  ///
  /// In en, this message translates to:
  /// **'Save consent'**
  String get adminAppNewConsentSubmit;

  /// Closes the new consent without a change; the app keeps its current scopes.
  ///
  /// In en, this message translates to:
  /// **'Later'**
  String get adminAppNewConsentLater;

  /// Heading of the outgoing webhook subscriptions of an app (11 7).
  ///
  /// In en, this message translates to:
  /// **'Outgoing webhooks'**
  String get adminHookTitle;

  /// The app has no outgoing webhook subscription.
  ///
  /// In en, this message translates to:
  /// **'No subscriptions.'**
  String get adminHookEmpty;

  /// Button that opens the form of a new outgoing webhook subscription.
  ///
  /// In en, this message translates to:
  /// **'New subscription'**
  String get adminHookCreate;

  /// State of a subscription that delivers.
  ///
  /// In en, this message translates to:
  /// **'Active'**
  String get adminHookStateActive;

  /// State of a paused subscription (by the owner or the delivery quota).
  ///
  /// In en, this message translates to:
  /// **'Paused'**
  String get adminHookStatePaused;

  /// State of a subscription the server disabled.
  ///
  /// In en, this message translates to:
  /// **'Disabled'**
  String get adminHookStateDisabled;

  /// State value of a newer server.
  ///
  /// In en, this message translates to:
  /// **'Unknown state'**
  String get adminHookStateUnknown;

  /// The event types of a subscription, comma separated.
  ///
  /// In en, this message translates to:
  /// **'Events: {events}'**
  String adminHookEvents(String events);

  /// Failed delivery attempts in a row of a subscription.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{No failures in a row} =1{1 failure in a row} other{{count} failures in a row}}'**
  String adminHookFailures(int count);

  /// The signature schemes of a subscription and the key ID of the last accepted delivery.
  ///
  /// In en, this message translates to:
  /// **'Signed with {schemes}, key {kid}'**
  String adminHookSigned(String schemes, String kid);

  /// When the server disabled a subscription.
  ///
  /// In en, this message translates to:
  /// **'Disabled {when}'**
  String adminHookDisabledAt(String when);

  /// Why a subscription is disabled: its newest delivery ended with event_expired (11 6).
  ///
  /// In en, this message translates to:
  /// **'A gap: events left the event log before they were delivered.'**
  String get adminHookReasonGap;

  /// Why a subscription is disabled: the receiver answered HTTP 410.
  ///
  /// In en, this message translates to:
  /// **'The receiver answered 410 Gone.'**
  String get adminHookReasonGone;

  /// Why a subscription is disabled: too many failed attempts in a row.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{Disabled after 1 failure in a row.} other{Disabled after {count} failures in a row.}}'**
  String adminHookReasonFailures(int count);

  /// A disabled subscription whose newest delivery explains nothing (for example after the app was uninstalled).
  ///
  /// In en, this message translates to:
  /// **'The server names no reason.'**
  String get adminHookReasonNone;

  /// Button that pauses a subscription.
  ///
  /// In en, this message translates to:
  /// **'Pause'**
  String get adminHookPause;

  /// Button that resumes a paused or disabled subscription.
  ///
  /// In en, this message translates to:
  /// **'Resume'**
  String get adminHookResume;

  /// Button that deletes a subscription.
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get adminHookDelete;

  /// Button that opens the deliveries of a subscription.
  ///
  /// In en, this message translates to:
  /// **'Deliveries'**
  String get adminHookDeliveries;

  /// Title of the confirmation of pausing a subscription.
  ///
  /// In en, this message translates to:
  /// **'Pause the subscription to {url}?'**
  String adminHookPauseTitle(String url);

  /// What pausing a subscription does (11 6: rows keep being created while paused).
  ///
  /// In en, this message translates to:
  /// **'Nothing goes to this address until you resume. Matching events keep being recorded and go out in order after resuming.'**
  String get adminHookPauseBody;

  /// Title of the confirmation of resuming a subscription.
  ///
  /// In en, this message translates to:
  /// **'Resume the subscription to {url}?'**
  String adminHookResumeTitle(String url);

  /// What resuming a subscription does.
  ///
  /// In en, this message translates to:
  /// **'Waiting deliveries go out to this address again, in order.'**
  String get adminHookResumeBody;

  /// Title of the confirmation of deleting a subscription.
  ///
  /// In en, this message translates to:
  /// **'Delete the subscription to {url}?'**
  String adminHookDeleteTitle(String url);

  /// What deleting a subscription does.
  ///
  /// In en, this message translates to:
  /// **'No further events go to this address.'**
  String get adminHookDeleteBody;

  /// Notice after pausing.
  ///
  /// In en, this message translates to:
  /// **'The subscription to {url} is paused.'**
  String adminHookPausedNotice(String url);

  /// Notice after resuming.
  ///
  /// In en, this message translates to:
  /// **'The subscription to {url} is active.'**
  String adminHookResumedNotice(String url);

  /// Notice after deleting.
  ///
  /// In en, this message translates to:
  /// **'The subscription to {url} is deleted.'**
  String adminHookDeletedNotice(String url);

  /// Notice after creating.
  ///
  /// In en, this message translates to:
  /// **'The subscription to {url} is created.'**
  String adminHookCreatedNotice(String url);

  /// Title of the form of a new outgoing webhook subscription.
  ///
  /// In en, this message translates to:
  /// **'New subscription for {app}'**
  String adminHookFormTitle(String app);

  /// Explanation of the subscription form (11 7: https only, webhook.allowed_hosts is checked by the server).
  ///
  /// In en, this message translates to:
  /// **'Events of the chosen types go to this address, signed with v1a. The server accepts only https addresses its webhook settings allow.'**
  String get adminHookFormBody;

  /// Label of the address field of a subscription.
  ///
  /// In en, this message translates to:
  /// **'Address'**
  String get adminHookUrl;

  /// Error under the address field when the scheme is not https.
  ///
  /// In en, this message translates to:
  /// **'The address must start with https://.'**
  String get adminHookUrlHttps;

  /// Label above the event type checkboxes of a subscription.
  ///
  /// In en, this message translates to:
  /// **'Event types'**
  String get adminHookEventTypes;

  /// The app has no granted events, so no subscription can be created.
  ///
  /// In en, this message translates to:
  /// **'The installation grants no events to subscribe to.'**
  String get adminHookNoEvents;

  /// Checkbox that adds the symmetric v1 signature to a subscription.
  ///
  /// In en, this message translates to:
  /// **'Also sign with HMAC (v1)'**
  String get adminHookHmac;

  /// Button that creates the subscription.
  ///
  /// In en, this message translates to:
  /// **'Create subscription'**
  String get adminHookSubmit;

  /// Step-up purpose of creating a subscription.
  ///
  /// In en, this message translates to:
  /// **'Create a webhook subscription for {app}'**
  String adminHookPurposeCreate(String app);

  /// Step-up purpose of pausing or resuming a subscription.
  ///
  /// In en, this message translates to:
  /// **'Change the webhook subscription to {url}'**
  String adminHookPurposeChange(String url);

  /// Step-up purpose of deleting a subscription.
  ///
  /// In en, this message translates to:
  /// **'Delete the webhook subscription to {url}'**
  String adminHookPurposeDelete(String url);

  /// Purpose text of a redelivery (the server asks no step-up for it).
  ///
  /// In en, this message translates to:
  /// **'Deliver the event again'**
  String get adminHookPurposeRedeliver;

  /// Heading of the deliveries of a subscription.
  ///
  /// In en, this message translates to:
  /// **'Deliveries to {url}'**
  String adminHookDeliveriesTitle(String url);

  /// The subscription has no deliveries.
  ///
  /// In en, this message translates to:
  /// **'No deliveries yet.'**
  String get adminHookDeliveriesEmpty;

  /// Button that loads the deliveries again.
  ///
  /// In en, this message translates to:
  /// **'Reload'**
  String get adminHookReload;

  /// Button that loads the next page of deliveries.
  ///
  /// In en, this message translates to:
  /// **'Load more'**
  String get adminHookMore;

  /// Status of a delivery waiting for its attempt.
  ///
  /// In en, this message translates to:
  /// **'Waiting'**
  String get adminHookDeliveryPending;

  /// Status of a delivery the receiver accepted.
  ///
  /// In en, this message translates to:
  /// **'Delivered'**
  String get adminHookDeliverySucceeded;

  /// Status of a delivery ended by a 410 of the receiver.
  ///
  /// In en, this message translates to:
  /// **'Failed'**
  String get adminHookDeliveryFailed;

  /// Status of a delivery the server gave up.
  ///
  /// In en, this message translates to:
  /// **'Given up'**
  String get adminHookDeliveryDead;

  /// Status value of a newer server.
  ///
  /// In en, this message translates to:
  /// **'Unknown status'**
  String get adminHookDeliveryUnknown;

  /// Attempts of a delivery so far.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 attempt} other{{count} attempts}}'**
  String adminHookAttempts(int count);

  /// When the next attempt of a delivery is due.
  ///
  /// In en, this message translates to:
  /// **'next attempt {when}'**
  String adminHookNextAttempt(String when);

  /// The last HTTP status code of a delivery.
  ///
  /// In en, this message translates to:
  /// **'last status {code}'**
  String adminHookLastStatus(String code);

  /// The last error of a delivery as text.
  ///
  /// In en, this message translates to:
  /// **'last error: {error}'**
  String adminHookLastError(String error);

  /// Event type and sequence number of a delivery.
  ///
  /// In en, this message translates to:
  /// **'{type}, sequence {seq}'**
  String adminHookEventSeq(String type, String seq);

  /// A delivery whose event is no longer in the event log, so its type is unknown.
  ///
  /// In en, this message translates to:
  /// **'Event left the log, sequence {seq}'**
  String adminHookEventGone(String seq);

  /// When a delivery was created.
  ///
  /// In en, this message translates to:
  /// **'created {when}'**
  String adminHookCreatedAt(String when);

  /// When a delivery succeeded.
  ///
  /// In en, this message translates to:
  /// **'delivered {when}'**
  String adminHookDeliveredAt(String when);

  /// Delivery error code timeout (D-M2-26-6).
  ///
  /// In en, this message translates to:
  /// **'timed out'**
  String get adminHookErrorTimeout;

  /// Delivery error code connection_failed.
  ///
  /// In en, this message translates to:
  /// **'connection failed'**
  String get adminHookErrorConnectionFailed;

  /// Delivery error code tls_failed.
  ///
  /// In en, this message translates to:
  /// **'TLS handshake or certificate failed'**
  String get adminHookErrorTlsFailed;

  /// Delivery error code address_blocked (the address check after DNS resolution).
  ///
  /// In en, this message translates to:
  /// **'the address resolves to a blocked network'**
  String get adminHookErrorAddressBlocked;

  /// Delivery error code host_not_allowed.
  ///
  /// In en, this message translates to:
  /// **'the target address is not allowed'**
  String get adminHookErrorHostNotAllowed;

  /// Delivery error code http_status (any status that is no success and no throttle).
  ///
  /// In en, this message translates to:
  /// **'unexpected HTTP status'**
  String get adminHookErrorHttpStatus;

  /// Delivery error code throttled (429, 502, 503, 504; not counted as a failure).
  ///
  /// In en, this message translates to:
  /// **'throttled by the receiver'**
  String get adminHookErrorThrottled;

  /// Delivery error code gone.
  ///
  /// In en, this message translates to:
  /// **'the receiver answered 410 Gone'**
  String get adminHookErrorGone;

  /// Delivery error code not_authorized.
  ///
  /// In en, this message translates to:
  /// **'the app may no longer receive this event'**
  String get adminHookErrorNotAuthorized;

  /// Delivery error code not_subscribed.
  ///
  /// In en, this message translates to:
  /// **'the subscription no longer covers this event type'**
  String get adminHookErrorNotSubscribed;

  /// Delivery error code event_expired.
  ///
  /// In en, this message translates to:
  /// **'the event left the event log'**
  String get adminHookErrorEventExpired;

  /// Button that makes a delivery due again.
  ///
  /// In en, this message translates to:
  /// **'Redeliver'**
  String get adminHookRedeliver;

  /// Notice after a redelivery.
  ///
  /// In en, this message translates to:
  /// **'The delivery is due again.'**
  String get adminHookRedelivered;

  /// The server refused a redelivery with 409 conflict, reason event_expired.
  ///
  /// In en, this message translates to:
  /// **'The event left the event log; it cannot be delivered again.'**
  String get adminHookRedeliverExpired;

  /// Heading of the public keys of the webhook signatures.
  ///
  /// In en, this message translates to:
  /// **'Signing keys'**
  String get adminHookKeysTitle;

  /// What the signing keys are for (11 7).
  ///
  /// In en, this message translates to:
  /// **'Receivers verify the v1a signature of every delivery with one of these public keys.'**
  String get adminHookKeysBody;

  /// State of the key that signs now.
  ///
  /// In en, this message translates to:
  /// **'Active'**
  String get adminHookKeyActive;

  /// State of the key that signs after the next rotation.
  ///
  /// In en, this message translates to:
  /// **'Next'**
  String get adminHookKeyNext;

  /// State of a key that is being retired.
  ///
  /// In en, this message translates to:
  /// **'Retiring'**
  String get adminHookKeyRetiring;

  /// When a retiring key ends.
  ///
  /// In en, this message translates to:
  /// **'valid until {when}'**
  String adminHookKeyUntil(String when);

  /// Title of the once-only dialog of the HMAC secret of a subscription.
  ///
  /// In en, this message translates to:
  /// **'HMAC secret for {url}'**
  String adminSecretHmacTitle(String url);

  /// Label above the HMAC secret.
  ///
  /// In en, this message translates to:
  /// **'HMAC secret (v1)'**
  String get adminSecretHmacLabel;

  /// Warning of the once-only HMAC secret.
  ///
  /// In en, this message translates to:
  /// **'This secret is shown only now. Store it at the receiver, which checks the v1 signature with it.'**
  String get adminSecretHmacOnce;

  /// Name of the file type in the file dialog for the manifest of an app (11 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'App manifest'**
  String get adminManifestFileType;

  /// Heading of the incoming webhooks of a channel (11 8).
  ///
  /// In en, this message translates to:
  /// **'Incoming webhooks'**
  String get adminIncomingTitle;

  /// Label of the channel picker of the incoming webhooks.
  ///
  /// In en, this message translates to:
  /// **'Channel'**
  String get adminIncomingChannel;

  /// The caller sees no channel, so no incoming webhook can be shown.
  ///
  /// In en, this message translates to:
  /// **'You see no channel.'**
  String get adminIncomingNoChannels;

  /// The chosen channel has no active incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'This channel has no incoming webhooks.'**
  String get adminIncomingEmpty;

  /// Button that opens the dialog of a new incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'Create incoming webhook'**
  String get adminIncomingCreate;

  /// The app an incoming webhook posts as.
  ///
  /// In en, this message translates to:
  /// **'writes as {app}'**
  String adminIncomingWritesAs(String app);

  /// When an incoming webhook was created.
  ///
  /// In en, this message translates to:
  /// **'created {when}'**
  String adminIncomingCreatedAt(String when);

  /// When an incoming webhook was last called.
  ///
  /// In en, this message translates to:
  /// **'last used {when}'**
  String adminIncomingLastUsed(String when);

  /// An incoming webhook that was never called.
  ///
  /// In en, this message translates to:
  /// **'never used'**
  String get adminIncomingNeverUsed;

  /// Button that revokes an incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get adminIncomingRevoke;

  /// Title of the confirmation of revoking an incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'Revoke the incoming webhook {name}?'**
  String adminIncomingRevokeTitle(String name);

  /// What revoking an incoming webhook does (11 8.1).
  ///
  /// In en, this message translates to:
  /// **'Calls of its address are answered with 404 from now on.'**
  String get adminIncomingRevokeBody;

  /// Notice after revoking.
  ///
  /// In en, this message translates to:
  /// **'The incoming webhook {name} is revoked.'**
  String adminIncomingRevokedNotice(String name);

  /// Notice after creating.
  ///
  /// In en, this message translates to:
  /// **'The incoming webhook {name} is created.'**
  String adminIncomingCreatedNotice(String name);

  /// Title of the dialog of a new incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'New incoming webhook in {path}'**
  String adminIncomingDialogTitle(String path);

  /// Explanation of the create dialog (11 8.1).
  ///
  /// In en, this message translates to:
  /// **'A service sends messages into this channel through the webhook\'s address. The webhook writes as an app.'**
  String get adminIncomingDialogBody;

  /// Label of the name field of an incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get adminIncomingName;

  /// Choice: the webhook writes as an installed app.
  ///
  /// In en, this message translates to:
  /// **'Write as an existing app'**
  String get adminIncomingModeExisting;

  /// Choice offered to owners: create a minimal app for this webhook (11 8.1).
  ///
  /// In en, this message translates to:
  /// **'New incoming app'**
  String get adminIncomingModeNewApp;

  /// What the option New incoming app does (11 8.1).
  ///
  /// In en, this message translates to:
  /// **'Installs a minimal app that may only write messages, with write access on exactly this channel.'**
  String get adminIncomingNewAppBody;

  /// Label of the app picker of an incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'App'**
  String get adminIncomingApp;

  /// The app picker has nothing to offer.
  ///
  /// In en, this message translates to:
  /// **'No active app is installed.'**
  String get adminIncomingNoApps;

  /// Button that creates the incoming webhook (the one confirmation of the action).
  ///
  /// In en, this message translates to:
  /// **'Create'**
  String get adminIncomingSubmit;

  /// Progress step 1 of New incoming app.
  ///
  /// In en, this message translates to:
  /// **'Install the incoming app'**
  String get adminIncomingStepInstall;

  /// Progress step 2 of New incoming app.
  ///
  /// In en, this message translates to:
  /// **'Grant write on {path}'**
  String adminIncomingStepGrant(String path);

  /// Progress step of creating the webhook.
  ///
  /// In en, this message translates to:
  /// **'Create the webhook'**
  String get adminIncomingStepWebhook;

  /// One progress line: the step and its state or the server's reason.
  ///
  /// In en, this message translates to:
  /// **'{step}: {state}'**
  String adminIncomingStepLine(String step, String state);

  /// State of a finished progress step.
  ///
  /// In en, this message translates to:
  /// **'done'**
  String get adminIncomingStepDone;

  /// State of a progress step that has not run.
  ///
  /// In en, this message translates to:
  /// **'waiting'**
  String get adminIncomingStepWaiting;

  /// Button that repeats the failed step and the rest.
  ///
  /// In en, this message translates to:
  /// **'Retry'**
  String get adminIncomingRetry;

  /// Button that uninstalls the half-installed incoming app.
  ///
  /// In en, this message translates to:
  /// **'Uninstall the incoming app'**
  String get adminIncomingUninstall;

  /// Notice after uninstalling the half-installed incoming app.
  ///
  /// In en, this message translates to:
  /// **'The incoming app is uninstalled.'**
  String get adminIncomingUninstalled;

  /// Step-up purpose of creating an incoming webhook (with an incoming app: every call of the action).
  ///
  /// In en, this message translates to:
  /// **'Create an incoming webhook in {path}'**
  String adminIncomingPurposeCreate(String path);

  /// Step-up purpose of revoking.
  ///
  /// In en, this message translates to:
  /// **'Revoke the incoming webhook {name}'**
  String adminIncomingPurposeRevoke(String name);

  /// Step-up purpose of uninstalling the half-installed incoming app.
  ///
  /// In en, this message translates to:
  /// **'Uninstall the incoming app {name}'**
  String adminIncomingPurposeUninstall(String name);

  /// Title of the once-only dialog of a new incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'Address of {name}'**
  String adminSecretIncomingTitle(String name);

  /// Label above the address of an incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'Webhook address with token'**
  String get adminSecretIncomingLabel;

  /// Warning of the once-only address of an incoming webhook.
  ///
  /// In en, this message translates to:
  /// **'This address holds the token and is shown only now. Copy it into the sending service.'**
  String get adminSecretIncomingOnce;

  /// How Bearer senders use the address (11 8.1); <token> stays as written.
  ///
  /// In en, this message translates to:
  /// **'Senders that can set a header drop the last segment of the address and send it as Authorization: Bearer <token>.'**
  String get adminSecretIncomingBearer;

  /// Entry of the administration navigation for the list of permission hints (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'Permission hints'**
  String get adminNavHints;

  /// Entry of the administration navigation for the list of access that expires soon (07 section 9.4).
  ///
  /// In en, this message translates to:
  /// **'Expiring soon'**
  String get adminNavExpiring;

  /// Title of the list of permission hints (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'Permission hints'**
  String get hintsTitle;

  /// Explanation under the title of the permission hints (07 sections 6 and 9.5).
  ///
  /// In en, this message translates to:
  /// **'Hints from the current permissions: what people can learn through agents and which access grants have no effect. A hint is not a block.'**
  String get hintsIntro;

  /// Label of the filter that chooses the instance or one space for the permission hints and the expiring access.
  ///
  /// In en, this message translates to:
  /// **'Show for'**
  String get hintsScopeLabel;

  /// Choice of the scope filter: the whole instance (owners only; the server refuses others).
  ///
  /// In en, this message translates to:
  /// **'Whole instance'**
  String get hintsScopeInstance;

  /// Label of the filter that keeps one kind of permission hint.
  ///
  /// In en, this message translates to:
  /// **'Kind'**
  String get hintsKindLabel;

  /// Choice of the kind filter that shows every kind of permission hint.
  ///
  /// In en, this message translates to:
  /// **'All kinds'**
  String get hintsKindAll;

  /// Kind of a permission hint: a person can learn content through an agent (07 section 6).
  ///
  /// In en, this message translates to:
  /// **'Read horizon (H1)'**
  String get hintsKindH1;

  /// Kind of a permission hint: an access grant whose subject is disabled or deleted (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'Access grant to a disabled or deleted account'**
  String get hintsKindGrantToDisabled;

  /// Kind of a permission hint: an access grant with the level trigger whose triggering SR2 or SR3 blocks (07 section 9.5).
  ///
  /// In en, this message translates to:
  /// **'Trigger agents without effect'**
  String get hintsKindTriggerBlocked;

  /// Kind of a permission hint: an agent in mode personal with its trigger set (07 section 9.5, I1).
  ///
  /// In en, this message translates to:
  /// **'Agent in Personal mode'**
  String get hintsKindAgentPersonal;

  /// Kind of a permission hint this version does not know.
  ///
  /// In en, this message translates to:
  /// **'Other hint'**
  String get hintsKindUnknown;

  /// Sentence of the hint H1 (07 section 6): the person may trigger the agent, which reads a channel the person may not read; channelPath is for example 'Marketing/Intern'.
  ///
  /// In en, this message translates to:
  /// **'{person} can learn content of {channelPath} through agent {agent}.'**
  String hintsH1Text(String person, String channelPath, String agent);

  /// Sentence of the hint for an access grant whose subject is disabled or deleted; target is for example 'Channel Marketing/Intern', level the level with its capabilities.
  ///
  /// In en, this message translates to:
  /// **'{subject} is disabled or deleted and still holds an access grant on {target}: {level}.'**
  String hintsGrantToDisabledText(String subject, String target, String level);

  /// Sentence of the hint for an access grant with the level trigger whose triggering a guard rule blocks; the rules follow as lines.
  ///
  /// In en, this message translates to:
  /// **'{subject} has the level Trigger agents on {target} but cannot trigger an agent there.'**
  String hintsTriggerBlockedText(String subject, String target);

  /// Line under a blocked trigger access grant: the guard rule SR2 (tool class, 07 section 5) blocks it.
  ///
  /// In en, this message translates to:
  /// **'SR2: an agent there has a tool class that only owners and people with a ready dev environment of their own may trigger.'**
  String get hintsRuleSr2;

  /// Line under a blocked trigger access grant: the guard rule SR3 (dev isolation, 07 section 5) blocks it.
  ///
  /// In en, this message translates to:
  /// **'SR3: a channel there is bound to a project; a run of a person who is not an owner needs a ready dev environment of their own.'**
  String get hintsRuleSr3;

  /// Sentence of the hint for an agent in mode personal with its credential holder (10 section 13).
  ///
  /// In en, this message translates to:
  /// **'Agent {agent} uses Personal mode on the subscription of {holder}.'**
  String hintsAgentPersonalText(String agent, String holder);

  /// Sentence of the hint for an agent in mode personal without a credential holder.
  ///
  /// In en, this message translates to:
  /// **'Agent {agent} uses Personal mode and has no holder.'**
  String hintsAgentPersonalNoHolder(String agent);

  /// Second sentence of the hint for an agent in mode personal: its trigger set, names separated by commas (10 section 5.4, I1).
  ///
  /// In en, this message translates to:
  /// **'I1 asks that only the holder can trigger it. These people can trigger it now: {people}.'**
  String hintsAgentPersonalSet(String people);

  /// Second sentence of the hint for an agent in mode personal whose trigger set is empty.
  ///
  /// In en, this message translates to:
  /// **'I1 asks that only the holder can trigger it. Nobody can trigger it now.'**
  String get hintsAgentPersonalNobody;

  /// Text of a permission hint of a kind this version does not know.
  ///
  /// In en, this message translates to:
  /// **'This version does not know this hint yet.'**
  String get hintsUnknownText;

  /// Shown when the list of permission hints is empty.
  ///
  /// In en, this message translates to:
  /// **'No permission hints.'**
  String get hintsEmpty;

  /// Action of a hint or an expiring access grant that opens 'Who has access?' for its channel or space (01 section 5.2 A3).
  ///
  /// In en, this message translates to:
  /// **'Explain'**
  String get hintsExplain;

  /// Action that replaces an expiring access grant with a new one (07 section 9.4).
  ///
  /// In en, this message translates to:
  /// **'Extend'**
  String get hintsExtend;

  /// Action of the hint for an agent in mode personal that opens the page of the agent.
  ///
  /// In en, this message translates to:
  /// **'Open agent'**
  String get hintsOpenAgent;

  /// Button that closes the dialog with 'Who has access?' opened from a hint.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get hintsClose;

  /// Title of the grant wizard when it extends an access grant.
  ///
  /// In en, this message translates to:
  /// **'Extend the access grant for {name}'**
  String hintsExtendTitle(String name);

  /// Note in the grant wizard when it extends an access grant (07 section 9.4).
  ///
  /// In en, this message translates to:
  /// **'Extending creates a new access grant with a new reason and the last day you enter, then revokes the old one.'**
  String get hintsExtendNote;

  /// Notice after an access grant was extended.
  ///
  /// In en, this message translates to:
  /// **'The access grant for {name} was extended: the new one applies, the old one is revoked.'**
  String hintsExtended(String name);

  /// Shown when extending created the new access grant and the revocation of the old one failed; reason is the server's reason.
  ///
  /// In en, this message translates to:
  /// **'The new access grant for {name} applies, but the old one could not be revoked: {reason}'**
  String hintsExtendRevokeFailed(String name, String reason);

  /// Title of the list of access that expires soon (07 section 9.4).
  ///
  /// In en, this message translates to:
  /// **'Expiring soon'**
  String get expiringTitle;

  /// Explanation under the title of the list of access that expires soon (07 section 9.4).
  ///
  /// In en, this message translates to:
  /// **'Access grants, group memberships and tokens of agents and apps that end soon. Extending replaces an access grant with a new one; a token is renewed by creating a new one and revoking the old one.'**
  String get expiringIntro;

  /// Shown when no access expires soon.
  ///
  /// In en, this message translates to:
  /// **'Nothing expires soon.'**
  String get expiringEmpty;

  /// Kind of an expiring access: an access grant.
  ///
  /// In en, this message translates to:
  /// **'Access grant'**
  String get expiringKindGrant;

  /// Kind of an expiring access: a group membership.
  ///
  /// In en, this message translates to:
  /// **'Group membership'**
  String get expiringKindMembership;

  /// Kind of an expiring access: an API token of an agent or an app.
  ///
  /// In en, this message translates to:
  /// **'Token'**
  String get expiringKindToken;

  /// Kind of an expiring access this version does not know.
  ///
  /// In en, this message translates to:
  /// **'Other access'**
  String get expiringKindUnknown;

  /// An expiring access grant: subject, target (for example 'Channel Marketing/Intern') and the level with its capabilities.
  ///
  /// In en, this message translates to:
  /// **'{subject} on {target}: {level}'**
  String expiringGrantText(String subject, String target, String level);

  /// An expiring group membership.
  ///
  /// In en, this message translates to:
  /// **'{person} in the group {group}'**
  String expiringMembershipText(String person, String group);

  /// An expiring API token of an agent (never its value).
  ///
  /// In en, this message translates to:
  /// **'Token of the agent {agent}'**
  String expiringAgentTokenText(String agent);

  /// An expiring API token of an app (never its value).
  ///
  /// In en, this message translates to:
  /// **'Token of the app {app}'**
  String expiringAppTokenText(String app);

  /// Note under an expiring token of an app: the administration has no token screen for apps yet, and a token is renewed by creating a new one and revoking the old one (08 section 12).
  ///
  /// In en, this message translates to:
  /// **'There is no token screen for apps here yet: create a new token for the app, switch the app over, then revoke the old one.'**
  String get expiringAppTokenNote;

  /// Text of an expiring access of a kind this version does not know.
  ///
  /// In en, this message translates to:
  /// **'This version does not know this kind of access yet.'**
  String get expiringUnknownText;

  /// Action of an expiring group membership that opens the group with its members.
  ///
  /// In en, this message translates to:
  /// **'Open group'**
  String get expiringOpenGroup;

  /// Action of an expiring agent token that opens the page of the agent with its tokens (08 section 12).
  ///
  /// In en, this message translates to:
  /// **'Renew token'**
  String get expiringRenewToken;

  /// Entry of the administration navigation: instance settings, branding, login providers and feature flags (01 section 5.5).
  ///
  /// In en, this message translates to:
  /// **'Instance and branding'**
  String get adminNavInstance;

  /// Heading of the instance administration (01 section 5.5).
  ///
  /// In en, this message translates to:
  /// **'Instance and branding'**
  String get instanceTitle;

  /// Part of the instance administration: the instance settings.
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get instanceTabSettings;

  /// Part of the instance administration: product name, logo and accent.
  ///
  /// In en, this message translates to:
  /// **'Branding'**
  String get instanceTabBranding;

  /// Part of the instance administration: the login providers (08 section 2).
  ///
  /// In en, this message translates to:
  /// **'Login providers'**
  String get instanceTabProviders;

  /// Part of the instance administration: the feature flags.
  ///
  /// In en, this message translates to:
  /// **'Feature flags'**
  String get instanceTabFlags;

  /// Button that saves a change of the instance administration after the confirmation.
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get instanceSave;

  /// Shown for a setting without a value.
  ///
  /// In en, this message translates to:
  /// **'no value'**
  String get instanceValueNone;

  /// Shown for a setting whose value is the empty text.
  ///
  /// In en, this message translates to:
  /// **'empty'**
  String get instanceValueEmpty;

  /// Shown for a setting or flag that was never written.
  ///
  /// In en, this message translates to:
  /// **'Default, never changed'**
  String get instanceNeverChanged;

  /// Version of a setting or flag, who changed it last and when.
  ///
  /// In en, this message translates to:
  /// **'Version {version}, changed by {name}, {when}'**
  String instanceChangedBy(int version, String name, String when);

  /// Version of a setting or flag and when it was changed last, when the person is unknown.
  ///
  /// In en, this message translates to:
  /// **'Version {version}, changed {when}'**
  String instanceChangedAt(int version, String when);

  /// What a change does, stated before the confirmation: the old and the new value.
  ///
  /// In en, this message translates to:
  /// **'From: {from}\nTo: {to}'**
  String instanceChangeFromTo(String from, String to);

  /// Shown after a concurrent change (412): the newer stored value next to the person's input.
  ///
  /// In en, this message translates to:
  /// **'Another owner changed this setting in the meantime. Now stored: {current}. Your input: {input}. Save again to replace the stored value with your input.'**
  String instanceConflict(String current, String input);

  /// Field error of the server: a required value is missing.
  ///
  /// In en, this message translates to:
  /// **'This field is required.'**
  String get instanceFieldRequired;

  /// Field error of the server: the text is longer than allowed.
  ///
  /// In en, this message translates to:
  /// **'This is too long for the server.'**
  String get instanceFieldTooLong;

  /// Field error of the server: control or bidi characters (SEC-IN-4).
  ///
  /// In en, this message translates to:
  /// **'Contains a character that is not allowed, for example a control or text direction character.'**
  String get instanceFieldInvalidText;

  /// Field error of the server: the accent is no palette key.
  ///
  /// In en, this message translates to:
  /// **'Only one of the palette colors can be chosen.'**
  String get instanceFieldUnknownColor;

  /// Field error of the server: the value is outside its list.
  ///
  /// In en, this message translates to:
  /// **'This value is not one of the allowed values.'**
  String get instanceFieldInvalidEnum;

  /// Field error of the server: the value has the wrong form.
  ///
  /// In en, this message translates to:
  /// **'This value does not have the expected form.'**
  String get instanceFieldInvalidFormat;

  /// Field error of the server: the JSON type is wrong.
  ///
  /// In en, this message translates to:
  /// **'This value has the wrong type.'**
  String get instanceFieldInvalidType;

  /// Field error of the server: the member does not fit the other members.
  ///
  /// In en, this message translates to:
  /// **'This field is not allowed with these settings.'**
  String get instanceFieldNotAllowed;

  /// Field error of the server: a duplicate entry.
  ///
  /// In en, this message translates to:
  /// **'An entry appears twice.'**
  String get instanceFieldDuplicate;

  /// Field error of the server: an unknown member.
  ///
  /// In en, this message translates to:
  /// **'The server does not know this field.'**
  String get instanceFieldUnknown;

  /// Field error of the server with a code this version does not know.
  ///
  /// In en, this message translates to:
  /// **'The server refused this value ({code}).'**
  String instanceFieldOther(String code);

  /// Introduction of the instance settings.
  ///
  /// In en, this message translates to:
  /// **'These settings apply to the whole instance and take effect immediately.'**
  String get instanceSettingsIntro;

  /// Type of a setting: free text.
  ///
  /// In en, this message translates to:
  /// **'Text'**
  String get instanceKindText;

  /// Type of a setting: a UUID of a file.
  ///
  /// In en, this message translates to:
  /// **'File ID'**
  String get instanceKindUuid;

  /// Type of a setting: one palette key, with the allowed keys.
  ///
  /// In en, this message translates to:
  /// **'One of the palette colors: {keys}'**
  String instanceKindPalette(String keys);

  /// Type of a setting: a language tag, with the languages of the product.
  ///
  /// In en, this message translates to:
  /// **'Language: {tags}'**
  String instanceKindLocale(String tags);

  /// Button that opens the editor of a setting.
  ///
  /// In en, this message translates to:
  /// **'Change'**
  String get instanceSettingChange;

  /// Note of a branding key in the settings list: it is changed on the branding screen.
  ///
  /// In en, this message translates to:
  /// **'Changed under Branding.'**
  String get instanceSettingOnBranding;

  /// Note of a setting key this version does not know.
  ///
  /// In en, this message translates to:
  /// **'This version has no editor for this key; change it through the API.'**
  String get instanceSettingUnknownKey;

  /// Label of the input of a setting.
  ///
  /// In en, this message translates to:
  /// **'Value'**
  String get instanceSettingValue;

  /// Title of the confirmation before a setting is changed.
  ///
  /// In en, this message translates to:
  /// **'Change {key}?'**
  String instanceSettingConfirmTitle(String key);

  /// Notice after a setting was saved.
  ///
  /// In en, this message translates to:
  /// **'{key} is now {value}.'**
  String instanceSettingSaved(String key, String value);

  /// Label and heading of the product name of the branding.
  ///
  /// In en, this message translates to:
  /// **'Product name'**
  String get instanceBrandingName;

  /// Helper text of the product name.
  ///
  /// In en, this message translates to:
  /// **'Shown to everyone in the apps and the console. The server checks the length and refuses control and text direction characters.'**
  String get instanceBrandingNameHelp;

  /// Heading of the accent picker.
  ///
  /// In en, this message translates to:
  /// **'Accent'**
  String get instanceAccentLabel;

  /// Help text of the accent picker (R-PR-08).
  ///
  /// In en, this message translates to:
  /// **'The accent comes only from the checked palette; there is no free color value.'**
  String get instanceAccentHelp;

  /// Name of the accent spruce.
  ///
  /// In en, this message translates to:
  /// **'Spruce'**
  String get instanceAccentSpruce;

  /// Name of the accent loch (a Scottish lake).
  ///
  /// In en, this message translates to:
  /// **'Loch'**
  String get instanceAccentLoch;

  /// Name of the accent heather.
  ///
  /// In en, this message translates to:
  /// **'Heather'**
  String get instanceAccentHeather;

  /// Name of the accent rowan.
  ///
  /// In en, this message translates to:
  /// **'Rowan'**
  String get instanceAccentRowan;

  /// Name of the accent amber.
  ///
  /// In en, this message translates to:
  /// **'Amber'**
  String get instanceAccentAmber;

  /// Name of the accent gorse.
  ///
  /// In en, this message translates to:
  /// **'Gorse'**
  String get instanceAccentGorse;

  /// Name of the accent moss.
  ///
  /// In en, this message translates to:
  /// **'Moss'**
  String get instanceAccentMoss;

  /// Label of the light preview of the accent.
  ///
  /// In en, this message translates to:
  /// **'Light'**
  String get instancePreviewLight;

  /// Label of the dark preview of the accent.
  ///
  /// In en, this message translates to:
  /// **'Dark'**
  String get instancePreviewDark;

  /// Sample button in the accent preview.
  ///
  /// In en, this message translates to:
  /// **'Send'**
  String get instancePreviewButton;

  /// Sample marker in the accent preview.
  ///
  /// In en, this message translates to:
  /// **'Selected'**
  String get instancePreviewChip;

  /// Sample link in the accent preview.
  ///
  /// In en, this message translates to:
  /// **'A link'**
  String get instancePreviewLink;

  /// Title of the confirmation before the branding is changed.
  ///
  /// In en, this message translates to:
  /// **'Change the branding?'**
  String get instanceBrandingConfirmTitle;

  /// Line of the branding confirmation: the product name changes.
  ///
  /// In en, this message translates to:
  /// **'Product name: {from} becomes {to}'**
  String instanceBrandingNameChange(String from, String to);

  /// Line of the branding confirmation: the accent changes.
  ///
  /// In en, this message translates to:
  /// **'Accent: {from} becomes {to}'**
  String instanceBrandingAccentChange(String from, String to);

  /// Line of the branding confirmation: the effect on everyone.
  ///
  /// In en, this message translates to:
  /// **'Everyone sees the change immediately.'**
  String get instanceBrandingEveryone;

  /// Notice after the branding was saved.
  ///
  /// In en, this message translates to:
  /// **'The branding is saved.'**
  String get instanceBrandingSaved;

  /// Shown after a concurrent change of the branding (412).
  ///
  /// In en, this message translates to:
  /// **'Another owner changed the branding in the meantime: now {name} with the accent {accent}. Your input stays; save again to replace it.'**
  String instanceBrandingConflict(String name, String accent);

  /// Heading of the logo of the branding.
  ///
  /// In en, this message translates to:
  /// **'Logo'**
  String get instanceLogoTitle;

  /// Shown when the instance has no logo.
  ///
  /// In en, this message translates to:
  /// **'No logo is set.'**
  String get instanceLogoNone;

  /// Shown while the uploaded logo is not deliverable yet.
  ///
  /// In en, this message translates to:
  /// **'The logo is being checked and appears when it is ready.'**
  String get instanceLogoPending;

  /// Button that loads the logo preview again.
  ///
  /// In en, this message translates to:
  /// **'Check again'**
  String get instanceLogoRefresh;

  /// Button that uploads a new logo.
  ///
  /// In en, this message translates to:
  /// **'Upload logo'**
  String get instanceLogoUpload;

  /// Button that removes the logo.
  ///
  /// In en, this message translates to:
  /// **'Remove logo'**
  String get instanceLogoRemove;

  /// Title of the confirmation before the logo is uploaded.
  ///
  /// In en, this message translates to:
  /// **'Upload the logo?'**
  String get instanceLogoConfirmTitle;

  /// Body of the confirmation before the logo is uploaded.
  ///
  /// In en, this message translates to:
  /// **'{file} replaces the current logo for everyone. The server checks it and removes metadata before it is shown.'**
  String instanceLogoConfirmBody(String file);

  /// Notice after the logo was uploaded.
  ///
  /// In en, this message translates to:
  /// **'The logo is uploaded.'**
  String get instanceLogoUploaded;

  /// Title of the confirmation before the logo is removed.
  ///
  /// In en, this message translates to:
  /// **'Remove the logo?'**
  String get instanceLogoRemoveTitle;

  /// Body of the confirmation before the logo is removed.
  ///
  /// In en, this message translates to:
  /// **'Everyone sees the product name without a logo.'**
  String get instanceLogoRemoveBody;

  /// Notice after the logo was removed.
  ///
  /// In en, this message translates to:
  /// **'The logo is removed.'**
  String get instanceLogoRemoved;

  /// Introduction of the login providers.
  ///
  /// In en, this message translates to:
  /// **'People sign in through these providers. Changes take effect immediately.'**
  String get instanceProvidersIntro;

  /// Shown when the instance has no login provider.
  ///
  /// In en, this message translates to:
  /// **'No login provider is set up.'**
  String get instanceProvidersEmpty;

  /// Button that adds a login provider.
  ///
  /// In en, this message translates to:
  /// **'Add provider'**
  String get instanceProviderAdd;

  /// Driver of a login provider: OIDC.
  ///
  /// In en, this message translates to:
  /// **'OpenID Connect'**
  String get instanceProviderKindOidc;

  /// Driver of a login provider: trusted proxy header (08 section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Proxy header'**
  String get instanceProviderKindHeader;

  /// Driver of a login provider: local accounts (08 section 2.5).
  ///
  /// In en, this message translates to:
  /// **'Local accounts'**
  String get instanceProviderKindLocal;

  /// Driver of a login provider this version does not know.
  ///
  /// In en, this message translates to:
  /// **'Unknown driver'**
  String get instanceProviderKindUnknown;

  /// State of a login provider: enabled.
  ///
  /// In en, this message translates to:
  /// **'Enabled'**
  String get instanceProviderStateEnabled;

  /// State of a login provider: disabled.
  ///
  /// In en, this message translates to:
  /// **'Disabled'**
  String get instanceProviderStateDisabled;

  /// Marker of a login provider that creates accounts for unknown identities.
  ///
  /// In en, this message translates to:
  /// **'Creates accounts'**
  String get instanceProviderProvisioningShort;

  /// Line of a proxy header provider: its login header.
  ///
  /// In en, this message translates to:
  /// **'Login header: {header}'**
  String instanceProviderHeaderLine(String header);

  /// Line of an OIDC provider: its issuer.
  ///
  /// In en, this message translates to:
  /// **'Issuer: {issuer}'**
  String instanceProviderIssuerLine(String issuer);

  /// Button that opens the form of a login provider.
  ///
  /// In en, this message translates to:
  /// **'Change'**
  String get instanceProviderChange;

  /// Button that enables a login provider.
  ///
  /// In en, this message translates to:
  /// **'Enable'**
  String get instanceProviderEnable;

  /// Button that disables a login provider.
  ///
  /// In en, this message translates to:
  /// **'Disable'**
  String get instanceProviderDisable;

  /// Button that deletes a login provider.
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get instanceProviderDelete;

  /// Title of the confirmation before a login provider is disabled.
  ///
  /// In en, this message translates to:
  /// **'Disable {name}?'**
  String instanceProviderDisableTitle(String name);

  /// Body of the confirmation before a login provider is disabled (R-ID-12).
  ///
  /// In en, this message translates to:
  /// **'All sessions that started with {name} end immediately. Nobody can sign in with it until it is enabled again.'**
  String instanceProviderDisableBody(String name);

  /// Title of the confirmation before a login provider is enabled.
  ///
  /// In en, this message translates to:
  /// **'Enable {name}?'**
  String instanceProviderEnableTitle(String name);

  /// Body of the confirmation before a login provider is enabled.
  ///
  /// In en, this message translates to:
  /// **'People can sign in with {name} again.'**
  String instanceProviderEnableBody(String name);

  /// Title of the confirmation before a login provider is deleted.
  ///
  /// In en, this message translates to:
  /// **'Delete {name}?'**
  String instanceProviderDeleteTitle(String name);

  /// Body of the confirmation before a login provider is deleted (R-ID-12).
  ///
  /// In en, this message translates to:
  /// **'All sessions that started with {name} end immediately. The server refuses while identities are linked to it; disable it then.'**
  String instanceProviderDeleteBody(String name);

  /// Notice after a login provider was disabled.
  ///
  /// In en, this message translates to:
  /// **'{name} is disabled; its sessions ended.'**
  String instanceProviderDisabled(String name);

  /// Notice after a login provider was enabled.
  ///
  /// In en, this message translates to:
  /// **'{name} is enabled.'**
  String instanceProviderEnabled(String name);

  /// Notice after a login provider was deleted.
  ///
  /// In en, this message translates to:
  /// **'{name} is deleted.'**
  String instanceProviderDeleted(String name);

  /// Notice after a login provider was added.
  ///
  /// In en, this message translates to:
  /// **'{name} is added.'**
  String instanceProviderAdded(String name);

  /// Notice after a login provider was changed.
  ///
  /// In en, this message translates to:
  /// **'{name} is saved.'**
  String instanceProviderSaved(String name);

  /// Note of the proxy header driver: the loopback rule (08 section 2.4 rule 1).
  ///
  /// In en, this message translates to:
  /// **'The proxy header provider works only while the server listens on a loopback address (127.0.0.1 or ::1); otherwise the server refuses to enable it.'**
  String get instanceProviderLoopbackRule;

  /// Explanation of the 409 of the loopback rule; the server's hint follows.
  ///
  /// In en, this message translates to:
  /// **'The server refused: the proxy header provider needs a server that listens only on a loopback address.'**
  String get instanceProviderLoopbackRefused;

  /// Label of the redirect URI of this server (08 section 2.2).
  ///
  /// In en, this message translates to:
  /// **'Redirect URI to register at the OIDC provider'**
  String get instanceProviderRedirectLabel;

  /// Button that copies the redirect URI.
  ///
  /// In en, this message translates to:
  /// **'Copy'**
  String get instanceProviderRedirectCopy;

  /// Button label after the redirect URI was copied.
  ///
  /// In en, this message translates to:
  /// **'Copied'**
  String get instanceProviderRedirectCopied;

  /// Title of the form of a new login provider.
  ///
  /// In en, this message translates to:
  /// **'Add {kind} provider'**
  String instanceProviderAddTitle(String kind);

  /// Title of the form of an existing login provider.
  ///
  /// In en, this message translates to:
  /// **'Change {name}'**
  String instanceProviderEditTitle(String name);

  /// Label of the name of a login provider.
  ///
  /// In en, this message translates to:
  /// **'Name'**
  String get instanceProviderName;

  /// Helper text of the name of a login provider.
  ///
  /// In en, this message translates to:
  /// **'Shown on the login button.'**
  String get instanceProviderNameHelp;

  /// Label of the OIDC issuer.
  ///
  /// In en, this message translates to:
  /// **'Issuer'**
  String get instanceProviderIssuer;

  /// Helper text of the OIDC issuer.
  ///
  /// In en, this message translates to:
  /// **'Exact https address of the issuer, for example https://login.example.com'**
  String get instanceProviderIssuerHelp;

  /// Label of the OIDC client ID.
  ///
  /// In en, this message translates to:
  /// **'Client ID'**
  String get instanceProviderClientId;

  /// Label of the OIDC client authentication method.
  ///
  /// In en, this message translates to:
  /// **'Client authentication'**
  String get instanceProviderClientAuth;

  /// Label of the reference to the client secret.
  ///
  /// In en, this message translates to:
  /// **'Client secret reference'**
  String get instanceProviderSecretRef;

  /// Label of the reference to the private key.
  ///
  /// In en, this message translates to:
  /// **'Private key reference'**
  String get instanceProviderKeyRef;

  /// Helper text of a secret reference (08 section 2.6).
  ///
  /// In en, this message translates to:
  /// **'A reference such as file:/srv/secrets/idp/client or env:BOTHY_IDP_CLIENT, never the secret itself. It is never shown again.'**
  String get instanceProviderSecretRefHelp;

  /// Helper text of a secret reference when editing.
  ///
  /// In en, this message translates to:
  /// **'Leave empty to keep the stored reference.'**
  String get instanceProviderSecretKeep;

  /// Label of the algorithm of the client assertion.
  ///
  /// In en, this message translates to:
  /// **'Key algorithm'**
  String get instanceProviderKeyAlg;

  /// Label of the key ID of the client assertion.
  ///
  /// In en, this message translates to:
  /// **'Key ID'**
  String get instanceProviderKeyId;

  /// Label of the certificate thumbprint of the client assertion.
  ///
  /// In en, this message translates to:
  /// **'Certificate thumbprint (x5t#S256)'**
  String get instanceProviderX5t;

  /// Label of the allowed tenant IDs.
  ///
  /// In en, this message translates to:
  /// **'Allowed tenant IDs'**
  String get instanceProviderTenants;

  /// Helper text of the allowed tenant IDs.
  ///
  /// In en, this message translates to:
  /// **'Separated by commas; required when the issuer contains {tenantid}.'**
  String instanceProviderTenantsHelp(String tenantid);

  /// Label of the app redirect URIs.
  ///
  /// In en, this message translates to:
  /// **'App redirect URIs'**
  String get instanceProviderAppRedirects;

  /// Helper text of the app redirect URIs.
  ///
  /// In en, this message translates to:
  /// **'One per line: private schemes or claimed https addresses of the apps.'**
  String get instanceProviderAppRedirectsHelp;

  /// Label of the group claim.
  ///
  /// In en, this message translates to:
  /// **'Group claim'**
  String get instanceProviderGroupsClaim;

  /// Helper text of the group claim.
  ///
  /// In en, this message translates to:
  /// **'Without a group claim, groups are not synchronized.'**
  String get instanceProviderGroupsClaimHelp;

  /// Label of the login header of the proxy header driver.
  ///
  /// In en, this message translates to:
  /// **'Login header'**
  String get instanceProviderLoginHeader;

  /// Helper text of the login header.
  ///
  /// In en, this message translates to:
  /// **'For example Tailscale-User-Login'**
  String get instanceProviderLoginHeaderHelp;

  /// Label of the name header of the proxy header driver.
  ///
  /// In en, this message translates to:
  /// **'Name header (optional)'**
  String get instanceProviderNameHeader;

  /// Note of the local accounts driver.
  ///
  /// In en, this message translates to:
  /// **'Local accounts sign in with password and TOTP or a passkey; there is nothing to configure.'**
  String get instanceProviderLocalNote;

  /// Switch of a new login provider: enabled when created.
  ///
  /// In en, this message translates to:
  /// **'Enable at once'**
  String get instanceProviderEnableNow;

  /// Switch: unknown identities become accounts without an enrollment code (08 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Create accounts for unknown people'**
  String get instanceProviderProvisioning;

  /// Helper text of the provisioning switch.
  ///
  /// In en, this message translates to:
  /// **'Otherwise a person needs an enrollment code for the first login.'**
  String get instanceProviderProvisioningHelp;

  /// Line of the confirmation before a login provider is added.
  ///
  /// In en, this message translates to:
  /// **'A new {kind} provider named {name} is added.'**
  String instanceProviderCreateSummary(String kind, String name);

  /// Line of the confirmation: the new provider starts enabled.
  ///
  /// In en, this message translates to:
  /// **'It is enabled at once.'**
  String get instanceProviderStartsEnabled;

  /// Line of the confirmation: the new provider starts disabled.
  ///
  /// In en, this message translates to:
  /// **'It starts disabled.'**
  String get instanceProviderStartsDisabled;

  /// Line of the confirmation: provisioning is on.
  ///
  /// In en, this message translates to:
  /// **'Unknown people get an account at their first login.'**
  String get instanceProviderProvisioningOn;

  /// Line of the confirmation: provisioning is off.
  ///
  /// In en, this message translates to:
  /// **'Unknown people need an enrollment code.'**
  String get instanceProviderProvisioningOff;

  /// Line of the confirmation: the name of a provider changes.
  ///
  /// In en, this message translates to:
  /// **'Name: {from} becomes {to}'**
  String instanceProviderNameChange(String from, String to);

  /// Line of the confirmation: the configuration members that change (never a value).
  ///
  /// In en, this message translates to:
  /// **'Changed settings: {members}'**
  String instanceProviderMembersChange(String members);

  /// Purpose of the step-up for a change of an instance setting.
  ///
  /// In en, this message translates to:
  /// **'Change the instance setting {key}'**
  String stepUpPurposeInstanceSetting(String key);

  /// Purpose of the step-up for a change of the branding.
  ///
  /// In en, this message translates to:
  /// **'Change the branding of the instance'**
  String get stepUpPurposeBranding;

  /// Purpose of the step-up for a logo upload.
  ///
  /// In en, this message translates to:
  /// **'Upload the logo of the instance'**
  String get stepUpPurposeLogo;

  /// Purpose of the step-up for a logo removal.
  ///
  /// In en, this message translates to:
  /// **'Remove the logo of the instance'**
  String get stepUpPurposeLogoRemove;

  /// Purpose of the step-up for a new login provider.
  ///
  /// In en, this message translates to:
  /// **'Add the login provider {name}'**
  String stepUpPurposeProviderAdd(String name);

  /// Purpose of the step-up for a change of a login provider.
  ///
  /// In en, this message translates to:
  /// **'Change the login provider {name}'**
  String stepUpPurposeProviderEdit(String name);

  /// Purpose of the step-up for disabling a login provider.
  ///
  /// In en, this message translates to:
  /// **'Disable the login provider {name}'**
  String stepUpPurposeProviderDisable(String name);

  /// Purpose of the step-up for enabling a login provider.
  ///
  /// In en, this message translates to:
  /// **'Enable the login provider {name}'**
  String stepUpPurposeProviderEnable(String name);

  /// Purpose of the step-up for deleting a login provider.
  ///
  /// In en, this message translates to:
  /// **'Delete the login provider {name}'**
  String stepUpPurposeProviderDelete(String name);

  /// Purpose of the step-up for switching a flag on.
  ///
  /// In en, this message translates to:
  /// **'Switch on the feature flag {key}'**
  String stepUpPurposeFlagOn(String key);

  /// Purpose of the step-up for switching a flag off.
  ///
  /// In en, this message translates to:
  /// **'Switch off the feature flag {key}'**
  String stepUpPurposeFlagOff(String key);

  /// Introduction of the feature flags.
  ///
  /// In en, this message translates to:
  /// **'Feature flags switch functions for the whole instance. The state for you includes rollouts to people and groups.'**
  String get instanceFlagsIntro;

  /// State of a flag: on.
  ///
  /// In en, this message translates to:
  /// **'on'**
  String get instanceFlagOn;

  /// State of a flag: off.
  ///
  /// In en, this message translates to:
  /// **'off'**
  String get instanceFlagOff;

  /// Kind of a flag: release.
  ///
  /// In en, this message translates to:
  /// **'Release switch'**
  String get instanceFlagKindRelease;

  /// Kind of a flag: option.
  ///
  /// In en, this message translates to:
  /// **'Option'**
  String get instanceFlagKindOption;

  /// Kind of a flag this version does not know.
  ///
  /// In en, this message translates to:
  /// **'Unknown kind'**
  String get instanceFlagKindUnknown;

  /// Marker of an orphaned flag.
  ///
  /// In en, this message translates to:
  /// **'No longer registered'**
  String get instanceFlagOrphaned;

  /// The state of a flag evaluated for the signed-in person.
  ///
  /// In en, this message translates to:
  /// **'For you: {state}'**
  String instanceFlagForYou(String state);

  /// Shown when the server evaluated the flag not for the caller.
  ///
  /// In en, this message translates to:
  /// **'For you: not evaluated'**
  String get instanceFlagForYouUnknown;

  /// The stored global value of a flag.
  ///
  /// In en, this message translates to:
  /// **'Stored: {state}'**
  String instanceFlagStored(String state);

  /// The registered default of a flag.
  ///
  /// In en, this message translates to:
  /// **'Default: {state}'**
  String instanceFlagDefault(String state);

  /// How many people and groups have the global value inverted (rollout).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{Inverted for 1 person or group} other{Inverted for {count} people or groups}}'**
  String instanceFlagRollout(int count);

  /// Line of the switch dialog: the rollout is kept.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{The rollout to 1 person or group stays.} other{The rollout to {count} people or groups stays.}}'**
  String instanceFlagRolloutStays(int count);

  /// Title of the dialog that switches a flag on.
  ///
  /// In en, this message translates to:
  /// **'Switch on {key}?'**
  String instanceFlagOnTitle(String key);

  /// Title of the dialog that switches a flag off.
  ///
  /// In en, this message translates to:
  /// **'Switch off {key}?'**
  String instanceFlagOffTitle(String key);

  /// Line of the switch dialog: the effect.
  ///
  /// In en, this message translates to:
  /// **'The global value changes for everyone immediately.'**
  String get instanceFlagEveryone;

  /// Label of the reason of a flag switch; the audit log keeps it.
  ///
  /// In en, this message translates to:
  /// **'Reason (required)'**
  String get instanceFlagReason;

  /// Button that switches a flag on.
  ///
  /// In en, this message translates to:
  /// **'Switch on'**
  String get instanceFlagSwitchOn;

  /// Button that switches a flag off.
  ///
  /// In en, this message translates to:
  /// **'Switch off'**
  String get instanceFlagSwitchOff;

  /// Notice after a flag was switched on.
  ///
  /// In en, this message translates to:
  /// **'{key} is switched on.'**
  String instanceFlagSwitchedOn(String key);

  /// Notice after a flag was switched off.
  ///
  /// In en, this message translates to:
  /// **'{key} is switched off.'**
  String instanceFlagSwitchedOff(String key);

  /// Shown after a concurrent change of a flag (412).
  ///
  /// In en, this message translates to:
  /// **'Another owner changed {key} in the meantime; it is now {state}. Nothing was changed; switch again if needed.'**
  String instanceFlagConflict(String key, String state);

  /// Name of the file type group of the logo in the system file dialog (PNG, JPEG, WebP).
  ///
  /// In en, this message translates to:
  /// **'Image'**
  String get adminLogoFileType;

  /// Neutral text of a local notification without preview fields (01 section 10.2).
  ///
  /// In en, this message translates to:
  /// **'New message'**
  String get notificationPlaceholder;

  /// Title of a local notification with the two names of 01 section 10.2.
  ///
  /// In en, this message translates to:
  /// **'{name} · {title}'**
  String notificationTitle(String name, String title);

  /// Text of a local notification: author and shortened text (01 section 10.2).
  ///
  /// In en, this message translates to:
  /// **'{author}: {excerpt}'**
  String notificationText(String author, String excerpt);

  /// First line of a bundled local notification with the unread count of the thread (13 section 4.5).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 new message} other{{count} new messages}}'**
  String notificationBundled(int count);

  /// Title of the local notification of kind security: a device of the person waits for approval (13 section 2).
  ///
  /// In en, this message translates to:
  /// **'New device'**
  String get notificationSecurityTitle;

  /// Text of the local notification of kind security (13 section 2).
  ///
  /// In en, this message translates to:
  /// **'A new device is waiting for your approval.'**
  String get notificationSecurityText;

  /// Title of the local notification of kind admin (13 section 2).
  ///
  /// In en, this message translates to:
  /// **'Administration'**
  String get notificationAdminTitle;

  /// Text of the local notification of kind admin (13 section 2).
  ///
  /// In en, this message translates to:
  /// **'A reminder from the administration is waiting.'**
  String get notificationAdminText;

  /// Menu item of the icon in the Windows notification area that shows the window (15 section 10.3).
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get trayOpen;

  /// Menu item of the icon in the Windows notification area that ends the app and its connection (15 section 10.3).
  ///
  /// In en, this message translates to:
  /// **'Quit'**
  String get trayQuit;

  /// Note in the Notifications settings: a closed app gets no notifications (13 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'Notifications arrive only while the app is running.'**
  String get settingsNotificationsWhileRunning;

  /// Switch in the Notifications settings: start the app when the person signs in to the computer; a device setting (15 section 10.3).
  ///
  /// In en, this message translates to:
  /// **'Start at login'**
  String get settingsStartAtLogin;

  /// Shown instead of the switch Start at login where the system offers no login start (15 section 10.3).
  ///
  /// In en, this message translates to:
  /// **'This system does not offer starting at login for the app.'**
  String get settingsStartAtLoginUnavailable;

  /// Heading of the import page of the console (01 section 3.7 and 5.5).
  ///
  /// In en, this message translates to:
  /// **'Import from Discord'**
  String get importTitle;

  /// Button that adds an import source with its mapping file.
  ///
  /// In en, this message translates to:
  /// **'Add source'**
  String get importAdd;

  /// Button that reads the import sources again (the console has no live updates).
  ///
  /// In en, this message translates to:
  /// **'Refresh'**
  String get importRefresh;

  /// Text of the import list without a source.
  ///
  /// In en, this message translates to:
  /// **'No import source yet. Add one with the mapping file of the import tool.'**
  String get importEmpty;

  /// State draft of an import source.
  ///
  /// In en, this message translates to:
  /// **'Draft'**
  String get importStateDraft;

  /// State running of an import source: the owner allows imports.
  ///
  /// In en, this message translates to:
  /// **'Started'**
  String get importStateRunning;

  /// State paused of an import source.
  ///
  /// In en, this message translates to:
  /// **'Paused'**
  String get importStatePaused;

  /// State completed of an import source.
  ///
  /// In en, this message translates to:
  /// **'Completed'**
  String get importStateCompleted;

  /// State failed of an import source.
  ///
  /// In en, this message translates to:
  /// **'Failed'**
  String get importStateFailed;

  /// State of an import source the owner cancelled (failed with the counter cancelled).
  ///
  /// In en, this message translates to:
  /// **'Cancelled'**
  String get importStateCancelled;

  /// Progress of an import source in the list: the number of posted items.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 item imported} other{{count} items imported}}'**
  String importItemsImported(int count);

  /// Button back from one import source to the list.
  ///
  /// In en, this message translates to:
  /// **'All import sources'**
  String get importBack;

  /// Heading of the server counters of an import source (stats.server).
  ///
  /// In en, this message translates to:
  /// **'Counters of the server'**
  String get importServerCounters;

  /// Heading of the counters of the import tool (stats.tool).
  ///
  /// In en, this message translates to:
  /// **'Counters of the import tool'**
  String get importToolCounters;

  /// Text of a counter section without counters.
  ///
  /// In en, this message translates to:
  /// **'No counters yet.'**
  String get importNoCounters;

  /// Heading of the import items in state failed.
  ///
  /// In en, this message translates to:
  /// **'Failed items'**
  String get importFailedItems;

  /// Text without failed import items.
  ///
  /// In en, this message translates to:
  /// **'No failed items.'**
  String get importNoFailedItems;

  /// Heading of the table of mapped channels of an import source.
  ///
  /// In en, this message translates to:
  /// **'Channel mapping'**
  String get importChannelMapping;

  /// Text of the channel table when the mapping has no channel other than skip.
  ///
  /// In en, this message translates to:
  /// **'The mapping names no channel to import.'**
  String get importNoChannels;

  /// The Discord channel ID of a row of the channel table.
  ///
  /// In en, this message translates to:
  /// **'Discord ID {id}'**
  String importChannelDiscord(String id);

  /// Name of a mapped existing channel that could not be read.
  ///
  /// In en, this message translates to:
  /// **'Name not readable'**
  String get importChannelUnknownName;

  /// Row state: the channel item is not posted yet.
  ///
  /// In en, this message translates to:
  /// **'Not imported'**
  String get importPhaseNotImported;

  /// Row state: Discord leads, the channel here is read-only.
  ///
  /// In en, this message translates to:
  /// **'Mirror, read-only'**
  String get importPhaseMirror;

  /// Row state: the channel is led here and the operator is the planned one.
  ///
  /// In en, this message translates to:
  /// **'Led here, operator ok'**
  String get importPhaseOperatorOk;

  /// Row state: the channel is led here and the planned operator is not set.
  ///
  /// In en, this message translates to:
  /// **'Led here, no operator'**
  String get importPhaseNoOperator;

  /// Row state: the channel is led here and the operator is not the planned one.
  ///
  /// In en, this message translates to:
  /// **'Led here, other operator than planned'**
  String get importPhaseOtherOperator;

  /// The channel of a row is read-only.
  ///
  /// In en, this message translates to:
  /// **'Read-only'**
  String get importReadOnly;

  /// The channel of a row is writable.
  ///
  /// In en, this message translates to:
  /// **'Writable'**
  String get importWritable;

  /// The planned operator of a mapped channel.
  ///
  /// In en, this message translates to:
  /// **'Planned operator: {agent}'**
  String importPlanned(String agent);

  /// A mapped channel without planned operator.
  ///
  /// In en, this message translates to:
  /// **'No operator planned'**
  String get importPlannedNone;

  /// A row whose channel getChannel refused.
  ///
  /// In en, this message translates to:
  /// **'The channel could not be read.'**
  String get importChannelUnreadable;

  /// Button of a channel row that hands the lead to this server.
  ///
  /// In en, this message translates to:
  /// **'Lead here'**
  String get importLeadHereAction;

  /// Button of a channel row that hands the lead back to Discord.
  ///
  /// In en, this message translates to:
  /// **'Discord leads'**
  String get importLeadDiscordAction;

  /// Button of a channel row led here whose operator is not the planned one.
  ///
  /// In en, this message translates to:
  /// **'Set operator'**
  String get importSetOperatorAction;

  /// Button that starts or continues an import source.
  ///
  /// In en, this message translates to:
  /// **'Start'**
  String get importStart;

  /// Button that pauses an import source.
  ///
  /// In en, this message translates to:
  /// **'Pause'**
  String get importPause;

  /// Button that cancels an import source.
  ///
  /// In en, this message translates to:
  /// **'Cancel import'**
  String get importCancel;

  /// Button that completes an import source.
  ///
  /// In en, this message translates to:
  /// **'Complete'**
  String get importComplete;

  /// Why the completion is not offered yet (22 section 13.5).
  ///
  /// In en, this message translates to:
  /// **'Completing is possible once every imported channel is led here.'**
  String get importCompleteHint;

  /// Title of the confirmation of starting an import source.
  ///
  /// In en, this message translates to:
  /// **'Start {name}?'**
  String importStartTitle(String name);

  /// Effect of starting an import source.
  ///
  /// In en, this message translates to:
  /// **'The import tool may then write into this source with --apply. Nothing is imported until you use the tool.'**
  String get importStartBody;

  /// Title of the confirmation of pausing an import source.
  ///
  /// In en, this message translates to:
  /// **'Pause {name}?'**
  String importPauseTitle(String name);

  /// Effect of pausing an import source.
  ///
  /// In en, this message translates to:
  /// **'The import tool cannot write into this source until you start it again. The leads stay as they are.'**
  String get importPauseBody;

  /// Title of the confirmation of cancelling an import source.
  ///
  /// In en, this message translates to:
  /// **'Cancel {name}?'**
  String importCancelTitle(String name);

  /// Effect of cancelling an import source.
  ///
  /// In en, this message translates to:
  /// **'The source ends as cancelled and cannot be started again. What was imported stays; the import tool can no longer write into this source.'**
  String get importCancelBody;

  /// Title of the confirmation of completing an import source.
  ///
  /// In en, this message translates to:
  /// **'Complete {name}?'**
  String importCompleteTitle(String name);

  /// Effect of completing an import source with the retention warning (22 section 13.5).
  ///
  /// In en, this message translates to:
  /// **'The mirror ends and the source is completed. The mapping in import_items stays until the retention class import is deleted. After that deletion a repeated import is no longer idempotent: it would import everything again.'**
  String get importCompleteBody;

  /// Title of the confirmation that hands the lead of a channel to this server.
  ///
  /// In en, this message translates to:
  /// **'Lead {name} here?'**
  String importLeadHereTitle(String name);

  /// Introduction of the steps of switching a channel over (22 section 13.3).
  ///
  /// In en, this message translates to:
  /// **'Keep this order, so two places never lead at the same time:'**
  String get importLeadHereOrder;

  /// Step 1 of switching a channel over: the owner in Discord.
  ///
  /// In en, this message translates to:
  /// **'1. In Discord you set the channel read-only and removed the agent there.'**
  String get importLeadHereStepDiscord;

  /// Step 2 of switching a channel over: the switch command of the import tool.
  ///
  /// In en, this message translates to:
  /// **'2. {command} succeeded.'**
  String importLeadHereStepSwitch(String command);

  /// Step 3 of switching a channel over: this confirmation.
  ///
  /// In en, this message translates to:
  /// **'3. Now this server leads: the channel here becomes writable.'**
  String get importLeadHereStepLead;

  /// Step 4 of switching a channel over: the planned operator.
  ///
  /// In en, this message translates to:
  /// **'4. Then you set the operator {agent}.'**
  String importLeadHereStepOperator(String agent);

  /// Step 4 of switching a channel over without a planned operator.
  ///
  /// In en, this message translates to:
  /// **'4. The mapping plans no operator for this channel.'**
  String get importLeadHereStepNoOperator;

  /// Warning of the switch-over when the operator of the channel is not the planned one (R-IM-08).
  ///
  /// In en, this message translates to:
  /// **'The channel has another operator than the planned {agent}. Imported threads keep {agent}; with another operator they stay without an agent.'**
  String importLeadHereOtherOperator(String agent);

  /// Title of the confirmation that hands the lead of a channel back to Discord.
  ///
  /// In en, this message translates to:
  /// **'Discord leads in {name}?'**
  String importLeadDiscordTitle(String name);

  /// Effect of reverting a channel (22 section 13.4).
  ///
  /// In en, this message translates to:
  /// **'The channel here becomes read-only for everyone, owners too, and answers of agents in progress are cancelled. Messages written here stay here; {command} counts them. Afterwards you can remove the operator.'**
  String importLeadDiscordBody(String command);

  /// Notice after the lead was handed to this server.
  ///
  /// In en, this message translates to:
  /// **'{name} is led here.'**
  String importLeadHereDone(String name);

  /// Notice after the lead was handed back to Discord.
  ///
  /// In en, this message translates to:
  /// **'Discord leads in {name}; it is read-only here.'**
  String importLeadDiscordDone(String name);

  /// Notice after starting an import source.
  ///
  /// In en, this message translates to:
  /// **'{name} is started.'**
  String importStarted(String name);

  /// Notice after pausing an import source.
  ///
  /// In en, this message translates to:
  /// **'{name} is paused.'**
  String importPausedDone(String name);

  /// Notice after cancelling an import source.
  ///
  /// In en, this message translates to:
  /// **'{name} is cancelled.'**
  String importCancelledDone(String name);

  /// Notice after completing an import source.
  ///
  /// In en, this message translates to:
  /// **'{name} is completed.'**
  String importCompletedDone(String name);

  /// The server answered 409 conflict with reason import_state.
  ///
  /// In en, this message translates to:
  /// **'The source is in another state now. The page shows the current state.'**
  String get importConflictState;

  /// The server answered 409 conflict because an imported channel is not led here.
  ///
  /// In en, this message translates to:
  /// **'Not every imported channel is led here yet. Switch each channel over first.'**
  String get importConflictLeadNotHere;

  /// The server answered 409 conflict with reason channel_not_imported.
  ///
  /// In en, this message translates to:
  /// **'This channel is not imported yet.'**
  String get importConflictNotImported;

  /// Title of the dialog that adds an import source.
  ///
  /// In en, this message translates to:
  /// **'Add import source'**
  String get importCreateTitle;

  /// Field of the name of a new import source.
  ///
  /// In en, this message translates to:
  /// **'Name of the source'**
  String get importCreateName;

  /// Explanation of the mapping file in the dialog (22 section 5.2).
  ///
  /// In en, this message translates to:
  /// **'The mapping file is the YAML of the import tool (bothy import discord init-map writes a skeleton). It holds no secret and is not stored in the browser.'**
  String get importCreateHint;

  /// Button that opens the file input for the mapping file.
  ///
  /// In en, this message translates to:
  /// **'Choose mapping file'**
  String get importCreatePick;

  /// The chosen mapping file was converted.
  ///
  /// In en, this message translates to:
  /// **'Mapping file read.'**
  String get importCreateFileRead;

  /// Button that adds the import source.
  ///
  /// In en, this message translates to:
  /// **'Add'**
  String get importCreateSubmit;

  /// The mapping file could not be parsed.
  ///
  /// In en, this message translates to:
  /// **'The file is no valid YAML (line {line}, column {column}). A key named twice is refused too.'**
  String importFileInvalidYaml(int line, int column);

  /// The mapping file could not be parsed, without a position.
  ///
  /// In en, this message translates to:
  /// **'The file is no valid YAML.'**
  String get importFileInvalid;

  /// The mapping file is no mapping.
  ///
  /// In en, this message translates to:
  /// **'The file must be one YAML mapping with text keys.'**
  String get importFileNotMapping;

  /// The mapping file is not UTF-8.
  ///
  /// In en, this message translates to:
  /// **'The file is no UTF-8 text.'**
  String get importFileUnreadable;

  /// Heading of the field errors of createImport.
  ///
  /// In en, this message translates to:
  /// **'The server refused the mapping:'**
  String get importServerErrors;

  /// One field error of the mapping file: its place in the file and the problem.
  ///
  /// In en, this message translates to:
  /// **'{path}: {problem}'**
  String importFieldError(String path, String problem);

  /// Place of a field error that concerns the whole mapping.
  ///
  /// In en, this message translates to:
  /// **'the file'**
  String get importFieldWholeFile;

  /// Field error required.
  ///
  /// In en, this message translates to:
  /// **'is required'**
  String get importFieldRequired;

  /// Field error unknown_member.
  ///
  /// In en, this message translates to:
  /// **'is not a member of the mapping file'**
  String get importFieldUnknownMember;

  /// Field error invalid.
  ///
  /// In en, this message translates to:
  /// **'has a wrong type or value'**
  String get importFieldInvalid;

  /// Field error not_digits.
  ///
  /// In en, this message translates to:
  /// **'must be a Discord ID in quotes, digits only'**
  String get importFieldNotDigits;

  /// Field error duplicate.
  ///
  /// In en, this message translates to:
  /// **'names a Discord ID a second time'**
  String get importFieldDuplicate;

  /// Field error too_long.
  ///
  /// In en, this message translates to:
  /// **'is too long'**
  String get importFieldTooLong;

  /// Field error unknown_handle.
  ///
  /// In en, this message translates to:
  /// **'names no person or agent with this handle'**
  String get importFieldUnknownHandle;

  /// Field error not_active.
  ///
  /// In en, this message translates to:
  /// **'names an agent or person that is not active'**
  String get importFieldNotActive;

  /// Field error unknown_space or unknown_channel: the existing UUID names nothing.
  ///
  /// In en, this message translates to:
  /// **'names nothing that exists here'**
  String get importFieldUnknownExisting;

  /// Field error unknown_category; spaces is the key of the mapping file.
  ///
  /// In en, this message translates to:
  /// **'names no category of the list spaces'**
  String get importFieldUnknownCategory;

  /// Field error not_enabled.
  ///
  /// In en, this message translates to:
  /// **'is not available yet'**
  String get importFieldNotEnabled;

  /// Title of the confirmation of adding an import source.
  ///
  /// In en, this message translates to:
  /// **'Add {name}?'**
  String importCreateConfirmTitle(String name);

  /// Effect of adding an import source.
  ///
  /// In en, this message translates to:
  /// **'The server stores the mapping without secrets. The source starts as a draft; nothing is imported yet.'**
  String get importCreateConfirmBody;

  /// Heading of the steps after adding an import source.
  ///
  /// In en, this message translates to:
  /// **'Next steps:'**
  String get importNextSteps;

  /// First step after adding a source: the check of the tool without --apply.
  ///
  /// In en, this message translates to:
  /// **'1. Check the mapping with {command}; it only reads.'**
  String importNextCheck(String command);

  /// Second step after adding a source.
  ///
  /// In en, this message translates to:
  /// **'2. Start the source here.'**
  String get importNextStart;

  /// Third step after adding a source.
  ///
  /// In en, this message translates to:
  /// **'3. Import with {command}.'**
  String importNextApply(String command);

  /// Title of the dialog that sets the operator of an imported channel.
  ///
  /// In en, this message translates to:
  /// **'Operator of {name}'**
  String importOperatorTitle(String name);

  /// Explanation of the operator step of the switch-over (22 section 13.3 step 3).
  ///
  /// In en, this message translates to:
  /// **'The channel is led here now. Set the planned operator, so the imported threads and the channel have the same agent.'**
  String get importOperatorBody;

  /// Field of the operator dialog: the agent.
  ///
  /// In en, this message translates to:
  /// **'Agent'**
  String get importOperatorAgent;

  /// Warning of the operator dialog when another agent than the planned one is chosen (R-IM-08).
  ///
  /// In en, this message translates to:
  /// **'{agent} is not the planned operator {planned}. Imported threads keep {planned}; they stay without an agent in this channel.'**
  String importOperatorNotPlanned(String agent, String planned);

  /// Button of the operator dialog.
  ///
  /// In en, this message translates to:
  /// **'Set operator'**
  String get importOperatorSet;

  /// Button that closes the operator dialog without a change.
  ///
  /// In en, this message translates to:
  /// **'Not now'**
  String get importOperatorSkip;

  /// Notice after the operator was set.
  ///
  /// In en, this message translates to:
  /// **'{agent} is the operator of {name}.'**
  String importOperatorDone(String agent, String name);

  /// Title of the dialog that removes the operator after the lead went back to Discord.
  ///
  /// In en, this message translates to:
  /// **'Remove the operator of {name}?'**
  String importOperatorRemoveTitle(String name);

  /// Effect of removing the operator (22 section 13.4 step 1).
  ///
  /// In en, this message translates to:
  /// **'Discord leads again. Without an operator no agent answers here.'**
  String get importOperatorRemoveBody;

  /// Button that removes the operator.
  ///
  /// In en, this message translates to:
  /// **'Remove operator'**
  String get importOperatorRemove;

  /// Notice after the operator was removed.
  ///
  /// In en, this message translates to:
  /// **'{name} has no operator now.'**
  String importOperatorRemoved(String name);

  /// One failed import item: kind, origin key (Discord IDs only) and the code of the error.
  ///
  /// In en, this message translates to:
  /// **'{kind} {origin}: {error}'**
  String importFailedItem(String kind, String origin, String error);

  /// Purpose of the step-up of import.manage when creating a source.
  ///
  /// In en, this message translates to:
  /// **'Add the import source {name}'**
  String stepUpPurposeImportCreate(String name);

  /// Purpose of the step-up of import.manage when starting a source.
  ///
  /// In en, this message translates to:
  /// **'Start the import {name}'**
  String stepUpPurposeImportStart(String name);

  /// Purpose of the step-up of import.manage when pausing a source.
  ///
  /// In en, this message translates to:
  /// **'Pause the import {name}'**
  String stepUpPurposeImportPause(String name);

  /// Purpose of the step-up of import.manage when cancelling a source.
  ///
  /// In en, this message translates to:
  /// **'Cancel the import {name}'**
  String stepUpPurposeImportCancel(String name);

  /// Purpose of the step-up of import.manage when completing a source.
  ///
  /// In en, this message translates to:
  /// **'Complete the import {name}'**
  String stepUpPurposeImportComplete(String name);

  /// Purpose of the step-up of import.manage when changing the lead of a channel.
  ///
  /// In en, this message translates to:
  /// **'Change the lead of {name}'**
  String stepUpPurposeImportLead(String name);

  /// Purpose of the step-up of channel.operator.set in the import.
  ///
  /// In en, this message translates to:
  /// **'Set the operator of {name}'**
  String stepUpPurposeImportOperator(String name);

  /// Notification level all (13 section 3.1): every message notifies.
  ///
  /// In en, this message translates to:
  /// **'All'**
  String get notifyLevelAll;

  /// Notification level mentions (13 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Mentions'**
  String get notifyLevelMentions;

  /// Notification level mute (13 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'Mute'**
  String get notifyLevelMute;

  /// Shown for a target without an own level: the level it inherits (01 5.1 D9).
  ///
  /// In en, this message translates to:
  /// **'inherits: {level}'**
  String notifyInherits(String level);

  /// Menu item that deletes the own level of a target, with the inherited level and its source.
  ///
  /// In en, this message translates to:
  /// **'Inherit ({level} from {source})'**
  String notifyInheritFrom(String level, String source);

  /// Source of an effective level: the own level of the thread (step 1 of 13 section 3.1).
  ///
  /// In en, this message translates to:
  /// **'the thread'**
  String get notifySourceThread;

  /// Source of an effective level: the own level of the channel (step 2).
  ///
  /// In en, this message translates to:
  /// **'the channel'**
  String get notifySourceChannel;

  /// Source of an effective level: the own level of the space (step 3).
  ///
  /// In en, this message translates to:
  /// **'the space'**
  String get notifySourceSpace;

  /// Source of an effective level: the default level an operator set on the channel (step 4).
  ///
  /// In en, this message translates to:
  /// **'the default of the channel'**
  String get notifySourceChannelDefault;

  /// Source of an effective level: a direct message channel notifies every message (step 5).
  ///
  /// In en, this message translates to:
  /// **'the direct message'**
  String get notifySourceDm;

  /// Source of an effective level: the personal default notify.default_level (step 6).
  ///
  /// In en, this message translates to:
  /// **'your default'**
  String get notifySourcePersonalDefault;

  /// Source of an effective level: the default of the instance (step 7).
  ///
  /// In en, this message translates to:
  /// **'the default of the instance'**
  String get notifySourceInstanceDefault;

  /// Shown for a thread the person follows with level all, as the server sets it after a write in the thread (13 section 3.2).
  ///
  /// In en, this message translates to:
  /// **'Following'**
  String get notifyFollowing;

  /// Shown while a level is muted until a time.
  ///
  /// In en, this message translates to:
  /// **'muted until {when}'**
  String notifyMutedUntil(String when);

  /// Submenu of the menu of a space, channel or thread with the notification levels; also the button in the thread header that opens it.
  ///
  /// In en, this message translates to:
  /// **'Notifications'**
  String get notifyMenuNotifications;

  /// Submenu with the fixed choices of a temporary mute.
  ///
  /// In en, this message translates to:
  /// **'Mute until'**
  String get notifyMenuMuteUntil;

  /// Choice of the Mute until submenu: mute for one hour.
  ///
  /// In en, this message translates to:
  /// **'For 1 hour'**
  String get notifyMuteOneHour;

  /// Choice of the Mute until submenu: mute until tomorrow at the given time in the person's time zone.
  ///
  /// In en, this message translates to:
  /// **'Tomorrow {time}'**
  String notifyMuteTomorrow(String time);

  /// Choice of the Mute until submenu: pick the end of the mute.
  ///
  /// In en, this message translates to:
  /// **'Choose date and time'**
  String get notifyMuteChooseDate;

  /// Screen reader label of the menu of a space, channel or thread.
  ///
  /// In en, this message translates to:
  /// **'Menu of {name}'**
  String notifyMenuOf(String name);

  /// Button in the thread header that opens the menu of the thread or the channel.
  ///
  /// In en, this message translates to:
  /// **'More'**
  String get notifyMenuOpen;

  /// Item of the channel menu, only where the server gives the capability export (07 section 2.4).
  ///
  /// In en, this message translates to:
  /// **'Export history'**
  String get channelMenuExport;

  /// Title of the export dialog of a channel.
  ///
  /// In en, this message translates to:
  /// **'Export {name}'**
  String exportTitle(String name);

  /// First day of the export range (included).
  ///
  /// In en, this message translates to:
  /// **'From'**
  String get exportFrom;

  /// Last day of the export range (included).
  ///
  /// In en, this message translates to:
  /// **'To'**
  String get exportTo;

  /// Error of the export dialog for a range that ends before it starts.
  ///
  /// In en, this message translates to:
  /// **'The first day must not be after the last day.'**
  String get exportRangeInvalid;

  /// Button of the export dialog: asks for the file, then streams the history into it.
  ///
  /// In en, this message translates to:
  /// **'Choose file and export'**
  String get exportStart;

  /// Progress of a running export.
  ///
  /// In en, this message translates to:
  /// **'{size} written'**
  String exportProgress(String size);

  /// Shown when the export is complete.
  ///
  /// In en, this message translates to:
  /// **'Export saved: {size}.'**
  String exportDone(String size);

  /// Shown after the person cancelled a running export.
  ///
  /// In en, this message translates to:
  /// **'Export cancelled. The incomplete file was deleted.'**
  String get exportCancelled;

  /// Shown when the stream ended with an error after it started.
  ///
  /// In en, this message translates to:
  /// **'The export broke off. The incomplete file was deleted.'**
  String get exportBrokeOff;

  /// Button of the export dialog: closes it, or aborts a running export.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get exportCancel;

  /// Button of the export dialog after the export ended.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get exportClose;

  /// Purpose shown in the step-up dialog of a channel export.
  ///
  /// In en, this message translates to:
  /// **'Export the history of {name}'**
  String stepUpPurposeChannelExport(String name);

  /// Row of D9: the personal default notify.default_level.
  ///
  /// In en, this message translates to:
  /// **'Default for each new channel'**
  String get notifySettingsDefault;

  /// Hint of the personal default level in D9.
  ///
  /// In en, this message translates to:
  /// **'Applies where nothing else is set.'**
  String get notifySettingsDefaultHint;

  /// Row of D9 that explains the notifications of the person's own runs (01 section 10.1); there is no setting for it.
  ///
  /// In en, this message translates to:
  /// **'Runs I triggered'**
  String get notifySettingsRuns;

  /// Explanation of the row Runs I triggered in D9 (01 section 10.1).
  ///
  /// In en, this message translates to:
  /// **'A question of your run always notifies you, also when muted. Its end or failure follows the level.'**
  String get notifySettingsRunsExplain;

  /// Row of D9: notify.preview_mode.
  ///
  /// In en, this message translates to:
  /// **'Content in notifications'**
  String get notifySettingsPreview;

  /// notify.preview_mode full: the app loads the text (13 section 6.4).
  ///
  /// In en, this message translates to:
  /// **'Text and names'**
  String get notifyPreviewFull;

  /// notify.preview_mode names: only channel, thread and sender names.
  ///
  /// In en, this message translates to:
  /// **'Names only'**
  String get notifyPreviewNames;

  /// notify.preview_mode none: only the placeholder text.
  ///
  /// In en, this message translates to:
  /// **'Only New message'**
  String get notifyPreviewNone;

  /// Row of D9 with the quiet hours rules (13 section 3.4).
  ///
  /// In en, this message translates to:
  /// **'Quiet hours'**
  String get notifySettingsQuietHours;

  /// Row of D9: notify.input_needed_breaks_quiet_hours.
  ///
  /// In en, this message translates to:
  /// **'Questions break quiet hours'**
  String get notifySettingsInputBreaks;

  /// Hint of the switch Questions break quiet hours.
  ///
  /// In en, this message translates to:
  /// **'A question of your own run stays audible.'**
  String get notifySettingsInputBreaksHint;

  /// Row of D9 for owners: notify.admin_enabled.
  ///
  /// In en, this message translates to:
  /// **'Administration reminders'**
  String get notifySettingsAdmin;

  /// Hint of the switch Administration reminders.
  ///
  /// In en, this message translates to:
  /// **'For example an access grant that expires.'**
  String get notifySettingsAdminHint;

  /// Heading of the tree of D9 with the level of each target.
  ///
  /// In en, this message translates to:
  /// **'Per space, channel and thread'**
  String get notifySettingsTree;

  /// Shown in the tree of D9 while no space or channel is known.
  ///
  /// In en, this message translates to:
  /// **'Nothing to show yet.'**
  String get notifySettingsTreeEmpty;

  /// Choice of the level control in the tree of D9: delete the own level.
  ///
  /// In en, this message translates to:
  /// **'Inherit'**
  String get notifyChoiceInherit;

  /// Shown in D9 while the person has no quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'No quiet hours.'**
  String get quietHoursNone;

  /// Button of D9 that opens the editor for a new quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Add quiet hours'**
  String get quietHoursAdd;

  /// Button of a quiet hours rule in D9.
  ///
  /// In en, this message translates to:
  /// **'Edit'**
  String get quietHoursEdit;

  /// Button of a quiet hours rule in D9 and its editor.
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get quietHoursDelete;

  /// A quiet hours rule from 00:00 to 00:00, 24 hours on each chosen day (D-M3-17-4).
  ///
  /// In en, this message translates to:
  /// **'All day'**
  String get quietHoursAllDay;

  /// The time range of a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'{start} to {end}'**
  String quietHoursRange(String start, String end);

  /// Marks a disabled quiet hours rule in D9.
  ///
  /// In en, this message translates to:
  /// **'off'**
  String get quietHoursOff;

  /// Short name of Monday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Mon'**
  String get quietHoursDayMon;

  /// Short name of Tuesday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Tue'**
  String get quietHoursDayTue;

  /// Short name of Wednesday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Wed'**
  String get quietHoursDayWed;

  /// Short name of Thursday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Thu'**
  String get quietHoursDayThu;

  /// Short name of Friday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Fri'**
  String get quietHoursDayFri;

  /// Short name of Saturday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Sat'**
  String get quietHoursDaySat;

  /// Short name of Sunday in a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Sun'**
  String get quietHoursDaySun;

  /// Title of the editor of one quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Quiet hours'**
  String get quietHoursEditorTitle;

  /// Label of the weekday choice of a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Days'**
  String get quietHoursDays;

  /// Start time of a quiet hours rule (HH:MM).
  ///
  /// In en, this message translates to:
  /// **'Start'**
  String get quietHoursStart;

  /// End time of a quiet hours rule (HH:MM); before the start means across midnight.
  ///
  /// In en, this message translates to:
  /// **'End'**
  String get quietHoursEnd;

  /// Time zone of a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'Time zone'**
  String get quietHoursTimeZone;

  /// Choice of the time zone of a quiet hours rule: follow the person's time zone.
  ///
  /// In en, this message translates to:
  /// **'Mine ({zone})'**
  String quietHoursTimeZoneMine(String zone);

  /// Switch of a quiet hours rule.
  ///
  /// In en, this message translates to:
  /// **'On'**
  String get quietHoursEnabled;

  /// Button of the quiet hours editor.
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get quietHoursSave;

  /// Button of the quiet hours editor.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get quietHoursCancel;

  /// Error of a time field of the quiet hours editor.
  ///
  /// In en, this message translates to:
  /// **'Use HH:MM, for example 07:30.'**
  String get quietHoursTimeInvalid;

  /// Error of the quiet hours editor without a weekday.
  ///
  /// In en, this message translates to:
  /// **'Choose at least one day.'**
  String get quietHoursNoDays;

  /// Heading of the search view D7 (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'Search'**
  String get searchTitle;

  /// Label of the text field of the search view D7.
  ///
  /// In en, this message translates to:
  /// **'Search text'**
  String get searchFieldLabel;

  /// Placeholder of the search field; the notation from:, in:, has:file, before: and after: is accepted (14 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'Search messages, threads and files'**
  String get searchFieldHint;

  /// Button that starts the search.
  ///
  /// In en, this message translates to:
  /// **'Search'**
  String get searchSubmit;

  /// Heading of the filter column of D7.
  ///
  /// In en, this message translates to:
  /// **'Filter'**
  String get searchFilters;

  /// Filter of the search by space (14 section 4.4).
  ///
  /// In en, this message translates to:
  /// **'Space'**
  String get searchFilterSpace;

  /// Filter of the search by channel (14 section 4.4).
  ///
  /// In en, this message translates to:
  /// **'Channel'**
  String get searchFilterChannel;

  /// Filter of the search by author (14 section 4.4).
  ///
  /// In en, this message translates to:
  /// **'From'**
  String get searchFilterFrom;

  /// Value of a filter that does not restrict the search.
  ///
  /// In en, this message translates to:
  /// **'all'**
  String get searchFilterAll;

  /// Filter group of the result type (14 section 11).
  ///
  /// In en, this message translates to:
  /// **'Type'**
  String get searchFilterType;

  /// Result type messages.
  ///
  /// In en, this message translates to:
  /// **'Messages'**
  String get searchTypeMessages;

  /// Result type thread titles.
  ///
  /// In en, this message translates to:
  /// **'Threads'**
  String get searchTypeThreads;

  /// Result type files: names and transcripts (14 section 2).
  ///
  /// In en, this message translates to:
  /// **'Files'**
  String get searchTypeFiles;

  /// Filter group of the time range (14 section 4.4, after and before).
  ///
  /// In en, this message translates to:
  /// **'Time'**
  String get searchFilterTime;

  /// Time filter without a limit.
  ///
  /// In en, this message translates to:
  /// **'any time'**
  String get searchTimeAny;

  /// Time filter of the last day.
  ///
  /// In en, this message translates to:
  /// **'last 24 hours'**
  String get searchTimeDay;

  /// Time filter of the last week.
  ///
  /// In en, this message translates to:
  /// **'last 7 days'**
  String get searchTimeWeek;

  /// Time filter of the last month.
  ///
  /// In en, this message translates to:
  /// **'last 30 days'**
  String get searchTimeMonth;

  /// Filter group of the thread state (14 section 4.4, thread_state).
  ///
  /// In en, this message translates to:
  /// **'Status'**
  String get searchFilterStatus;

  /// Thread state filter that keeps both states.
  ///
  /// In en, this message translates to:
  /// **'open and done'**
  String get searchStatusAll;

  /// Thread state filter open.
  ///
  /// In en, this message translates to:
  /// **'open'**
  String get searchStatusOpen;

  /// Thread state filter closed (term Done of 01 section 9).
  ///
  /// In en, this message translates to:
  /// **'done'**
  String get searchStatusDone;

  /// Filter of messages with at least one attachment (14 section 4.4, has_file).
  ///
  /// In en, this message translates to:
  /// **'With files only'**
  String get searchHasFile;

  /// Filter group of the order (14 section 4.5).
  ///
  /// In en, this message translates to:
  /// **'Sort'**
  String get searchSort;

  /// Order by relevance.
  ///
  /// In en, this message translates to:
  /// **'Relevance'**
  String get searchSortRelevance;

  /// Order by creation time.
  ///
  /// In en, this message translates to:
  /// **'Newest first'**
  String get searchSortNewest;

  /// Count of the loaded hits above the list of D7; it counts only what is visible (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 result, only from channels you may see} other{{count} results, only from channels you may see}}'**
  String searchResultCount(int count);

  /// The search found nothing.
  ///
  /// In en, this message translates to:
  /// **'No results'**
  String get searchNoResults;

  /// Note above local hits of the offline search (14 section 6).
  ///
  /// In en, this message translates to:
  /// **'Messages stored only on this device'**
  String get searchLocalOnly;

  /// Note when the server reports candidates_capped (14 section 4.5).
  ///
  /// In en, this message translates to:
  /// **'There are too many matches. Narrow the search to see the best ones.'**
  String get searchCandidatesCapped;

  /// Button that loads the next page of hits.
  ///
  /// In en, this message translates to:
  /// **'Load more'**
  String get searchLoadMore;

  /// The server rejected the search text with the reason empty (14 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'Enter at least one word.'**
  String get searchQueryEmpty;

  /// The server rejected the search text with the reason too_long (14 section 4.1).
  ///
  /// In en, this message translates to:
  /// **'The search text is too long.'**
  String get searchQueryTooLong;

  /// The search request failed for another reason.
  ///
  /// In en, this message translates to:
  /// **'The search did not work. Try again.'**
  String get searchFailed;

  /// Marker of a hit on a thread title.
  ///
  /// In en, this message translates to:
  /// **'Thread'**
  String get searchHitThread;

  /// Marker of a hit on a file.
  ///
  /// In en, this message translates to:
  /// **'File'**
  String get searchHitFile;

  /// Marker of a local hit in the transcript of a voice message.
  ///
  /// In en, this message translates to:
  /// **'Transcript'**
  String get searchHitTranscript;

  /// Screen reader label of one search hit.
  ///
  /// In en, this message translates to:
  /// **'{path}, {author}, {time}: {text}'**
  String searchHitLabel(String path, String author, String time, String text);

  /// Screen reader name of the quick switcher D6 (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'Quick switcher'**
  String get switcherLabel;

  /// Placeholder of the field of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Jump to a channel, thread, person, agent or command'**
  String get switcherHint;

  /// Filter of the quick switcher without a restriction.
  ///
  /// In en, this message translates to:
  /// **'All'**
  String get switcherFilterAll;

  /// Filter of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Channels'**
  String get switcherFilterChannels;

  /// Filter of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Threads'**
  String get switcherFilterThreads;

  /// Filter of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'People'**
  String get switcherFilterPeople;

  /// Filter of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Agents'**
  String get switcherFilterAgents;

  /// Filter of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Commands'**
  String get switcherFilterCommands;

  /// Footer of the quick switcher: Tab cycles the filter row.
  ///
  /// In en, this message translates to:
  /// **'Tab: Filter'**
  String get switcherTabHint;

  /// Kind of a hit of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Space'**
  String get switcherKindSpace;

  /// Kind of a hit of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Channel'**
  String get switcherKindChannel;

  /// Kind of a hit of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Thread'**
  String get switcherKindThread;

  /// Kind of a hit of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Person'**
  String get switcherKindPerson;

  /// Kind of a hit of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Agent'**
  String get switcherKindAgent;

  /// Kind of a hit of the quick switcher.
  ///
  /// In en, this message translates to:
  /// **'Command'**
  String get switcherKindCommand;

  /// The quick switcher has no hit.
  ///
  /// In en, this message translates to:
  /// **'Nothing found'**
  String get switcherNothing;

  /// Button that closes the quick switcher (Esc).
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get switcherClose;

  /// Heading of the view of the own mentions (01 section 3.5).
  ///
  /// In en, this message translates to:
  /// **'Mentions'**
  String get mentionsTitle;

  /// The mentions view has no entry.
  ///
  /// In en, this message translates to:
  /// **'Nobody has mentioned you yet.'**
  String get mentionsEmpty;

  /// listMyMentions failed.
  ///
  /// In en, this message translates to:
  /// **'The mentions could not be loaded. Try again.'**
  String get mentionsFailed;

  /// Button that loads the mentions again.
  ///
  /// In en, this message translates to:
  /// **'Try again'**
  String get mentionsRetry;

  /// Heading of the view D10 with the count of shown runs (01 section 5.1).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{Running now} other{Running now ({count})}}'**
  String runningTitle(int count);

  /// Filter of Running now by space.
  ///
  /// In en, this message translates to:
  /// **'Space'**
  String get runningFilterSpace;

  /// Filter of Running now by agent.
  ///
  /// In en, this message translates to:
  /// **'Agent'**
  String get runningFilterAgent;

  /// Value of a filter of Running now that does not restrict.
  ///
  /// In en, this message translates to:
  /// **'all'**
  String get runningFilterAll;

  /// Filter of Running now: only runs the person triggered.
  ///
  /// In en, this message translates to:
  /// **'mine only'**
  String get runningMineOnly;

  /// Button that stops a run; shown only where the server allows run.cancel (01 D10).
  ///
  /// In en, this message translates to:
  /// **'Stop'**
  String get runningStop;

  /// Button that opens the thread of a run.
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get runningOpen;

  /// Running now has no row.
  ///
  /// In en, this message translates to:
  /// **'No run is active.'**
  String get runningEmpty;

  /// Footer of D10: the sum of the shown runs in Organization mode; estimate is the text of costEstimate (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Organization: sum of active runs {amount} ({estimate}).'**
  String runningSumOrganization(String amount, String estimate);

  /// Footer of D10 about the costs of runs in Personal mode (01 section 6.7).
  ///
  /// In en, this message translates to:
  /// **'Personal runs: information only.'**
  String get runningPersonalNote;

  /// Fill level of the context window of a run in a row of D10.
  ///
  /// In en, this message translates to:
  /// **'{percent} context'**
  String runningContext(String percent);

  /// Screen reader label of a row of Running now.
  ///
  /// In en, this message translates to:
  /// **'{agent}, {path}, {state}, {duration}'**
  String runningRowLabel(
    String agent,
    String path,
    String state,
    String duration,
  );

  /// listActiveRuns failed; the view shows the local runs table.
  ///
  /// In en, this message translates to:
  /// **'The active runs could not be loaded; the list shows what this device knows.'**
  String get runningFailed;

  /// Screen reader label of the sidebar entry Running now with its counter.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{Running now, 1 active run} other{Running now, {count} active runs}}'**
  String sidebarRunningNowCount(int count);

  /// Web console terminal (24 section 5.1, 15 section 23.3, ADR 0070): the sentence below the dialog 'Open terminal' and the key change warning; the console's code comes from the server, so the pin cannot protect against it.
  ///
  /// In en, this message translates to:
  /// **'In the browser this check only notices a key change that the server reports; it does not protect against the server itself.'**
  String get consoleTerminalPinningNotice;

  /// Web console terminal (ADR 0070 item 1): title of the notice when the person has no passkey.
  ///
  /// In en, this message translates to:
  /// **'A passkey is needed'**
  String get consoleTerminalPasskeyTitle;

  /// Web console terminal (ADR 0070 item 1, 08 section 6.3 item 3): the way to a passkey for a person without one; no other step-up opens a terminal in the console.
  ///
  /// In en, this message translates to:
  /// **'Opening a terminal in the console needs a passkey with user verification, and your account has none yet. Set up a passkey for your account first, then open the terminal again. In the desktop and mobile apps the terminal opens with the device key instead.'**
  String get consoleTerminalPasskeySetup;

  /// Web console terminal: button that closes the passkey notice.
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get consoleTerminalPasskeyClose;

  /// Web console page 'Hosts and terminal' (01 section 5.5): button on the page of a host that opens the terminal.
  ///
  /// In en, this message translates to:
  /// **'Open terminal'**
  String get consoleTerminalOpen;

  /// Web console terminal: button that closes the view of a session that no longer runs and shows the host again.
  ///
  /// In en, this message translates to:
  /// **'Back to host'**
  String get consoleTerminalBack;

  /// Lock note of a card when the reader lacks the level trigger in the channel (09 11.2, components.md 8).
  ///
  /// In en, this message translates to:
  /// **'You may not answer agents in this channel'**
  String get componentsDeniedLevel;

  /// Reason of a run that ended because the adapter no longer reported it (09 5.5, reason interrupted).
  ///
  /// In en, this message translates to:
  /// **'Run interrupted'**
  String get liveBoxReasonInterrupted;

  /// Button after an interrupted or stale run that puts the triggering text into the outbox again (09 5.5).
  ///
  /// In en, this message translates to:
  /// **'Send again'**
  String get liveBoxSendAgain;

  /// Typing line of an agent while its run is running (D-L9).
  ///
  /// In en, this message translates to:
  /// **'{agent} is working'**
  String agentWorking(String agent);

  /// Description of the built-in command /stop (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Stop the active run'**
  String get commandBuiltinStop;

  /// Description of the built-in command /status (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Show the run and session details'**
  String get commandBuiltinStatus;

  /// Description of the built-in command /new (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Start a new agent session'**
  String get commandBuiltinNew;

  /// Description of the built-in command /resume (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Continue an earlier session'**
  String get commandBuiltinResume;

  /// Description of the built-in command /title (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Set the thread title'**
  String get commandBuiltinTitle;

  /// Description of the built-in command /close (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Mark the thread done'**
  String get commandBuiltinClose;

  /// Title of the dialog of /status (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Run and session'**
  String get commandStatusTitle;

  /// Line of the /status dialog without an active run.
  ///
  /// In en, this message translates to:
  /// **'No run is active'**
  String get commandStatusNoRun;

  /// Title of the selection of /resume (09 10.1).
  ///
  /// In en, this message translates to:
  /// **'Continue a session'**
  String get commandResumeTitle;

  /// The selection of /resume without sessions.
  ///
  /// In en, this message translates to:
  /// **'No earlier sessions'**
  String get commandResumeEmpty;

  /// Session column, agent part: the backend policy row (01 D1).
  ///
  /// In en, this message translates to:
  /// **'Backend'**
  String get sessionAgentBackend;

  /// Session column, agent part: the model row.
  ///
  /// In en, this message translates to:
  /// **'Model'**
  String get sessionAgentModel;

  /// Session column, agent part: the agent session row.
  ///
  /// In en, this message translates to:
  /// **'Session'**
  String get sessionAgentSession;

  /// Session column, agent part: the project binding row.
  ///
  /// In en, this message translates to:
  /// **'Project'**
  String get sessionAgentProject;

  /// Session column, agent part: the context share row.
  ///
  /// In en, this message translates to:
  /// **'Context'**
  String get sessionAgentContext;

  /// Session column, agent part: the thread cost row (01 6.7).
  ///
  /// In en, this message translates to:
  /// **'Thread cost'**
  String get sessionAgentThreadCost;

  /// Session column, agent part: the number of runs.
  ///
  /// In en, this message translates to:
  /// **'Runs'**
  String get sessionAgentRuns;

  /// Session column: a run count with more pages.
  ///
  /// In en, this message translates to:
  /// **'{count}+'**
  String sessionAgentRunsMore(int count);

  /// Session column, agent part: heading of the earlier sessions.
  ///
  /// In en, this message translates to:
  /// **'Earlier sessions ({count})'**
  String sessionAgentEarlier(int count);

  /// Note when a built-in command the server's answers do not allow is sent (09 10.1: never sent as a message).
  ///
  /// In en, this message translates to:
  /// **'{command} is not available here; nothing was sent'**
  String commandNotAvailable(String command);

  /// Title of the list of shortcuts (Cmd+/ or Ctrl+/, 01 7; M4-24).
  ///
  /// In en, this message translates to:
  /// **'Keyboard shortcuts'**
  String get shortcutListTitle;

  /// Button that closes the list of shortcuts (M4-24).
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get shortcutListClose;

  /// Note after a shortcut whose action does not exist yet (Open hosts and terminal before M4-36).
  ///
  /// In en, this message translates to:
  /// **'Not available yet'**
  String get shortcutListDisabled;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Quick switcher'**
  String get shortcutQuickSwitcher;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Search'**
  String get shortcutSearch;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Search in thread'**
  String get shortcutSearchInThread;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'New post or new chat in the current forum'**
  String get shortcutNewPost;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Focus the input'**
  String get shortcutFocusInput;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Next or previous channel'**
  String get shortcutChannel;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Next or previous unread channel'**
  String get shortcutUnreadChannel;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Switch column (sidebar, list, history, session)'**
  String get shortcutColumn;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Mark thread as read'**
  String get shortcutMarkRead;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Mark as unread (from a message)'**
  String get shortcutMarkUnread;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Mark thread as done or reopen it'**
  String get shortcutDone;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Stop the run in the current thread'**
  String get shortcutStopRun;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Expand and collapse the live box'**
  String get shortcutLiveBox;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Show and hide the session column'**
  String get shortcutSessionColumn;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Edit the last own message'**
  String get shortcutEditLast;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Send / new line'**
  String get shortcutSend;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Running now'**
  String get shortcutRunningNow;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Mentions'**
  String get shortcutMentions;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get shortcutSettings;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'List of shortcuts'**
  String get shortcutList;

  /// Row of the list of shortcuts (01 7).
  ///
  /// In en, this message translates to:
  /// **'Open hosts and terminal'**
  String get shortcutHostsTerminal;

  /// Name of the Command key of macOS in the list of shortcuts.
  ///
  /// In en, this message translates to:
  /// **'Cmd'**
  String get shortcutKeyCmd;

  /// Name of the Control key in the list of shortcuts.
  ///
  /// In en, this message translates to:
  /// **'Ctrl'**
  String get shortcutKeyCtrl;

  /// Name of the Option key of macOS in the list of shortcuts.
  ///
  /// In en, this message translates to:
  /// **'Option'**
  String get shortcutKeyOption;

  /// Name of the Alt key of Windows in the list of shortcuts.
  ///
  /// In en, this message translates to:
  /// **'Alt'**
  String get shortcutKeyAlt;

  /// Name of the Shift key in the list of shortcuts.
  ///
  /// In en, this message translates to:
  /// **'Shift'**
  String get shortcutKeyShift;

  /// The word before the direction of an arrow key (Arrow up).
  ///
  /// In en, this message translates to:
  /// **'Arrow'**
  String get shortcutKeyArrow;

  /// Direction of the arrow key up.
  ///
  /// In en, this message translates to:
  /// **'up'**
  String get shortcutKeyUp;

  /// Direction of the arrow key down.
  ///
  /// In en, this message translates to:
  /// **'down'**
  String get shortcutKeyDown;

  /// Name of the Escape key.
  ///
  /// In en, this message translates to:
  /// **'Esc'**
  String get shortcutKeyEscape;

  /// Name of the Tab key.
  ///
  /// In en, this message translates to:
  /// **'Tab'**
  String get shortcutKeyTab;

  /// Name of the Enter key.
  ///
  /// In en, this message translates to:
  /// **'Enter'**
  String get shortcutKeyEnter;

  /// A click of the mouse in a shortcut (Option+Click).
  ///
  /// In en, this message translates to:
  /// **'Click'**
  String get shortcutKeyClick;

  /// Word between two keys pressed one after the other (Esc then Tab).
  ///
  /// In en, this message translates to:
  /// **'then'**
  String get shortcutKeyThen;

  /// Word between two alternative shortcuts of one action.
  ///
  /// In en, this message translates to:
  /// **'or'**
  String get shortcutKeyOr;

  /// Remark after arrow up: it edits the last own message only in the empty input.
  ///
  /// In en, this message translates to:
  /// **'in the empty field'**
  String get shortcutInEmptyField;

  /// Remark after Enter / Shift+Enter: the setting Enter sends swaps them.
  ///
  /// In en, this message translates to:
  /// **'(reversible)'**
  String get shortcutReversible;

  /// Title of the macOS menu with the navigation actions (M4-24).
  ///
  /// In en, this message translates to:
  /// **'Go'**
  String get menuGo;

  /// Title of the macOS menu with the thread actions (M4-24).
  ///
  /// In en, this message translates to:
  /// **'Thread'**
  String get menuThread;

  /// Title of the macOS menu Window (M4-24).
  ///
  /// In en, this message translates to:
  /// **'Window'**
  String get menuWindow;

  /// Title of the macOS menu Help (M4-24).
  ///
  /// In en, this message translates to:
  /// **'Help'**
  String get menuHelp;

  /// Title of the in-thread search (Cmd+F or Ctrl+F, M4-24).
  ///
  /// In en, this message translates to:
  /// **'Search in this thread'**
  String get threadSearchTitle;

  /// Phone tab bar: the tab with the spaces, channels and threads (01 section 5.3 M1).
  ///
  /// In en, this message translates to:
  /// **'Spaces'**
  String get phoneTabSpaces;

  /// Phone tab bar: the tab with the mentions of the person (01 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'Mentions'**
  String get phoneTabMentions;

  /// Phone tab bar: the tab with the active runs (Running now, 01 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'Running'**
  String get phoneTabRunning;

  /// Phone tab bar: the tab of the person with settings, devices and logout (01 section 5.3).
  ///
  /// In en, this message translates to:
  /// **'Me'**
  String get phoneTabMe;

  /// Phone: the back button when the page below has no name yet.
  ///
  /// In en, this message translates to:
  /// **'Back'**
  String get phoneBack;

  /// Phone: the screen reader label of the back button, naming the page below.
  ///
  /// In en, this message translates to:
  /// **'Back to {name}'**
  String phoneBackTo(String name);

  /// Phone: the overflow menu of the thread header (01 section 5.3 M3).
  ///
  /// In en, this message translates to:
  /// **'More actions'**
  String get phoneMoreActions;

  /// Phone: the offline banner with the number of outbox entries waiting to be sent (01 section 5.3 M7).
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =0{Offline} =1{Offline. 1 change waiting} other{Offline. {count} changes waiting}}'**
  String phoneOfflineWaiting(int count);

  /// Phone: the button that opens the search filters as a sheet.
  ///
  /// In en, this message translates to:
  /// **'Filters'**
  String get phoneSearchFilters;

  /// Message components on the phone: the button that opens a form as a sheet (01 section 5.3 M4).
  ///
  /// In en, this message translates to:
  /// **'Fill in'**
  String get componentsOpenForm;

  /// Android notification channel (01 section 10.3): mentions and direct messages; the name in the system settings.
  ///
  /// In en, this message translates to:
  /// **'Mentions'**
  String get pushChannelMentions;

  /// Android notification channel Mentions: the description in the system settings.
  ///
  /// In en, this message translates to:
  /// **'When someone mentions you or writes to you directly'**
  String get pushChannelMentionsDescription;

  /// Android notification channel (01 section 10.3): questions and approvals of runs, new devices and administration reminders.
  ///
  /// In en, this message translates to:
  /// **'Questions'**
  String get pushChannelQuestions;

  /// Android notification channel Questions: the description in the system settings.
  ///
  /// In en, this message translates to:
  /// **'When an agent asks you, a new device of yours waits for approval, or the administration needs you'**
  String get pushChannelQuestionsDescription;

  /// Android notification channel (01 section 10.3): every new message at the level All.
  ///
  /// In en, this message translates to:
  /// **'All messages'**
  String get pushChannelMessages;

  /// Android notification channel All messages: the description in the system settings.
  ///
  /// In en, this message translates to:
  /// **'New messages in channels and threads you follow with All'**
  String get pushChannelMessagesDescription;

  /// Android notification channel (01 section 10.3): the end of runs you started.
  ///
  /// In en, this message translates to:
  /// **'Runs'**
  String get pushChannelRuns;

  /// Android notification channel Runs: the description in the system settings.
  ///
  /// In en, this message translates to:
  /// **'When a run you started is done or failed'**
  String get pushChannelRunsDescription;
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

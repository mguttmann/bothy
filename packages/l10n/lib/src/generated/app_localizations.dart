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

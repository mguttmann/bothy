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

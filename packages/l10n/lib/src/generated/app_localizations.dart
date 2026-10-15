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

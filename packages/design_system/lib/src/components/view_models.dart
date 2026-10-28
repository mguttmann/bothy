// SPDX-License-Identifier: Apache-2.0

/// The view models of the core components (docs/DECISIONS.md D-M4-07-1).
///
/// Components take these small immutable values instead of domain entities, so they stay
/// testable with literal data and never import a provider or a store; the apps map their
/// entities to them (M4-08). Every enum carries the code of docs/spec/03 as [DsCode.code]
/// and an `unknown` member for a value a newer server may send (06 section 13); `fromCode`
/// never throws.
library;

import 'package:flutter/foundation.dart';

/// An enum value with the code of docs/spec/03.
abstract interface class DsCode {
  /// The code as the server sends it, for example `waiting_input`; empty for `unknown`.
  String get code;
}

/// Looks up [code] in [values]; [unknown] when no value has that code.
T _fromCode<T extends DsCode>(List<T> values, String? code, T unknown) {
  for (final value in values) {
    if (value.code == code && value != unknown) return value;
  }
  return unknown;
}

/// Who wrote a message (`messages.origin`).
enum DsMessageOrigin implements DsCode {
  /// A person.
  human('human'),

  /// An agent.
  agent('agent'),

  /// An app.
  app('app'),

  /// An import.
  import('import'),

  /// The server.
  system('system'),

  /// A value this app does not know; rendered like `system` with no badge text of its own.
  unknown('');

  const DsMessageOrigin(this.code);

  @override
  final String code;

  /// The origin of [code]; [unknown] for any other value.
  static DsMessageOrigin fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The kind of a message (`messages.kind`).
enum DsMessageKind implements DsCode {
  /// Markdown text.
  text('text'),

  /// A voice message with an optional transcript.
  voice('voice'),

  /// The anchor of a live box.
  run('run'),

  /// A message with components (question, approval).
  card('card'),

  /// One line written by the server.
  system('system'),

  /// A value this app does not know; rendered as text.
  unknown('');

  const DsMessageKind(this.code);

  @override
  final String code;

  /// The kind of [code]; [unknown] for any other value.
  static DsMessageKind fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The trigger state of a triggering message (`messages.trigger_state`).
enum DsTriggerState implements DsCode {
  /// The server has accepted the message.
  received('received'),

  /// A run was created and waits.
  queued('queued'),

  /// The agent works.
  working('working'),

  /// The run is finished.
  done('done'),

  /// Triggering or the run failed.
  failed('failed'),

  /// The run was canceled.
  cancelled('cancelled'),

  /// The follow-up reached the running agent.
  followupDelivered('followup_delivered'),

  /// A value this app does not know.
  unknown('');

  const DsTriggerState(this.code);

  @override
  final String code;

  /// The state of [code]; [unknown] for any other value.
  static DsTriggerState fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The state of a run (`runs.state`).
enum DsRunState implements DsCode {
  /// Created, not started.
  queued('queued'),

  /// The agent works.
  running('running'),

  /// The run waits for an answer.
  waitingInput('waiting_input'),

  /// Finished.
  completed('completed'),

  /// Failed, with a reason.
  failed('failed'),

  /// Canceled.
  cancelled('cancelled'),

  /// A value this app does not know.
  unknown('');

  const DsRunState(this.code);

  @override
  final String code;

  /// Whether the run is still active (`queued`, `running`, `waiting_input`).
  bool get isActive =>
      this == queued || this == running || this == waitingInput;

  /// The state of [code]; [unknown] for any other value.
  static DsRunState fromCode(String? code) => _fromCode(values, code, unknown);
}

/// The kind of a step of a run (`run_steps.kind`).
enum DsStepKind implements DsCode {
  /// A tool call.
  toolCall('tool_call'),

  /// A status step.
  status('status'),

  /// A subagent.
  subagent('subagent'),

  /// A workflow.
  workflow('workflow'),

  /// A background job.
  backgroundJob('background_job'),

  /// A value this app does not know.
  unknown('');

  const DsStepKind(this.code);

  @override
  final String code;

  /// The kind of [code]; [unknown] for any other value.
  static DsStepKind fromCode(String? code) => _fromCode(values, code, unknown);
}

/// The status of a step (`run_steps.status`).
enum DsStepStatus implements DsCode {
  /// Not started.
  pending('pending'),

  /// Running.
  running('running'),

  /// Finished.
  completed('completed'),

  /// Failed.
  failed('failed'),

  /// Canceled.
  cancelled('cancelled'),

  /// A value this app does not know.
  unknown('');

  const DsStepStatus(this.code);

  @override
  final String code;

  /// The status of [code]; [unknown] for any other value.
  static DsStepStatus fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The backend policy of an agent (`agents.billing_mode`, `reported_billing_mode`).
enum DsBillingMode implements DsCode {
  /// Runs on the subscription of one person.
  personal('personal'),

  /// Runs on the organization's backend.
  organization('organization'),

  /// A value this app does not know.
  unknown('');

  const DsBillingMode(this.code);

  @override
  final String code;

  /// The mode of [code]; [unknown] for any other value.
  static DsBillingMode fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The type of a component on a message (`message_components.type`).
enum DsComponentType implements DsCode {
  /// A button.
  button('button'),

  /// A select field.
  select('select'),

  /// A form.
  form('form'),

  /// A value this app does not know; rendered as a note that a newer app is needed.
  unknown('');

  const DsComponentType(this.code);

  @override
  final String code;

  /// The type of [code]; [unknown] for any other value.
  static DsComponentType fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The state of the components of a card (`message_components.state`).
enum DsComponentState implements DsCode {
  /// Operable.
  active('active'),

  /// Used by someone.
  used('used'),

  /// Expired (`component.default_ttl`).
  expired('expired'),

  /// Disabled, for example at the end of the run.
  disabled('disabled'),

  /// A value this app does not know; rendered like `disabled`.
  unknown('');

  const DsComponentState(this.code);

  @override
  final String code;

  /// The state of [code]; [unknown] for any other value.
  static DsComponentState fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The style role of a button (`component.v1` `style`); never a free color.
enum DsButtonStyle implements DsCode {
  /// Main action, `color.accent.bg`.
  primary('primary'),

  /// Bordered, `color.border.control`.
  secondary('secondary'),

  /// Positive, `color.success.bg`.
  success('success'),

  /// Destructive, `color.danger.bg`.
  danger('danger'),

  /// Quiet, text only (components.md "quiet").
  ghost('ghost'),

  /// A value this app does not know; rendered as `secondary`.
  unknown('');

  const DsButtonStyle(this.code);

  @override
  final String code;

  /// The style of [code]; [unknown] for any other value.
  static DsButtonStyle fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// Who may use the components of a card (`message_components.restricted_to`).
enum DsApprover implements DsCode {
  /// Every person who may use the component.
  any('any'),

  /// The person who triggered the run, or an owner.
  runStarter('run_starter'),

  /// An owner only.
  owner('owner'),

  /// A value this app does not know; no note is shown.
  unknown('');

  const DsApprover(this.code);

  @override
  final String code;

  /// The approver rule of [code]; [unknown] for any other value.
  static DsApprover fromCode(String? code) => _fromCode(values, code, unknown);
}

/// The kind of a channel (`channels.kind`).
enum DsChannelKind implements DsCode {
  /// Text channel.
  text('text'),

  /// Forum.
  forum('forum'),

  /// Direct message.
  dm('dm'),

  /// A value this app does not know; drawn with the text icon.
  unknown('');

  const DsChannelKind(this.code);

  @override
  final String code;

  /// The kind of [code]; [unknown] for any other value.
  static DsChannelKind fromCode(String? code) =>
      _fromCode(values, code, unknown);
}

/// The eight category colors of spaces and tags (`category.<name>.*`).
enum DsCategory implements DsCode {
  /// `category.spruce`.
  spruce('spruce'),

  /// `category.loch`.
  loch('loch'),

  /// `category.heather`.
  heather('heather'),

  /// `category.rowan`.
  rowan('rowan'),

  /// `category.amber`.
  amber('amber'),

  /// `category.gorse`.
  gorse('gorse'),

  /// `category.moss`.
  moss('moss'),

  /// `category.granite`; also the fallback for an unknown name.
  granite('granite');

  const DsCategory(this.code);

  @override
  final String code;

  /// The category of [code]; [granite] for any other value.
  static DsCategory fromCode(String? code) => _fromCode(values, code, granite);
}

/// The delivery of an own message through the outbox (01 section 6.6).
enum DsDelivery {
  /// Confirmed by the server.
  sent,

  /// In the outbox, not confirmed yet.
  sending,

  /// In the outbox for longer than `client.outbox_stuck_after`: "Sending delayed" (15 7.5).
  delayed,

  /// Failed permanently; the message shows the reason and Resend, Edit, Discard.
  failed,
}

/// A money amount with its ISO 4217 currency.
@immutable
final class DsMoney {
  /// Creates the amount.
  const DsMoney(this.amount, this.currency);

  /// The amount in the currency unit.
  final num amount;

  /// The currency code, for example `USD`.
  final String currency;
}

/// The cost of a run with its suffix rule (components.md section 10).
@immutable
final class DsCost {
  /// Creates the cost.
  ///
  /// [isEstimate] is `cost_is_estimate`; [mode] decides the suffix: "Estimate" for an
  /// estimate in `organization` mode, "for information only" in `personal` mode.
  const DsCost(this.money, {required this.mode, this.isEstimate = false});

  /// The amount.
  final DsMoney money;

  /// The backend policy the cost was incurred under.
  final DsBillingMode mode;

  /// Whether the amount is an estimate that is not reconciled yet.
  final bool isEstimate;
}

/// One step of a run (`run_steps`, run document `steps`).
@immutable
final class DsStepData {
  /// Creates the step.
  const DsStepData({
    required this.id,
    required this.kind,
    required this.name,
    required this.status,
    this.title,
    this.duration,
    this.depth = 0,
    this.detail,
  });

  /// The step ID; keys the chip so a patch keeps its state.
  final String id;

  /// The kind; selects the `step.*` icon.
  final DsStepKind kind;

  /// The name, for example the tool name; shown in mono.
  final String name;

  /// The short description, or null.
  final String? title;

  /// The status.
  final DsStepStatus status;

  /// The run time of a running step or the duration of a finished one, or null.
  final Duration? duration;

  /// The nesting under `parent_step_id`; 0 for a top-level step.
  final int depth;

  /// The text of `detail` (`run_step_detail.v1`) already cut to `run.step_detail_max_bytes`,
  /// or null when the step has none.
  final String? detail;
}

/// A run as the live box shows it (run document `run_document.v1`, 04 section 11.1).
@immutable
final class DsRunData {
  /// Creates the run.
  const DsRunData({
    required this.id,
    required this.agentName,
    required this.state,
    this.stateReason,
    this.cancelledByName,
    this.waitingForName,
    this.elapsed = Duration.zero,
    this.contextPercent,
    this.tokens,
    this.cost,
    this.billingMode,
    this.billingHolderName,
    this.statusLine,
    this.lastTextExcerpt,
    this.steps = const [],
    this.toolCount = 0,
    this.subagentCount = 0,
    this.stalledFor,
    this.questionId,
  });

  /// The run ID.
  final String id;

  /// The display name of the agent (a foreign name, isolated when drawn).
  final String agentName;

  /// The state.
  final DsRunState state;

  /// `state_reason`, a code, or null.
  final String? stateReason;

  /// The name of the person who stopped the run, for the reason `user_cancelled`.
  final String? cancelledByName;

  /// For `waiting_input`: the person the question waits for, or null when it waits for the
  /// reader ("Waiting for you").
  final String? waitingForName;

  /// The run time, or the total duration of a finished run.
  final Duration elapsed;

  /// `metrics.context_pct`, or null.
  final num? contextPercent;

  /// The token count, or null.
  final int? tokens;

  /// The cost, or null.
  final DsCost? cost;

  /// `billing_mode_used`, or null.
  final DsBillingMode? billingMode;

  /// For `personal`: the person whose subscription the agent uses.
  final String? billingHolderName;

  /// `status_line`, or null.
  final String? statusLine;

  /// `last_text_excerpt`, or null.
  final String? lastTextExcerpt;

  /// The steps in `ordinal` order.
  final List<DsStepData> steps;

  /// The number of tools of the summary (collapsed line).
  final int toolCount;

  /// The number of subagents of the summary (collapsed line).
  final int subagentCount;

  /// The time without progress once `run.stall_after` has passed, or null when the run is
  /// not stalled; the stall banner shows the whole minutes.
  final Duration? stalledFor;

  /// The ID of the open question of a `waiting_input` run, or null; a new ID is announced.
  final String? questionId;

  /// Whether the stall banner shows.
  bool get stalled => stalledFor != null;
}

/// A reaction under a message.
@immutable
final class DsReaction {
  /// Creates the reaction.
  const DsReaction(this.emoji, this.count, {this.mine = false});

  /// The emoji as sent (content, not an interface text).
  final String emoji;

  /// How many principals reacted.
  final int count;

  /// Whether the reader is one of them.
  final bool mine;
}

/// A voice message (`kind = voice`).
@immutable
final class DsVoiceData {
  /// Creates the voice part.
  const DsVoiceData({
    required this.duration,
    this.progress = 0,
    this.playing = false,
    this.transcript,
  });

  /// The length.
  final Duration duration;

  /// The played share from 0 to 1.
  final double progress;

  /// Whether it plays now.
  final bool playing;

  /// The transcript once `transcript_status = done`, else null.
  final String? transcript;
}

/// A message as [DsMessage] shows it (`messages`).
@immutable
final class DsMessageData {
  /// Creates the message.
  const DsMessageData({
    required this.id,
    required this.authorName,
    required this.origin,
    required this.kind,
    required this.timeLabel,
    this.content = '',
    this.mentionsMe = false,
    this.edited = false,
    this.deleted = false,
    this.triggerState,
    this.triggerReason,
    this.delivery = DsDelivery.sent,
    this.failureText,
    this.replyToName,
    this.replyExcerpt,
    this.attachmentNames = const [],
    this.reactions = const [],
    this.voice,
    this.systemCode,
    this.systemName,
    this.mentionHandles = const {},
  });

  /// The message ID.
  final String id;

  /// The display name of the author (a foreign name, isolated when drawn).
  final String authorName;

  /// `origin`.
  final DsMessageOrigin origin;

  /// `kind`.
  final DsMessageKind kind;

  /// The time stamp, formatted by the caller in the person's time zone (`L10nFormat.time`).
  final String timeLabel;

  /// `content_md`, unchanged agent or person text.
  final String content;

  /// Whether the reader is in `mentions`.
  final bool mentionsMe;

  /// Whether `edited_at` is set.
  final bool edited;

  /// Whether `deleted_at` is set.
  final bool deleted;

  /// `trigger_state`, or null for a message without a trigger.
  final DsTriggerState? triggerState;

  /// `trigger_state_reason`, a code, or null.
  final String? triggerReason;

  /// The outbox state of an own message.
  final DsDelivery delivery;

  /// For [DsDelivery.failed]: the translated reason (`problemText`).
  final String? failureText;

  /// The author of the quoted message of a reply, or null.
  final String? replyToName;

  /// The quoted excerpt of a reply, or null.
  final String? replyExcerpt;

  /// The file names of the attachments (foreign names).
  final List<String> attachmentNames;

  /// The reactions.
  final List<DsReaction> reactions;

  /// The voice part of `kind = voice`.
  final DsVoiceData? voice;

  /// `system_code` of `kind = system`.
  final String? systemCode;

  /// The name the system line names (`thread.reopened`: the author of the reopening
  /// message), or null.
  final String? systemName;

  /// The handles (lower case, without `@`) whose principals the server resolved into
  /// `mention_ids`; only their tokens render as mentions (03 content format item 4).
  final Set<String> mentionHandles;
}

/// One button, select or form of a card (`message_components`, `component.v1`).
@immutable
final class DsComponentItem {
  /// Creates the component.
  const DsComponentItem({
    required this.customId,
    required this.type,
    this.label = '',
    this.style = DsButtonStyle.secondary,
    this.placeholder,
    this.options = const [],
    this.minValues = 1,
    this.maxValues = 1,
    this.title,
    this.submitLabel,
    this.fields = const [],
  });

  /// `custom_id`, handed back on activation.
  final String customId;

  /// `type`.
  final DsComponentType type;

  /// The button label (agent text).
  final String label;

  /// The button style role.
  final DsButtonStyle style;

  /// The select placeholder, or null for the default text.
  final String? placeholder;

  /// The select options.
  final List<DsSelectOption> options;

  /// `min_values` of a select.
  final int minValues;

  /// `max_values` of a select; more than 1 makes it a multiple selection.
  final int maxValues;

  /// The form title, or null.
  final String? title;

  /// The submit label of a form, or null for the default text.
  final String? submitLabel;

  /// The form fields.
  final List<DsFormField> fields;
}

/// An option of a select.
@immutable
final class DsSelectOption {
  /// Creates the option.
  const DsSelectOption({
    required this.label,
    required this.value,
    this.description,
  });

  /// The label (agent text).
  final String label;

  /// The value handed back.
  final String value;

  /// An optional description.
  final String? description;
}

/// The type of a form field (`component.v1` field `type`).
enum DsFieldType implements DsCode {
  /// One line of text.
  text('text'),

  /// Several lines of text.
  textarea('textarea'),

  /// A number.
  number('number'),

  /// A switch.
  boolean('boolean'),

  /// One of `choices`.
  choice('choice'),

  /// A value this app does not know; rendered as text.
  unknown('');

  const DsFieldType(this.code);

  @override
  final String code;

  /// The type of [code]; [unknown] for any other value.
  static DsFieldType fromCode(String? code) => _fromCode(values, code, unknown);
}

/// A field of a form.
@immutable
final class DsFormField {
  /// Creates the field.
  const DsFormField({
    required this.name,
    required this.label,
    this.type = DsFieldType.text,
    this.required = false,
    this.maxLength,
    this.choices = const [],
  });

  /// The field name, the key of the submitted value.
  final String name;

  /// The label above the field (agent text).
  final String label;

  /// The field type.
  final DsFieldType type;

  /// Whether a value is required.
  final bool required;

  /// `max_length`, or null.
  final int? maxLength;

  /// The choices of a `choice` field.
  final List<String> choices;
}

/// The components of one card with their common state.
@immutable
final class DsComponentData {
  /// Creates the card data.
  const DsComponentData({
    required this.messageId,
    required this.items,
    this.state = DsComponentState.active,
    this.title,
    this.pendingCustomId,
    this.usedByName,
    this.usedChoice,
    this.usedAtLabel,
    this.errorText,
  });

  /// The message ID of the card.
  final String messageId;

  /// The components in `position` order.
  final List<DsComponentItem> items;

  /// The common state of the single-use group.
  final DsComponentState state;

  /// The card title, the group label for screen readers, or null.
  final String? title;

  /// The `custom_id` whose answer is pending (sent, not confirmed), or null.
  final String? pendingCustomId;

  /// For `used`: who chose.
  final String? usedByName;

  /// For `used`: the label of the choice.
  final String? usedChoice;

  /// For `used`: the time, formatted by the caller.
  final String? usedAtLabel;

  /// The translated reason of a failed interaction (`problemText`), or null.
  final String? errorText;
}

/// The header of a card, built only from server fields (SEC-OUT-4).
@immutable
final class DsComponentHeader {
  /// Creates the header.
  const DsComponentHeader({
    required this.agentName,
    this.runLabel,
    this.requiredCapability,
    this.approver = DsApprover.any,
    this.runStarterName,
  });

  /// The agent of the message, from the principal of the message.
  final String agentName;

  /// The run the question belongs to, or null.
  final String? runLabel;

  /// `required_capability`, a code such as `approve_actions`, or null.
  final String? requiredCapability;

  /// `restricted_to`.
  final DsApprover approver;

  /// For [DsApprover.runStarter]: the person who triggered the run.
  final String? runStarterName;
}

/// A tag of a forum post.
@immutable
final class DsTagData {
  /// Creates the tag.
  const DsTagData(this.name, this.category);

  /// The tag name (a foreign name).
  final String name;

  /// The category color.
  final DsCategory category;
}

/// A row of the forum post list (components.md section 6).
@immutable
final class DsThreadRowData {
  /// Creates the row.
  const DsThreadRowData({
    required this.id,
    required this.title,
    required this.timeLabel,
    this.unreadCount = 0,
    this.mentionCount = 0,
    this.runState,
    this.waitingForMe = false,
    this.operatorName,
    this.tags = const [],
    this.errorText,
    this.done = false,
  });

  /// The thread ID.
  final String id;

  /// The title (a foreign name).
  final String title;

  /// The time of the last activity, formatted by the caller.
  final String timeLabel;

  /// Unread messages.
  final int unreadCount;

  /// Unread mentions.
  final int mentionCount;

  /// The state of an active run, or null.
  final DsRunState? runState;

  /// Whether a question waits for the reader.
  final bool waitingForMe;

  /// The operator agent, or null.
  final String? operatorName;

  /// The tags.
  final List<DsTagData> tags;

  /// The translated failure reason of the last run, or null.
  final String? errorText;

  /// Whether the post is done (`state = closed`).
  final bool done;
}

/// A channel row of the sidebar (components.md section 7).
@immutable
final class DsChannelRowData {
  /// Creates the row.
  const DsChannelRowData({
    required this.id,
    required this.name,
    this.kind = DsChannelKind.text,
    this.unreadCount = 0,
    this.mentionCount = 0,
    this.runActive = false,
    this.muted = false,
    this.projectBound = false,
  });

  /// The channel ID.
  final String id;

  /// The channel name (a foreign name).
  final String name;

  /// The kind; selects the icon.
  final DsChannelKind kind;

  /// Whether the forum has a project binding (`Channel.project_bound`): its posts are agent
  /// sessions in one project, so the row shows `channel.project` and the project kind
  /// sentence (01 6.8 item 11). Ignored for every kind but [DsChannelKind.forum].
  final bool projectBound;

  /// Unread messages; more than 0 is the state "unread".
  final int unreadCount;

  /// Unread mentions; more than 0 shows the counter.
  final int mentionCount;

  /// Whether a run is active in the channel.
  final bool runActive;

  /// Whether the reader muted the channel.
  final bool muted;
}

/// A thread row of the sidebar, nested below its channel (components.md section 7, 01 6.8).
///
/// A forum post or a thread from a message of a text channel; the sidebar shows only open
/// threads, and a done one only while it is selected (01 6.8 item 2), so [done] is rare.
@immutable
final class DsSidebarThreadRowData {
  /// Creates the row.
  const DsSidebarThreadRowData({
    required this.id,
    required this.title,
    this.runState,
    this.unreadCount = 0,
    this.mentionCount = 0,
    this.muted = false,
    this.done = false,
  });

  /// The thread ID.
  final String id;

  /// The title (a foreign name); empty while the server has not named the thread yet, which
  /// the row shows as "New chat" with the "automatic" mark (01 6.8 item 6).
  final String title;

  /// The state of the thread's latest run, or null without a run. Only `queued`, `running`,
  /// `waiting_input` and `failed` show an icon and a state word (01 6.8 item 6).
  final DsRunState? runState;

  /// Unread messages; more than 0 is the state "unread" (a mark, never a number).
  final int unreadCount;

  /// Unread mentions; more than 0 shows the counter.
  final int mentionCount;

  /// Whether the reader muted the thread or its channel.
  final bool muted;

  /// Whether the thread is done (`state = closed`).
  final bool done;
}

/// A space row of the sidebar (components.md section 7).
@immutable
final class DsSpaceRowData {
  /// Creates the row.
  const DsSpaceRowData({
    required this.id,
    required this.name,
    required this.icon,
    required this.category,
    this.expanded = true,
    this.hasUnread = false,
  });

  /// The space ID.
  final String id;

  /// The space name (a foreign name).
  final String name;

  /// The space icon, one of `DsIcons.spaceIcons`; another name draws the first one.
  final String icon;

  /// The category color.
  final DsCategory category;

  /// Whether the space is expanded.
  final bool expanded;

  /// Whether a channel of the space has unread items (dot when collapsed).
  final bool hasUnread;
}

/// A row of the suggestion list (components.md section 9).
@immutable
final class DsSuggestion {
  /// Creates the suggestion.
  const DsSuggestion({
    required this.id,
    required this.command,
    this.description,
    this.originName,
    this.control = false,
  });

  /// A stable ID of the suggestion.
  final String id;

  /// The command or mention as inserted, for example `/deploy` or `@robin`.
  final String command;

  /// The description, or null.
  final String? description;

  /// The agent that registered the command, or null.
  final String? originName;

  /// Whether it is a control command (`control = true`).
  final bool control;
}

/// A command chosen in the input, with its options (components.md section 9).
@immutable
final class DsComposerCommand {
  /// Creates the command.
  const DsComposerCommand({required this.name, this.options = const []});

  /// The command, for example `/deploy`.
  final String name;

  /// The options in order.
  final List<DsCommandOption> options;
}

/// An option of a chosen command.
@immutable
final class DsCommandOption {
  /// Creates the option.
  const DsCommandOption({
    required this.name,
    this.required = false,
    this.value,
  });

  /// The option name, shown as the placeholder.
  final String name;

  /// Whether sending needs a value.
  final bool required;

  /// The entered value, or null.
  final String? value;

  /// Whether the option blocks sending.
  bool get missing => required && (value == null || value!.isEmpty);
}

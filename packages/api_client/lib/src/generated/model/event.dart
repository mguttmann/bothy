//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/reaction_added_event.dart';
import 'package:api_client/src/generated/model/group_changed_event.dart';
import 'package:api_client/src/generated/model/host_updated_event.dart';
import 'package:api_client/src/generated/model/channel_settings_changed_event.dart';
import 'package:api_client/src/generated/model/event_resource.dart';
import 'package:api_client/src/generated/model/access_changed_event.dart';
import 'package:api_client/src/generated/model/gdpr_request_updated_event.dart';
import 'package:api_client/src/generated/model/thread_reopened_event.dart';
import 'package:api_client/src/generated/model/draft_deleted_event.dart';
import 'package:api_client/src/generated/model/agent_config_changed_event.dart';
import 'package:api_client/src/generated/model/thread_updated_event.dart';
import 'package:api_client/src/generated/model/access_expiring_event.dart';
import 'package:api_client/src/generated/model/settings_updated_event.dart';
import 'package:api_client/src/generated/model/agent_updated_event.dart';
import 'package:api_client/src/generated/model/channel_created_event.dart';
import 'package:api_client/src/generated/model/instance_updated_event.dart';
import 'package:api_client/src/generated/model/agent_followup_event.dart';
import 'package:api_client/src/generated/model/user_updated_event.dart';
import 'package:api_client/src/generated/model/agent_invocation_event.dart';
import 'package:api_client/src/generated/model/terminal_session_changed_event.dart';
import 'package:api_client/src/generated/model/thread_moved_out_event.dart';
import 'package:api_client/src/generated/model/message_created_event.dart';
import 'package:api_client/src/generated/model/thread_closed_event.dart';
import 'package:api_client/src/generated/model/sidebar_updated_event.dart';
import 'package:api_client/src/generated/model/space_created_event.dart';
import 'package:api_client/src/generated/model/terminal_session.dart';
import 'package:api_client/src/generated/model/run_cancel_requested_event.dart';
import 'package:api_client/src/generated/model/agent_session_changed_event.dart';
import 'package:api_client/src/generated/model/pin_removed_event.dart';
import 'package:api_client/src/generated/model/command_invoked_event.dart';
import 'package:api_client/src/generated/model/read_state_updated_event.dart';
import 'package:api_client/src/generated/model/notification_pref_updated_event.dart';
import 'package:api_client/src/generated/model/file_updated_event.dart';
import 'package:api_client/src/generated/model/quiet_hours_updated_event.dart';
import 'package:api_client/src/generated/model/space_archived_event.dart';
import 'package:api_client/src/generated/model/event_actor.dart';
import 'package:api_client/src/generated/model/message_deleted_event.dart';
import 'package:api_client/src/generated/model/thread_moved_event.dart';
import 'package:api_client/src/generated/model/command_changed_event.dart';
import 'package:api_client/src/generated/model/thread_created_event.dart';
import 'package:api_client/src/generated/model/event_scope.dart';
import 'package:api_client/src/generated/model/space_updated_event.dart';
import 'package:api_client/src/generated/model/pin_added_event.dart';
import 'package:api_client/src/generated/model/import_progress_event.dart';
import 'package:api_client/src/generated/model/draft_updated_event.dart';
import 'package:api_client/src/generated/model/channel_archived_event.dart';
import 'package:api_client/src/generated/model/run_state_changed_event.dart';
import 'package:api_client/src/generated/model/reaction_removed_event.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/interaction_created_event.dart';
import 'package:api_client/src/generated/model/channel_updated_event.dart';
import 'package:api_client/src/generated/model/status_updated_event.dart';
import 'package:api_client/src/generated/model/message_updated_event.dart';
import 'package:api_client/src/generated/model/grant_changed_event.dart';
import 'package:api_client/src/generated/model/session_revoked_event.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';
import 'package:one_of/one_of.dart';

part 'event.g.dart';

/// Any durable event, discriminated by `type`. Clients skip unknown types and still advance their cursor.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [seq] - Global commit-ordered sequence; absent for ephemeral events.
/// * [schemaVersion]
/// * [occurredAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [actor]
/// * [scope]
/// * [resource]
/// * [audience]
/// * [redacted]
/// * [truncated] - The payload was cut to `sync.event_payload_max_bytes`; load the resource via REST.
/// * [correlationId] - W3C trace ID (16 bytes) of the request, written as a lowercase UUID; not a UUIDv7. Generated by the server.
/// * [traceContext] - W3C `traceparent` of the triggering request; only in deliveries to agent adapters and only if the request was recorded.
/// * [clientMsgId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [permissionVersion]
/// * [invocationToken] - Fresh invocation token (JWS, I3); only on agent-addressed invocation events.
/// * [redelivered] - Set on redelivery to an agent consumer after `agent.ack_timeout`.
/// * [syncEpoch] - Delivery-only member of outgoing webhooks; the time line of the event log (04, 7.5; ADR 0059). A receiver that sees a new epoch resets its `seq` state.
/// * [type]
/// * [payload]
@BuiltValue()
abstract class Event implements Built<Event, EventBuilder> {
  /// One Of [AccessChangedEvent], [AccessExpiringEvent], [AgentConfigChangedEvent], [AgentFollowupEvent], [AgentInvocationEvent], [AgentSessionChangedEvent], [AgentUpdatedEvent], [ChannelArchivedEvent], [ChannelCreatedEvent], [ChannelSettingsChangedEvent], [ChannelUpdatedEvent], [CommandChangedEvent], [CommandInvokedEvent], [DraftDeletedEvent], [DraftUpdatedEvent], [FileUpdatedEvent], [GdprRequestUpdatedEvent], [GrantChangedEvent], [GroupChangedEvent], [HostUpdatedEvent], [ImportProgressEvent], [InstanceUpdatedEvent], [InteractionCreatedEvent], [MessageCreatedEvent], [MessageDeletedEvent], [MessageUpdatedEvent], [NotificationPrefUpdatedEvent], [PinAddedEvent], [PinRemovedEvent], [QuietHoursUpdatedEvent], [ReactionAddedEvent], [ReactionRemovedEvent], [ReadStateUpdatedEvent], [RunCancelRequestedEvent], [RunStateChangedEvent], [SessionRevokedEvent], [SettingsUpdatedEvent], [SidebarUpdatedEvent], [SpaceArchivedEvent], [SpaceCreatedEvent], [SpaceUpdatedEvent], [StatusUpdatedEvent], [TerminalSessionChangedEvent], [ThreadClosedEvent], [ThreadCreatedEvent], [ThreadMovedEvent], [ThreadMovedOutEvent], [ThreadReopenedEvent], [ThreadUpdatedEvent], [UserUpdatedEvent]
  OneOf get oneOf;

  static const String discriminatorFieldName = r'type';

  static const Map<String, Type> discriminatorMapping = {
    r'access.changed': AccessChangedEvent,
    r'access.expiring': AccessExpiringEvent,
    r'agent.config_changed': AgentConfigChangedEvent,
    r'agent.followup': AgentFollowupEvent,
    r'agent.invocation': AgentInvocationEvent,
    r'agent.updated': AgentUpdatedEvent,
    r'agent_session.changed': AgentSessionChangedEvent,
    r'channel.archived': ChannelArchivedEvent,
    r'channel.created': ChannelCreatedEvent,
    r'channel.settings_changed': ChannelSettingsChangedEvent,
    r'channel.updated': ChannelUpdatedEvent,
    r'command.changed': CommandChangedEvent,
    r'command.invoked': CommandInvokedEvent,
    r'draft.deleted': DraftDeletedEvent,
    r'draft.updated': DraftUpdatedEvent,
    r'file.updated': FileUpdatedEvent,
    r'gdpr.request_updated': GdprRequestUpdatedEvent,
    r'grant.changed': GrantChangedEvent,
    r'group.changed': GroupChangedEvent,
    r'host.updated': HostUpdatedEvent,
    r'import.progress': ImportProgressEvent,
    r'instance.updated': InstanceUpdatedEvent,
    r'interaction.created': InteractionCreatedEvent,
    r'message.created': MessageCreatedEvent,
    r'message.deleted': MessageDeletedEvent,
    r'message.updated': MessageUpdatedEvent,
    r'notification_pref.updated': NotificationPrefUpdatedEvent,
    r'pin.added': PinAddedEvent,
    r'pin.removed': PinRemovedEvent,
    r'quiet_hours.updated': QuietHoursUpdatedEvent,
    r'reaction.added': ReactionAddedEvent,
    r'reaction.removed': ReactionRemovedEvent,
    r'read_state.updated': ReadStateUpdatedEvent,
    r'run.cancel_requested': RunCancelRequestedEvent,
    r'run.state_changed': RunStateChangedEvent,
    r'session.revoked': SessionRevokedEvent,
    r'settings.updated': SettingsUpdatedEvent,
    r'sidebar.updated': SidebarUpdatedEvent,
    r'space.archived': SpaceArchivedEvent,
    r'space.created': SpaceCreatedEvent,
    r'space.updated': SpaceUpdatedEvent,
    r'status.updated': StatusUpdatedEvent,
    r'terminal_session.changed': TerminalSessionChangedEvent,
    r'thread.closed': ThreadClosedEvent,
    r'thread.created': ThreadCreatedEvent,
    r'thread.moved': ThreadMovedEvent,
    r'thread.moved_out': ThreadMovedOutEvent,
    r'thread.reopened': ThreadReopenedEvent,
    r'thread.updated': ThreadUpdatedEvent,
    r'user.updated': UserUpdatedEvent,
  };

  Event._();

  factory Event([void updates(EventBuilder b)]) = _$Event;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(EventBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Event> get serializer => _$EventSerializer();
}

extension EventDiscriminatorExt on Event {
  String? get discriminatorValue {
    if (this is AccessChangedEvent) {
      return r'access.changed';
    }
    if (this is AccessExpiringEvent) {
      return r'access.expiring';
    }
    if (this is AgentConfigChangedEvent) {
      return r'agent.config_changed';
    }
    if (this is AgentFollowupEvent) {
      return r'agent.followup';
    }
    if (this is AgentInvocationEvent) {
      return r'agent.invocation';
    }
    if (this is AgentUpdatedEvent) {
      return r'agent.updated';
    }
    if (this is AgentSessionChangedEvent) {
      return r'agent_session.changed';
    }
    if (this is ChannelArchivedEvent) {
      return r'channel.archived';
    }
    if (this is ChannelCreatedEvent) {
      return r'channel.created';
    }
    if (this is ChannelSettingsChangedEvent) {
      return r'channel.settings_changed';
    }
    if (this is ChannelUpdatedEvent) {
      return r'channel.updated';
    }
    if (this is CommandChangedEvent) {
      return r'command.changed';
    }
    if (this is CommandInvokedEvent) {
      return r'command.invoked';
    }
    if (this is DraftDeletedEvent) {
      return r'draft.deleted';
    }
    if (this is DraftUpdatedEvent) {
      return r'draft.updated';
    }
    if (this is FileUpdatedEvent) {
      return r'file.updated';
    }
    if (this is GdprRequestUpdatedEvent) {
      return r'gdpr.request_updated';
    }
    if (this is GrantChangedEvent) {
      return r'grant.changed';
    }
    if (this is GroupChangedEvent) {
      return r'group.changed';
    }
    if (this is HostUpdatedEvent) {
      return r'host.updated';
    }
    if (this is ImportProgressEvent) {
      return r'import.progress';
    }
    if (this is InstanceUpdatedEvent) {
      return r'instance.updated';
    }
    if (this is InteractionCreatedEvent) {
      return r'interaction.created';
    }
    if (this is MessageCreatedEvent) {
      return r'message.created';
    }
    if (this is MessageDeletedEvent) {
      return r'message.deleted';
    }
    if (this is MessageUpdatedEvent) {
      return r'message.updated';
    }
    if (this is NotificationPrefUpdatedEvent) {
      return r'notification_pref.updated';
    }
    if (this is PinAddedEvent) {
      return r'pin.added';
    }
    if (this is PinRemovedEvent) {
      return r'pin.removed';
    }
    if (this is QuietHoursUpdatedEvent) {
      return r'quiet_hours.updated';
    }
    if (this is ReactionAddedEvent) {
      return r'reaction.added';
    }
    if (this is ReactionRemovedEvent) {
      return r'reaction.removed';
    }
    if (this is ReadStateUpdatedEvent) {
      return r'read_state.updated';
    }
    if (this is RunCancelRequestedEvent) {
      return r'run.cancel_requested';
    }
    if (this is RunStateChangedEvent) {
      return r'run.state_changed';
    }
    if (this is SessionRevokedEvent) {
      return r'session.revoked';
    }
    if (this is SettingsUpdatedEvent) {
      return r'settings.updated';
    }
    if (this is SidebarUpdatedEvent) {
      return r'sidebar.updated';
    }
    if (this is SpaceArchivedEvent) {
      return r'space.archived';
    }
    if (this is SpaceCreatedEvent) {
      return r'space.created';
    }
    if (this is SpaceUpdatedEvent) {
      return r'space.updated';
    }
    if (this is StatusUpdatedEvent) {
      return r'status.updated';
    }
    if (this is TerminalSessionChangedEvent) {
      return r'terminal_session.changed';
    }
    if (this is ThreadClosedEvent) {
      return r'thread.closed';
    }
    if (this is ThreadCreatedEvent) {
      return r'thread.created';
    }
    if (this is ThreadMovedEvent) {
      return r'thread.moved';
    }
    if (this is ThreadMovedOutEvent) {
      return r'thread.moved_out';
    }
    if (this is ThreadReopenedEvent) {
      return r'thread.reopened';
    }
    if (this is ThreadUpdatedEvent) {
      return r'thread.updated';
    }
    if (this is UserUpdatedEvent) {
      return r'user.updated';
    }
    return null;
  }
}

extension EventBuilderDiscriminatorExt on EventBuilder {
  String? get discriminatorValue {
    if (this is AccessChangedEventBuilder) {
      return r'access.changed';
    }
    if (this is AccessExpiringEventBuilder) {
      return r'access.expiring';
    }
    if (this is AgentConfigChangedEventBuilder) {
      return r'agent.config_changed';
    }
    if (this is AgentFollowupEventBuilder) {
      return r'agent.followup';
    }
    if (this is AgentInvocationEventBuilder) {
      return r'agent.invocation';
    }
    if (this is AgentUpdatedEventBuilder) {
      return r'agent.updated';
    }
    if (this is AgentSessionChangedEventBuilder) {
      return r'agent_session.changed';
    }
    if (this is ChannelArchivedEventBuilder) {
      return r'channel.archived';
    }
    if (this is ChannelCreatedEventBuilder) {
      return r'channel.created';
    }
    if (this is ChannelSettingsChangedEventBuilder) {
      return r'channel.settings_changed';
    }
    if (this is ChannelUpdatedEventBuilder) {
      return r'channel.updated';
    }
    if (this is CommandChangedEventBuilder) {
      return r'command.changed';
    }
    if (this is CommandInvokedEventBuilder) {
      return r'command.invoked';
    }
    if (this is DraftDeletedEventBuilder) {
      return r'draft.deleted';
    }
    if (this is DraftUpdatedEventBuilder) {
      return r'draft.updated';
    }
    if (this is FileUpdatedEventBuilder) {
      return r'file.updated';
    }
    if (this is GdprRequestUpdatedEventBuilder) {
      return r'gdpr.request_updated';
    }
    if (this is GrantChangedEventBuilder) {
      return r'grant.changed';
    }
    if (this is GroupChangedEventBuilder) {
      return r'group.changed';
    }
    if (this is HostUpdatedEventBuilder) {
      return r'host.updated';
    }
    if (this is ImportProgressEventBuilder) {
      return r'import.progress';
    }
    if (this is InstanceUpdatedEventBuilder) {
      return r'instance.updated';
    }
    if (this is InteractionCreatedEventBuilder) {
      return r'interaction.created';
    }
    if (this is MessageCreatedEventBuilder) {
      return r'message.created';
    }
    if (this is MessageDeletedEventBuilder) {
      return r'message.deleted';
    }
    if (this is MessageUpdatedEventBuilder) {
      return r'message.updated';
    }
    if (this is NotificationPrefUpdatedEventBuilder) {
      return r'notification_pref.updated';
    }
    if (this is PinAddedEventBuilder) {
      return r'pin.added';
    }
    if (this is PinRemovedEventBuilder) {
      return r'pin.removed';
    }
    if (this is QuietHoursUpdatedEventBuilder) {
      return r'quiet_hours.updated';
    }
    if (this is ReactionAddedEventBuilder) {
      return r'reaction.added';
    }
    if (this is ReactionRemovedEventBuilder) {
      return r'reaction.removed';
    }
    if (this is ReadStateUpdatedEventBuilder) {
      return r'read_state.updated';
    }
    if (this is RunCancelRequestedEventBuilder) {
      return r'run.cancel_requested';
    }
    if (this is RunStateChangedEventBuilder) {
      return r'run.state_changed';
    }
    if (this is SessionRevokedEventBuilder) {
      return r'session.revoked';
    }
    if (this is SettingsUpdatedEventBuilder) {
      return r'settings.updated';
    }
    if (this is SidebarUpdatedEventBuilder) {
      return r'sidebar.updated';
    }
    if (this is SpaceArchivedEventBuilder) {
      return r'space.archived';
    }
    if (this is SpaceCreatedEventBuilder) {
      return r'space.created';
    }
    if (this is SpaceUpdatedEventBuilder) {
      return r'space.updated';
    }
    if (this is StatusUpdatedEventBuilder) {
      return r'status.updated';
    }
    if (this is TerminalSessionChangedEventBuilder) {
      return r'terminal_session.changed';
    }
    if (this is ThreadClosedEventBuilder) {
      return r'thread.closed';
    }
    if (this is ThreadCreatedEventBuilder) {
      return r'thread.created';
    }
    if (this is ThreadMovedEventBuilder) {
      return r'thread.moved';
    }
    if (this is ThreadMovedOutEventBuilder) {
      return r'thread.moved_out';
    }
    if (this is ThreadReopenedEventBuilder) {
      return r'thread.reopened';
    }
    if (this is ThreadUpdatedEventBuilder) {
      return r'thread.updated';
    }
    if (this is UserUpdatedEventBuilder) {
      return r'user.updated';
    }
    return null;
  }
}

class _$EventSerializer implements PrimitiveSerializer<Event> {
  @override
  final Iterable<Type> types = const [Event, _$Event];

  @override
  final String wireName = r'Event';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Event object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {}

  @override
  Object serialize(
    Serializers serializers,
    Event object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final oneOf = object.oneOf;
    return serializers.serialize(
      oneOf.value,
      specifiedType: FullType(oneOf.valueType),
    )!;
  }

  @override
  Event deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = EventBuilder();
    Object? oneOfDataSrc;
    final serializedList = (serialized as Iterable<Object?>).toList();
    final discIndex = serializedList.indexOf(Event.discriminatorFieldName) + 1;
    final discValue = serializers.deserialize(
      serializedList[discIndex],
      specifiedType: FullType(String),
    ) as String;
    oneOfDataSrc = serialized;
    final oneOfTypes = [
      AccessChangedEvent,
      AccessExpiringEvent,
      AgentConfigChangedEvent,
      AgentFollowupEvent,
      AgentInvocationEvent,
      AgentUpdatedEvent,
      AgentSessionChangedEvent,
      ChannelArchivedEvent,
      ChannelCreatedEvent,
      ChannelSettingsChangedEvent,
      ChannelUpdatedEvent,
      CommandChangedEvent,
      CommandInvokedEvent,
      DraftDeletedEvent,
      DraftUpdatedEvent,
      FileUpdatedEvent,
      GdprRequestUpdatedEvent,
      GrantChangedEvent,
      GroupChangedEvent,
      HostUpdatedEvent,
      ImportProgressEvent,
      InstanceUpdatedEvent,
      InteractionCreatedEvent,
      MessageCreatedEvent,
      MessageDeletedEvent,
      MessageUpdatedEvent,
      NotificationPrefUpdatedEvent,
      PinAddedEvent,
      PinRemovedEvent,
      QuietHoursUpdatedEvent,
      ReactionAddedEvent,
      ReactionRemovedEvent,
      ReadStateUpdatedEvent,
      RunCancelRequestedEvent,
      RunStateChangedEvent,
      SessionRevokedEvent,
      SettingsUpdatedEvent,
      SidebarUpdatedEvent,
      SpaceArchivedEvent,
      SpaceCreatedEvent,
      SpaceUpdatedEvent,
      StatusUpdatedEvent,
      TerminalSessionChangedEvent,
      ThreadClosedEvent,
      ThreadCreatedEvent,
      ThreadMovedEvent,
      ThreadMovedOutEvent,
      ThreadReopenedEvent,
      ThreadUpdatedEvent,
      UserUpdatedEvent,
    ];
    Object oneOfResult;
    Type oneOfType;
    switch (discValue) {
      case r'access.changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AccessChangedEvent),
        ) as AccessChangedEvent;
        oneOfType = AccessChangedEvent;
        break;
      case r'access.expiring':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AccessExpiringEvent),
        ) as AccessExpiringEvent;
        oneOfType = AccessExpiringEvent;
        break;
      case r'agent.config_changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AgentConfigChangedEvent),
        ) as AgentConfigChangedEvent;
        oneOfType = AgentConfigChangedEvent;
        break;
      case r'agent.followup':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AgentFollowupEvent),
        ) as AgentFollowupEvent;
        oneOfType = AgentFollowupEvent;
        break;
      case r'agent.invocation':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AgentInvocationEvent),
        ) as AgentInvocationEvent;
        oneOfType = AgentInvocationEvent;
        break;
      case r'agent.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AgentUpdatedEvent),
        ) as AgentUpdatedEvent;
        oneOfType = AgentUpdatedEvent;
        break;
      case r'agent_session.changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(AgentSessionChangedEvent),
        ) as AgentSessionChangedEvent;
        oneOfType = AgentSessionChangedEvent;
        break;
      case r'channel.archived':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ChannelArchivedEvent),
        ) as ChannelArchivedEvent;
        oneOfType = ChannelArchivedEvent;
        break;
      case r'channel.created':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ChannelCreatedEvent),
        ) as ChannelCreatedEvent;
        oneOfType = ChannelCreatedEvent;
        break;
      case r'channel.settings_changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ChannelSettingsChangedEvent),
        ) as ChannelSettingsChangedEvent;
        oneOfType = ChannelSettingsChangedEvent;
        break;
      case r'channel.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ChannelUpdatedEvent),
        ) as ChannelUpdatedEvent;
        oneOfType = ChannelUpdatedEvent;
        break;
      case r'command.changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(CommandChangedEvent),
        ) as CommandChangedEvent;
        oneOfType = CommandChangedEvent;
        break;
      case r'command.invoked':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(CommandInvokedEvent),
        ) as CommandInvokedEvent;
        oneOfType = CommandInvokedEvent;
        break;
      case r'draft.deleted':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(DraftDeletedEvent),
        ) as DraftDeletedEvent;
        oneOfType = DraftDeletedEvent;
        break;
      case r'draft.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(DraftUpdatedEvent),
        ) as DraftUpdatedEvent;
        oneOfType = DraftUpdatedEvent;
        break;
      case r'file.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(FileUpdatedEvent),
        ) as FileUpdatedEvent;
        oneOfType = FileUpdatedEvent;
        break;
      case r'gdpr.request_updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(GdprRequestUpdatedEvent),
        ) as GdprRequestUpdatedEvent;
        oneOfType = GdprRequestUpdatedEvent;
        break;
      case r'grant.changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(GrantChangedEvent),
        ) as GrantChangedEvent;
        oneOfType = GrantChangedEvent;
        break;
      case r'group.changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(GroupChangedEvent),
        ) as GroupChangedEvent;
        oneOfType = GroupChangedEvent;
        break;
      case r'host.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(HostUpdatedEvent),
        ) as HostUpdatedEvent;
        oneOfType = HostUpdatedEvent;
        break;
      case r'import.progress':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ImportProgressEvent),
        ) as ImportProgressEvent;
        oneOfType = ImportProgressEvent;
        break;
      case r'instance.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(InstanceUpdatedEvent),
        ) as InstanceUpdatedEvent;
        oneOfType = InstanceUpdatedEvent;
        break;
      case r'interaction.created':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(InteractionCreatedEvent),
        ) as InteractionCreatedEvent;
        oneOfType = InteractionCreatedEvent;
        break;
      case r'message.created':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(MessageCreatedEvent),
        ) as MessageCreatedEvent;
        oneOfType = MessageCreatedEvent;
        break;
      case r'message.deleted':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(MessageDeletedEvent),
        ) as MessageDeletedEvent;
        oneOfType = MessageDeletedEvent;
        break;
      case r'message.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(MessageUpdatedEvent),
        ) as MessageUpdatedEvent;
        oneOfType = MessageUpdatedEvent;
        break;
      case r'notification_pref.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(NotificationPrefUpdatedEvent),
        ) as NotificationPrefUpdatedEvent;
        oneOfType = NotificationPrefUpdatedEvent;
        break;
      case r'pin.added':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(PinAddedEvent),
        ) as PinAddedEvent;
        oneOfType = PinAddedEvent;
        break;
      case r'pin.removed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(PinRemovedEvent),
        ) as PinRemovedEvent;
        oneOfType = PinRemovedEvent;
        break;
      case r'quiet_hours.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(QuietHoursUpdatedEvent),
        ) as QuietHoursUpdatedEvent;
        oneOfType = QuietHoursUpdatedEvent;
        break;
      case r'reaction.added':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ReactionAddedEvent),
        ) as ReactionAddedEvent;
        oneOfType = ReactionAddedEvent;
        break;
      case r'reaction.removed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ReactionRemovedEvent),
        ) as ReactionRemovedEvent;
        oneOfType = ReactionRemovedEvent;
        break;
      case r'read_state.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ReadStateUpdatedEvent),
        ) as ReadStateUpdatedEvent;
        oneOfType = ReadStateUpdatedEvent;
        break;
      case r'run.cancel_requested':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(RunCancelRequestedEvent),
        ) as RunCancelRequestedEvent;
        oneOfType = RunCancelRequestedEvent;
        break;
      case r'run.state_changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(RunStateChangedEvent),
        ) as RunStateChangedEvent;
        oneOfType = RunStateChangedEvent;
        break;
      case r'session.revoked':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(SessionRevokedEvent),
        ) as SessionRevokedEvent;
        oneOfType = SessionRevokedEvent;
        break;
      case r'settings.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(SettingsUpdatedEvent),
        ) as SettingsUpdatedEvent;
        oneOfType = SettingsUpdatedEvent;
        break;
      case r'sidebar.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(SidebarUpdatedEvent),
        ) as SidebarUpdatedEvent;
        oneOfType = SidebarUpdatedEvent;
        break;
      case r'space.archived':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(SpaceArchivedEvent),
        ) as SpaceArchivedEvent;
        oneOfType = SpaceArchivedEvent;
        break;
      case r'space.created':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(SpaceCreatedEvent),
        ) as SpaceCreatedEvent;
        oneOfType = SpaceCreatedEvent;
        break;
      case r'space.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(SpaceUpdatedEvent),
        ) as SpaceUpdatedEvent;
        oneOfType = SpaceUpdatedEvent;
        break;
      case r'status.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(StatusUpdatedEvent),
        ) as StatusUpdatedEvent;
        oneOfType = StatusUpdatedEvent;
        break;
      case r'terminal_session.changed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(TerminalSessionChangedEvent),
        ) as TerminalSessionChangedEvent;
        oneOfType = TerminalSessionChangedEvent;
        break;
      case r'thread.closed':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ThreadClosedEvent),
        ) as ThreadClosedEvent;
        oneOfType = ThreadClosedEvent;
        break;
      case r'thread.created':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ThreadCreatedEvent),
        ) as ThreadCreatedEvent;
        oneOfType = ThreadCreatedEvent;
        break;
      case r'thread.moved':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ThreadMovedEvent),
        ) as ThreadMovedEvent;
        oneOfType = ThreadMovedEvent;
        break;
      case r'thread.moved_out':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ThreadMovedOutEvent),
        ) as ThreadMovedOutEvent;
        oneOfType = ThreadMovedOutEvent;
        break;
      case r'thread.reopened':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ThreadReopenedEvent),
        ) as ThreadReopenedEvent;
        oneOfType = ThreadReopenedEvent;
        break;
      case r'thread.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(ThreadUpdatedEvent),
        ) as ThreadUpdatedEvent;
        oneOfType = ThreadUpdatedEvent;
        break;
      case r'user.updated':
        oneOfResult = serializers.deserialize(
          oneOfDataSrc,
          specifiedType: FullType(UserUpdatedEvent),
        ) as UserUpdatedEvent;
        oneOfType = UserUpdatedEvent;
        break;
      default:
        throw UnsupportedError(
          "Couldn't deserialize oneOf for the discriminator value: ${discValue}",
        );
    }
    result.oneOf = OneOfDynamic(
      typeIndex: oneOfTypes.indexOf(oneOfType),
      types: oneOfTypes,
      value: oneOfResult,
    );
    return result.build();
  }
}

class EventAudienceEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'self')
  static const EventAudienceEnum self = _$eventAudienceEnum_self;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const EventAudienceEnum unknownDefaultOpenApi =
      _$eventAudienceEnum_unknownDefaultOpenApi;

  static Serializer<EventAudienceEnum> get serializer =>
      _$eventAudienceEnumSerializer;

  const EventAudienceEnum._(String name) : super(name);

  static BuiltSet<EventAudienceEnum> get values => _$eventAudienceEnumValues;
  static EventAudienceEnum valueOf(String name) =>
      _$eventAudienceEnumValueOf(name);
}

class EventTypeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'message.created')
  static const EventTypeEnum messagePeriodCreated =
      _$eventTypeEnum_messagePeriodCreated;
  @BuiltValueEnumConst(wireName: r'message.updated')
  static const EventTypeEnum messagePeriodUpdated =
      _$eventTypeEnum_messagePeriodUpdated;
  @BuiltValueEnumConst(wireName: r'message.deleted')
  static const EventTypeEnum messagePeriodDeleted =
      _$eventTypeEnum_messagePeriodDeleted;
  @BuiltValueEnumConst(wireName: r'reaction.added')
  static const EventTypeEnum reactionPeriodAdded =
      _$eventTypeEnum_reactionPeriodAdded;
  @BuiltValueEnumConst(wireName: r'reaction.removed')
  static const EventTypeEnum reactionPeriodRemoved =
      _$eventTypeEnum_reactionPeriodRemoved;
  @BuiltValueEnumConst(wireName: r'pin.added')
  static const EventTypeEnum pinPeriodAdded = _$eventTypeEnum_pinPeriodAdded;
  @BuiltValueEnumConst(wireName: r'pin.removed')
  static const EventTypeEnum pinPeriodRemoved =
      _$eventTypeEnum_pinPeriodRemoved;
  @BuiltValueEnumConst(wireName: r'thread.created')
  static const EventTypeEnum threadPeriodCreated =
      _$eventTypeEnum_threadPeriodCreated;
  @BuiltValueEnumConst(wireName: r'thread.updated')
  static const EventTypeEnum threadPeriodUpdated =
      _$eventTypeEnum_threadPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'thread.closed')
  static const EventTypeEnum threadPeriodClosed =
      _$eventTypeEnum_threadPeriodClosed;
  @BuiltValueEnumConst(wireName: r'thread.reopened')
  static const EventTypeEnum threadPeriodReopened =
      _$eventTypeEnum_threadPeriodReopened;
  @BuiltValueEnumConst(wireName: r'thread.moved')
  static const EventTypeEnum threadPeriodMoved =
      _$eventTypeEnum_threadPeriodMoved;
  @BuiltValueEnumConst(wireName: r'thread.moved_out')
  static const EventTypeEnum threadPeriodMovedOut =
      _$eventTypeEnum_threadPeriodMovedOut;
  @BuiltValueEnumConst(wireName: r'agent_session.changed')
  static const EventTypeEnum agentSessionPeriodChanged =
      _$eventTypeEnum_agentSessionPeriodChanged;
  @BuiltValueEnumConst(wireName: r'run.state_changed')
  static const EventTypeEnum runPeriodStateChanged =
      _$eventTypeEnum_runPeriodStateChanged;
  @BuiltValueEnumConst(wireName: r'space.created')
  static const EventTypeEnum spacePeriodCreated =
      _$eventTypeEnum_spacePeriodCreated;
  @BuiltValueEnumConst(wireName: r'space.updated')
  static const EventTypeEnum spacePeriodUpdated =
      _$eventTypeEnum_spacePeriodUpdated;
  @BuiltValueEnumConst(wireName: r'space.archived')
  static const EventTypeEnum spacePeriodArchived =
      _$eventTypeEnum_spacePeriodArchived;
  @BuiltValueEnumConst(wireName: r'channel.created')
  static const EventTypeEnum channelPeriodCreated =
      _$eventTypeEnum_channelPeriodCreated;
  @BuiltValueEnumConst(wireName: r'channel.updated')
  static const EventTypeEnum channelPeriodUpdated =
      _$eventTypeEnum_channelPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'channel.settings_changed')
  static const EventTypeEnum channelPeriodSettingsChanged =
      _$eventTypeEnum_channelPeriodSettingsChanged;
  @BuiltValueEnumConst(wireName: r'channel.archived')
  static const EventTypeEnum channelPeriodArchived =
      _$eventTypeEnum_channelPeriodArchived;
  @BuiltValueEnumConst(wireName: r'command.changed')
  static const EventTypeEnum commandPeriodChanged =
      _$eventTypeEnum_commandPeriodChanged;
  @BuiltValueEnumConst(wireName: r'file.updated')
  static const EventTypeEnum filePeriodUpdated =
      _$eventTypeEnum_filePeriodUpdated;
  @BuiltValueEnumConst(wireName: r'user.updated')
  static const EventTypeEnum userPeriodUpdated =
      _$eventTypeEnum_userPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'agent.updated')
  static const EventTypeEnum agentPeriodUpdated =
      _$eventTypeEnum_agentPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'instance.updated')
  static const EventTypeEnum instancePeriodUpdated =
      _$eventTypeEnum_instancePeriodUpdated;
  @BuiltValueEnumConst(wireName: r'status.updated')
  static const EventTypeEnum statusPeriodUpdated =
      _$eventTypeEnum_statusPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'grant.changed')
  static const EventTypeEnum grantPeriodChanged =
      _$eventTypeEnum_grantPeriodChanged;
  @BuiltValueEnumConst(wireName: r'group.changed')
  static const EventTypeEnum groupPeriodChanged =
      _$eventTypeEnum_groupPeriodChanged;
  @BuiltValueEnumConst(wireName: r'access.changed')
  static const EventTypeEnum accessPeriodChanged =
      _$eventTypeEnum_accessPeriodChanged;
  @BuiltValueEnumConst(wireName: r'access.expiring')
  static const EventTypeEnum accessPeriodExpiring =
      _$eventTypeEnum_accessPeriodExpiring;
  @BuiltValueEnumConst(wireName: r'session.revoked')
  static const EventTypeEnum sessionPeriodRevoked =
      _$eventTypeEnum_sessionPeriodRevoked;
  @BuiltValueEnumConst(wireName: r'read_state.updated')
  static const EventTypeEnum readStatePeriodUpdated =
      _$eventTypeEnum_readStatePeriodUpdated;
  @BuiltValueEnumConst(wireName: r'draft.updated')
  static const EventTypeEnum draftPeriodUpdated =
      _$eventTypeEnum_draftPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'draft.deleted')
  static const EventTypeEnum draftPeriodDeleted =
      _$eventTypeEnum_draftPeriodDeleted;
  @BuiltValueEnumConst(wireName: r'settings.updated')
  static const EventTypeEnum settingsPeriodUpdated =
      _$eventTypeEnum_settingsPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'sidebar.updated')
  static const EventTypeEnum sidebarPeriodUpdated =
      _$eventTypeEnum_sidebarPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'notification_pref.updated')
  static const EventTypeEnum notificationPrefPeriodUpdated =
      _$eventTypeEnum_notificationPrefPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'quiet_hours.updated')
  static const EventTypeEnum quietHoursPeriodUpdated =
      _$eventTypeEnum_quietHoursPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'gdpr.request_updated')
  static const EventTypeEnum gdprPeriodRequestUpdated =
      _$eventTypeEnum_gdprPeriodRequestUpdated;
  @BuiltValueEnumConst(wireName: r'import.progress')
  static const EventTypeEnum importPeriodProgress =
      _$eventTypeEnum_importPeriodProgress;
  @BuiltValueEnumConst(wireName: r'agent.invocation')
  static const EventTypeEnum agentPeriodInvocation =
      _$eventTypeEnum_agentPeriodInvocation;
  @BuiltValueEnumConst(wireName: r'agent.followup')
  static const EventTypeEnum agentPeriodFollowup =
      _$eventTypeEnum_agentPeriodFollowup;
  @BuiltValueEnumConst(wireName: r'run.cancel_requested')
  static const EventTypeEnum runPeriodCancelRequested =
      _$eventTypeEnum_runPeriodCancelRequested;
  @BuiltValueEnumConst(wireName: r'interaction.created')
  static const EventTypeEnum interactionPeriodCreated =
      _$eventTypeEnum_interactionPeriodCreated;
  @BuiltValueEnumConst(wireName: r'command.invoked')
  static const EventTypeEnum commandPeriodInvoked =
      _$eventTypeEnum_commandPeriodInvoked;
  @BuiltValueEnumConst(wireName: r'agent.config_changed')
  static const EventTypeEnum agentPeriodConfigChanged =
      _$eventTypeEnum_agentPeriodConfigChanged;
  @BuiltValueEnumConst(wireName: r'host.updated')
  static const EventTypeEnum hostPeriodUpdated =
      _$eventTypeEnum_hostPeriodUpdated;
  @BuiltValueEnumConst(wireName: r'terminal_session.changed')
  static const EventTypeEnum terminalSessionPeriodChanged =
      _$eventTypeEnum_terminalSessionPeriodChanged;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const EventTypeEnum unknownDefaultOpenApi =
      _$eventTypeEnum_unknownDefaultOpenApi;

  static Serializer<EventTypeEnum> get serializer => _$eventTypeEnumSerializer;

  const EventTypeEnum._(String name) : super(name);

  static BuiltSet<EventTypeEnum> get values => _$eventTypeEnumValues;
  static EventTypeEnum valueOf(String name) => _$eventTypeEnumValueOf(name);
}

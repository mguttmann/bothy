//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/channel_view.dart';
import 'package:api_client/src/generated/model/space_view.dart';
import 'package:api_client/src/generated/model/draft_ref.dart';
import 'package:api_client/src/generated/model/agent_presence.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_summary.dart';
import 'package:api_client/src/generated/model/unread_thread.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:api_client/src/generated/model/sidebar_state.dart';
import 'package:api_client/src/generated/model/evaluated_flag.dart';
import 'package:api_client/src/generated/model/user_setting.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'snapshot.g.dart';

/// State without thread contents, delivered by `ready` and `GET /sync/snapshot`.
///
/// Properties:
/// * [headSeq]
/// * [permissionVersion]
/// * [syncEpoch] - Time line of the event log (04, 7.5).
/// * [principal]
/// * [isOwner]
/// * [spaces]
/// * [channels]
/// * [sidebar]
/// * [unreadThreads] - Only threads with unread messages or mentions.
/// * [activeRuns]
/// * [agents]
/// * [settings]
/// * [drafts]
/// * [features] - Feature flags evaluated for the principal, as in `InstanceInfo.features` (04 section 6.3, step 4).
@BuiltValue()
abstract class Snapshot implements Built<Snapshot, SnapshotBuilder> {
  @BuiltValueField(wireName: r'head_seq')
  int get headSeq;

  @BuiltValueField(wireName: r'permission_version')
  int get permissionVersion;

  /// Time line of the event log (04, 7.5).
  @BuiltValueField(wireName: r'sync_epoch')
  String get syncEpoch;

  @BuiltValueField(wireName: r'principal')
  PrincipalRef get principal;

  @BuiltValueField(wireName: r'is_owner')
  bool get isOwner;

  @BuiltValueField(wireName: r'spaces')
  BuiltList<SpaceView> get spaces;

  @BuiltValueField(wireName: r'channels')
  BuiltList<ChannelView> get channels;

  @BuiltValueField(wireName: r'sidebar')
  BuiltList<SidebarState> get sidebar;

  /// Only threads with unread messages or mentions.
  @BuiltValueField(wireName: r'unread_threads')
  BuiltList<UnreadThread> get unreadThreads;

  @BuiltValueField(wireName: r'active_runs')
  BuiltList<RunSummary> get activeRuns;

  @BuiltValueField(wireName: r'agents')
  BuiltList<AgentPresence> get agents;

  @BuiltValueField(wireName: r'settings')
  BuiltList<UserSetting> get settings;

  @BuiltValueField(wireName: r'drafts')
  BuiltList<DraftRef> get drafts;

  /// Feature flags evaluated for the principal, as in `InstanceInfo.features` (04 section 6.3, step 4).
  @BuiltValueField(wireName: r'features')
  BuiltList<EvaluatedFlag> get features;

  Snapshot._();

  factory Snapshot([void updates(SnapshotBuilder b)]) = _$Snapshot;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SnapshotBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Snapshot> get serializer => _$SnapshotSerializer();
}

class _$SnapshotSerializer implements PrimitiveSerializer<Snapshot> {
  @override
  final Iterable<Type> types = const [Snapshot, _$Snapshot];

  @override
  final String wireName = r'Snapshot';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Snapshot object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'head_seq';
    yield serializers.serialize(
      object.headSeq,
      specifiedType: const FullType(int),
    );
    yield r'permission_version';
    yield serializers.serialize(
      object.permissionVersion,
      specifiedType: const FullType(int),
    );
    yield r'sync_epoch';
    yield serializers.serialize(
      object.syncEpoch,
      specifiedType: const FullType(String),
    );
    yield r'principal';
    yield serializers.serialize(
      object.principal,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'is_owner';
    yield serializers.serialize(
      object.isOwner,
      specifiedType: const FullType(bool),
    );
    yield r'spaces';
    yield serializers.serialize(
      object.spaces,
      specifiedType: const FullType(BuiltList, [FullType(SpaceView)]),
    );
    yield r'channels';
    yield serializers.serialize(
      object.channels,
      specifiedType: const FullType(BuiltList, [FullType(ChannelView)]),
    );
    yield r'sidebar';
    yield serializers.serialize(
      object.sidebar,
      specifiedType: const FullType(BuiltList, [FullType(SidebarState)]),
    );
    yield r'unread_threads';
    yield serializers.serialize(
      object.unreadThreads,
      specifiedType: const FullType(BuiltList, [FullType(UnreadThread)]),
    );
    yield r'active_runs';
    yield serializers.serialize(
      object.activeRuns,
      specifiedType: const FullType(BuiltList, [FullType(RunSummary)]),
    );
    yield r'agents';
    yield serializers.serialize(
      object.agents,
      specifiedType: const FullType(BuiltList, [FullType(AgentPresence)]),
    );
    yield r'settings';
    yield serializers.serialize(
      object.settings,
      specifiedType: const FullType(BuiltList, [FullType(UserSetting)]),
    );
    yield r'drafts';
    yield serializers.serialize(
      object.drafts,
      specifiedType: const FullType(BuiltList, [FullType(DraftRef)]),
    );
    yield r'features';
    yield serializers.serialize(
      object.features,
      specifiedType: const FullType(BuiltList, [FullType(EvaluatedFlag)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Snapshot object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required SnapshotBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'head_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.headSeq = valueDes;
          break;
        case r'permission_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.permissionVersion = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.syncEpoch = valueDes;
          break;
        case r'principal':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.principal.replace(valueDes);
          break;
        case r'is_owner':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.isOwner = valueDes;
          break;
        case r'spaces':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(SpaceView)]),
          ) as BuiltList<SpaceView>;
          result.spaces.replace(valueDes);
          break;
        case r'channels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ChannelView)]),
          ) as BuiltList<ChannelView>;
          result.channels.replace(valueDes);
          break;
        case r'sidebar':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(SidebarState)]),
          ) as BuiltList<SidebarState>;
          result.sidebar.replace(valueDes);
          break;
        case r'unread_threads':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(UnreadThread)]),
          ) as BuiltList<UnreadThread>;
          result.unreadThreads.replace(valueDes);
          break;
        case r'active_runs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(RunSummary)]),
          ) as BuiltList<RunSummary>;
          result.activeRuns.replace(valueDes);
          break;
        case r'agents':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AgentPresence)]),
          ) as BuiltList<AgentPresence>;
          result.agents.replace(valueDes);
          break;
        case r'settings':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(UserSetting)]),
          ) as BuiltList<UserSetting>;
          result.settings.replace(valueDes);
          break;
        case r'drafts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(DraftRef)]),
          ) as BuiltList<DraftRef>;
          result.drafts.replace(valueDes);
          break;
        case r'features':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(EvaluatedFlag)]),
          ) as BuiltList<EvaluatedFlag>;
          result.features.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Snapshot deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SnapshotBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/channel_view.dart';
import 'package:api_client/src/generated/model/space_view.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'view_as_snapshot.g.dart';

/// Read-only view of what a person can see, for owners (07, 9.2). Direct messages never appear.
///
/// Properties:
/// * [user]
/// * [expiresAt] - End of the view, `authz.view_as_max_duration` after its start.
/// * [spaces]
/// * [channels]
@BuiltValue()
abstract class ViewAsSnapshot
    implements Built<ViewAsSnapshot, ViewAsSnapshotBuilder> {
  @BuiltValueField(wireName: r'user')
  PrincipalRef get user;

  /// End of the view, `authz.view_as_max_duration` after its start.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  @BuiltValueField(wireName: r'spaces')
  BuiltList<SpaceView> get spaces;

  @BuiltValueField(wireName: r'channels')
  BuiltList<ChannelView> get channels;

  ViewAsSnapshot._();

  factory ViewAsSnapshot([void updates(ViewAsSnapshotBuilder b)]) =
      _$ViewAsSnapshot;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ViewAsSnapshotBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ViewAsSnapshot> get serializer =>
      _$ViewAsSnapshotSerializer();
}

class _$ViewAsSnapshotSerializer
    implements PrimitiveSerializer<ViewAsSnapshot> {
  @override
  final Iterable<Type> types = const [ViewAsSnapshot, _$ViewAsSnapshot];

  @override
  final String wireName = r'ViewAsSnapshot';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ViewAsSnapshot object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user';
    yield serializers.serialize(
      object.user,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
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
  }

  @override
  Object serialize(
    Serializers serializers,
    ViewAsSnapshot object, {
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
    required ViewAsSnapshotBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.user.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ViewAsSnapshot deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ViewAsSnapshotBuilder();
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

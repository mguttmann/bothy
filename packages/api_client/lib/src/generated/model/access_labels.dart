//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/channel_label.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:api_client/src/generated/model/access_label.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_labels.g.dart';

/// The names of the principals, groups, spaces and channels a page of hints or expiring access names, as the administration lists show them; never content.
///
/// Properties:
/// * [principals]
/// * [groups]
/// * [spaces]
/// * [channels]
@BuiltValue()
abstract class AccessLabels
    implements Built<AccessLabels, AccessLabelsBuilder> {
  @BuiltValueField(wireName: r'principals')
  BuiltList<PrincipalRef> get principals;

  @BuiltValueField(wireName: r'groups')
  BuiltList<AccessLabel> get groups;

  @BuiltValueField(wireName: r'spaces')
  BuiltList<AccessLabel> get spaces;

  @BuiltValueField(wireName: r'channels')
  BuiltList<ChannelLabel> get channels;

  AccessLabels._();

  factory AccessLabels([void updates(AccessLabelsBuilder b)]) = _$AccessLabels;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessLabelsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessLabels> get serializer => _$AccessLabelsSerializer();
}

class _$AccessLabelsSerializer implements PrimitiveSerializer<AccessLabels> {
  @override
  final Iterable<Type> types = const [AccessLabels, _$AccessLabels];

  @override
  final String wireName = r'AccessLabels';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessLabels object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'principals';
    yield serializers.serialize(
      object.principals,
      specifiedType: const FullType(BuiltList, [FullType(PrincipalRef)]),
    );
    yield r'groups';
    yield serializers.serialize(
      object.groups,
      specifiedType: const FullType(BuiltList, [FullType(AccessLabel)]),
    );
    yield r'spaces';
    yield serializers.serialize(
      object.spaces,
      specifiedType: const FullType(BuiltList, [FullType(AccessLabel)]),
    );
    yield r'channels';
    yield serializers.serialize(
      object.channels,
      specifiedType: const FullType(BuiltList, [FullType(ChannelLabel)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessLabels object, {
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
    required AccessLabelsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'principals':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(PrincipalRef)]),
          ) as BuiltList<PrincipalRef>;
          result.principals.replace(valueDes);
          break;
        case r'groups':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessLabel)]),
          ) as BuiltList<AccessLabel>;
          result.groups.replace(valueDes);
          break;
        case r'spaces':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessLabel)]),
          ) as BuiltList<AccessLabel>;
          result.spaces.replace(valueDes);
          break;
        case r'channels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ChannelLabel)]),
          ) as BuiltList<ChannelLabel>;
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
  AccessLabels deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessLabelsBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_state.g.dart';

/// SidebarState
///
/// Properties:
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [collapsed]
/// * [sortKey] - Personal order; absent uses the global order.
/// * [version]
/// * [updatedByDeviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class SidebarState
    implements Built<SidebarState, SidebarStateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String get spaceId;

  @BuiltValueField(wireName: r'collapsed')
  bool get collapsed;

  /// Personal order; absent uses the global order.
  @BuiltValueField(wireName: r'sort_key')
  String? get sortKey;

  @BuiltValueField(wireName: r'version')
  int get version;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'updated_by_device_id')
  String? get updatedByDeviceId;

  SidebarState._();

  factory SidebarState([void updates(SidebarStateBuilder b)]) = _$SidebarState;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarStateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarState> get serializer => _$SidebarStateSerializer();
}

class _$SidebarStateSerializer implements PrimitiveSerializer<SidebarState> {
  @override
  final Iterable<Type> types = const [SidebarState, _$SidebarState];

  @override
  final String wireName = r'SidebarState';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarState object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'space_id';
    yield serializers.serialize(
      object.spaceId,
      specifiedType: const FullType(String),
    );
    yield r'collapsed';
    yield serializers.serialize(
      object.collapsed,
      specifiedType: const FullType(bool),
    );
    if (object.sortKey != null) {
      yield r'sort_key';
      yield serializers.serialize(
        object.sortKey,
        specifiedType: const FullType(String),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    if (object.updatedByDeviceId != null) {
      yield r'updated_by_device_id';
      yield serializers.serialize(
        object.updatedByDeviceId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarState object, {
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
    required SidebarStateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.spaceId = valueDes;
          break;
        case r'collapsed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.collapsed = valueDes;
          break;
        case r'sort_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.sortKey = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'updated_by_device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.updatedByDeviceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SidebarState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarStateBuilder();
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

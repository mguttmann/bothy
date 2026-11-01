//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'space.g.dart';

/// Space
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [icon] - Glyph name from the list `space-icons` of the design tokens (`kernel/designkeys`). No emoji.
/// * [color] - Key of a category color of the design system, never a hex value.
/// * [sortKey] - Fractional index, compared bytewise.
/// * [defaultOperatorAgentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [archivedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [deletedAt] - Set while the item is deleted, pending or final (01 6.18); clients remove it and its local content, and a later version without it brings it back.
/// * [version]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class Space implements Built<Space, SpaceBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'name')
  String get name;

  /// Glyph name from the list `space-icons` of the design tokens (`kernel/designkeys`). No emoji.
  @BuiltValueField(wireName: r'icon')
  String? get icon;

  /// Key of a category color of the design system, never a hex value.
  @BuiltValueField(wireName: r'color')
  String? get color;

  /// Fractional index, compared bytewise.
  @BuiltValueField(wireName: r'sort_key')
  String get sortKey;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'default_operator_agent_id')
  String? get defaultOperatorAgentId;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'archived_at')
  DateTime? get archivedAt;

  /// Set while the item is deleted, pending or final (01 6.18); clients remove it and its local content, and a later version without it brings it back.
  @BuiltValueField(wireName: r'deleted_at')
  DateTime? get deletedAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  Space._();

  factory Space([void updates(SpaceBuilder b)]) = _$Space;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SpaceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Space> get serializer => _$SpaceSerializer();
}

class _$SpaceSerializer implements PrimitiveSerializer<Space> {
  @override
  final Iterable<Type> types = const [Space, _$Space];

  @override
  final String wireName = r'Space';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Space object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.icon != null) {
      yield r'icon';
      yield serializers.serialize(
        object.icon,
        specifiedType: const FullType(String),
      );
    }
    if (object.color != null) {
      yield r'color';
      yield serializers.serialize(
        object.color,
        specifiedType: const FullType(String),
      );
    }
    yield r'sort_key';
    yield serializers.serialize(
      object.sortKey,
      specifiedType: const FullType(String),
    );
    if (object.defaultOperatorAgentId != null) {
      yield r'default_operator_agent_id';
      yield serializers.serialize(
        object.defaultOperatorAgentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.archivedAt != null) {
      yield r'archived_at';
      yield serializers.serialize(
        object.archivedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.deletedAt != null) {
      yield r'deleted_at';
      yield serializers.serialize(
        object.deletedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Space object, {
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
    required SpaceBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'icon':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.icon = valueDes;
          break;
        case r'color':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.color = valueDes;
          break;
        case r'sort_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sortKey = valueDes;
          break;
        case r'default_operator_agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.defaultOperatorAgentId = valueDes;
          break;
        case r'archived_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.archivedAt = valueDes;
          break;
        case r'deleted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.deletedAt = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Space deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SpaceBuilder();
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

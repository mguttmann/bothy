//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/principal_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'principal_ref.g.dart';

/// Minimal public view of a principal, embedded in other objects.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [handle]
/// * [displayName]
/// * [avatarFileId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class PrincipalRef
    implements Built<PrincipalRef, PrincipalRefBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'kind')
  PrincipalKind get kind;
  // enum kindEnum {  user,  agent,  app,  };

  @BuiltValueField(wireName: r'handle')
  String get handle;

  @BuiltValueField(wireName: r'display_name')
  String get displayName;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'avatar_file_id')
  String? get avatarFileId;

  PrincipalRef._();

  factory PrincipalRef([void updates(PrincipalRefBuilder b)]) = _$PrincipalRef;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PrincipalRefBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PrincipalRef> get serializer => _$PrincipalRefSerializer();
}

class _$PrincipalRefSerializer implements PrimitiveSerializer<PrincipalRef> {
  @override
  final Iterable<Type> types = const [PrincipalRef, _$PrincipalRef];

  @override
  final String wireName = r'PrincipalRef';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PrincipalRef object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(PrincipalKind),
    );
    yield r'handle';
    yield serializers.serialize(
      object.handle,
      specifiedType: const FullType(String),
    );
    yield r'display_name';
    yield serializers.serialize(
      object.displayName,
      specifiedType: const FullType(String),
    );
    if (object.avatarFileId != null) {
      yield r'avatar_file_id';
      yield serializers.serialize(
        object.avatarFileId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PrincipalRef object, {
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
    required PrincipalRefBuilder result,
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
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalKind),
          ) as PrincipalKind;
          result.kind = valueDes;
          break;
        case r'handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.handle = valueDes;
          break;
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.displayName = valueDes;
          break;
        case r'avatar_file_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.avatarFileId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PrincipalRef deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PrincipalRefBuilder();
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

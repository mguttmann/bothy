//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/resolve_i1.dart';
import 'package:api_client/src/generated/model/target_kind.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level.dart';
import 'package:api_client/src/generated/model/grant_subject_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'grant_create.g.dart';

/// `level` is required for every target except `host`, where it must be absent and `capabilities` must be exactly `[terminal]` (07, 2.12).
///
/// Properties:
/// * [subjectKind]
/// * [subjectId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [targetKind]
/// * [targetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [level]
/// * [capabilities]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [reason]
/// * [resolveI1]
@BuiltValue()
abstract class GrantCreate implements Built<GrantCreate, GrantCreateBuilder> {
  @BuiltValueField(wireName: r'subject_kind')
  GrantSubjectKind get subjectKind;
  // enum subjectKindEnum {  user,  group,  agent,  app,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'subject_id')
  String get subjectId;

  @BuiltValueField(wireName: r'target_kind')
  TargetKind get targetKind;
  // enum targetKindEnum {  instance,  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_id')
  String? get targetId;

  @BuiltValueField(wireName: r'level')
  Level? get level;
  // enum levelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability>? get capabilities;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime? get expiresAt;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  @BuiltValueField(wireName: r'resolve_i1')
  ResolveI1? get resolveI1;

  GrantCreate._();

  factory GrantCreate([void updates(GrantCreateBuilder b)]) = _$GrantCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GrantCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GrantCreate> get serializer => _$GrantCreateSerializer();
}

class _$GrantCreateSerializer implements PrimitiveSerializer<GrantCreate> {
  @override
  final Iterable<Type> types = const [GrantCreate, _$GrantCreate];

  @override
  final String wireName = r'GrantCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GrantCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'subject_kind';
    yield serializers.serialize(
      object.subjectKind,
      specifiedType: const FullType(GrantSubjectKind),
    );
    yield r'subject_id';
    yield serializers.serialize(
      object.subjectId,
      specifiedType: const FullType(String),
    );
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(TargetKind),
    );
    if (object.targetId != null) {
      yield r'target_id';
      yield serializers.serialize(
        object.targetId,
        specifiedType: const FullType(String),
      );
    }
    if (object.level != null) {
      yield r'level';
      yield serializers.serialize(
        object.level,
        specifiedType: const FullType(Level),
      );
    }
    if (object.capabilities != null) {
      yield r'capabilities';
      yield serializers.serialize(
        object.capabilities,
        specifiedType: const FullType(BuiltList, [FullType(Capability)]),
      );
    }
    if (object.expiresAt != null) {
      yield r'expires_at';
      yield serializers.serialize(
        object.expiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
    if (object.resolveI1 != null) {
      yield r'resolve_i1';
      yield serializers.serialize(
        object.resolveI1,
        specifiedType: const FullType(ResolveI1),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GrantCreate object, {
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
    required GrantCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'subject_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GrantSubjectKind),
          ) as GrantSubjectKind;
          result.subjectKind = valueDes;
          break;
        case r'subject_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.subjectId = valueDes;
          break;
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TargetKind),
          ) as TargetKind;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.targetId = valueDes;
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Level),
          ) as Level?;
          if (valueDes == null) continue;
          result.level = valueDes;
          break;
        case r'capabilities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(Capability),
            ]),
          ) as BuiltList<Capability>?;
          if (valueDes == null) continue;
          result.capabilities.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.expiresAt = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        case r'resolve_i1':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ResolveI1),
          ) as ResolveI1?;
          if (valueDes == null) continue;
          result.resolveI1.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GrantCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GrantCreateBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/access_source.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level_or_none.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_entry.g.dart';

/// AccessEntry
///
/// Properties:
/// * [principal]
/// * [level] - `none` only for entries whose sources are all `via_agent`.
/// * [inactive] - True when the principal is not `active`; the level is what its grants would give (07, 9.1).
/// * [capabilities]
/// * [sources]
@BuiltValue()
abstract class AccessEntry implements Built<AccessEntry, AccessEntryBuilder> {
  @BuiltValueField(wireName: r'principal')
  PrincipalRef get principal;

  /// `none` only for entries whose sources are all `via_agent`.
  @BuiltValueField(wireName: r'level')
  LevelOrNone get level;
  // enum levelEnum {  none,  read,  write,  trigger,  };

  /// True when the principal is not `active`; the level is what its grants would give (07, 9.1).
  @BuiltValueField(wireName: r'inactive')
  bool? get inactive;

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability> get capabilities;

  @BuiltValueField(wireName: r'sources')
  BuiltList<AccessSource> get sources;

  AccessEntry._();

  factory AccessEntry([void updates(AccessEntryBuilder b)]) = _$AccessEntry;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessEntryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessEntry> get serializer => _$AccessEntrySerializer();
}

class _$AccessEntrySerializer implements PrimitiveSerializer<AccessEntry> {
  @override
  final Iterable<Type> types = const [AccessEntry, _$AccessEntry];

  @override
  final String wireName = r'AccessEntry';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessEntry object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'principal';
    yield serializers.serialize(
      object.principal,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'level';
    yield serializers.serialize(
      object.level,
      specifiedType: const FullType(LevelOrNone),
    );
    if (object.inactive != null) {
      yield r'inactive';
      yield serializers.serialize(
        object.inactive,
        specifiedType: const FullType(bool),
      );
    }
    yield r'capabilities';
    yield serializers.serialize(
      object.capabilities,
      specifiedType: const FullType(BuiltList, [FullType(Capability)]),
    );
    yield r'sources';
    yield serializers.serialize(
      object.sources,
      specifiedType: const FullType(BuiltList, [FullType(AccessSource)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessEntry object, {
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
    required AccessEntryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'principal':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.principal.replace(valueDes);
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(LevelOrNone),
          ) as LevelOrNone;
          result.level = valueDes;
          break;
        case r'inactive':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.inactive = valueDes;
          break;
        case r'capabilities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Capability)]),
          ) as BuiltList<Capability>;
          result.capabilities.replace(valueDes);
          break;
        case r'sources':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessSource)]),
          ) as BuiltList<AccessSource>;
          result.sources.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessEntry deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessEntryBuilder();
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

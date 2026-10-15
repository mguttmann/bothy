//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/target_kind.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_manifest_requested_grants_inner.g.dart';

/// AppManifestRequestedGrantsInner
///
/// Properties:
/// * [targetKind]
/// * [level]
/// * [capabilities]
/// * [purpose]
@BuiltValue()
abstract class AppManifestRequestedGrantsInner
    implements
        Built<
          AppManifestRequestedGrantsInner,
          AppManifestRequestedGrantsInnerBuilder
        > {
  @BuiltValueField(wireName: r'target_kind')
  TargetKind get targetKind;
  // enum targetKindEnum {  instance,  space,  channel,  host,  };

  @BuiltValueField(wireName: r'level')
  Level get level;
  // enum levelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability>? get capabilities;

  @BuiltValueField(wireName: r'purpose')
  String get purpose;

  AppManifestRequestedGrantsInner._();

  factory AppManifestRequestedGrantsInner([
    void updates(AppManifestRequestedGrantsInnerBuilder b),
  ]) = _$AppManifestRequestedGrantsInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppManifestRequestedGrantsInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppManifestRequestedGrantsInner> get serializer =>
      _$AppManifestRequestedGrantsInnerSerializer();
}

class _$AppManifestRequestedGrantsInnerSerializer
    implements PrimitiveSerializer<AppManifestRequestedGrantsInner> {
  @override
  final Iterable<Type> types = const [
    AppManifestRequestedGrantsInner,
    _$AppManifestRequestedGrantsInner,
  ];

  @override
  final String wireName = r'AppManifestRequestedGrantsInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppManifestRequestedGrantsInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(TargetKind),
    );
    yield r'level';
    yield serializers.serialize(
      object.level,
      specifiedType: const FullType(Level),
    );
    if (object.capabilities != null) {
      yield r'capabilities';
      yield serializers.serialize(
        object.capabilities,
        specifiedType: const FullType(BuiltList, [FullType(Capability)]),
      );
    }
    yield r'purpose';
    yield serializers.serialize(
      object.purpose,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppManifestRequestedGrantsInner object, {
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
    required AppManifestRequestedGrantsInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TargetKind),
          ) as TargetKind;
          result.targetKind = valueDes;
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Level),
          ) as Level;
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
        case r'purpose':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.purpose = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppManifestRequestedGrantsInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppManifestRequestedGrantsInnerBuilder();
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

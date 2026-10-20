//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_runner_config.g.dart';

/// Schema `project_runner_config.v1`. No secrets.
///
/// Properties:
/// * [model]
/// * [allowedTools]
@BuiltValue()
abstract class ProjectRunnerConfig
    implements Built<ProjectRunnerConfig, ProjectRunnerConfigBuilder> {
  @BuiltValueField(wireName: r'model')
  String? get model;

  @BuiltValueField(wireName: r'allowed_tools')
  BuiltSet<String>? get allowedTools;

  ProjectRunnerConfig._();

  factory ProjectRunnerConfig([void updates(ProjectRunnerConfigBuilder b)]) =
      _$ProjectRunnerConfig;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectRunnerConfigBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectRunnerConfig> get serializer =>
      _$ProjectRunnerConfigSerializer();
}

class _$ProjectRunnerConfigSerializer
    implements PrimitiveSerializer<ProjectRunnerConfig> {
  @override
  final Iterable<Type> types = const [
    ProjectRunnerConfig,
    _$ProjectRunnerConfig,
  ];

  @override
  final String wireName = r'ProjectRunnerConfig';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectRunnerConfig object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
    if (object.allowedTools != null) {
      yield r'allowed_tools';
      yield serializers.serialize(
        object.allowedTools,
        specifiedType: const FullType(BuiltSet, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectRunnerConfig object, {
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
    required ProjectRunnerConfigBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        case r'allowed_tools':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltSet, [
              FullType(String),
            ]),
          ) as BuiltSet<String>?;
          if (valueDes == null) continue;
          result.allowedTools.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectRunnerConfig deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectRunnerConfigBuilder();
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

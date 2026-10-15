//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_scope.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_scopes_put.g.dart';

/// AppScopesPut
///
/// Properties:
/// * [grantedScopes]
/// * [eventSubscriptions]
/// * [reason]
@BuiltValue()
abstract class AppScopesPut
    implements Built<AppScopesPut, AppScopesPutBuilder> {
  @BuiltValueField(wireName: r'granted_scopes')
  BuiltList<AppScope> get grantedScopes;

  @BuiltValueField(wireName: r'event_subscriptions')
  BuiltList<String>? get eventSubscriptions;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  AppScopesPut._();

  factory AppScopesPut([void updates(AppScopesPutBuilder b)]) = _$AppScopesPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppScopesPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppScopesPut> get serializer => _$AppScopesPutSerializer();
}

class _$AppScopesPutSerializer implements PrimitiveSerializer<AppScopesPut> {
  @override
  final Iterable<Type> types = const [AppScopesPut, _$AppScopesPut];

  @override
  final String wireName = r'AppScopesPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppScopesPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'granted_scopes';
    yield serializers.serialize(
      object.grantedScopes,
      specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
    );
    if (object.eventSubscriptions != null) {
      yield r'event_subscriptions';
      yield serializers.serialize(
        object.eventSubscriptions,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppScopesPut object, {
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
    required AppScopesPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'granted_scopes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
          ) as BuiltList<AppScope>;
          result.grantedScopes.replace(valueDes);
          break;
        case r'event_subscriptions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.eventSubscriptions.replace(valueDes);
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppScopesPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppScopesPutBuilder();
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

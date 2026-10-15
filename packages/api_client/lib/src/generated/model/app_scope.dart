//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_scope.g.dart';

class AppScope extends EnumClass {
  @BuiltValueEnumConst(wireName: r'messages:read')
  static const AppScope messagesColonRead = _$messagesColonRead;
  @BuiltValueEnumConst(wireName: r'messages:write')
  static const AppScope messagesColonWrite = _$messagesColonWrite;
  @BuiltValueEnumConst(wireName: r'files:read')
  static const AppScope filesColonRead = _$filesColonRead;
  @BuiltValueEnumConst(wireName: r'files:write')
  static const AppScope filesColonWrite = _$filesColonWrite;
  @BuiltValueEnumConst(wireName: r'commands:write')
  static const AppScope commandsColonWrite = _$commandsColonWrite;
  @BuiltValueEnumConst(wireName: r'components:write')
  static const AppScope componentsColonWrite = _$componentsColonWrite;
  @BuiltValueEnumConst(wireName: r'users:read')
  static const AppScope usersColonRead = _$usersColonRead;
  @BuiltValueEnumConst(wireName: r'events:subscribe')
  static const AppScope eventsColonSubscribe = _$eventsColonSubscribe;
  @BuiltValueEnumConst(wireName: r'costs:reconcile')
  static const AppScope costsColonReconcile = _$costsColonReconcile;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AppScope unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<AppScope> get serializer => _$appScopeSerializer;

  const AppScope._(String name) : super(name);

  static BuiltSet<AppScope> get values => _$values;
  static AppScope valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class AppScopeMixin = Object with _$AppScopeMixin;

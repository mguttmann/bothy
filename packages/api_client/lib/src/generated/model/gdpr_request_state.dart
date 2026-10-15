//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gdpr_request_state.g.dart';

class GdprRequestState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'requested')
  static const GdprRequestState requested = _$requested;
  @BuiltValueEnumConst(wireName: r'approved')
  static const GdprRequestState approved = _$approved;
  @BuiltValueEnumConst(wireName: r'running')
  static const GdprRequestState running = _$running;
  @BuiltValueEnumConst(wireName: r'completed')
  static const GdprRequestState completed = _$completed;
  @BuiltValueEnumConst(wireName: r'failed')
  static const GdprRequestState failed = _$failed;
  @BuiltValueEnumConst(wireName: r'rejected')
  static const GdprRequestState rejected = _$rejected;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const GdprRequestState cancelled = _$cancelled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GdprRequestState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<GdprRequestState> get serializer =>
      _$gdprRequestStateSerializer;

  const GdprRequestState._(String name) : super(name);

  static BuiltSet<GdprRequestState> get values => _$values;
  static GdprRequestState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class GdprRequestStateMixin = Object with _$GdprRequestStateMixin;

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_close_reason.g.dart';

/// Why a computer session ended (docs/spec/27-computer-use.md 8.3).
class ComputerCloseReason extends EnumClass {
  @BuiltValueEnumConst(wireName: r'done')
  static const ComputerCloseReason done = _$done;
  @BuiltValueEnumConst(wireName: r'stopped_by_user')
  static const ComputerCloseReason stoppedByUser = _$stoppedByUser;
  @BuiltValueEnumConst(wireName: r'device_disabled')
  static const ComputerCloseReason deviceDisabled = _$deviceDisabled;
  @BuiltValueEnumConst(wireName: r'run_ended')
  static const ComputerCloseReason runEnded = _$runEnded;
  @BuiltValueEnumConst(wireName: r'access_revoked')
  static const ComputerCloseReason accessRevoked = _$accessRevoked;
  @BuiltValueEnumConst(wireName: r'host_disabled')
  static const ComputerCloseReason hostDisabled = _$hostDisabled;
  @BuiltValueEnumConst(wireName: r'host_retired')
  static const ComputerCloseReason hostRetired = _$hostRetired;
  @BuiltValueEnumConst(wireName: r'host_rekeyed')
  static const ComputerCloseReason hostRekeyed = _$hostRekeyed;
  @BuiltValueEnumConst(wireName: r'host_restarted')
  static const ComputerCloseReason hostRestarted = _$hostRestarted;
  @BuiltValueEnumConst(wireName: r'connection_lost')
  static const ComputerCloseReason connectionLost = _$connectionLost;
  @BuiltValueEnumConst(wireName: r'open_failed')
  static const ComputerCloseReason openFailed = _$openFailed;
  @BuiltValueEnumConst(wireName: r'error')
  static const ComputerCloseReason error = _$error;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComputerCloseReason unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<ComputerCloseReason> get serializer =>
      _$computerCloseReasonSerializer;

  const ComputerCloseReason._(String name) : super(name);

  static BuiltSet<ComputerCloseReason> get values => _$values;
  static ComputerCloseReason valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ComputerCloseReasonMixin = Object
    with _$ComputerCloseReasonMixin;

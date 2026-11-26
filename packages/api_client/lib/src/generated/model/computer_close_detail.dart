//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_close_detail.g.dart';

/// Closed list of 27 8.3: only with `stopped_by_user` (`local_escape`, `indicator_stop`, `client_stop`), `open_failed` (`token_invalid`, `token_expired`, `token_replayed`, `handshake_failed`, `helper_absent`, `helper_untrusted`, `not_enabled`, `busy`, `open_timeout`) and `error` (`stream_fault`, `helper_crashed`).
class ComputerCloseDetail extends EnumClass {
  @BuiltValueEnumConst(wireName: r'local_escape')
  static const ComputerCloseDetail localEscape = _$localEscape;
  @BuiltValueEnumConst(wireName: r'indicator_stop')
  static const ComputerCloseDetail indicatorStop = _$indicatorStop;
  @BuiltValueEnumConst(wireName: r'client_stop')
  static const ComputerCloseDetail clientStop = _$clientStop;
  @BuiltValueEnumConst(wireName: r'token_invalid')
  static const ComputerCloseDetail tokenInvalid = _$tokenInvalid;
  @BuiltValueEnumConst(wireName: r'token_expired')
  static const ComputerCloseDetail tokenExpired = _$tokenExpired;
  @BuiltValueEnumConst(wireName: r'token_replayed')
  static const ComputerCloseDetail tokenReplayed = _$tokenReplayed;
  @BuiltValueEnumConst(wireName: r'handshake_failed')
  static const ComputerCloseDetail handshakeFailed = _$handshakeFailed;
  @BuiltValueEnumConst(wireName: r'helper_absent')
  static const ComputerCloseDetail helperAbsent = _$helperAbsent;
  @BuiltValueEnumConst(wireName: r'helper_untrusted')
  static const ComputerCloseDetail helperUntrusted = _$helperUntrusted;
  @BuiltValueEnumConst(wireName: r'not_enabled')
  static const ComputerCloseDetail notEnabled = _$notEnabled;
  @BuiltValueEnumConst(wireName: r'busy')
  static const ComputerCloseDetail busy = _$busy;
  @BuiltValueEnumConst(wireName: r'open_timeout')
  static const ComputerCloseDetail openTimeout = _$openTimeout;
  @BuiltValueEnumConst(wireName: r'stream_fault')
  static const ComputerCloseDetail streamFault = _$streamFault;
  @BuiltValueEnumConst(wireName: r'helper_crashed')
  static const ComputerCloseDetail helperCrashed = _$helperCrashed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComputerCloseDetail unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<ComputerCloseDetail> get serializer =>
      _$computerCloseDetailSerializer;

  const ComputerCloseDetail._(String name) : super(name);

  static BuiltSet<ComputerCloseDetail> get values => _$values;
  static ComputerCloseDetail valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ComputerCloseDetailMixin = Object
    with _$ComputerCloseDetailMixin;

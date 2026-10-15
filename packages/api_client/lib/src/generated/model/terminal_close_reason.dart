//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_close_reason.g.dart';

/// Why a session ended (24, 10.3).
class TerminalCloseReason extends EnumClass {
  @BuiltValueEnumConst(wireName: r'client_closed')
  static const TerminalCloseReason clientClosed = _$clientClosed;
  @BuiltValueEnumConst(wireName: r'closed_by_owner')
  static const TerminalCloseReason closedByOwner = _$closedByOwner;
  @BuiltValueEnumConst(wireName: r'process_exited')
  static const TerminalCloseReason processExited = _$processExited;
  @BuiltValueEnumConst(wireName: r'idle_timeout')
  static const TerminalCloseReason idleTimeout = _$idleTimeout;
  @BuiltValueEnumConst(wireName: r'max_duration')
  static const TerminalCloseReason maxDuration = _$maxDuration;
  @BuiltValueEnumConst(wireName: r'detach_expired')
  static const TerminalCloseReason detachExpired = _$detachExpired;
  @BuiltValueEnumConst(wireName: r'access_revoked')
  static const TerminalCloseReason accessRevoked = _$accessRevoked;
  @BuiltValueEnumConst(wireName: r'i6_violation')
  static const TerminalCloseReason i6Violation = _$i6Violation;
  @BuiltValueEnumConst(wireName: r'host_disabled')
  static const TerminalCloseReason hostDisabled = _$hostDisabled;
  @BuiltValueEnumConst(wireName: r'host_retired')
  static const TerminalCloseReason hostRetired = _$hostRetired;
  @BuiltValueEnumConst(wireName: r'host_rekeyed')
  static const TerminalCloseReason hostRekeyed = _$hostRekeyed;
  @BuiltValueEnumConst(wireName: r'host_restarted')
  static const TerminalCloseReason hostRestarted = _$hostRestarted;
  @BuiltValueEnumConst(wireName: r'user_disabled')
  static const TerminalCloseReason userDisabled = _$userDisabled;
  @BuiltValueEnumConst(wireName: r'device_revoked')
  static const TerminalCloseReason deviceRevoked = _$deviceRevoked;
  @BuiltValueEnumConst(wireName: r'session_revoked')
  static const TerminalCloseReason sessionRevoked = _$sessionRevoked;
  @BuiltValueEnumConst(wireName: r'logout')
  static const TerminalCloseReason logout = _$logout;
  @BuiltValueEnumConst(wireName: r'open_failed')
  static const TerminalCloseReason openFailed = _$openFailed;
  @BuiltValueEnumConst(wireName: r'host_policy_violation')
  static const TerminalCloseReason hostPolicyViolation = _$hostPolicyViolation;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TerminalCloseReason unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<TerminalCloseReason> get serializer =>
      _$terminalCloseReasonSerializer;

  const TerminalCloseReason._(String name) : super(name);

  static BuiltSet<TerminalCloseReason> get values => _$values;
  static TerminalCloseReason valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TerminalCloseReasonMixin = Object
    with _$TerminalCloseReasonMixin;

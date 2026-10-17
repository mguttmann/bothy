//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_close_detail.g.dart';

class TerminalCloseDetail extends EnumClass {
  @BuiltValueEnumConst(wireName: r'token_invalid')
  static const TerminalCloseDetail tokenInvalid = _$tokenInvalid;
  @BuiltValueEnumConst(wireName: r'token_expired')
  static const TerminalCloseDetail tokenExpired = _$tokenExpired;
  @BuiltValueEnumConst(wireName: r'token_replayed')
  static const TerminalCloseDetail tokenReplayed = _$tokenReplayed;
  @BuiltValueEnumConst(wireName: r'handshake_failed')
  static const TerminalCloseDetail handshakeFailed = _$handshakeFailed;
  @BuiltValueEnumConst(wireName: r'profile_unknown')
  static const TerminalCloseDetail profileUnknown = _$profileUnknown;
  @BuiltValueEnumConst(wireName: r'session_user_unknown')
  static const TerminalCloseDetail sessionUserUnknown = _$sessionUserUnknown;
  @BuiltValueEnumConst(wireName: r'session_user_mismatch')
  static const TerminalCloseDetail sessionUserMismatch = _$sessionUserMismatch;
  @BuiltValueEnumConst(wireName: r'server_host_owner_only')
  static const TerminalCloseDetail serverHostOwnerOnly = _$serverHostOwnerOnly;
  @BuiltValueEnumConst(wireName: r'local_confirm_denied')
  static const TerminalCloseDetail localConfirmDenied = _$localConfirmDenied;
  @BuiltValueEnumConst(wireName: r'subscription_login_detected')
  static const TerminalCloseDetail subscriptionLoginDetected =
      _$subscriptionLoginDetected;
  @BuiltValueEnumConst(wireName: r'helper_untrusted')
  static const TerminalCloseDetail helperUntrusted = _$helperUntrusted;
  @BuiltValueEnumConst(wireName: r'shared_location')
  static const TerminalCloseDetail sharedLocation = _$sharedLocation;
  @BuiltValueEnumConst(wireName: r'pty_failed')
  static const TerminalCloseDetail ptyFailed = _$ptyFailed;
  @BuiltValueEnumConst(wireName: r'open_timeout')
  static const TerminalCloseDetail openTimeout = _$openTimeout;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TerminalCloseDetail unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<TerminalCloseDetail> get serializer =>
      _$terminalCloseDetailSerializer;

  const TerminalCloseDetail._(String name) : super(name);

  static BuiltSet<TerminalCloseDetail> get values => _$values;
  static TerminalCloseDetail valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TerminalCloseDetailMixin = Object
    with _$TerminalCloseDetailMixin;

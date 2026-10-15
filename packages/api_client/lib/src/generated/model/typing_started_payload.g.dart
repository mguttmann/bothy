// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'typing_started_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TypingStartedPayload extends TypingStartedPayload {
  @override
  final String threadId;
  @override
  final String principalId;
  @override
  final DateTime expiresAt;

  factory _$TypingStartedPayload([
    void Function(TypingStartedPayloadBuilder)? updates,
  ]) => (TypingStartedPayloadBuilder()..update(updates))._build();

  _$TypingStartedPayload._({
    required this.threadId,
    required this.principalId,
    required this.expiresAt,
  }) : super._();
  @override
  TypingStartedPayload rebuild(
    void Function(TypingStartedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TypingStartedPayloadBuilder toBuilder() =>
      TypingStartedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TypingStartedPayload &&
        threadId == other.threadId &&
        principalId == other.principalId &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, principalId.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TypingStartedPayload')
          ..add('threadId', threadId)
          ..add('principalId', principalId)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class TypingStartedPayloadBuilder
    implements Builder<TypingStartedPayload, TypingStartedPayloadBuilder> {
  _$TypingStartedPayload? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _principalId;
  String? get principalId => _$this._principalId;
  set principalId(String? principalId) => _$this._principalId = principalId;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  TypingStartedPayloadBuilder() {
    TypingStartedPayload._defaults(this);
  }

  TypingStartedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _principalId = $v.principalId;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TypingStartedPayload other) {
    _$v = other as _$TypingStartedPayload;
  }

  @override
  void update(void Function(TypingStartedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TypingStartedPayload build() => _build();

  _$TypingStartedPayload _build() {
    final _$result =
        _$v ??
        _$TypingStartedPayload._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'TypingStartedPayload',
            'threadId',
          ),
          principalId: BuiltValueNullFieldError.checkNotNull(
            principalId,
            r'TypingStartedPayload',
            'principalId',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'TypingStartedPayload',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

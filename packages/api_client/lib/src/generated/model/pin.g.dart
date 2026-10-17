// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pin.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Pin extends Pin {
  @override
  final String messageId;
  @override
  final String threadId;
  @override
  final String pinnedBy;
  @override
  final DateTime pinnedAt;

  factory _$Pin([void Function(PinBuilder)? updates]) =>
      (PinBuilder()..update(updates))._build();

  _$Pin._({
    required this.messageId,
    required this.threadId,
    required this.pinnedBy,
    required this.pinnedAt,
  }) : super._();
  @override
  Pin rebuild(void Function(PinBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PinBuilder toBuilder() => PinBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Pin &&
        messageId == other.messageId &&
        threadId == other.threadId &&
        pinnedBy == other.pinnedBy &&
        pinnedAt == other.pinnedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, pinnedBy.hashCode);
    _$hash = $jc(_$hash, pinnedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Pin')
          ..add('messageId', messageId)
          ..add('threadId', threadId)
          ..add('pinnedBy', pinnedBy)
          ..add('pinnedAt', pinnedAt))
        .toString();
  }
}

class PinBuilder implements Builder<Pin, PinBuilder> {
  _$Pin? _$v;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _pinnedBy;
  String? get pinnedBy => _$this._pinnedBy;
  set pinnedBy(String? pinnedBy) => _$this._pinnedBy = pinnedBy;

  DateTime? _pinnedAt;
  DateTime? get pinnedAt => _$this._pinnedAt;
  set pinnedAt(DateTime? pinnedAt) => _$this._pinnedAt = pinnedAt;

  PinBuilder() {
    Pin._defaults(this);
  }

  PinBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageId = $v.messageId;
      _threadId = $v.threadId;
      _pinnedBy = $v.pinnedBy;
      _pinnedAt = $v.pinnedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Pin other) {
    _$v = other as _$Pin;
  }

  @override
  void update(void Function(PinBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Pin build() => _build();

  _$Pin _build() {
    final _$result =
        _$v ??
        _$Pin._(
          messageId: BuiltValueNullFieldError.checkNotNull(
            messageId,
            r'Pin',
            'messageId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'Pin',
            'threadId',
          ),
          pinnedBy: BuiltValueNullFieldError.checkNotNull(
            pinnedBy,
            r'Pin',
            'pinnedBy',
          ),
          pinnedAt: BuiltValueNullFieldError.checkNotNull(
            pinnedAt,
            r'Pin',
            'pinnedAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

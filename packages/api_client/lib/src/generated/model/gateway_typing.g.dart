// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_typing.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayTyping extends GatewayTyping {
  @override
  final String threadId;

  factory _$GatewayTyping([void Function(GatewayTypingBuilder)? updates]) =>
      (GatewayTypingBuilder()..update(updates))._build();

  _$GatewayTyping._({required this.threadId}) : super._();
  @override
  GatewayTyping rebuild(void Function(GatewayTypingBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayTypingBuilder toBuilder() => GatewayTypingBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayTyping && threadId == other.threadId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GatewayTyping',
    )..add('threadId', threadId)).toString();
  }
}

class GatewayTypingBuilder
    implements Builder<GatewayTyping, GatewayTypingBuilder> {
  _$GatewayTyping? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  GatewayTypingBuilder() {
    GatewayTyping._defaults(this);
  }

  GatewayTypingBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayTyping other) {
    _$v = other as _$GatewayTyping;
  }

  @override
  void update(void Function(GatewayTypingBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayTyping build() => _build();

  _$GatewayTyping _build() {
    final _$result =
        _$v ??
        _$GatewayTyping._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'GatewayTyping',
            'threadId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

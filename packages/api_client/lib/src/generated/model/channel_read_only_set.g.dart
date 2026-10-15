// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_read_only_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelReadOnlySet extends ChannelReadOnlySet {
  @override
  final bool readOnly;
  @override
  final String reason;

  factory _$ChannelReadOnlySet([
    void Function(ChannelReadOnlySetBuilder)? updates,
  ]) => (ChannelReadOnlySetBuilder()..update(updates))._build();

  _$ChannelReadOnlySet._({required this.readOnly, required this.reason})
    : super._();
  @override
  ChannelReadOnlySet rebuild(
    void Function(ChannelReadOnlySetBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ChannelReadOnlySetBuilder toBuilder() =>
      ChannelReadOnlySetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelReadOnlySet &&
        readOnly == other.readOnly &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, readOnly.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelReadOnlySet')
          ..add('readOnly', readOnly)
          ..add('reason', reason))
        .toString();
  }
}

class ChannelReadOnlySetBuilder
    implements Builder<ChannelReadOnlySet, ChannelReadOnlySetBuilder> {
  _$ChannelReadOnlySet? _$v;

  bool? _readOnly;
  bool? get readOnly => _$this._readOnly;
  set readOnly(bool? readOnly) => _$this._readOnly = readOnly;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ChannelReadOnlySetBuilder() {
    ChannelReadOnlySet._defaults(this);
  }

  ChannelReadOnlySetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _readOnly = $v.readOnly;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelReadOnlySet other) {
    _$v = other as _$ChannelReadOnlySet;
  }

  @override
  void update(void Function(ChannelReadOnlySetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelReadOnlySet build() => _build();

  _$ChannelReadOnlySet _build() {
    final _$result =
        _$v ??
        _$ChannelReadOnlySet._(
          readOnly: BuiltValueNullFieldError.checkNotNull(
            readOnly,
            r'ChannelReadOnlySet',
            'readOnly',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'ChannelReadOnlySet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

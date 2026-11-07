// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_autonomous_runs_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelAutonomousRunsSet extends ChannelAutonomousRunsSet {
  @override
  final bool allowAutonomousRuns;

  factory _$ChannelAutonomousRunsSet([
    void Function(ChannelAutonomousRunsSetBuilder)? updates,
  ]) => (ChannelAutonomousRunsSetBuilder()..update(updates))._build();

  _$ChannelAutonomousRunsSet._({required this.allowAutonomousRuns}) : super._();
  @override
  ChannelAutonomousRunsSet rebuild(
    void Function(ChannelAutonomousRunsSetBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ChannelAutonomousRunsSetBuilder toBuilder() =>
      ChannelAutonomousRunsSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelAutonomousRunsSet &&
        allowAutonomousRuns == other.allowAutonomousRuns;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, allowAutonomousRuns.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ChannelAutonomousRunsSet',
    )..add('allowAutonomousRuns', allowAutonomousRuns)).toString();
  }
}

class ChannelAutonomousRunsSetBuilder
    implements
        Builder<ChannelAutonomousRunsSet, ChannelAutonomousRunsSetBuilder> {
  _$ChannelAutonomousRunsSet? _$v;

  bool? _allowAutonomousRuns;
  bool? get allowAutonomousRuns => _$this._allowAutonomousRuns;
  set allowAutonomousRuns(bool? allowAutonomousRuns) =>
      _$this._allowAutonomousRuns = allowAutonomousRuns;

  ChannelAutonomousRunsSetBuilder() {
    ChannelAutonomousRunsSet._defaults(this);
  }

  ChannelAutonomousRunsSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _allowAutonomousRuns = $v.allowAutonomousRuns;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelAutonomousRunsSet other) {
    _$v = other as _$ChannelAutonomousRunsSet;
  }

  @override
  void update(void Function(ChannelAutonomousRunsSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelAutonomousRunsSet build() => _build();

  _$ChannelAutonomousRunsSet _build() {
    final _$result =
        _$v ??
        _$ChannelAutonomousRunsSet._(
          allowAutonomousRuns: BuiltValueNullFieldError.checkNotNull(
            allowAutonomousRuns,
            r'ChannelAutonomousRunsSet',
            'allowAutonomousRuns',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

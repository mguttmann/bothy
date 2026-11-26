// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_disable.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostComputerDisable extends HostComputerDisable {
  @override
  final int generation;

  factory _$HostComputerDisable([
    void Function(HostComputerDisableBuilder)? updates,
  ]) => (HostComputerDisableBuilder()..update(updates))._build();

  _$HostComputerDisable._({required this.generation}) : super._();
  @override
  HostComputerDisable rebuild(
    void Function(HostComputerDisableBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostComputerDisableBuilder toBuilder() =>
      HostComputerDisableBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerDisable && generation == other.generation;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, generation.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'HostComputerDisable',
    )..add('generation', generation)).toString();
  }
}

class HostComputerDisableBuilder
    implements Builder<HostComputerDisable, HostComputerDisableBuilder> {
  _$HostComputerDisable? _$v;

  int? _generation;
  int? get generation => _$this._generation;
  set generation(int? generation) => _$this._generation = generation;

  HostComputerDisableBuilder() {
    HostComputerDisable._defaults(this);
  }

  HostComputerDisableBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _generation = $v.generation;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerDisable other) {
    _$v = other as _$HostComputerDisable;
  }

  @override
  void update(void Function(HostComputerDisableBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerDisable build() => _build();

  _$HostComputerDisable _build() {
    final _$result =
        _$v ??
        _$HostComputerDisable._(
          generation: BuiltValueNullFieldError.checkNotNull(
            generation,
            r'HostComputerDisable',
            'generation',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

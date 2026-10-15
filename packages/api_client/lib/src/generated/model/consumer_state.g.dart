// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'consumer_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ConsumerState extends ConsumerState {
  @override
  final String consumer;
  @override
  final int ackedSeq;
  @override
  final int headSeq;

  factory _$ConsumerState([void Function(ConsumerStateBuilder)? updates]) =>
      (ConsumerStateBuilder()..update(updates))._build();

  _$ConsumerState._({
    required this.consumer,
    required this.ackedSeq,
    required this.headSeq,
  }) : super._();
  @override
  ConsumerState rebuild(void Function(ConsumerStateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ConsumerStateBuilder toBuilder() => ConsumerStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ConsumerState &&
        consumer == other.consumer &&
        ackedSeq == other.ackedSeq &&
        headSeq == other.headSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, consumer.hashCode);
    _$hash = $jc(_$hash, ackedSeq.hashCode);
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ConsumerState')
          ..add('consumer', consumer)
          ..add('ackedSeq', ackedSeq)
          ..add('headSeq', headSeq))
        .toString();
  }
}

class ConsumerStateBuilder
    implements Builder<ConsumerState, ConsumerStateBuilder> {
  _$ConsumerState? _$v;

  String? _consumer;
  String? get consumer => _$this._consumer;
  set consumer(String? consumer) => _$this._consumer = consumer;

  int? _ackedSeq;
  int? get ackedSeq => _$this._ackedSeq;
  set ackedSeq(int? ackedSeq) => _$this._ackedSeq = ackedSeq;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  ConsumerStateBuilder() {
    ConsumerState._defaults(this);
  }

  ConsumerStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _consumer = $v.consumer;
      _ackedSeq = $v.ackedSeq;
      _headSeq = $v.headSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ConsumerState other) {
    _$v = other as _$ConsumerState;
  }

  @override
  void update(void Function(ConsumerStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ConsumerState build() => _build();

  _$ConsumerState _build() {
    final _$result =
        _$v ??
        _$ConsumerState._(
          consumer: BuiltValueNullFieldError.checkNotNull(
            consumer,
            r'ConsumerState',
            'consumer',
          ),
          ackedSeq: BuiltValueNullFieldError.checkNotNull(
            ackedSeq,
            r'ConsumerState',
            'ackedSeq',
          ),
          headSeq: BuiltValueNullFieldError.checkNotNull(
            headSeq,
            r'ConsumerState',
            'headSeq',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

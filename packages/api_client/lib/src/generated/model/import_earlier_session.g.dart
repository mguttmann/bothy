// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_earlier_session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportEarlierSession extends ImportEarlierSession {
  @override
  final String agentId;
  @override
  final String externalSessionRef;
  @override
  final int? costBaselineMicros;
  @override
  final String? currency;

  factory _$ImportEarlierSession([
    void Function(ImportEarlierSessionBuilder)? updates,
  ]) => (ImportEarlierSessionBuilder()..update(updates))._build();

  _$ImportEarlierSession._({
    required this.agentId,
    required this.externalSessionRef,
    this.costBaselineMicros,
    this.currency,
  }) : super._();
  @override
  ImportEarlierSession rebuild(
    void Function(ImportEarlierSessionBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportEarlierSessionBuilder toBuilder() =>
      ImportEarlierSessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportEarlierSession &&
        agentId == other.agentId &&
        externalSessionRef == other.externalSessionRef &&
        costBaselineMicros == other.costBaselineMicros &&
        currency == other.currency;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jc(_$hash, costBaselineMicros.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportEarlierSession')
          ..add('agentId', agentId)
          ..add('externalSessionRef', externalSessionRef)
          ..add('costBaselineMicros', costBaselineMicros)
          ..add('currency', currency))
        .toString();
  }
}

class ImportEarlierSessionBuilder
    implements Builder<ImportEarlierSession, ImportEarlierSessionBuilder> {
  _$ImportEarlierSession? _$v;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _externalSessionRef;
  String? get externalSessionRef => _$this._externalSessionRef;
  set externalSessionRef(String? externalSessionRef) =>
      _$this._externalSessionRef = externalSessionRef;

  int? _costBaselineMicros;
  int? get costBaselineMicros => _$this._costBaselineMicros;
  set costBaselineMicros(int? costBaselineMicros) =>
      _$this._costBaselineMicros = costBaselineMicros;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  ImportEarlierSessionBuilder() {
    ImportEarlierSession._defaults(this);
  }

  ImportEarlierSessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _agentId = $v.agentId;
      _externalSessionRef = $v.externalSessionRef;
      _costBaselineMicros = $v.costBaselineMicros;
      _currency = $v.currency;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportEarlierSession other) {
    _$v = other as _$ImportEarlierSession;
  }

  @override
  void update(void Function(ImportEarlierSessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportEarlierSession build() => _build();

  _$ImportEarlierSession _build() {
    final _$result =
        _$v ??
        _$ImportEarlierSession._(
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'ImportEarlierSession',
            'agentId',
          ),
          externalSessionRef: BuiltValueNullFieldError.checkNotNull(
            externalSessionRef,
            r'ImportEarlierSession',
            'externalSessionRef',
          ),
          costBaselineMicros: costBaselineMicros,
          currency: currency,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

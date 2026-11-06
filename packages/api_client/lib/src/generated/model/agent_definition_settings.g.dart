// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_settings.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinitionSettings extends AgentDefinitionSettings {
  @override
  final String? model;
  @override
  final ToolClass toolClassWish;
  @override
  final BuiltList<String>? allowedTools;
  @override
  final double? maxCostUsdPerRun;

  factory _$AgentDefinitionSettings([
    void Function(AgentDefinitionSettingsBuilder)? updates,
  ]) => (AgentDefinitionSettingsBuilder()..update(updates))._build();

  _$AgentDefinitionSettings._({
    this.model,
    required this.toolClassWish,
    this.allowedTools,
    this.maxCostUsdPerRun,
  }) : super._();
  @override
  AgentDefinitionSettings rebuild(
    void Function(AgentDefinitionSettingsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionSettingsBuilder toBuilder() =>
      AgentDefinitionSettingsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionSettings &&
        model == other.model &&
        toolClassWish == other.toolClassWish &&
        allowedTools == other.allowedTools &&
        maxCostUsdPerRun == other.maxCostUsdPerRun;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, toolClassWish.hashCode);
    _$hash = $jc(_$hash, allowedTools.hashCode);
    _$hash = $jc(_$hash, maxCostUsdPerRun.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionSettings')
          ..add('model', model)
          ..add('toolClassWish', toolClassWish)
          ..add('allowedTools', allowedTools)
          ..add('maxCostUsdPerRun', maxCostUsdPerRun))
        .toString();
  }
}

class AgentDefinitionSettingsBuilder
    implements
        Builder<AgentDefinitionSettings, AgentDefinitionSettingsBuilder> {
  _$AgentDefinitionSettings? _$v;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  ToolClass? _toolClassWish;
  ToolClass? get toolClassWish => _$this._toolClassWish;
  set toolClassWish(ToolClass? toolClassWish) =>
      _$this._toolClassWish = toolClassWish;

  ListBuilder<String>? _allowedTools;
  ListBuilder<String> get allowedTools =>
      _$this._allowedTools ??= ListBuilder<String>();
  set allowedTools(ListBuilder<String>? allowedTools) =>
      _$this._allowedTools = allowedTools;

  double? _maxCostUsdPerRun;
  double? get maxCostUsdPerRun => _$this._maxCostUsdPerRun;
  set maxCostUsdPerRun(double? maxCostUsdPerRun) =>
      _$this._maxCostUsdPerRun = maxCostUsdPerRun;

  AgentDefinitionSettingsBuilder() {
    AgentDefinitionSettings._defaults(this);
  }

  AgentDefinitionSettingsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _model = $v.model;
      _toolClassWish = $v.toolClassWish;
      _allowedTools = $v.allowedTools?.toBuilder();
      _maxCostUsdPerRun = $v.maxCostUsdPerRun;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionSettings other) {
    _$v = other as _$AgentDefinitionSettings;
  }

  @override
  void update(void Function(AgentDefinitionSettingsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionSettings build() => _build();

  _$AgentDefinitionSettings _build() {
    _$AgentDefinitionSettings _$result;
    try {
      _$result =
          _$v ??
          _$AgentDefinitionSettings._(
            model: model,
            toolClassWish: BuiltValueNullFieldError.checkNotNull(
              toolClassWish,
              r'AgentDefinitionSettings',
              'toolClassWish',
            ),
            allowedTools: _allowedTools?.build(),
            maxCostUsdPerRun: maxCostUsdPerRun,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'allowedTools';
        _allowedTools?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDefinitionSettings',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

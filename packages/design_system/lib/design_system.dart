// SPDX-License-Identifier: Apache-2.0

/// Design tokens (generated), theme, components, icons and fonts of the apps.
///
/// Colors, type, spacing and motion come only from the generated token classes
/// (ADR 0030): `DsColors`, `DsAccent`, `DsElevation`, `DsDensity` and `DsMotion` as theme
/// extensions, `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`,
/// `DsFonts` and `DsType` as constants. [buildDsTheme] turns a [DsContext] into a
/// `ThemeData`, [resolveDsContext] derives the context from the personal settings, and
/// [DsThemeScope] applies it below a widget. [DsIcon] draws a semantic icon of [DsIcons] from
/// the compiled Phosphor assets ([DsIconAssets]).
///
/// The core components (docs/design/components.md) take the view models of
/// `view_models.dart` and read every text from `package:l10n`: [DsMessage], [DsMarkdown],
/// [DsLiveBox], [DsToolChip], [DsStatusLine], [DsTriggerStateIcon], [DsThreadRow],
/// [DsSpaceRow], [DsChannelRow], [DsMessageComponents], [DsComposer], [DsSuggestionList],
/// [DsBillingBadge], [DsCostText], [DsBanner] and the [LiveAnnouncer].
library;

export 'src/components/ds_banner.dart';
export 'src/components/ds_billing_badge.dart';
export 'src/components/ds_channel_row.dart';
export 'src/components/ds_common.dart';
export 'src/components/ds_composer.dart';
export 'src/components/ds_live_box.dart';
export 'src/components/ds_message.dart';
export 'src/components/ds_message_components.dart';
export 'src/components/ds_space_row.dart';
export 'src/components/ds_status_line.dart';
export 'src/components/ds_suggestion_list.dart';
export 'src/components/ds_thread_row.dart';
export 'src/components/ds_tool_chip.dart';
export 'src/components/ds_trigger_state.dart';
export 'src/components/live_announcer.dart';
export 'src/components/markdown/ds_markdown.dart';
export 'src/components/view_models.dart';
export 'src/generated/design_tokens.g.dart';
export 'src/generated/ds_icon_assets.g.dart';
export 'src/icons/ds_icon.dart';
export 'src/theme/ds_context.dart';
export 'src/theme/ds_settings_resolver.dart';
export 'src/theme/ds_theme.dart';
export 'src/theme/ds_theme_scope.dart';

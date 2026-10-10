// GENERIERT von tools/design-tokens/tokens.py aus design/tokens. Nicht bearbeiten; Quelle ändern und neu erzeugen.
// ignore_for_file: public_member_api_docs, constant_identifier_names

import 'dart:ui' show lerpDouble;

import 'package:flutter/material.dart';

/// Primitive Tonleitern. Komponenten nutzen DsColors, nie diese Werte direkt.
abstract final class DsPalette {
  static const Color amber100 = Color(0xFFFEE6D4);
  static const Color amber200 = Color(0xFFFDCFAA);
  static const Color amber300 = Color(0xFFFCAF6F);
  static const Color amber400 = Color(0xFFEC9340);
  static const Color amber50 = Color(0xFFFEF3EA);
  static const Color amber500 = Color(0xFFD1791D);
  static const Color amber600 = Color(0xFFAE6416);
  static const Color amber700 = Color(0xFF8C4F10);
  static const Color amber800 = Color(0xFF6B3B09);
  static const Color amber900 = Color(0xFF4C2905);
  static const Color amber950 = Color(0xFF331902);
  static const Color gorse100 = Color(0xFFFBEF8E);
  static const Color gorse200 = Color(0xFFEDDC59);
  static const Color gorse300 = Color(0xFFD9C42B);
  static const Color gorse400 = Color(0xFFC2AB24);
  static const Color gorse50 = Color(0xFFFEF5AA);
  static const Color gorse500 = Color(0xFFA8921D);
  static const Color gorse600 = Color(0xFF8D7716);
  static const Color gorse700 = Color(0xFF735E10);
  static const Color gorse800 = Color(0xFF59470A);
  static const Color gorse900 = Color(0xFF3F3105);
  static const Color gorse950 = Color(0xFF291F02);
  static const Color granite0 = Color(0xFFFBFEFF);
  static const Color granite100 = Color(0xFFE5EEF0);
  static const Color granite150 = Color(0xFFDAE5E7);
  static const Color granite200 = Color(0xFFCCD9DB);
  static const Color granite25 = Color(0xFFF5FBFC);
  static const Color granite300 = Color(0xFFAFBEC0);
  static const Color granite400 = Color(0xFF8F9FA1);
  static const Color granite50 = Color(0xFFEFF6F7);
  static const Color granite500 = Color(0xFF748486);
  static const Color granite600 = Color(0xFF59696C);
  static const Color granite700 = Color(0xFF3F5154);
  static const Color granite750 = Color(0xFF304245);
  static const Color granite800 = Color(0xFF24363A);
  static const Color granite850 = Color(0xFF1B2E31);
  static const Color granite875 = Color(0xFF15282B);
  static const Color granite900 = Color(0xFF102326);
  static const Color granite925 = Color(0xFF0C1E21);
  static const Color granite950 = Color(0xFF08191C);
  static const Color granite975 = Color(0xFF061416);
  static const Color heather100 = Color(0xFFFEE3EE);
  static const Color heather200 = Color(0xFFFDC8DF);
  static const Color heather300 = Color(0xFFFCA3CD);
  static const Color heather400 = Color(0xFFF878BA);
  static const Color heather50 = Color(0xFFFEF1F7);
  static const Color heather500 = Color(0xFFE158A2);
  static const Color heather600 = Color(0xFFC53A88);
  static const Color heather700 = Color(0xFFA51F6F);
  static const Color heather800 = Color(0xFF810E55);
  static const Color heather900 = Color(0xFF5D073C);
  static const Color heather950 = Color(0xFF3E0327);
  static const Color loch100 = Color(0xFFDCEEFE);
  static const Color loch200 = Color(0xFFBBDEFD);
  static const Color loch300 = Color(0xFF8EC9FB);
  static const Color loch400 = Color(0xFF58B3FA);
  static const Color loch50 = Color(0xFFEEF6FE);
  static const Color loch500 = Color(0xFF2F99E5);
  static const Color loch600 = Color(0xFF197FC3);
  static const Color loch700 = Color(0xFF12659D);
  static const Color loch800 = Color(0xFF0B4D79);
  static const Color loch900 = Color(0xFF053656);
  static const Color loch950 = Color(0xFF02223A);
  static const Color moss100 = Color(0xFFCFF8D6);
  static const Color moss200 = Color(0xFFAFEABA);
  static const Color moss300 = Color(0xFF89D89A);
  static const Color moss400 = Color(0xFF62C37A);
  static const Color moss50 = Color(0xFFE4FDE8);
  static const Color moss500 = Color(0xFF3CAB5E);
  static const Color moss600 = Color(0xFF1B9046);
  static const Color moss700 = Color(0xFF137337);
  static const Color moss800 = Color(0xFF0C5828);
  static const Color moss900 = Color(0xFF063E1A);
  static const Color moss950 = Color(0xFF03280F);
  static const Color rowan100 = Color(0xFFFEE4E2);
  static const Color rowan200 = Color(0xFFFDCBC6);
  static const Color rowan300 = Color(0xFFFCAAA3);
  static const Color rowan400 = Color(0xFFFB827A);
  static const Color rowan50 = Color(0xFFFEF2F0);
  static const Color rowan500 = Color(0xFFF15653);
  static const Color rowan600 = Color(0xFFD43538);
  static const Color rowan700 = Color(0xFFB21622);
  static const Color rowan800 = Color(0xFF890F18);
  static const Color rowan900 = Color(0xFF63080E);
  static const Color rowan950 = Color(0xFF430307);
  static const Color spruce100 = Color(0xFFCCF5F0);
  static const Color spruce200 = Color(0xFFA8E8E1);
  static const Color spruce300 = Color(0xFF7CD5CD);
  static const Color spruce400 = Color(0xFF49C0B7);
  static const Color spruce50 = Color(0xFFE4FBF8);
  static const Color spruce500 = Color(0xFF22A79E);
  static const Color spruce600 = Color(0xFF1B8A83);
  static const Color spruce700 = Color(0xFF136F69);
  static const Color spruce800 = Color(0xFF0C5450);
  static const Color spruce850 = Color(0xFF0F4643);
  static const Color spruce900 = Color(0xFF063B38);
  static const Color spruce925 = Color(0xFF08312E);
  static const Color spruce950 = Color(0xFF032624);
  static const Color spruce975 = Color(0xFF041C1A);
}

/// Semantische Farbrollen je Theme.
@immutable
final class DsColors extends ThemeExtension<DsColors> {
  const DsColors({
    required this.accentBg,
    required this.accentBgHover,
    required this.accentBgPressed,
    required this.accentFg,
    required this.accentSubtle,
    required this.attentionBg,
    required this.attentionBorder,
    required this.attentionFg,
    required this.attentionOnBg,
    required this.attentionSubtle,
    required this.badgeOrganizationBg,
    required this.badgeOrganizationFg,
    required this.badgePersonalBg,
    required this.badgePersonalFg,
    required this.borderControl,
    required this.borderDefault,
    required this.borderOnSidebar,
    required this.borderSubtle,
    required this.categoryAmberChipBg,
    required this.categoryAmberChipFg,
    required this.categoryAmberFg,
    required this.categoryAmberOnSidebar,
    required this.categoryGorseChipBg,
    required this.categoryGorseChipFg,
    required this.categoryGorseFg,
    required this.categoryGorseOnSidebar,
    required this.categoryGraniteChipBg,
    required this.categoryGraniteChipFg,
    required this.categoryGraniteFg,
    required this.categoryGraniteOnSidebar,
    required this.categoryHeatherChipBg,
    required this.categoryHeatherChipFg,
    required this.categoryHeatherFg,
    required this.categoryHeatherOnSidebar,
    required this.categoryLochChipBg,
    required this.categoryLochChipFg,
    required this.categoryLochFg,
    required this.categoryLochOnSidebar,
    required this.categoryMossChipBg,
    required this.categoryMossChipFg,
    required this.categoryMossFg,
    required this.categoryMossOnSidebar,
    required this.categoryRowanChipBg,
    required this.categoryRowanChipFg,
    required this.categoryRowanFg,
    required this.categoryRowanOnSidebar,
    required this.categorySpruceChipBg,
    required this.categorySpruceChipFg,
    required this.categorySpruceFg,
    required this.categorySpruceOnSidebar,
    required this.codeBg,
    required this.codeFg,
    required this.dangerBg,
    required this.dangerBorder,
    required this.dangerFg,
    required this.dangerOnBg,
    required this.dangerSubtle,
    required this.focusRing,
    required this.focusRingOnSidebar,
    required this.infoBg,
    required this.infoBorder,
    required this.infoFg,
    required this.infoOnBg,
    required this.infoSubtle,
    required this.runCancelled,
    required this.runCompleted,
    required this.runFailed,
    required this.runQueued,
    required this.runRunning,
    required this.runWaiting,
    required this.scrim,
    required this.shadow,
    required this.successBg,
    required this.successBorder,
    required this.successFg,
    required this.successOnBg,
    required this.successSubtle,
    required this.surfaceCanvas,
    required this.surfaceHover,
    required this.surfaceInput,
    required this.surfaceList,
    required this.surfaceOverlay,
    required this.surfaceRaised,
    required this.surfaceSelected,
    required this.surfaceSidebar,
    required this.surfaceSidebarHover,
    required this.surfaceSidebarSelected,
    required this.surfaceSunken,
    required this.textLink,
    required this.textMuted,
    required this.textOnAccent,
    required this.textOnSidebar,
    required this.textOnSidebarMuted,
    required this.textOnSidebarSecondary,
    required this.textPrimary,
    required this.textSecondary,
    required this.triggerDone,
    required this.triggerFailed,
    required this.triggerFollowup,
    required this.triggerQueued,
    required this.triggerReceived,
    required this.triggerWorking,
    required this.warningBg,
    required this.warningBorder,
    required this.warningFg,
    required this.warningOnBg,
    required this.warningSubtle,
  });

  final Color accentBg;
  final Color accentBgHover;
  final Color accentBgPressed;
  final Color accentFg;
  final Color accentSubtle;
  final Color attentionBg;
  final Color attentionBorder;
  final Color attentionFg;
  final Color attentionOnBg;
  final Color attentionSubtle;
  final Color badgeOrganizationBg;
  final Color badgeOrganizationFg;
  final Color badgePersonalBg;
  final Color badgePersonalFg;
  final Color borderControl;
  final Color borderDefault;
  final Color borderOnSidebar;
  final Color borderSubtle;
  final Color categoryAmberChipBg;
  final Color categoryAmberChipFg;
  final Color categoryAmberFg;
  final Color categoryAmberOnSidebar;
  final Color categoryGorseChipBg;
  final Color categoryGorseChipFg;
  final Color categoryGorseFg;
  final Color categoryGorseOnSidebar;
  final Color categoryGraniteChipBg;
  final Color categoryGraniteChipFg;
  final Color categoryGraniteFg;
  final Color categoryGraniteOnSidebar;
  final Color categoryHeatherChipBg;
  final Color categoryHeatherChipFg;
  final Color categoryHeatherFg;
  final Color categoryHeatherOnSidebar;
  final Color categoryLochChipBg;
  final Color categoryLochChipFg;
  final Color categoryLochFg;
  final Color categoryLochOnSidebar;
  final Color categoryMossChipBg;
  final Color categoryMossChipFg;
  final Color categoryMossFg;
  final Color categoryMossOnSidebar;
  final Color categoryRowanChipBg;
  final Color categoryRowanChipFg;
  final Color categoryRowanFg;
  final Color categoryRowanOnSidebar;
  final Color categorySpruceChipBg;
  final Color categorySpruceChipFg;
  final Color categorySpruceFg;
  final Color categorySpruceOnSidebar;
  final Color codeBg;
  final Color codeFg;
  final Color dangerBg;
  final Color dangerBorder;
  final Color dangerFg;
  final Color dangerOnBg;
  final Color dangerSubtle;
  final Color focusRing;
  final Color focusRingOnSidebar;
  final Color infoBg;
  final Color infoBorder;
  final Color infoFg;
  final Color infoOnBg;
  final Color infoSubtle;
  final Color runCancelled;
  final Color runCompleted;
  final Color runFailed;
  final Color runQueued;
  final Color runRunning;
  final Color runWaiting;
  final Color scrim;
  final Color shadow;
  final Color successBg;
  final Color successBorder;
  final Color successFg;
  final Color successOnBg;
  final Color successSubtle;
  final Color surfaceCanvas;
  final Color surfaceHover;
  final Color surfaceInput;
  final Color surfaceList;
  final Color surfaceOverlay;
  final Color surfaceRaised;
  final Color surfaceSelected;
  final Color surfaceSidebar;
  final Color surfaceSidebarHover;
  final Color surfaceSidebarSelected;
  final Color surfaceSunken;
  final Color textLink;
  final Color textMuted;
  final Color textOnAccent;
  final Color textOnSidebar;
  final Color textOnSidebarMuted;
  final Color textOnSidebarSecondary;
  final Color textPrimary;
  final Color textSecondary;
  final Color triggerDone;
  final Color triggerFailed;
  final Color triggerFollowup;
  final Color triggerQueued;
  final Color triggerReceived;
  final Color triggerWorking;
  final Color warningBg;
  final Color warningBorder;
  final Color warningFg;
  final Color warningOnBg;
  final Color warningSubtle;

  static const DsColors light = DsColors(
    accentBg: Color(0xFF136F69),
    accentBgHover: Color(0xFF0C5450),
    accentBgPressed: Color(0xFF063B38),
    accentFg: Color(0xFF136F69),
    accentSubtle: Color(0xFFE4FBF8),
    attentionBg: Color(0xFFEDDC59),
    attentionBorder: Color(0xFF8D7716),
    attentionFg: Color(0xFF735E10),
    attentionOnBg: Color(0xFF291F02),
    attentionSubtle: Color(0xFFFEF5AA),
    badgeOrganizationBg: Color(0xFFDCEEFE),
    badgeOrganizationFg: Color(0xFF0B4D79),
    badgePersonalBg: Color(0xFFFEE3EE),
    badgePersonalFg: Color(0xFF810E55),
    borderControl: Color(0xFF748486),
    borderDefault: Color(0xFFCCD9DB),
    borderOnSidebar: Color(0xFF0C5450),
    borderSubtle: Color(0xFFDAE5E7),
    categoryAmberChipBg: Color(0xFFFEE6D4),
    categoryAmberChipFg: Color(0xFF6B3B09),
    categoryAmberFg: Color(0xFF8C4F10),
    categoryAmberOnSidebar: Color(0xFFFCAF6F),
    categoryGorseChipBg: Color(0xFFFBEF8E),
    categoryGorseChipFg: Color(0xFF59470A),
    categoryGorseFg: Color(0xFF735E10),
    categoryGorseOnSidebar: Color(0xFFD9C42B),
    categoryGraniteChipBg: Color(0xFFE5EEF0),
    categoryGraniteChipFg: Color(0xFF24363A),
    categoryGraniteFg: Color(0xFF3F5154),
    categoryGraniteOnSidebar: Color(0xFFCCD9DB),
    categoryHeatherChipBg: Color(0xFFFEE3EE),
    categoryHeatherChipFg: Color(0xFF810E55),
    categoryHeatherFg: Color(0xFFA51F6F),
    categoryHeatherOnSidebar: Color(0xFFFCA3CD),
    categoryLochChipBg: Color(0xFFDCEEFE),
    categoryLochChipFg: Color(0xFF0B4D79),
    categoryLochFg: Color(0xFF12659D),
    categoryLochOnSidebar: Color(0xFF8EC9FB),
    categoryMossChipBg: Color(0xFFCFF8D6),
    categoryMossChipFg: Color(0xFF0C5828),
    categoryMossFg: Color(0xFF137337),
    categoryMossOnSidebar: Color(0xFF89D89A),
    categoryRowanChipBg: Color(0xFFFEE4E2),
    categoryRowanChipFg: Color(0xFF890F18),
    categoryRowanFg: Color(0xFFB21622),
    categoryRowanOnSidebar: Color(0xFFFCAAA3),
    categorySpruceChipBg: Color(0xFFCCF5F0),
    categorySpruceChipFg: Color(0xFF0C5450),
    categorySpruceFg: Color(0xFF136F69),
    categorySpruceOnSidebar: Color(0xFF7CD5CD),
    codeBg: Color(0xFFE5EEF0),
    codeFg: Color(0xFF102326),
    dangerBg: Color(0xFFB21622),
    dangerBorder: Color(0xFFD43538),
    dangerFg: Color(0xFFB21622),
    dangerOnBg: Color(0xFFFBFEFF),
    dangerSubtle: Color(0xFFFEF2F0),
    focusRing: Color(0xFF1B8A83),
    focusRingOnSidebar: Color(0xFFD9C42B),
    infoBg: Color(0xFF12659D),
    infoBorder: Color(0xFF197FC3),
    infoFg: Color(0xFF12659D),
    infoOnBg: Color(0xFFFBFEFF),
    infoSubtle: Color(0xFFEEF6FE),
    runCancelled: Color(0xFF59696C),
    runCompleted: Color(0xFF1B9046),
    runFailed: Color(0xFFD43538),
    runQueued: Color(0xFF748486),
    runRunning: Color(0xFF1B8A83),
    runWaiting: Color(0xFF8D7716),
    scrim: Color(0x7A061416),
    shadow: Color(0x2908191C),
    successBg: Color(0xFF137337),
    successBorder: Color(0xFF1B9046),
    successFg: Color(0xFF137337),
    successOnBg: Color(0xFFFBFEFF),
    successSubtle: Color(0xFFE4FDE8),
    surfaceCanvas: Color(0xFFFBFEFF),
    surfaceHover: Color(0xFFE5EEF0),
    surfaceInput: Color(0xFFF5FBFC),
    surfaceList: Color(0xFFEFF6F7),
    surfaceOverlay: Color(0xFFFBFEFF),
    surfaceRaised: Color(0xFFF5FBFC),
    surfaceSelected: Color(0xFFCCF5F0),
    surfaceSidebar: Color(0xFF063B38),
    surfaceSidebarHover: Color(0xFF0F4643),
    surfaceSidebarSelected: Color(0xFF0C5450),
    surfaceSunken: Color(0xFFE5EEF0),
    textLink: Color(0xFF136F69),
    textMuted: Color(0xFF59696C),
    textOnAccent: Color(0xFFFBFEFF),
    textOnSidebar: Color(0xFFE4FBF8),
    textOnSidebarMuted: Color(0xFF7CD5CD),
    textOnSidebarSecondary: Color(0xFFA8E8E1),
    textPrimary: Color(0xFF08191C),
    textSecondary: Color(0xFF3F5154),
    triggerDone: Color(0xFF1B9046),
    triggerFailed: Color(0xFFD43538),
    triggerFollowup: Color(0xFF197FC3),
    triggerQueued: Color(0xFF59696C),
    triggerReceived: Color(0xFF748486),
    triggerWorking: Color(0xFF1B8A83),
    warningBg: Color(0xFFFCAF6F),
    warningBorder: Color(0xFFAE6416),
    warningFg: Color(0xFF8C4F10),
    warningOnBg: Color(0xFF331902),
    warningSubtle: Color(0xFFFEF3EA),
  );

  static const DsColors dark = DsColors(
    accentBg: Color(0xFF49C0B7),
    accentBgHover: Color(0xFF7CD5CD),
    accentBgPressed: Color(0xFFA8E8E1),
    accentFg: Color(0xFF7CD5CD),
    accentSubtle: Color(0xFF032624),
    attentionBg: Color(0xFFD9C42B),
    attentionBorder: Color(0xFFC2AB24),
    attentionFg: Color(0xFFD9C42B),
    attentionOnBg: Color(0xFF291F02),
    attentionSubtle: Color(0xFF291F02),
    badgeOrganizationBg: Color(0xFF053656),
    badgeOrganizationFg: Color(0xFFBBDEFD),
    badgePersonalBg: Color(0xFF5D073C),
    badgePersonalFg: Color(0xFFFDC8DF),
    borderControl: Color(0xFF748486),
    borderDefault: Color(0xFF304245),
    borderOnSidebar: Color(0xFF063B38),
    borderSubtle: Color(0xFF24363A),
    categoryAmberChipBg: Color(0xFF4C2905),
    categoryAmberChipFg: Color(0xFFFDCFAA),
    categoryAmberFg: Color(0xFFFCAF6F),
    categoryAmberOnSidebar: Color(0xFFFCAF6F),
    categoryGorseChipBg: Color(0xFF3F3105),
    categoryGorseChipFg: Color(0xFFEDDC59),
    categoryGorseFg: Color(0xFFD9C42B),
    categoryGorseOnSidebar: Color(0xFFD9C42B),
    categoryGraniteChipBg: Color(0xFF102326),
    categoryGraniteChipFg: Color(0xFFCCD9DB),
    categoryGraniteFg: Color(0xFFAFBEC0),
    categoryGraniteOnSidebar: Color(0xFFCCD9DB),
    categoryHeatherChipBg: Color(0xFF5D073C),
    categoryHeatherChipFg: Color(0xFFFDC8DF),
    categoryHeatherFg: Color(0xFFFCA3CD),
    categoryHeatherOnSidebar: Color(0xFFFCA3CD),
    categoryLochChipBg: Color(0xFF053656),
    categoryLochChipFg: Color(0xFFBBDEFD),
    categoryLochFg: Color(0xFF8EC9FB),
    categoryLochOnSidebar: Color(0xFF8EC9FB),
    categoryMossChipBg: Color(0xFF063E1A),
    categoryMossChipFg: Color(0xFFAFEABA),
    categoryMossFg: Color(0xFF89D89A),
    categoryMossOnSidebar: Color(0xFF89D89A),
    categoryRowanChipBg: Color(0xFF63080E),
    categoryRowanChipFg: Color(0xFFFDCBC6),
    categoryRowanFg: Color(0xFFFCAAA3),
    categoryRowanOnSidebar: Color(0xFFFCAAA3),
    categorySpruceChipBg: Color(0xFF063B38),
    categorySpruceChipFg: Color(0xFFA8E8E1),
    categorySpruceFg: Color(0xFF7CD5CD),
    categorySpruceOnSidebar: Color(0xFF7CD5CD),
    codeBg: Color(0xFF0C1E21),
    codeFg: Color(0xFFE5EEF0),
    dangerBg: Color(0xFFFB827A),
    dangerBorder: Color(0xFFFB827A),
    dangerFg: Color(0xFFFCAAA3),
    dangerOnBg: Color(0xFF430307),
    dangerSubtle: Color(0xFF430307),
    focusRing: Color(0xFFD9C42B),
    focusRingOnSidebar: Color(0xFFD9C42B),
    infoBg: Color(0xFF58B3FA),
    infoBorder: Color(0xFF58B3FA),
    infoFg: Color(0xFF8EC9FB),
    infoOnBg: Color(0xFF02223A),
    infoSubtle: Color(0xFF02223A),
    runCancelled: Color(0xFF8F9FA1),
    runCompleted: Color(0xFF62C37A),
    runFailed: Color(0xFFFB827A),
    runQueued: Color(0xFF8F9FA1),
    runRunning: Color(0xFF49C0B7),
    runWaiting: Color(0xFFD9C42B),
    scrim: Color(0xA3061416),
    shadow: Color(0x80061416),
    successBg: Color(0xFF89D89A),
    successBorder: Color(0xFF62C37A),
    successFg: Color(0xFF89D89A),
    successOnBg: Color(0xFF03280F),
    successSubtle: Color(0xFF03280F),
    surfaceCanvas: Color(0xFF15282B),
    surfaceHover: Color(0xFF1B2E31),
    surfaceInput: Color(0xFF1B2E31),
    surfaceList: Color(0xFF102326),
    surfaceOverlay: Color(0xFF24363A),
    surfaceRaised: Color(0xFF1B2E31),
    surfaceSelected: Color(0xFF08312E),
    surfaceSidebar: Color(0xFF032624),
    surfaceSidebarHover: Color(0xFF08312E),
    surfaceSidebarSelected: Color(0xFF063B38),
    surfaceSunken: Color(0xFF0C1E21),
    textLink: Color(0xFF7CD5CD),
    textMuted: Color(0xFFAFBEC0),
    textOnAccent: Color(0xFF041C1A),
    textOnSidebar: Color(0xFFE4FBF8),
    textOnSidebarMuted: Color(0xFF7CD5CD),
    textOnSidebarSecondary: Color(0xFFA8E8E1),
    textPrimary: Color(0xFFF5FBFC),
    textSecondary: Color(0xFFCCD9DB),
    triggerDone: Color(0xFF62C37A),
    triggerFailed: Color(0xFFFB827A),
    triggerFollowup: Color(0xFF58B3FA),
    triggerQueued: Color(0xFFAFBEC0),
    triggerReceived: Color(0xFF8F9FA1),
    triggerWorking: Color(0xFF49C0B7),
    warningBg: Color(0xFFFCAF6F),
    warningBorder: Color(0xFFEC9340),
    warningFg: Color(0xFFFCAF6F),
    warningOnBg: Color(0xFF331902),
    warningSubtle: Color(0xFF331902),
  );

  @override
  DsColors copyWith({
    Color? accentBg,
    Color? accentBgHover,
    Color? accentBgPressed,
    Color? accentFg,
    Color? accentSubtle,
    Color? attentionBg,
    Color? attentionBorder,
    Color? attentionFg,
    Color? attentionOnBg,
    Color? attentionSubtle,
    Color? badgeOrganizationBg,
    Color? badgeOrganizationFg,
    Color? badgePersonalBg,
    Color? badgePersonalFg,
    Color? borderControl,
    Color? borderDefault,
    Color? borderOnSidebar,
    Color? borderSubtle,
    Color? categoryAmberChipBg,
    Color? categoryAmberChipFg,
    Color? categoryAmberFg,
    Color? categoryAmberOnSidebar,
    Color? categoryGorseChipBg,
    Color? categoryGorseChipFg,
    Color? categoryGorseFg,
    Color? categoryGorseOnSidebar,
    Color? categoryGraniteChipBg,
    Color? categoryGraniteChipFg,
    Color? categoryGraniteFg,
    Color? categoryGraniteOnSidebar,
    Color? categoryHeatherChipBg,
    Color? categoryHeatherChipFg,
    Color? categoryHeatherFg,
    Color? categoryHeatherOnSidebar,
    Color? categoryLochChipBg,
    Color? categoryLochChipFg,
    Color? categoryLochFg,
    Color? categoryLochOnSidebar,
    Color? categoryMossChipBg,
    Color? categoryMossChipFg,
    Color? categoryMossFg,
    Color? categoryMossOnSidebar,
    Color? categoryRowanChipBg,
    Color? categoryRowanChipFg,
    Color? categoryRowanFg,
    Color? categoryRowanOnSidebar,
    Color? categorySpruceChipBg,
    Color? categorySpruceChipFg,
    Color? categorySpruceFg,
    Color? categorySpruceOnSidebar,
    Color? codeBg,
    Color? codeFg,
    Color? dangerBg,
    Color? dangerBorder,
    Color? dangerFg,
    Color? dangerOnBg,
    Color? dangerSubtle,
    Color? focusRing,
    Color? focusRingOnSidebar,
    Color? infoBg,
    Color? infoBorder,
    Color? infoFg,
    Color? infoOnBg,
    Color? infoSubtle,
    Color? runCancelled,
    Color? runCompleted,
    Color? runFailed,
    Color? runQueued,
    Color? runRunning,
    Color? runWaiting,
    Color? scrim,
    Color? shadow,
    Color? successBg,
    Color? successBorder,
    Color? successFg,
    Color? successOnBg,
    Color? successSubtle,
    Color? surfaceCanvas,
    Color? surfaceHover,
    Color? surfaceInput,
    Color? surfaceList,
    Color? surfaceOverlay,
    Color? surfaceRaised,
    Color? surfaceSelected,
    Color? surfaceSidebar,
    Color? surfaceSidebarHover,
    Color? surfaceSidebarSelected,
    Color? surfaceSunken,
    Color? textLink,
    Color? textMuted,
    Color? textOnAccent,
    Color? textOnSidebar,
    Color? textOnSidebarMuted,
    Color? textOnSidebarSecondary,
    Color? textPrimary,
    Color? textSecondary,
    Color? triggerDone,
    Color? triggerFailed,
    Color? triggerFollowup,
    Color? triggerQueued,
    Color? triggerReceived,
    Color? triggerWorking,
    Color? warningBg,
    Color? warningBorder,
    Color? warningFg,
    Color? warningOnBg,
    Color? warningSubtle,
  }) {
    return DsColors(
      accentBg: accentBg ?? this.accentBg,
      accentBgHover: accentBgHover ?? this.accentBgHover,
      accentBgPressed: accentBgPressed ?? this.accentBgPressed,
      accentFg: accentFg ?? this.accentFg,
      accentSubtle: accentSubtle ?? this.accentSubtle,
      attentionBg: attentionBg ?? this.attentionBg,
      attentionBorder: attentionBorder ?? this.attentionBorder,
      attentionFg: attentionFg ?? this.attentionFg,
      attentionOnBg: attentionOnBg ?? this.attentionOnBg,
      attentionSubtle: attentionSubtle ?? this.attentionSubtle,
      badgeOrganizationBg: badgeOrganizationBg ?? this.badgeOrganizationBg,
      badgeOrganizationFg: badgeOrganizationFg ?? this.badgeOrganizationFg,
      badgePersonalBg: badgePersonalBg ?? this.badgePersonalBg,
      badgePersonalFg: badgePersonalFg ?? this.badgePersonalFg,
      borderControl: borderControl ?? this.borderControl,
      borderDefault: borderDefault ?? this.borderDefault,
      borderOnSidebar: borderOnSidebar ?? this.borderOnSidebar,
      borderSubtle: borderSubtle ?? this.borderSubtle,
      categoryAmberChipBg: categoryAmberChipBg ?? this.categoryAmberChipBg,
      categoryAmberChipFg: categoryAmberChipFg ?? this.categoryAmberChipFg,
      categoryAmberFg: categoryAmberFg ?? this.categoryAmberFg,
      categoryAmberOnSidebar: categoryAmberOnSidebar ?? this.categoryAmberOnSidebar,
      categoryGorseChipBg: categoryGorseChipBg ?? this.categoryGorseChipBg,
      categoryGorseChipFg: categoryGorseChipFg ?? this.categoryGorseChipFg,
      categoryGorseFg: categoryGorseFg ?? this.categoryGorseFg,
      categoryGorseOnSidebar: categoryGorseOnSidebar ?? this.categoryGorseOnSidebar,
      categoryGraniteChipBg: categoryGraniteChipBg ?? this.categoryGraniteChipBg,
      categoryGraniteChipFg: categoryGraniteChipFg ?? this.categoryGraniteChipFg,
      categoryGraniteFg: categoryGraniteFg ?? this.categoryGraniteFg,
      categoryGraniteOnSidebar: categoryGraniteOnSidebar ?? this.categoryGraniteOnSidebar,
      categoryHeatherChipBg: categoryHeatherChipBg ?? this.categoryHeatherChipBg,
      categoryHeatherChipFg: categoryHeatherChipFg ?? this.categoryHeatherChipFg,
      categoryHeatherFg: categoryHeatherFg ?? this.categoryHeatherFg,
      categoryHeatherOnSidebar: categoryHeatherOnSidebar ?? this.categoryHeatherOnSidebar,
      categoryLochChipBg: categoryLochChipBg ?? this.categoryLochChipBg,
      categoryLochChipFg: categoryLochChipFg ?? this.categoryLochChipFg,
      categoryLochFg: categoryLochFg ?? this.categoryLochFg,
      categoryLochOnSidebar: categoryLochOnSidebar ?? this.categoryLochOnSidebar,
      categoryMossChipBg: categoryMossChipBg ?? this.categoryMossChipBg,
      categoryMossChipFg: categoryMossChipFg ?? this.categoryMossChipFg,
      categoryMossFg: categoryMossFg ?? this.categoryMossFg,
      categoryMossOnSidebar: categoryMossOnSidebar ?? this.categoryMossOnSidebar,
      categoryRowanChipBg: categoryRowanChipBg ?? this.categoryRowanChipBg,
      categoryRowanChipFg: categoryRowanChipFg ?? this.categoryRowanChipFg,
      categoryRowanFg: categoryRowanFg ?? this.categoryRowanFg,
      categoryRowanOnSidebar: categoryRowanOnSidebar ?? this.categoryRowanOnSidebar,
      categorySpruceChipBg: categorySpruceChipBg ?? this.categorySpruceChipBg,
      categorySpruceChipFg: categorySpruceChipFg ?? this.categorySpruceChipFg,
      categorySpruceFg: categorySpruceFg ?? this.categorySpruceFg,
      categorySpruceOnSidebar: categorySpruceOnSidebar ?? this.categorySpruceOnSidebar,
      codeBg: codeBg ?? this.codeBg,
      codeFg: codeFg ?? this.codeFg,
      dangerBg: dangerBg ?? this.dangerBg,
      dangerBorder: dangerBorder ?? this.dangerBorder,
      dangerFg: dangerFg ?? this.dangerFg,
      dangerOnBg: dangerOnBg ?? this.dangerOnBg,
      dangerSubtle: dangerSubtle ?? this.dangerSubtle,
      focusRing: focusRing ?? this.focusRing,
      focusRingOnSidebar: focusRingOnSidebar ?? this.focusRingOnSidebar,
      infoBg: infoBg ?? this.infoBg,
      infoBorder: infoBorder ?? this.infoBorder,
      infoFg: infoFg ?? this.infoFg,
      infoOnBg: infoOnBg ?? this.infoOnBg,
      infoSubtle: infoSubtle ?? this.infoSubtle,
      runCancelled: runCancelled ?? this.runCancelled,
      runCompleted: runCompleted ?? this.runCompleted,
      runFailed: runFailed ?? this.runFailed,
      runQueued: runQueued ?? this.runQueued,
      runRunning: runRunning ?? this.runRunning,
      runWaiting: runWaiting ?? this.runWaiting,
      scrim: scrim ?? this.scrim,
      shadow: shadow ?? this.shadow,
      successBg: successBg ?? this.successBg,
      successBorder: successBorder ?? this.successBorder,
      successFg: successFg ?? this.successFg,
      successOnBg: successOnBg ?? this.successOnBg,
      successSubtle: successSubtle ?? this.successSubtle,
      surfaceCanvas: surfaceCanvas ?? this.surfaceCanvas,
      surfaceHover: surfaceHover ?? this.surfaceHover,
      surfaceInput: surfaceInput ?? this.surfaceInput,
      surfaceList: surfaceList ?? this.surfaceList,
      surfaceOverlay: surfaceOverlay ?? this.surfaceOverlay,
      surfaceRaised: surfaceRaised ?? this.surfaceRaised,
      surfaceSelected: surfaceSelected ?? this.surfaceSelected,
      surfaceSidebar: surfaceSidebar ?? this.surfaceSidebar,
      surfaceSidebarHover: surfaceSidebarHover ?? this.surfaceSidebarHover,
      surfaceSidebarSelected: surfaceSidebarSelected ?? this.surfaceSidebarSelected,
      surfaceSunken: surfaceSunken ?? this.surfaceSunken,
      textLink: textLink ?? this.textLink,
      textMuted: textMuted ?? this.textMuted,
      textOnAccent: textOnAccent ?? this.textOnAccent,
      textOnSidebar: textOnSidebar ?? this.textOnSidebar,
      textOnSidebarMuted: textOnSidebarMuted ?? this.textOnSidebarMuted,
      textOnSidebarSecondary: textOnSidebarSecondary ?? this.textOnSidebarSecondary,
      textPrimary: textPrimary ?? this.textPrimary,
      textSecondary: textSecondary ?? this.textSecondary,
      triggerDone: triggerDone ?? this.triggerDone,
      triggerFailed: triggerFailed ?? this.triggerFailed,
      triggerFollowup: triggerFollowup ?? this.triggerFollowup,
      triggerQueued: triggerQueued ?? this.triggerQueued,
      triggerReceived: triggerReceived ?? this.triggerReceived,
      triggerWorking: triggerWorking ?? this.triggerWorking,
      warningBg: warningBg ?? this.warningBg,
      warningBorder: warningBorder ?? this.warningBorder,
      warningFg: warningFg ?? this.warningFg,
      warningOnBg: warningOnBg ?? this.warningOnBg,
      warningSubtle: warningSubtle ?? this.warningSubtle,
    );
  }

  @override
  DsColors lerp(covariant ThemeExtension<DsColors>? other, double t) {
    if (other is! DsColors) {
      return this;
    }
    return DsColors(
      accentBg: Color.lerp(accentBg, other.accentBg, t)!,
      accentBgHover: Color.lerp(accentBgHover, other.accentBgHover, t)!,
      accentBgPressed: Color.lerp(accentBgPressed, other.accentBgPressed, t)!,
      accentFg: Color.lerp(accentFg, other.accentFg, t)!,
      accentSubtle: Color.lerp(accentSubtle, other.accentSubtle, t)!,
      attentionBg: Color.lerp(attentionBg, other.attentionBg, t)!,
      attentionBorder: Color.lerp(attentionBorder, other.attentionBorder, t)!,
      attentionFg: Color.lerp(attentionFg, other.attentionFg, t)!,
      attentionOnBg: Color.lerp(attentionOnBg, other.attentionOnBg, t)!,
      attentionSubtle: Color.lerp(attentionSubtle, other.attentionSubtle, t)!,
      badgeOrganizationBg: Color.lerp(badgeOrganizationBg, other.badgeOrganizationBg, t)!,
      badgeOrganizationFg: Color.lerp(badgeOrganizationFg, other.badgeOrganizationFg, t)!,
      badgePersonalBg: Color.lerp(badgePersonalBg, other.badgePersonalBg, t)!,
      badgePersonalFg: Color.lerp(badgePersonalFg, other.badgePersonalFg, t)!,
      borderControl: Color.lerp(borderControl, other.borderControl, t)!,
      borderDefault: Color.lerp(borderDefault, other.borderDefault, t)!,
      borderOnSidebar: Color.lerp(borderOnSidebar, other.borderOnSidebar, t)!,
      borderSubtle: Color.lerp(borderSubtle, other.borderSubtle, t)!,
      categoryAmberChipBg: Color.lerp(categoryAmberChipBg, other.categoryAmberChipBg, t)!,
      categoryAmberChipFg: Color.lerp(categoryAmberChipFg, other.categoryAmberChipFg, t)!,
      categoryAmberFg: Color.lerp(categoryAmberFg, other.categoryAmberFg, t)!,
      categoryAmberOnSidebar: Color.lerp(categoryAmberOnSidebar, other.categoryAmberOnSidebar, t)!,
      categoryGorseChipBg: Color.lerp(categoryGorseChipBg, other.categoryGorseChipBg, t)!,
      categoryGorseChipFg: Color.lerp(categoryGorseChipFg, other.categoryGorseChipFg, t)!,
      categoryGorseFg: Color.lerp(categoryGorseFg, other.categoryGorseFg, t)!,
      categoryGorseOnSidebar: Color.lerp(categoryGorseOnSidebar, other.categoryGorseOnSidebar, t)!,
      categoryGraniteChipBg: Color.lerp(categoryGraniteChipBg, other.categoryGraniteChipBg, t)!,
      categoryGraniteChipFg: Color.lerp(categoryGraniteChipFg, other.categoryGraniteChipFg, t)!,
      categoryGraniteFg: Color.lerp(categoryGraniteFg, other.categoryGraniteFg, t)!,
      categoryGraniteOnSidebar: Color.lerp(categoryGraniteOnSidebar, other.categoryGraniteOnSidebar, t)!,
      categoryHeatherChipBg: Color.lerp(categoryHeatherChipBg, other.categoryHeatherChipBg, t)!,
      categoryHeatherChipFg: Color.lerp(categoryHeatherChipFg, other.categoryHeatherChipFg, t)!,
      categoryHeatherFg: Color.lerp(categoryHeatherFg, other.categoryHeatherFg, t)!,
      categoryHeatherOnSidebar: Color.lerp(categoryHeatherOnSidebar, other.categoryHeatherOnSidebar, t)!,
      categoryLochChipBg: Color.lerp(categoryLochChipBg, other.categoryLochChipBg, t)!,
      categoryLochChipFg: Color.lerp(categoryLochChipFg, other.categoryLochChipFg, t)!,
      categoryLochFg: Color.lerp(categoryLochFg, other.categoryLochFg, t)!,
      categoryLochOnSidebar: Color.lerp(categoryLochOnSidebar, other.categoryLochOnSidebar, t)!,
      categoryMossChipBg: Color.lerp(categoryMossChipBg, other.categoryMossChipBg, t)!,
      categoryMossChipFg: Color.lerp(categoryMossChipFg, other.categoryMossChipFg, t)!,
      categoryMossFg: Color.lerp(categoryMossFg, other.categoryMossFg, t)!,
      categoryMossOnSidebar: Color.lerp(categoryMossOnSidebar, other.categoryMossOnSidebar, t)!,
      categoryRowanChipBg: Color.lerp(categoryRowanChipBg, other.categoryRowanChipBg, t)!,
      categoryRowanChipFg: Color.lerp(categoryRowanChipFg, other.categoryRowanChipFg, t)!,
      categoryRowanFg: Color.lerp(categoryRowanFg, other.categoryRowanFg, t)!,
      categoryRowanOnSidebar: Color.lerp(categoryRowanOnSidebar, other.categoryRowanOnSidebar, t)!,
      categorySpruceChipBg: Color.lerp(categorySpruceChipBg, other.categorySpruceChipBg, t)!,
      categorySpruceChipFg: Color.lerp(categorySpruceChipFg, other.categorySpruceChipFg, t)!,
      categorySpruceFg: Color.lerp(categorySpruceFg, other.categorySpruceFg, t)!,
      categorySpruceOnSidebar: Color.lerp(categorySpruceOnSidebar, other.categorySpruceOnSidebar, t)!,
      codeBg: Color.lerp(codeBg, other.codeBg, t)!,
      codeFg: Color.lerp(codeFg, other.codeFg, t)!,
      dangerBg: Color.lerp(dangerBg, other.dangerBg, t)!,
      dangerBorder: Color.lerp(dangerBorder, other.dangerBorder, t)!,
      dangerFg: Color.lerp(dangerFg, other.dangerFg, t)!,
      dangerOnBg: Color.lerp(dangerOnBg, other.dangerOnBg, t)!,
      dangerSubtle: Color.lerp(dangerSubtle, other.dangerSubtle, t)!,
      focusRing: Color.lerp(focusRing, other.focusRing, t)!,
      focusRingOnSidebar: Color.lerp(focusRingOnSidebar, other.focusRingOnSidebar, t)!,
      infoBg: Color.lerp(infoBg, other.infoBg, t)!,
      infoBorder: Color.lerp(infoBorder, other.infoBorder, t)!,
      infoFg: Color.lerp(infoFg, other.infoFg, t)!,
      infoOnBg: Color.lerp(infoOnBg, other.infoOnBg, t)!,
      infoSubtle: Color.lerp(infoSubtle, other.infoSubtle, t)!,
      runCancelled: Color.lerp(runCancelled, other.runCancelled, t)!,
      runCompleted: Color.lerp(runCompleted, other.runCompleted, t)!,
      runFailed: Color.lerp(runFailed, other.runFailed, t)!,
      runQueued: Color.lerp(runQueued, other.runQueued, t)!,
      runRunning: Color.lerp(runRunning, other.runRunning, t)!,
      runWaiting: Color.lerp(runWaiting, other.runWaiting, t)!,
      scrim: Color.lerp(scrim, other.scrim, t)!,
      shadow: Color.lerp(shadow, other.shadow, t)!,
      successBg: Color.lerp(successBg, other.successBg, t)!,
      successBorder: Color.lerp(successBorder, other.successBorder, t)!,
      successFg: Color.lerp(successFg, other.successFg, t)!,
      successOnBg: Color.lerp(successOnBg, other.successOnBg, t)!,
      successSubtle: Color.lerp(successSubtle, other.successSubtle, t)!,
      surfaceCanvas: Color.lerp(surfaceCanvas, other.surfaceCanvas, t)!,
      surfaceHover: Color.lerp(surfaceHover, other.surfaceHover, t)!,
      surfaceInput: Color.lerp(surfaceInput, other.surfaceInput, t)!,
      surfaceList: Color.lerp(surfaceList, other.surfaceList, t)!,
      surfaceOverlay: Color.lerp(surfaceOverlay, other.surfaceOverlay, t)!,
      surfaceRaised: Color.lerp(surfaceRaised, other.surfaceRaised, t)!,
      surfaceSelected: Color.lerp(surfaceSelected, other.surfaceSelected, t)!,
      surfaceSidebar: Color.lerp(surfaceSidebar, other.surfaceSidebar, t)!,
      surfaceSidebarHover: Color.lerp(surfaceSidebarHover, other.surfaceSidebarHover, t)!,
      surfaceSidebarSelected: Color.lerp(surfaceSidebarSelected, other.surfaceSidebarSelected, t)!,
      surfaceSunken: Color.lerp(surfaceSunken, other.surfaceSunken, t)!,
      textLink: Color.lerp(textLink, other.textLink, t)!,
      textMuted: Color.lerp(textMuted, other.textMuted, t)!,
      textOnAccent: Color.lerp(textOnAccent, other.textOnAccent, t)!,
      textOnSidebar: Color.lerp(textOnSidebar, other.textOnSidebar, t)!,
      textOnSidebarMuted: Color.lerp(textOnSidebarMuted, other.textOnSidebarMuted, t)!,
      textOnSidebarSecondary: Color.lerp(textOnSidebarSecondary, other.textOnSidebarSecondary, t)!,
      textPrimary: Color.lerp(textPrimary, other.textPrimary, t)!,
      textSecondary: Color.lerp(textSecondary, other.textSecondary, t)!,
      triggerDone: Color.lerp(triggerDone, other.triggerDone, t)!,
      triggerFailed: Color.lerp(triggerFailed, other.triggerFailed, t)!,
      triggerFollowup: Color.lerp(triggerFollowup, other.triggerFollowup, t)!,
      triggerQueued: Color.lerp(triggerQueued, other.triggerQueued, t)!,
      triggerReceived: Color.lerp(triggerReceived, other.triggerReceived, t)!,
      triggerWorking: Color.lerp(triggerWorking, other.triggerWorking, t)!,
      warningBg: Color.lerp(warningBg, other.warningBg, t)!,
      warningBorder: Color.lerp(warningBorder, other.warningBorder, t)!,
      warningFg: Color.lerp(warningFg, other.warningFg, t)!,
      warningOnBg: Color.lerp(warningOnBg, other.warningOnBg, t)!,
      warningSubtle: Color.lerp(warningSubtle, other.warningSubtle, t)!,
    );
  }
}

/// Schatten für Ebenen über dem Inhalt, je Theme.
@immutable
final class DsElevation extends ThemeExtension<DsElevation> {
  const DsElevation({
    required this.dialog,
    required this.menu,
  });

  final List<BoxShadow> dialog;
  final List<BoxShadow> menu;

  static const DsElevation light = DsElevation(
    dialog: [
      BoxShadow(color: Color(0x2908191C), offset: Offset(0.0, 4.0), blurRadius: 12.0),
      BoxShadow(color: Color(0x2908191C), offset: Offset(0.0, 24.0), blurRadius: 48.0, spreadRadius: -8.0),
    ],
    menu: [
      BoxShadow(color: Color(0x2908191C), offset: Offset(0.0, 2.0), blurRadius: 6.0),
      BoxShadow(color: Color(0x2908191C), offset: Offset(0.0, 8.0), blurRadius: 20.0, spreadRadius: -4.0),
    ],
  );

  static const DsElevation dark = DsElevation(
    dialog: [
      BoxShadow(color: Color(0x80061416), offset: Offset(0.0, 4.0), blurRadius: 12.0),
      BoxShadow(color: Color(0x80061416), offset: Offset(0.0, 24.0), blurRadius: 48.0, spreadRadius: -8.0),
    ],
    menu: [
      BoxShadow(color: Color(0x80061416), offset: Offset(0.0, 2.0), blurRadius: 6.0),
      BoxShadow(color: Color(0x80061416), offset: Offset(0.0, 8.0), blurRadius: 20.0, spreadRadius: -4.0),
    ],
  );

  @override
  DsElevation copyWith({
    List<BoxShadow>? dialog,
    List<BoxShadow>? menu,
  }) {
    return DsElevation(
      dialog: dialog ?? this.dialog,
      menu: menu ?? this.menu,
    );
  }

  @override
  DsElevation lerp(covariant ThemeExtension<DsElevation>? other, double t) {
    if (other is! DsElevation) {
      return this;
    }
    return DsElevation(
      dialog: BoxShadow.lerpList(dialog, other.dialog, t)!,
      menu: BoxShadow.lerpList(menu, other.menu, t)!,
    );
  }
}

/// Dichte: comfortable (Desktop), compact (wählbar), touch (Mobil).
@immutable
final class DsDensity extends ThemeExtension<DsDensity> {
  const DsDensity({
    required this.composerMinHeight,
    required this.controlHeight,
    required this.liveboxPad,
    required this.messageFontSize,
    required this.messageGap,
    required this.messagePadY,
    required this.rowChannel,
    required this.rowPost,
    required this.rowSuggestion,
    required this.targetMin,
  });

  final double composerMinHeight;
  final double controlHeight;
  final double liveboxPad;
  final double messageFontSize;
  final double messageGap;
  final double messagePadY;
  final double rowChannel;
  final double rowPost;
  final double rowSuggestion;
  final double targetMin;

  static const DsDensity comfortable = DsDensity(
    composerMinHeight: 44.0,
    controlHeight: 32.0,
    liveboxPad: 12.0,
    messageFontSize: 15.0,
    messageGap: 16.0,
    messagePadY: 4.0,
    rowChannel: 32.0,
    rowPost: 52.0,
    rowSuggestion: 36.0,
    targetMin: 24.0,
  );

  static const DsDensity compact = DsDensity(
    composerMinHeight: 40.0,
    controlHeight: 28.0,
    liveboxPad: 8.0,
    messageFontSize: 15.0,
    messageGap: 8.0,
    messagePadY: 2.0,
    rowChannel: 28.0,
    rowPost: 44.0,
    rowSuggestion: 30.0,
    targetMin: 24.0,
  );

  static const DsDensity touch = DsDensity(
    composerMinHeight: 48.0,
    controlHeight: 44.0,
    liveboxPad: 12.0,
    messageFontSize: 16.0,
    messageGap: 16.0,
    messagePadY: 6.0,
    rowChannel: 44.0,
    rowPost: 64.0,
    rowSuggestion: 48.0,
    targetMin: 48.0,
  );

  @override
  DsDensity copyWith({
    double? composerMinHeight,
    double? controlHeight,
    double? liveboxPad,
    double? messageFontSize,
    double? messageGap,
    double? messagePadY,
    double? rowChannel,
    double? rowPost,
    double? rowSuggestion,
    double? targetMin,
  }) {
    return DsDensity(
      composerMinHeight: composerMinHeight ?? this.composerMinHeight,
      controlHeight: controlHeight ?? this.controlHeight,
      liveboxPad: liveboxPad ?? this.liveboxPad,
      messageFontSize: messageFontSize ?? this.messageFontSize,
      messageGap: messageGap ?? this.messageGap,
      messagePadY: messagePadY ?? this.messagePadY,
      rowChannel: rowChannel ?? this.rowChannel,
      rowPost: rowPost ?? this.rowPost,
      rowSuggestion: rowSuggestion ?? this.rowSuggestion,
      targetMin: targetMin ?? this.targetMin,
    );
  }

  @override
  DsDensity lerp(covariant ThemeExtension<DsDensity>? other, double t) {
    if (other is! DsDensity) {
      return this;
    }
    return DsDensity(
      composerMinHeight: lerpDouble(composerMinHeight, other.composerMinHeight, t)!,
      controlHeight: lerpDouble(controlHeight, other.controlHeight, t)!,
      liveboxPad: lerpDouble(liveboxPad, other.liveboxPad, t)!,
      messageFontSize: lerpDouble(messageFontSize, other.messageFontSize, t)!,
      messageGap: lerpDouble(messageGap, other.messageGap, t)!,
      messagePadY: lerpDouble(messagePadY, other.messagePadY, t)!,
      rowChannel: lerpDouble(rowChannel, other.rowChannel, t)!,
      rowPost: lerpDouble(rowPost, other.rowPost, t)!,
      rowSuggestion: lerpDouble(rowSuggestion, other.rowSuggestion, t)!,
      targetMin: lerpDouble(targetMin, other.targetMin, t)!,
    );
  }
}

/// Bewegung: full oder reduced (Systemeinstellung bzw. appearance.reduce_motion).
@immutable
final class DsMotion extends ThemeExtension<DsMotion> {
  const DsMotion({
    required this.durationFade,
    required this.durationMove,
    required this.durationMoveFast,
    required this.durationMoveSlow,
    required this.durationPulse,
    required this.easingEnter,
    required this.easingExit,
    required this.easingStandard,
  });

  final Duration durationFade;
  final Duration durationMove;
  final Duration durationMoveFast;
  final Duration durationMoveSlow;
  final Duration durationPulse;
  final Cubic easingEnter;
  final Cubic easingExit;
  final Cubic easingStandard;

  static const DsMotion full = DsMotion(
    durationFade: Duration(milliseconds: 120),
    durationMove: Duration(milliseconds: 180),
    durationMoveFast: Duration(milliseconds: 120),
    durationMoveSlow: Duration(milliseconds: 260),
    durationPulse: Duration(milliseconds: 1400),
    easingEnter: Cubic(0.0, 0.0, 0.2, 1.0),
    easingExit: Cubic(0.4, 0.0, 1.0, 1.0),
    easingStandard: Cubic(0.2, 0.0, 0.0, 1.0),
  );

  static const DsMotion reduced = DsMotion(
    durationFade: Duration(milliseconds: 80),
    durationMove: Duration.zero,
    durationMoveFast: Duration.zero,
    durationMoveSlow: Duration.zero,
    durationPulse: Duration.zero,
    easingEnter: Cubic(0.0, 0.0, 1.0, 1.0),
    easingExit: Cubic(0.0, 0.0, 1.0, 1.0),
    easingStandard: Cubic(0.0, 0.0, 1.0, 1.0),
  );

  @override
  DsMotion copyWith({
    Duration? durationFade,
    Duration? durationMove,
    Duration? durationMoveFast,
    Duration? durationMoveSlow,
    Duration? durationPulse,
    Cubic? easingEnter,
    Cubic? easingExit,
    Cubic? easingStandard,
  }) {
    return DsMotion(
      durationFade: durationFade ?? this.durationFade,
      durationMove: durationMove ?? this.durationMove,
      durationMoveFast: durationMoveFast ?? this.durationMoveFast,
      durationMoveSlow: durationMoveSlow ?? this.durationMoveSlow,
      durationPulse: durationPulse ?? this.durationPulse,
      easingEnter: easingEnter ?? this.easingEnter,
      easingExit: easingExit ?? this.easingExit,
      easingStandard: easingStandard ?? this.easingStandard,
    );
  }

  @override
  DsMotion lerp(covariant ThemeExtension<DsMotion>? other, double t) {
    if (other is! DsMotion) {
      return this;
    }
    return DsMotion(
      durationFade: t < 0.5 ? durationFade : other.durationFade,
      durationMove: t < 0.5 ? durationMove : other.durationMove,
      durationMoveFast: t < 0.5 ? durationMoveFast : other.durationMoveFast,
      durationMoveSlow: t < 0.5 ? durationMoveSlow : other.durationMoveSlow,
      durationPulse: t < 0.5 ? durationPulse : other.durationPulse,
      easingEnter: t < 0.5 ? easingEnter : other.easingEnter,
      easingExit: t < 0.5 ? easingExit : other.easingExit,
      easingStandard: t < 0.5 ? easingStandard : other.easingStandard,
    );
  }
}

/// Abstände (Raster 4 px).
abstract final class DsSpace {
  static const double s0 = 0.0;
  static const double s2 = 2.0;
  static const double s4 = 4.0;
  static const double s6 = 6.0;
  static const double s8 = 8.0;
  static const double s12 = 12.0;
  static const double s16 = 16.0;
  static const double s20 = 20.0;
  static const double s24 = 24.0;
  static const double s32 = 32.0;
  static const double s40 = 40.0;
  static const double s48 = 48.0;
  static const double s64 = 64.0;
}

/// Eckenradien.
abstract final class DsRadius {
  static const double lg = 8.0;
  static const double md = 6.0;
  static const double none = 0.0;
  static const double pill = 999.0;
  static const double sm = 4.0;
  static const double xl = 12.0;
  static const double xs = 2.0;
}

/// Linienstärken.
abstract final class DsBorderWidth {
  static const double focus = 2.0;
  static const double hairline = 1.0;
  static const double spine = 3.0;
}

/// Startbreiten der Desktop-Spalten.
abstract final class DsLayout {
  static const double columnMin = 200.0;
  static const double listWidth = 340.0;
  static const double messageMaxWidth = 880.0;
  static const double overlayWidth = 560.0;
  static const double sessionWidth = 296.0;
  static const double sidebarWidth = 248.0;
}

/// Feste Maße.
abstract final class DsSize {
  static const double avatarLg = 40.0;
  static const double avatarMd = 32.0;
  static const double avatarSm = 20.0;
  static const double badgeHeight = 20.0;
  static const double statusDot = 8.0;
}

/// Icon-Größen.
abstract final class DsIconSize {
  static const double sizeLg = 24.0;
  static const double sizeMd = 20.0;
  static const double sizeSm = 16.0;
}

/// Schriftfamilien mit Rückfall-Liste.
abstract final class DsFonts {
  static const List<String> familyMono = ['Atkinson Hyperlegible Mono', 'ui-monospace', 'SF Mono', 'Cascadia Mono', 'Consolas', 'monospace'];
  static const List<String> familySans = ['Atkinson Hyperlegible Next', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'];
}

/// Typo-Rollen. Farbe kommt aus DsColors, Größe skaliert mit MediaQuery.textScaler.
abstract final class DsType {
  static const TextStyle body = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 15.0, fontWeight: FontWeight.w400, height: 1.467, letterSpacing: 0.0);
  static const TextStyle bodySm = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 14.0, fontWeight: FontWeight.w400, height: 1.43, letterSpacing: 0.0);
  static const TextStyle bodyStrong = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 15.0, fontWeight: FontWeight.w600, height: 1.467, letterSpacing: 0.0);
  static const TextStyle caption = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 12.0, fontWeight: FontWeight.w400, height: 1.333, letterSpacing: 0.0);
  static const TextStyle code = TextStyle(fontFamily: 'Atkinson Hyperlegible Mono', fontFamilyFallback: ['ui-monospace', 'SF Mono', 'Cascadia Mono', 'Consolas', 'monospace'], fontSize: 14.0, fontWeight: FontWeight.w400, height: 1.43, letterSpacing: 0.0);
  static const TextStyle codeSm = TextStyle(fontFamily: 'Atkinson Hyperlegible Mono', fontFamilyFallback: ['ui-monospace', 'SF Mono', 'Cascadia Mono', 'Consolas', 'monospace'], fontSize: 12.0, fontWeight: FontWeight.w500, height: 1.333, letterSpacing: 0.0);
  static const TextStyle display = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 30.0, fontWeight: FontWeight.w800, height: 1.2, letterSpacing: -0.4);
  static const TextStyle heading = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 16.0, fontWeight: FontWeight.w600, height: 1.375, letterSpacing: 0.0);
  static const TextStyle label = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 13.0, fontWeight: FontWeight.w600, height: 1.385, letterSpacing: 0.0);
  static const TextStyle section = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 12.0, fontWeight: FontWeight.w700, height: 1.333, letterSpacing: 0.3);
  static const TextStyle title = TextStyle(fontFamily: 'Atkinson Hyperlegible Next', fontFamilyFallback: ['system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'], fontSize: 20.0, fontWeight: FontWeight.w700, height: 1.3, letterSpacing: -0.15);
}

enum DsIconWeight { thin, light, regular, bold, fill, duotone }

/// Semantisches Icon: Name im Phosphor-Set und Strichstärke.
@immutable
final class DsGlyph {
  const DsGlyph(this.name, this.weight, {this.spins = false});
  final String name;
  final DsIconWeight weight;
  final bool spins;
}

/// Icons aus @phosphor-icons/core 2.1.1 (MIT).
abstract final class DsIcons {
  static const DsGlyph actionAttach = DsGlyph('paperclip', DsIconWeight.regular);
  static const DsGlyph actionBack = DsGlyph('caret-left', DsIconWeight.bold);
  static const DsGlyph actionCloseThread = DsGlyph('seal-check', DsIconWeight.regular);
  static const DsGlyph actionCollapse = DsGlyph('caret-right', DsIconWeight.bold);
  static const DsGlyph actionDelete = DsGlyph('trash', DsIconWeight.regular);
  static const DsGlyph actionDismiss = DsGlyph('x', DsIconWeight.bold);
  static const DsGlyph actionEdit = DsGlyph('pencil-simple', DsIconWeight.regular);
  static const DsGlyph actionExpand = DsGlyph('caret-down', DsIconWeight.bold);
  static const DsGlyph actionExport = DsGlyph('export', DsIconWeight.regular);
  static const DsGlyph actionFilter = DsGlyph('funnel', DsIconWeight.regular);
  static const DsGlyph actionMore = DsGlyph('dots-three', DsIconWeight.bold);
  static const DsGlyph actionMute = DsGlyph('bell-slash', DsIconWeight.regular);
  static const DsGlyph actionNew = DsGlyph('plus', DsIconWeight.bold);
  static const DsGlyph actionNotifications = DsGlyph('bell', DsIconWeight.regular);
  static const DsGlyph actionPin = DsGlyph('push-pin', DsIconWeight.regular);
  static const DsGlyph actionQuickSwitch = DsGlyph('command', DsIconWeight.regular);
  static const DsGlyph actionReact = DsGlyph('smiley', DsIconWeight.regular);
  static const DsGlyph actionReopen = DsGlyph('arrow-counter-clockwise', DsIconWeight.regular);
  static const DsGlyph actionReply = DsGlyph('arrow-bend-up-left', DsIconWeight.regular);
  static const DsGlyph actionSearch = DsGlyph('magnifying-glass', DsIconWeight.regular);
  static const DsGlyph actionSend = DsGlyph('paper-plane-right', DsIconWeight.fill);
  static const DsGlyph actionSettings = DsGlyph('gear-six', DsIconWeight.regular);
  static const DsGlyph actionShare = DsGlyph('share-network', DsIconWeight.regular);
  static const DsGlyph actionStop = DsGlyph('stop-circle', DsIconWeight.fill);
  static const DsGlyph actionTogglePanel = DsGlyph('sidebar-simple', DsIconWeight.regular);
  static const DsGlyph actionVoice = DsGlyph('microphone', DsIconWeight.regular);
  static const DsGlyph adminAudit = DsGlyph('scroll', DsIconWeight.regular);
  static const DsGlyph adminChecklist = DsGlyph('list-checks', DsIconWeight.regular);
  static const DsGlyph adminConsole = DsGlyph('sliders-horizontal', DsIconWeight.regular);
  static const DsGlyph adminStats = DsGlyph('chart-line-up', DsIconWeight.regular);
  static const DsGlyph adminViewAs = DsGlyph('eye', DsIconWeight.regular);
  static const DsGlyph agent = DsGlyph('robot', DsIconWeight.regular);
  static const DsGlyph billingOrganization = DsGlyph('buildings', DsIconWeight.fill);
  static const DsGlyph billingPersonal = DsGlyph('user-circle', DsIconWeight.fill);
  static const DsGlyph channelDm = DsGlyph('chat-teardrop', DsIconWeight.regular);
  static const DsGlyph channelForum = DsGlyph('chats-circle', DsIconWeight.regular);
  static const DsGlyph channelText = DsGlyph('hash', DsIconWeight.bold);
  static const DsGlyph context = DsGlyph('gauge', DsIconWeight.regular);
  static const DsGlyph cost = DsGlyph('coins', DsIconWeight.regular);
  static const DsGlyph group = DsGlyph('users-three', DsIconWeight.regular);
  static const DsGlyph person = DsGlyph('user', DsIconWeight.regular);
  static const DsGlyph runCancelled = DsGlyph('prohibit', DsIconWeight.regular);
  static const DsGlyph runCompleted = DsGlyph('check-circle', DsIconWeight.fill);
  static const DsGlyph runFailed = DsGlyph('warning-octagon', DsIconWeight.fill);
  static const DsGlyph runQueued = DsGlyph('hourglass-medium', DsIconWeight.regular);
  static const DsGlyph runRunning = DsGlyph('circle-notch', DsIconWeight.bold, spins: true);
  static const DsGlyph runWaiting = DsGlyph('bell-ringing', DsIconWeight.fill);
  static const DsGlyph securityKey = DsGlyph('key', DsIconWeight.regular);
  static const DsGlyph securityLock = DsGlyph('lock-simple', DsIconWeight.fill);
  static const DsGlyph securityStepUp = DsGlyph('shield-check', DsIconWeight.fill);
  static const DsGlyph stateError = DsGlyph('warning-circle', DsIconWeight.fill);
  static const DsGlyph stateInfo = DsGlyph('info', DsIconWeight.fill);
  static const DsGlyph stateOffline = DsGlyph('cloud-slash', DsIconWeight.regular);
  static const DsGlyph stateReconnecting = DsGlyph('arrows-clockwise', DsIconWeight.regular, spins: true);
  static const DsGlyph stateWarning = DsGlyph('warning', DsIconWeight.fill);
  static const DsGlyph stepBackgroundJob = DsGlyph('clock-countdown', DsIconWeight.regular);
  static const DsGlyph stepStatus = DsGlyph('dot-outline', DsIconWeight.fill);
  static const DsGlyph stepSubagent = DsGlyph('robot', DsIconWeight.regular);
  static const DsGlyph stepToolCall = DsGlyph('wrench', DsIconWeight.regular);
  static const DsGlyph stepWorkflow = DsGlyph('flow-arrow', DsIconWeight.regular);
  static const DsGlyph tag = DsGlyph('tag', DsIconWeight.regular);
  static const DsGlyph thread = DsGlyph('chat-text', DsIconWeight.regular);
  static const DsGlyph triggerDone = DsGlyph('check-circle', DsIconWeight.fill);
  static const DsGlyph triggerFailed = DsGlyph('x-circle', DsIconWeight.fill);
  static const DsGlyph triggerFollowup = DsGlyph('arrow-bend-down-right', DsIconWeight.regular);
  static const DsGlyph triggerQueued = DsGlyph('hourglass-medium', DsIconWeight.regular);
  static const DsGlyph triggerReceived = DsGlyph('check', DsIconWeight.regular);
  static const DsGlyph triggerWorking = DsGlyph('circle-notch', DsIconWeight.bold, spins: true);
  static const List<String> spaceIcons = ['folder-simple', 'headset', 'megaphone', 'code', 'terminal-window', 'flask', 'briefcase', 'book-open', 'chart-line-up', 'palette', 'globe-hemisphere-west', 'rocket-launch', 'house', 'lifebuoy', 'newspaper', 'users-three', 'robot', 'gear-six'];
}

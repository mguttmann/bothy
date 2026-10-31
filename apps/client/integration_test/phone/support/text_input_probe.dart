// SPDX-License-Identifier: Apache-2.0

// The text input probe of the phone integration tests (DICTATION-MOBILE): the method channel
// `bothy/text_input_probe`, which only debug builds of the runner register (iOS: AppDelegate.swift
// under `#if DEBUG`; Android: src/debug/kotlin/.../TextInputProbe.kt). It drives the focused text
// field from the native side the way an input source outside the app does, so a test sees what
// the person sees after dictation, a keyboard extension or a paste of the system:
//
// - iOS: `insertText:` of the focused `UITextInput` (what a keyboard extension's
//   `textDocumentProxy.insertText` and a dictation app's insert arrive as), the same in pieces
//   after reading the text before the cursor (a keyboard extension), `setMarkedText:` then
//   `unmarkText` (the system dictation), and the action `paste:` sent to the first responder
//   (the edit menu) after the text was put on the general pasteboard.
// - Android: `commitText`, `setComposingText` then `commitText` or `finishComposingText` (an
//   input method and the system voice typing) and `performContextMenuAction(paste)` (the
//   clipboard of an input method) on an `InputConnection` of the Flutter view, after the text
//   was put on the clipboard.
//
// Every call answers the text of the field as the native side holds it afterwards.

import 'dart:io';

import 'package:flutter/services.dart';

const MethodChannel _channel = MethodChannel('bothy/text_input_probe');

/// Whether the probe exists here: on an iOS simulator or an Android device or emulator (the
/// Flutter tester has no runner).
bool get probeAvailable => Platform.isIOS || Platform.isAndroid;

Future<String> _call(String method, [Map<String, Object?>? args]) async =>
    await _channel.invokeMethod<String>(method, args) ?? '';

/// The text of the focused field as the platform's text input holds it.
Future<String> nativeText() => _call('text');

/// iOS: `insertText:` with [text]; Android: `commitText` with [text].
Future<String> nativeInsert(String text) =>
    _call(Platform.isIOS ? 'insertText' : 'commitText', {'text': text});

/// iOS: a keyboard extension's insert, [chunks] one `insertText:` each after it read the text
/// before the cursor, with a space before the first chunk when that text does not end in one.
Future<String> keyboardExtensionInsert(List<String> chunks) =>
    _call('keyboardInsert', {'chunks': chunks});

/// iOS: the system dictation, each of [partials] as marked text, then `unmarkText`.
///
/// Android: voice typing, each of [partials] as composing text, then [commit] committed, or the
/// composition finished without one.
Future<String> nativeDictate(List<String> partials, {String? commit}) => _call(
  Platform.isIOS ? 'dictate' : 'compose',
  {'partials': partials, 'commit': commit},
);

/// Puts [text] on the system clipboard and pastes it the native way: the edit menu's `paste:`
/// on iOS, the input method's paste on Android.
Future<String> nativePaste(String text) => _call('paste', {'text': text});

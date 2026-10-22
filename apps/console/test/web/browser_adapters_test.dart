// SPDX-License-Identifier: Apache-2.0

// The web-only adapters in a real browser (brief M6-02 Tests): run with
// `flutter test --platform chrome test/web` from apps/console. `make test` runs test/ on the
// VM and skips these files (TestOn browser).
@TestOn('browser')
library;

import 'dart:js_interop';
import 'dart:js_interop_unsafe';
import 'dart:typed_data';

import 'package:console/platform/browser.dart';
import 'package:console/platform/browser_web.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:web/web.dart' as web;

@JS('Object.defineProperty')
external void _defineProperty(
  JSObject target,
  JSString name,
  JSObject descriptor,
);

/// Replaces `navigator.credentials` with an object whose `get` is [get].
void _fakeCredentials(JSFunction get) {
  final fake = JSObject()..['get'] = get;
  _defineProperty(
    web.window.navigator,
    'credentials'.toJS,
    JSObject()
      ..['value'] = fake
      ..['configurable'] = true.toJS,
  );
}

JSArrayBuffer _buffer(String text) =>
    Uint8List.fromList(text.codeUnits).buffer.toJS;

void main() {
  test('sessionStorage keeps, returns and deletes a flow entry', () {
    final store = SessionFlowStore(web.window.sessionStorage);
    store.write('bothy.console.oidc.login.a1', 'verifier');
    expect(
      web.window.sessionStorage.getItem('bothy.console.oidc.login.a1'),
      'verifier',
    );
    expect(store.read('bothy.console.oidc.login.a1'), 'verifier');
    store.delete('bothy.console.oidc.login.a1');
    expect(store.read('bothy.console.oidc.login.a1'), isNull);
    expect(web.window.localStorage.length, 0);
  });

  test('the document language follows the app language', () {
    WindowNavigation(web.window).setDocumentLanguage('de');
    expect(web.document.documentElement!.getAttribute('lang'), 'de');
  });

  test(
    'the passkey adapter asks navigator.credentials.get with user verification',
    () async {
      JSObject? seen;
      _fakeCredentials(
        ((JSObject options) {
          seen = options;
          final response = JSObject()
            ..['clientDataJSON'] = _buffer('{}')
            ..['authenticatorData'] = _buffer('auth')
            ..['signature'] = _buffer('sig')
            ..['userHandle'] = null;
          final credential = JSObject()
            ..['id'] = 'Y3JlZA'.toJS
            ..['rawId'] = _buffer('cred')
            ..['type'] = 'public-key'.toJS
            ..['authenticatorAttachment'] = 'platform'.toJS
            ..['response'] = response;
          return Future<JSObject?>.value(credential).toJS;
        }).toJS,
      );
      final json = await BrowserPasskeys(web.window.navigator.credentials).get({
        'challenge': 'Zm9v',
        'rpId': 'localhost',
        'userVerification': 'discouraged',
        'allowCredentials': [
          {'type': 'public-key', 'id': 'Y3JlZA'},
        ],
      });
      final publicKey = seen!['publicKey']! as JSObject;
      expect((publicKey['userVerification']! as JSString).toDart, 'required');
      expect((publicKey['rpId']! as JSString).toDart, 'localhost');
      expect((publicKey['challenge']! as JSUint8Array).toDart, 'foo'.codeUnits);
      expect(json['rawId'], 'Y3JlZA');
      expect((json['response']! as Map)['signature'], 'c2ln');
    },
  );

  test('a refused prompt is a cancelled passkey', () async {
    _fakeCredentials(
      ((JSObject options) => Future<JSObject?>.error(
        StateError('NotAllowedError'),
      ).toJS).toJS,
    );
    await expectLater(
      BrowserPasskeys(web.window.navigator.credentials)
          .get({'challenge': 'Zm9v'}),
      throwsA(isA<PasskeyCancelled>()),
    );
  });
}

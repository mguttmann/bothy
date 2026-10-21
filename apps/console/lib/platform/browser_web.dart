// SPDX-License-Identifier: Apache-2.0

/// The environment of the web build: sessionStorage, `window.location` and
/// `navigator.credentials` through package:web. The HTTP stack is Dio's browser adapter, which
/// sends the cookies of the same origin by itself.
library;

import 'dart:js_interop';
import 'dart:js_interop_unsafe';
import 'dart:typed_data';

import 'package:web/web.dart' as web;

import '../login/webauthn_json.dart';
import 'browser.dart';

/// The environment of the page the console runs in.
ConsoleEnvironment browserEnvironment() {
  final base = Uri.base;
  return ConsoleEnvironment(
    origin: Uri(scheme: base.scheme, host: base.host, port: base.port),
    flowStore: SessionFlowStore(web.window.sessionStorage),
    navigation: WindowNavigation(web.window),
    passkeys: BrowserPasskeys(web.window.navigator.credentials),
    downloads: AnchorDownloads(web.window.document),
  );
}

/// [FileDownloads] over a temporary `blob:` URL and an anchor with the `download` attribute:
/// the browser saves the file, nothing is kept in browser storage, and the URL is revoked
/// right after the click.
class AnchorDownloads implements FileDownloads {
  /// Creates the downloads of [document].
  AnchorDownloads(this.document);

  /// The document of the tab.
  final web.Document document;

  @override
  void save(String name, String type, Uint8List bytes) {
    final blob = web.Blob([bytes.toJS].toJS, web.BlobPropertyBag(type: type));
    final url = web.URL.createObjectURL(blob);
    final anchor = document.createElement('a') as web.HTMLAnchorElement
      ..href = url
      ..download = name
      ..style.display = 'none';
    document.body?.append(anchor);
    anchor.click();
    anchor.remove();
    web.URL.revokeObjectURL(url);
  }
}

/// [FlowStore] over a `Storage`, in the console the tab's `sessionStorage`.
class SessionFlowStore implements FlowStore {
  /// Creates the store over [storage].
  SessionFlowStore(this.storage);

  /// The storage area.
  final web.Storage storage;

  @override
  String? read(String key) => storage.getItem(key);

  @override
  void write(String key, String value) => storage.setItem(key, value);

  @override
  void delete(String key) => storage.removeItem(key);
}

/// [BrowserNavigation] over a window.
class WindowNavigation implements BrowserNavigation {
  /// Creates the navigation of [window].
  WindowNavigation(this.window);

  /// The window of the tab.
  final web.Window window;

  @override
  void assign(Uri url) => window.location.assign(url.toString());

  @override
  void reload() => window.location.reload();

  @override
  void setDocumentLanguage(String tag) =>
      window.document.documentElement?.setAttribute('lang', tag);
}

/// [PasskeyPort] over `navigator.credentials`.
class BrowserPasskeys implements PasskeyPort {
  /// Creates the adapter over [credentials].
  BrowserPasskeys(this.credentials);

  /// The credentials container of the browser.
  final web.CredentialsContainer credentials;

  @override
  Future<Map<String, Object?>> get(Map<String, Object?> options) async {
    final request = PasskeyRequest.fromJson(options);
    final publicKey = JSObject()
      ..['challenge'] = request.challenge.toJS
      ..['userVerification'] = 'required'.toJS
      ..['allowCredentials'] = [
        for (final c in request.allowCredentials)
          JSObject()
            ..['type'] = 'public-key'.toJS
            ..['id'] = c.id.toJS
            ..['transports'] = [for (final t in c.transports) t.toJS].toJS,
      ].toJS;
    final rpId = request.rpId;
    if (rpId != null) publicKey['rpId'] = rpId.toJS;
    final timeout = request.timeoutMs;
    if (timeout != null) publicKey['timeout'] = timeout.toJS;
    final web.Credential? credential;
    try {
      credential = await credentials
          .get(
            web.CredentialRequestOptions(
              publicKey: publicKey as web.PublicKeyCredentialRequestOptions,
            ),
          )
          .toDart;
    } on Object {
      // NotAllowedError (cancelled, timed out, no passkey), AbortError and SecurityError all
      // mean the same to the person: no passkey answered.
      throw const PasskeyCancelled();
    }
    if (credential == null) throw const PasskeyCancelled();
    final pk = credential as web.PublicKeyCredential;
    final response = pk.response as web.AuthenticatorAssertionResponse;
    return assertionJson(
      id: pk.id,
      rawId: _bytes(pk.rawId),
      clientDataJson: _bytes(response.clientDataJSON),
      authenticatorData: _bytes(response.authenticatorData),
      signature: _bytes(response.signature),
      userHandle: response.userHandle == null
          ? null
          : _bytes(response.userHandle!),
      authenticatorAttachment: pk.authenticatorAttachment,
    );
  }

  static Uint8List _bytes(JSArrayBuffer buffer) => buffer.toDart.asUint8List();
}

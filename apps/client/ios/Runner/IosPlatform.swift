// SPDX-License-Identifier: Apache-2.0

import AuthenticationServices
import Flutter
import UIKit

/// The runner's own plugin (brief M5-02 Design notes 3 and 6): the Pigeon host API
/// `IosPlatformApi` and the deep links of the URL scheme of the build branding
/// (`BRANDING_URL_SCHEME`, docs/spec/15-client-architecture.md 11).
///
/// Links arrive through the scene life cycle: at the start in the connection options, later in
/// `scene(_:openURLContexts:)`. They go to Dart over the channel of `DeepLinkChannel`; only the
/// Dart side parses them (`lib/app/deep_links.dart`).
final class IosPlatformPlugin: NSObject, FlutterPlugin, FlutterSceneLifeCycleDelegate {
  private let api = IosPlatformApiImpl()

  /// Connects the host API and the deep link channel to the engine of [registrar].
  static func register(with registrar: FlutterPluginRegistrar) {
    let plugin = IosPlatformPlugin()
    IosPlatformApiSetup.setUp(binaryMessenger: registrar.messenger(), api: plugin.api)
    DeepLinkChannel.shared.attach(messenger: registrar.messenger())
    registrar.addSceneDelegate(plugin)
  }

  func scene(
    _ scene: UIScene,
    willConnectTo session: UISceneSession,
    options connectionOptions: UIScene.ConnectionOptions?
  ) -> Bool {
    for context in connectionOptions?.urlContexts ?? [] {
      DeepLinkChannel.shared.receive(context.url.absoluteString)
    }
    return false
  }

  func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) -> Bool {
    for context in URLContexts {
      DeepLinkChannel.shared.receive(context.url.absoluteString)
    }
    return !URLContexts.isEmpty
  }
}

/// Forwards deep links to Dart over the method channel `bothy/deep_links`, like the macOS
/// runner: a link that arrives before Dart listens waits in `pending` until Dart asks with
/// `takePending`; later links go out at once with `open`.
final class DeepLinkChannel {
  /// The one instance of the app.
  static let shared = DeepLinkChannel()

  private var channel: FlutterMethodChannel?
  private var pending: [String] = []

  /// Connects the channel to the engine.
  func attach(messenger: FlutterBinaryMessenger) {
    let channel = FlutterMethodChannel(name: "bothy/deep_links", binaryMessenger: messenger)
    channel.setMethodCallHandler { [weak self] call, result in
      guard call.method == "takePending" else {
        result(FlutterMethodNotImplemented)
        return
      }
      let links = self?.pending ?? []
      self?.pending.removeAll()
      result(links)
    }
    self.channel = channel
  }

  /// Hands a link to Dart, or keeps it until Dart asks.
  func receive(_ link: String) {
    guard let channel = channel else {
      pending.append(link)
      return
    }
    channel.invokeMethod("open", arguments: link)
  }
}

/// The key window of the app, the anchor of the system sheets.
private func keyWindow() -> UIWindow {
  let windows = UIApplication.shared.connectedScenes
    .compactMap { $0 as? UIWindowScene }
    .flatMap { $0.windows }
  return windows.first { $0.isKeyWindow } ?? windows.first ?? UIWindow()
}

/// An error for Dart with a stable code and no local details.
private func failure(_ code: String) -> PigeonError {
  PigeonError(code: code, message: nil, details: nil)
}

/// The host side of `IosPlatformApi`: every system call runs on the main queue, every error
/// leaves as a `PigeonError` with the code `cancelled`, `unavailable` or `failed`.
final class IosPlatformApiImpl: NSObject, IosPlatformApi,
  ASWebAuthenticationPresentationContextProviding
{
  private var webSession: ASWebAuthenticationSession?
  private var ceremony: PasskeyCeremony?

  // MARK: Login in the system browser (15 8, 08 14.1)

  /// One `ASWebAuthenticationSession` without an ephemeral browser session, so the identity
  /// provider's session can be reused (RFC 8252); never a web view.
  func authenticate(url: String, callbackScheme: String) async throws -> String {
    guard let target = URL(string: url) else { throw failure("failed") }
    return try await withCheckedThrowingContinuation { continuation in
      DispatchQueue.main.async {
        let session = ASWebAuthenticationSession(
          url: target,
          callbackURLScheme: callbackScheme
        ) { [weak self] callback, error in
          self?.webSession = nil
          if let callback = callback {
            continuation.resume(returning: callback.absoluteString)
            return
          }
          let cancelled =
            (error as? ASWebAuthenticationSessionError)?.code == .canceledLogin
          continuation.resume(throwing: failure(cancelled ? "cancelled" : "failed"))
        }
        session.presentationContextProvider = self
        session.prefersEphemeralWebBrowserSession = false
        self.webSession = session
        if !session.start() {
          self.webSession = nil
          continuation.resume(throwing: failure("unavailable"))
        }
      }
    }
  }

  func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
    keyWindow()
  }

  // MARK: Passkeys (08 2.5; only with an associated domain, 15 8)

  func passkeyAssertion(requestOptionsJson: String) async throws -> String {
    let options = try json(requestOptionsJson)
    guard let rpId = options["rpId"] as? String,
      let challenge = base64url(options["challenge"])
    else { throw failure("failed") }
    let allowed = (options["allowCredentials"] as? [[String: Any]] ?? [])
      .compactMap { base64url($0["id"]) }
    let verification = options["userVerification"] as? String
    return try await withCheckedThrowingContinuation { continuation in
      DispatchQueue.main.async {
        let provider = ASAuthorizationPlatformPublicKeyCredentialProvider(
          relyingPartyIdentifier: rpId)
        let request = provider.createCredentialAssertionRequest(challenge: challenge)
        request.allowedCredentials = allowed.map {
          ASAuthorizationPlatformPublicKeyCredentialDescriptor(credentialID: $0)
        }
        if let verification = verification {
          request.userVerificationPreference =
            ASAuthorizationPublicKeyCredentialUserVerificationPreference(rawValue: verification)
        }
        self.run(request, continuation)
      }
    }
  }

  func passkeyRegistration(creationOptionsJson: String) async throws -> String {
    let options = try json(creationOptionsJson)
    guard let rp = options["rp"] as? [String: Any], let rpId = rp["id"] as? String,
      let user = options["user"] as? [String: Any], let userId = base64url(user["id"]),
      let name = user["name"] as? String,
      let challenge = base64url(options["challenge"])
    else { throw failure("failed") }
    let selection = options["authenticatorSelection"] as? [String: Any]
    let verification = selection?["userVerification"] as? String
    return try await withCheckedThrowingContinuation { continuation in
      DispatchQueue.main.async {
        let provider = ASAuthorizationPlatformPublicKeyCredentialProvider(
          relyingPartyIdentifier: rpId)
        let request = provider.createCredentialRegistrationRequest(
          challenge: challenge, name: name, userID: userId)
        if let verification = verification {
          request.userVerificationPreference =
            ASAuthorizationPublicKeyCredentialUserVerificationPreference(rawValue: verification)
        }
        self.run(request, continuation)
      }
    }
  }

  private func run(
    _ request: ASAuthorizationRequest,
    _ continuation: CheckedContinuation<String, Error>
  ) {
    let ceremony = PasskeyCeremony { [weak self] result in
      self?.ceremony = nil
      continuation.resume(with: result)
    }
    self.ceremony = ceremony
    let controller = ASAuthorizationController(authorizationRequests: [request])
    controller.delegate = ceremony
    controller.presentationContextProvider = ceremony
    controller.performRequests()
  }

  // MARK: File protection (15 6.3)

  func protectFiles(paths: [String]) throws {
    let files = FileManager.default
    for path in paths where files.fileExists(atPath: path) {
      do {
        try files.setAttributes([.protectionKey: FileProtectionType.complete], ofItemAtPath: path)
      } catch {
        throw failure("failed")
      }
    }
  }

  func fileProtection(path: String) throws -> String? {
    let files = FileManager.default
    guard files.fileExists(atPath: path) else { return nil }
    guard let attributes = try? files.attributesOfItem(atPath: path) else {
      throw failure("failed")
    }
    switch attributes[.protectionKey] as? FileProtectionType {
    case .complete?: return "complete"
    case .completeUnlessOpen?: return "completeUnlessOpen"
    case .completeUntilFirstUserAuthentication?: return "completeUntilFirstUserAuthentication"
    default: return "none"
    }
  }
}

/// One passkey ceremony of `ASAuthorizationController`: turns the platform credential into the
/// `PublicKeyCredential` JSON of WebAuthn Level 3 with base64url fields.
private final class PasskeyCeremony: NSObject, ASAuthorizationControllerDelegate,
  ASAuthorizationControllerPresentationContextProviding
{
  private let done: (Result<String, Error>) -> Void

  init(done: @escaping (Result<String, Error>) -> Void) {
    self.done = done
  }

  func presentationAnchor(for controller: ASAuthorizationController) -> ASPresentationAnchor {
    keyWindow()
  }

  func authorizationController(
    controller: ASAuthorizationController,
    didCompleteWithAuthorization authorization: ASAuthorization
  ) {
    var credential: [String: Any]?
    if let assertion = authorization.credential
      as? ASAuthorizationPlatformPublicKeyCredentialAssertion
    {
      credential = publicKeyCredential(
        id: assertion.credentialID,
        response: [
          "clientDataJSON": encode(assertion.rawClientDataJSON),
          "authenticatorData": encode(assertion.rawAuthenticatorData),
          "signature": encode(assertion.signature),
          "userHandle": encode(assertion.userID),
        ])
    } else if let registration = authorization.credential
      as? ASAuthorizationPlatformPublicKeyCredentialRegistration,
      let attestation = registration.rawAttestationObject
    {
      credential = publicKeyCredential(
        id: registration.credentialID,
        response: [
          "clientDataJSON": encode(registration.rawClientDataJSON),
          "attestationObject": encode(attestation),
          "transports": ["internal"],
        ])
    }
    guard let credential = credential,
      let data = try? JSONSerialization.data(withJSONObject: credential),
      let text = String(data: data, encoding: .utf8)
    else {
      done(.failure(failure("failed")))
      return
    }
    done(.success(text))
  }

  func authorizationController(
    controller: ASAuthorizationController, didCompleteWithError error: Error
  ) {
    let cancelled = (error as? ASAuthorizationError)?.code == .canceled
    done(.failure(failure(cancelled ? "cancelled" : "failed")))
  }

  private func publicKeyCredential(id: Data, response: [String: Any]) -> [String: Any] {
    [
      "id": encode(id),
      "rawId": encode(id),
      "type": "public-key",
      "authenticatorAttachment": "platform",
      "response": response,
      "clientExtensionResults": [String: Any](),
    ]
  }
}

/// The JSON object of [text], or a `failed` error.
private func json(_ text: String) throws -> [String: Any] {
  guard let data = text.data(using: .utf8),
    let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
  else { throw failure("failed") }
  return object
}

/// The bytes of a base64url string without padding (WebAuthn's JSON form), or nil.
private func base64url(_ value: Any?) -> Data? {
  guard var text = value as? String else { return nil }
  text = text.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
  while text.count % 4 != 0 { text += "=" }
  return Data(base64Encoded: text)
}

/// [data] as base64url without padding.
private func encode(_ data: Data) -> String {
  data.base64EncodedString()
    .replacingOccurrences(of: "+", with: "-")
    .replacingOccurrences(of: "/", with: "_")
    .replacingOccurrences(of: "=", with: "")
}

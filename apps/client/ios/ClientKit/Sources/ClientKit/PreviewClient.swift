// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The answer of `getPushPreview` (schema `PushPreview`, docs/spec/13-push-notifications.md
/// 6.4); which members are present depends on `notify.preview_mode`.
///
/// `description` names no value, so a log line never carries content.
public struct PushPreview: Equatable, CustomStringConvertible, CustomDebugStringConvertible {
    /// The kinds of `NotificationKind`.
    public static let kinds: Set<String> = ["message", "mention", "dm", "run_done", "run_failed", "input_needed", "security", "admin"]

    /// The kind, echoed from the request.
    public let kind: String
    /// The badge number at the time of the fetch.
    public let badge: Int
    /// The thread of the message.
    public let threadID: String?
    /// The message.
    public let messageID: String?
    /// The space name (`full`, `names`).
    public let spaceName: String?
    /// The channel name (`full`, `names`).
    public let channelName: String?
    /// The thread title (`full`, `names`).
    public let threadTitle: String?
    /// The author's display name (`full`, `names`).
    public let senderDisplayName: String?
    /// The excerpt (`full`).
    public let excerpt: String?
    /// The number of attachments (`full`).
    public let attachmentCount: Int?
    /// The fetch ticket of the first image's thumbnail (`full`); not used yet (D-M5-04-4).
    public let imageURL: String?

    /// A fixed text without content.
    public var description: String { "PushPreview(<content>)" }

    /// The same as `description`.
    public var debugDescription: String { description }

    /// Decodes a response body strictly against the members of 13 6.4.
    ///
    /// Unknown members are ignored (tolerance for additive changes); a missing or mistyped
    /// required member (`kind`, `badge`), an unknown kind or a mistyped optional member fails.
    ///
    /// - Parameter data: the JSON body.
    /// - Returns: the preview, or nil when the body does not match.
    public static func decode(_ data: Data) -> PushPreview? {
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let kind = object["kind"] as? String, kinds.contains(kind),
              let badge = integer(object["badge"])
        else {
            return nil
        }
        var valid = true
        func text(_ key: String) -> String? {
            guard let value = object[key] else { return nil }
            guard let string = value as? String else {
                valid = false
                return nil
            }
            return string
        }
        func count(_ key: String) -> Int? {
            guard let value = object[key] else { return nil }
            guard let number = integer(value) else {
                valid = false
                return nil
            }
            return number
        }
        let preview = PushPreview(
            kind: kind, badge: badge,
            threadID: text("thread_id"), messageID: text("message_id"),
            spaceName: text("space_name"), channelName: text("channel_name"),
            threadTitle: text("thread_title"), senderDisplayName: text("sender_display_name"),
            excerpt: text("excerpt"), attachmentCount: count("attachment_count"),
            imageURL: text("image_url"))
        return valid ? preview : nil
    }

    /// An integer of a JSON number without a fraction; a boolean is not a number here.
    private static func integer(_ value: Any?) -> Int? {
        guard let number = value as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID() else { return nil }
        let double = number.doubleValue
        guard double.isFinite, double.rounded() == double, abs(double) < 1e15 else { return nil }
        return number.intValue
    }
}

/// The outcome of one preview fetch (brief M5-04 Design note 3).
public enum PreviewResult: Equatable {
    /// The server answered with the preview.
    case content(PushPreview)
    /// `404`: no right, deleted or unknown message; the extension shows "Message not available".
    case notAvailable
    /// Any other status, a network error, a malformed body or no credentials; the placeholder stays.
    case failed
}

/// One HTTP exchange; `URLSession` in the extension, a stub in the tests.
public protocol HTTPTransport {
    /// Sends `request` and returns the answer.
    ///
    /// - Parameter request: the request, with its timeout set.
    /// - Returns: the body and the response.
    /// - Throws: a network error, a timeout or a cancellation.
    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse)
}

/// `HTTPTransport` over an ephemeral `URLSession`: no cookies, no cache, no credential storage,
/// and redirects are refused, so the preview token and the proof never travel to another URL.
public final class URLSessionTransport: NSObject, HTTPTransport, URLSessionTaskDelegate {
    private lazy var session: URLSession = {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpCookieStorage = nil
        configuration.httpShouldSetCookies = false
        configuration.urlCache = nil
        configuration.requestCachePolicy = .reloadIgnoringLocalCacheData
        configuration.urlCredentialStorage = nil
        configuration.waitsForConnectivity = false
        return URLSession(configuration: configuration, delegate: self, delegateQueue: nil)
    }()

    /// Creates the transport.
    override public init() {
        super.init()
    }

    /// See `HTTPTransport.send(_:)`.
    public func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        return (data, http)
    }

    /// Refuses every redirect: the answer of the redirect itself reaches the client as a failure.
    public func urlSession(
        _ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void
    ) {
        completionHandler(nil)
    }
}

/// `GET /api/v1/push/preview` with the preview token and a DPoP proof of the device key
/// (docs/spec/13-push-notifications.md 6.4; 15 10.1; brief M5-04 Design note 3).
///
/// It uses only the preview token of the push registration: never a refresh token, never a
/// login or session endpoint (R-CL-07, 08 7.4). On `401` with `use_dpop_nonce` and a
/// `DPoP-Nonce` header it repeats once with the new nonce; every `DPoP-Nonce` it sees goes to
/// the `NonceCache`. Only an `https` server address is used.
///
/// Example:
///
///     let client = PreviewClient(credentials: stored, signer: signer, nonces: cache,
///                                transport: URLSessionTransport(), timeout: timeout)
///     let result = await client.fetch(nid: nid, messageID: message, kind: "mention")
public final class PreviewClient {
    private let credentials: PushCredentials
    private let signer: ProofSigner
    private let nonces: NonceCache
    private let transport: HTTPTransport
    private let timeout: TimeInterval
    private let now: () -> Date

    /// Creates the client.
    ///
    /// - Parameters:
    ///   - credentials: the preview token and the server address from the shared keychain.
    ///   - signer: the `dpop` device key.
    ///   - nonces: the nonce cache of the App Group.
    ///   - transport: the HTTP exchange.
    ///   - timeout: `client.push.fetch_timeout` in seconds, the timeout of each request.
    ///   - now: the clock of `iat`.
    public init(
        credentials: PushCredentials, signer: ProofSigner, nonces: NonceCache,
        transport: HTTPTransport, timeout: TimeInterval, now: @escaping () -> Date = Date.init
    ) {
        self.credentials = credentials
        self.signer = signer
        self.nonces = nonces
        self.transport = transport
        self.timeout = timeout
        self.now = now
    }

    /// The preview URL of the server address with the query of 13 6.4.
    ///
    /// - Parameters:
    ///   - server: the server address; a base path is kept.
    ///   - nid: the notification ID of the push.
    ///   - messageID: the message of the push.
    ///   - kind: the kind of the push.
    /// - Returns: the URL, or nil for an address that is not `https`.
    public static func previewURL(server: URL, nid: String, messageID: String, kind: String) -> URL? {
        guard var parts = URLComponents(url: server, resolvingAgainstBaseURL: false),
              parts.scheme?.lowercased() == "https", parts.host?.isEmpty == false
        else {
            return nil
        }
        let base = parts.percentEncodedPath.hasSuffix("/") ? String(parts.percentEncodedPath.dropLast()) : parts.percentEncodedPath
        parts.percentEncodedPath = base + "/api/v1/push/preview"
        parts.queryItems = [
            URLQueryItem(name: "nid", value: nid),
            URLQueryItem(name: "message_id", value: messageID),
            URLQueryItem(name: "kind", value: kind),
        ]
        parts.fragment = nil
        return parts.url
    }

    /// Fetches the preview of one push.
    ///
    /// - Parameters:
    ///   - nid: `b.nid` of the payload.
    ///   - messageID: `b.m` of the payload.
    ///   - kind: `b.k` of the payload.
    /// - Returns: the outcome; never throws, every failure is `.failed`.
    public func fetch(nid: String, messageID: String, kind: String) async -> PreviewResult {
        guard let url = Self.previewURL(server: credentials.serverURL, nid: nid, messageID: messageID, kind: kind) else {
            return .failed
        }
        var nonce = nonces.nonce(for: url)
        for attempt in 0..<2 {
            guard let request = makeRequest(url: url, nonce: nonce) else { return .failed }
            let data: Data
            let response: HTTPURLResponse
            do {
                (data, response) = try await transport.send(request)
            } catch {
                return .failed
            }
            let newNonce = response.value(forHTTPHeaderField: "DPoP-Nonce").flatMap { $0.isEmpty ? nil : $0 }
            if let newNonce {
                nonces.remember(newNonce, for: url)
            }
            switch response.statusCode {
            case 200:
                return PushPreview.decode(data).map(PreviewResult.content) ?? .failed
            case 404:
                return .notAvailable
            case 401 where attempt == 0 && Self.asksForNonce(response: response, body: data):
                guard let newNonce, newNonce != nonce else { return .failed }
                nonce = newNonce
            default:
                return .failed
            }
        }
        return .failed
    }

    private func makeRequest(url: URL, nonce: String?) -> URLRequest? {
        guard let proof = try? DPoPProof.make(
            method: "GET", url: url, accessToken: credentials.previewToken, nonce: nonce, signer: signer, now: now())
        else {
            return nil
        }
        var request = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: timeout)
        request.httpMethod = "GET"
        request.httpShouldHandleCookies = false
        request.setValue("DPoP \(credentials.previewToken)", forHTTPHeaderField: "Authorization")
        request.setValue(proof, forHTTPHeaderField: "DPoP")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        return request
    }

    /// Whether a `401` asks for a new nonce: `WWW-Authenticate` with `error="use_dpop_nonce"`
    /// (RFC 9449 8) or the problem code `use_dpop_nonce` in the body.
    static func asksForNonce(response: HTTPURLResponse, body: Data) -> Bool {
        if response.value(forHTTPHeaderField: "WWW-Authenticate")?.contains("use_dpop_nonce") == true {
            return true
        }
        guard let object = try? JSONSerialization.jsonObject(with: body) as? [String: Any] else { return false }
        return object["code"] as? String == "use_dpop_nonce" || object["error"] as? String == "use_dpop_nonce"
    }
}

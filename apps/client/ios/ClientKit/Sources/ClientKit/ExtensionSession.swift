// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The tokens of the Share Extension's child session (`client_component = share_ext`,
/// docs/spec/08-identity.md 7.4; brief M5-06 Design note 3).
///
/// The app creates the child session with `createExtensionSession` and writes these tokens to
/// the shared keychain; the extension refreshes them by itself. `description` names no value,
/// so a log line never carries a token.
public struct ExtensionSessionTokens: Codable, Equatable, CustomStringConvertible, CustomDebugStringConvertible {
    /// The server address (origin and base path) of the instance.
    public let serverURL: URL
    /// The ID of the child session.
    public let sessionID: String
    /// The access token, sent as `Authorization: DPoP <token>`.
    public let accessToken: String
    /// When the access token expires.
    public let accessExpiresAt: Date
    /// The refresh token of the child family; only the extension presents it.
    public let refreshToken: String
    /// When the refresh token expires; after it only a new child session helps.
    public let refreshExpiresAt: Date

    /// Creates the tokens.
    ///
    /// - Parameters:
    ///   - serverURL: the server address.
    ///   - sessionID: the child session.
    ///   - accessToken: the access token.
    ///   - accessExpiresAt: its expiry.
    ///   - refreshToken: the refresh token.
    ///   - refreshExpiresAt: its expiry.
    public init(serverURL: URL, sessionID: String, accessToken: String, accessExpiresAt: Date, refreshToken: String, refreshExpiresAt: Date) {
        self.serverURL = serverURL
        self.sessionID = sessionID
        self.accessToken = accessToken
        self.accessExpiresAt = accessExpiresAt
        self.refreshToken = refreshToken
        self.refreshExpiresAt = refreshExpiresAt
    }

    enum CodingKeys: String, CodingKey {
        case serverURL = "server_url"
        case sessionID = "session_id"
        case accessToken = "access_token"
        case accessExpiresAt = "access_expires_at_ms"
        case refreshToken = "refresh_token"
        case refreshExpiresAt = "refresh_expires_at_ms"
    }

    /// Decodes the keychain JSON: the address as text, the times as Unix milliseconds.
    ///
    /// - Parameter decoder: the decoder.
    /// - Throws: `DecodingError` for a missing or mistyped member or an address that is no URL.
    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let address = try container.decode(String.self, forKey: .serverURL)
        guard let url = URL(string: address) else {
            throw DecodingError.dataCorruptedError(forKey: .serverURL, in: container, debugDescription: "not a URL")
        }
        serverURL = url
        sessionID = try container.decode(String.self, forKey: .sessionID)
        accessToken = try container.decode(String.self, forKey: .accessToken)
        accessExpiresAt = Self.date(millis: try container.decode(Int64.self, forKey: .accessExpiresAt))
        refreshToken = try container.decode(String.self, forKey: .refreshToken)
        refreshExpiresAt = Self.date(millis: try container.decode(Int64.self, forKey: .refreshExpiresAt))
    }

    /// Encodes the keychain JSON of the team contract.
    ///
    /// - Parameter encoder: the encoder.
    /// - Throws: what the encoder throws.
    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(serverURL.absoluteString, forKey: .serverURL)
        try container.encode(sessionID, forKey: .sessionID)
        try container.encode(accessToken, forKey: .accessToken)
        try container.encode(Self.millis(accessExpiresAt), forKey: .accessExpiresAt)
        try container.encode(refreshToken, forKey: .refreshToken)
        try container.encode(Self.millis(refreshExpiresAt), forKey: .refreshExpiresAt)
    }

    /// A fixed text without any value.
    public var description: String { "ExtensionSessionTokens(<redacted>)" }

    /// The same as `description`.
    public var debugDescription: String { description }

    static func millis(_ date: Date) -> Int64 {
        Int64((date.timeIntervalSince1970 * 1000).rounded())
    }

    static func date(millis: Int64) -> Date {
        Date(timeIntervalSince1970: TimeInterval(millis) / 1000)
    }
}

/// The child session's item in the shared keychain (brief M5-06 Design note 3; 15 9).
///
/// One generic password per instance under the service `clientkit`, account
/// `<sha256 hex of the instance ID>.share_session` (the hash of `SharedKeychain.account`), value
/// the JSON of `ExtensionSessionTokens`. One item, so a refresh replaces every field at once and
/// a reader never sees a new access token next to an old refresh token. The app writes it after
/// `createExtensionSession` and deletes it at every end of the account; the extension rewrites it
/// after each refresh and deletes it when the server ends the child session. It is the only item
/// the extension reads: the app's own tokens are never in reach (R-CL-07).
///
/// Example:
///
///     let keychain = ShareSessionKeychain(store: KeychainSecretStore(accessGroup: config.keychainGroup))
///     try keychain.save(tokens, instanceID: instance)
public final class ShareSessionKeychain {
    /// The name part of the account.
    public static let name = "share_session"

    private let store: SecretStore

    /// Creates the keychain view over `store`.
    ///
    /// - Parameter store: the keychain or a memory store.
    public init(store: SecretStore) {
        self.store = store
    }

    /// The account of the item of `instanceID`.
    ///
    /// - Parameter instanceID: the instance.
    /// - Returns: `<sha256 hex>.share_session`.
    public static func account(instanceID: String) -> String {
        SharedKeychain.account(instanceID: instanceID, name: name)
    }

    /// Stores the tokens of `instanceID`, replacing the item.
    ///
    /// - Parameters:
    ///   - tokens: the child session's tokens.
    ///   - instanceID: the instance.
    /// - Throws: `SecretStoreError`, or `EncodingError`.
    public func save(_ tokens: ExtensionSessionTokens, instanceID: String) throws {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
        try store.write(try encoder.encode(tokens), service: SharedKeychain.service, account: Self.account(instanceID: instanceID))
    }

    /// Reads the tokens of `instanceID`.
    ///
    /// - Parameter instanceID: the instance.
    /// - Returns: the tokens, or nil when there is no item or it is not the JSON of the contract
    ///   (then the extension has no session and queues, and the app creates a new child).
    /// - Throws: `SecretStoreError`.
    public func load(instanceID: String) throws -> ExtensionSessionTokens? {
        guard let data = try store.read(service: SharedKeychain.service, account: Self.account(instanceID: instanceID)) else {
            return nil
        }
        return try? JSONDecoder().decode(ExtensionSessionTokens.self, from: data)
    }

    /// Deletes the item of `instanceID`; a missing item is not an error.
    ///
    /// - Parameter instanceID: the instance.
    /// - Throws: `SecretStoreError`.
    public func delete(instanceID: String) throws {
        try store.delete(service: SharedKeychain.service, account: Self.account(instanceID: instanceID))
    }
}

/// Why a share step failed, in the classes of brief M5-06 Design note 4.
public enum ShareError: Error, Equatable {
    /// No usable child session: no item, an expired refresh token or a refresh answered with
    /// `400`. The share goes to the inbox; the app creates a new child at its next start.
    case noSession
    /// The server ended the child session (`401 session_revoked`, or `401` on the refresh); the
    /// item is deleted and the share goes to the inbox.
    case sessionRevoked
    /// No network, a timeout, `5xx`, `429`, `409 request_in_progress` or an answer that does
    /// not match the API; the share goes to the inbox and the app sends it later.
    case transient
    /// Any other `4xx`: shown in the sheet with `status` and the problem `code`; nothing is kept.
    case refused(status: Int, code: String?)
}

/// Sends a request with the credentials of a session: what `UploadClient`, `MessageClient` and
/// `TargetSearch` need, so a test can put a fake behind them.
public protocol AuthorizedRequester {
    /// The server address of the session.
    ///
    /// - Returns: the address (origin and base path).
    /// - Throws: `ShareError.noSession` without a session.
    func serverURL() async throws -> URL

    /// Sends `request` authorized.
    ///
    /// - Parameter request: a request to a URL of the session's server.
    /// - Returns: the body and the response of the final attempt, whatever its status.
    /// - Throws: `ShareError` for the classes the session handles itself.
    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse)
}

/// The Share Extension's own session: authorized requests with the child session's tokens and
/// the refresh of the child family only (docs/spec/08-identity.md 7.4; 15 8; brief M5-06 Design
/// note 3; ID-T16).
///
/// Every request carries `Authorization: DPoP <access>` and a DPoP proof with `ath`, signed with
/// the device key the app's family uses too, and the last nonce of the `NonceCache`. A
/// `401 use_dpop_nonce` with a new nonce repeats once; a `401 token_expired` refreshes once and
/// repeats once. The access token is refreshed before a request when it expires within
/// `refreshMargin`. Refresh is a single flight inside the process: the actor keeps one running
/// refresh task and every caller that needs a refresh meanwhile awaits it, because two refreshes
/// with one refresh token would look like theft to the server and revoke the family. The new
/// tokens are written to the keychain before the new access token is used (the rule of M4-09),
/// so a crash never leaves the only valid refresh token in memory. A `401` on the refresh or a
/// `401 session_revoked` on any call deletes the item. The token never travels to another
/// origin than the session's server, and only over `https` (or to a loopback address,
/// `ShareAPI.isAllowed`).
///
/// Example:
///
///     let session = ExtensionSession(instanceID: instance, keychain: keychain, signer: signer,
///                                    nonces: nonces, transport: URLSessionTransport())
///     let (data, response) = try await session.send(request)
public actor ExtensionSession: AuthorizedRequester {
    /// How long before its expiry the access token is refreshed ahead of a request, so it does
    /// not expire on the way to the server; a protocol constant, not a server limit.
    public static let refreshMargin: TimeInterval = 30

    private let instanceID: String
    private let keychain: ShareSessionKeychain
    private let signer: ProofSigner
    private let nonces: NonceCache
    private let transport: HTTPTransport
    private let now: () -> Date
    private let timeout: TimeInterval
    private var tokens: ExtensionSessionTokens?
    private var loaded = false
    private var refreshing: Task<ExtensionSessionTokens, Error>?
    private var joinObserver: (() -> Void)?

    /// Creates the session of `instanceID`; the keychain is read at the first request.
    ///
    /// - Parameters:
    ///   - instanceID: the instance of the signed-in account.
    ///   - keychain: the child session's item.
    ///   - signer: the `dpop` device key of the instance.
    ///   - nonces: the nonce cache of the App Group.
    ///   - transport: the HTTP exchange.
    ///   - now: the clock of the expiry checks and of `iat`.
    ///   - timeout: the timeout of each request, `ShareEnvironment.requestTimeout`.
    public init(
        instanceID: String, keychain: ShareSessionKeychain, signer: ProofSigner, nonces: NonceCache,
        transport: HTTPTransport, now: @escaping () -> Date = Date.init, timeout: TimeInterval = ShareEnvironment.requestTimeout
    ) {
        self.instanceID = instanceID
        self.keychain = keychain
        self.signer = signer
        self.nonces = nonces
        self.transport = transport
        self.now = now
        self.timeout = timeout
    }

    /// See `AuthorizedRequester.serverURL()`.
    public func serverURL() async throws -> URL {
        try current().serverURL
    }

    /// Sends `request` with the child session's credentials.
    ///
    /// - Parameter request: a request to the session's server; its `Authorization`, `DPoP` and
    ///   timeout are set here.
    /// - Returns: the body and response of the final attempt, any status.
    /// - Throws: `ShareError.noSession`, `.sessionRevoked` or `.transient` (network error,
    ///   timeout, refresh failure); `DPoPProofError.invalidURL` for a URL off the server.
    public func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        var tokens = try await usable()
        guard let url = request.url, ShareAPI.isAllowed(url), ShareAPI.origin(url) == ShareAPI.origin(tokens.serverURL) else {
            throw DPoPProofError.invalidURL
        }
        var nonce = nonces.nonce(for: url)
        var nonceRetried = false
        var refreshed = false
        while true {
            let (data, response) = try await exchange(try authorized(request, url: url, tokens: tokens, nonce: nonce))
            let fresh = remember(response, for: url)
            guard response.statusCode == 401 else { return (data, response) }
            switch ShareAPI.problemCode(data, response: response) ?? "" {
            case "session_revoked":
                end(tokens)
                throw ShareError.sessionRevoked
            case "use_dpop_nonce" where !nonceRetried && fresh != nil && fresh != nonce:
                nonceRetried = true
                nonce = fresh
            case "token_expired" where !refreshed:
                refreshed = true
                tokens = try await refresh(after: tokens.accessToken)
                nonce = nonces.nonce(for: url)
            default:
                return (data, response)
            }
        }
    }

    /// Calls `observer` whenever a caller joins a running refresh instead of starting one; the
    /// single flight test waits for it as an event.
    func observeRefreshJoins(_ observer: @escaping () -> Void) {
        joinObserver = observer
    }

    /// The tokens, loaded from the keychain at the first use.
    private func current() throws -> ExtensionSessionTokens {
        if !loaded {
            // A keychain failure (for example before the first unlock) counts as no session.
            tokens = (try? keychain.load(instanceID: instanceID)) ?? nil
            loaded = true
        }
        guard let tokens, ShareAPI.isAllowed(tokens.serverURL) else { throw ShareError.noSession }
        return tokens
    }

    /// Tokens whose access token is valid beyond `refreshMargin`, refreshing when needed.
    private func usable() async throws -> ExtensionSessionTokens {
        let tokens = try current()
        if isFresh(tokens) { return tokens }
        return try await refresh(after: tokens.accessToken)
    }

    private func isFresh(_ tokens: ExtensionSessionTokens) -> Bool {
        tokens.accessExpiresAt.timeIntervalSince(now()) > Self.refreshMargin
    }

    /// The single flight: joins the running refresh, takes tokens another caller refreshed after
    /// `used` was sent, or starts the one refresh.
    private func refresh(after used: String) async throws -> ExtensionSessionTokens {
        if let running = refreshing {
            joinObserver?()
            return try await running.value
        }
        if let tokens, tokens.accessToken != used, isFresh(tokens) {
            return tokens
        }
        let task = Task { try await self.performRefresh() }
        refreshing = task
        defer { refreshing = nil }
        return try await task.value
    }

    /// One `refreshToken` call of the child family.
    private func performRefresh() async throws -> ExtensionSessionTokens {
        // Read the item again: another extension process may have rotated the family since this
        // one loaded it, and presenting the older refresh token would be reuse; the app may have
        // deleted it at the end of the account.
        guard let base = (try? keychain.load(instanceID: instanceID)) ?? nil, ShareAPI.isAllowed(base.serverURL) else {
            tokens = nil
            throw ShareError.noSession
        }
        if base != tokens, isFresh(base) {
            tokens = base
            return base
        }
        tokens = base
        guard base.refreshExpiresAt > now() else { throw ShareError.noSession }
        guard let url = ShareAPI.url(server: base.serverURL, path: "/auth/token/refresh") else { throw ShareError.noSession }
        var nonce = nonces.nonce(for: url)
        var nonceRetried = false
        while true {
            var request = ShareAPI.jsonRequest("POST", url: url, body: ["refresh_token": base.refreshToken], timeout: timeout)
            request.setValue(
                try DPoPProof.make(method: "POST", url: url, accessToken: nil, nonce: nonce, signer: signer, now: now()),
                forHTTPHeaderField: "DPoP")
            let (data, response) = try await exchange(request)
            let fresh = remember(response, for: url)
            switch response.statusCode {
            case 200:
                guard let next = Self.tokens(from: data, base: base, now: now()) else { throw ShareError.transient }
                // Persist before use: the old refresh token is spent now.
                do {
                    try keychain.save(next, instanceID: instanceID)
                } catch {
                    throw ShareError.transient
                }
                tokens = next
                return next
            case 401 where !nonceRetried && fresh != nil && fresh != nonce
                && ShareAPI.problemCode(data, response: response) == "use_dpop_nonce":
                nonceRetried = true
                nonce = fresh
            case 401:
                end(base)
                throw ShareError.sessionRevoked
            case 400:
                throw ShareError.noSession
            default:
                throw ShareError.transient
            }
        }
    }

    /// The tokens of a `TokenResponse`, or nil when the body does not match it.
    static func tokens(from data: Data, base: ExtensionSessionTokens, now: Date) -> ExtensionSessionTokens? {
        guard let object = ShareAPI.object(data),
              let access = object["access_token"] as? String, !access.isEmpty,
              let expiresIn = (object["expires_in"] as? NSNumber)?.doubleValue,
              let refresh = object["refresh_token"] as? String, !refresh.isEmpty,
              let refreshExpiry = (object["refresh_expires_at"] as? String).flatMap(ShareAPI.timestamp)
        else {
            return nil
        }
        return ExtensionSessionTokens(
            serverURL: base.serverURL, sessionID: base.sessionID, accessToken: access,
            accessExpiresAt: now.addingTimeInterval(expiresIn), refreshToken: refresh, refreshExpiresAt: refreshExpiry)
    }

    /// Deletes the item: the server ended the child session `ended`. When the app has written a
    /// new child meanwhile, its item stays and the next request loads it.
    private func end(_ ended: ExtensionSessionTokens) {
        let stored = (try? keychain.load(instanceID: instanceID)) ?? nil
        if stored == nil || stored?.sessionID == ended.sessionID {
            try? keychain.delete(instanceID: instanceID)
            loaded = true
        } else {
            loaded = false
        }
        tokens = nil
    }

    private func authorized(_ request: URLRequest, url: URL, tokens: ExtensionSessionTokens, nonce: String?) throws -> URLRequest {
        var out = request
        out.timeoutInterval = timeout
        out.httpShouldHandleCookies = false
        out.cachePolicy = .reloadIgnoringLocalCacheData
        let proof = try DPoPProof.make(
            method: request.httpMethod ?? "GET", url: url, accessToken: tokens.accessToken, nonce: nonce, signer: signer, now: now())
        out.setValue("DPoP \(tokens.accessToken)", forHTTPHeaderField: "Authorization")
        out.setValue(proof, forHTTPHeaderField: "DPoP")
        if out.value(forHTTPHeaderField: "Accept") == nil {
            out.setValue("application/json", forHTTPHeaderField: "Accept")
        }
        return out
    }

    /// Keeps a `DPoP-Nonce` of the answer.
    ///
    /// - Returns: the nonce, or nil when the answer had none.
    private func remember(_ response: HTTPURLResponse, for url: URL) -> String? {
        guard let nonce = response.value(forHTTPHeaderField: "DPoP-Nonce"), !nonce.isEmpty else { return nil }
        nonces.remember(nonce, for: url)
        return nonce
    }

    private func exchange(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        try await ShareAPI.exchange(request, over: transport)
    }
}

/// The small HTTP helpers every share client uses: URLs under `/api/v1`, JSON requests, problem
/// codes and the failure classes of brief M5-06 Design note 4, in one place.
enum ShareAPI {
    /// The URL of `path` under `/api/v1` of `server`, keeping a base path of the address.
    ///
    /// - Parameters:
    ///   - server: the server address.
    ///   - path: a percent-encoded path starting with `/`.
    ///   - query: query items, or empty.
    /// - Returns: the URL, or nil for an address that is no URL with a host.
    static func url(server: URL, path: String, query: [URLQueryItem] = []) -> URL? {
        guard var parts = URLComponents(url: server, resolvingAgainstBaseURL: false), parts.host?.isEmpty == false else { return nil }
        let base = parts.percentEncodedPath.hasSuffix("/") ? String(parts.percentEncodedPath.dropLast()) : parts.percentEncodedPath
        parts.percentEncodedPath = base + "/api/v1" + path
        parts.queryItems = query.isEmpty ? nil : query
        // `URLComponents` leaves `+` as is, and the server reads a plain `+` in a query as a space.
        parts.percentEncodedQuery = parts.percentEncodedQuery?.replacingOccurrences(of: "+", with: "%2B")
        parts.fragment = nil
        return parts.url
    }

    /// One path segment, percent-encoded, so an ID can never add a segment.
    static func segment(_ value: String) -> String {
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/;?#")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? ""
    }

    /// A request with a compact JSON body.
    static func jsonRequest(_ method: String, url: URL, body: [String: Any]?, idempotencyKey: String? = nil, timeout: TimeInterval = ShareEnvironment.requestTimeout) -> URLRequest {
        var request = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: timeout)
        request.httpMethod = method
        request.httpShouldHandleCookies = false
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            request.httpBody = try? JSONSerialization.data(withJSONObject: body, options: [.sortedKeys, .withoutEscapingSlashes])
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        if let idempotencyKey {
            request.setValue(idempotencyKey, forHTTPHeaderField: "Idempotency-Key")
        }
        return request
    }

    /// Sends over `transport`; every failure of the exchange itself (no network, a timeout, a
    /// cancellation) is `ShareError.transient`.
    static func exchange(_ request: URLRequest, over transport: HTTPTransport) async throws -> (Data, HTTPURLResponse) {
        do {
            return try await transport.send(request)
        } catch {
            throw ShareError.transient
        }
    }

    /// Throws the class of a non-2xx answer; returns for `2xx`.
    ///
    /// - Throws: `ShareError.transient` for `5xx`, `429`, `409 request_in_progress`, `401`
    ///   (what the session could not mend) and anything outside `4xx`; `.refused` for every
    ///   other `4xx`.
    static func check(_ response: HTTPURLResponse, _ data: Data) throws {
        let status = response.statusCode
        if (200..<300).contains(status) { return }
        let code = problemCode(data, response: response)
        switch status {
        case 401, 429:
            throw ShareError.transient
        case 409 where code == "request_in_progress":
            throw ShareError.transient
        case 400..<500:
            throw ShareError.refused(status: status, code: code)
        default:
            throw ShareError.transient
        }
    }

    /// The problem `code` of an answer (RFC 9457 body), else the `error` of `WWW-Authenticate`.
    static func problemCode(_ data: Data, response: HTTPURLResponse) -> String? {
        if let code = object(data)?["code"] as? String { return code }
        guard let header = response.value(forHTTPHeaderField: "WWW-Authenticate"),
              let start = header.range(of: "error=\"")
        else {
            return nil
        }
        let rest = header[start.upperBound...]
        return rest.firstIndex(of: "\"").map { String(rest[..<$0]) }
    }

    /// The JSON object of a body, or nil.
    static func object(_ data: Data) -> [String: Any]? {
        (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
    }

    /// An RFC 3339 timestamp of the API, with or without fractional seconds.
    static func timestamp(_ text: String) -> Date? {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = formatter.date(from: text) { return date }
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.date(from: text)
    }

    /// The normalized origin of `url` (`scheme://host[:port]`), or nil.
    static func origin(_ url: URL) -> String? {
        try? DPoPProof.normalizeHtu(url.deletingAllPath())
    }

    /// Whether the extension may send to `url`: `https`, or plain `http` to a loopback address
    /// (`127.0.0.1`, `::1`, `localhost`), which no network can reach and where the fake server
    /// of the device tests runs; the app writes a plain address only from its test entry.
    static func isAllowed(_ url: URL) -> Bool {
        guard let host = url.host?.lowercased(), !host.isEmpty else { return false }
        switch url.scheme?.lowercased() {
        case "https":
            return true
        case "http":
            return ["127.0.0.1", "::1", "[::1]", "localhost"].contains(host)
        default:
            return false
        }
    }
}

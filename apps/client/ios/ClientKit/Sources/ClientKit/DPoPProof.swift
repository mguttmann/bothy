// SPDX-License-Identifier: Apache-2.0

import CryptoKit
import DeviceKeyCore
import Foundation
import Security

/// The signer of a DPoP proof: the public key as JWK members and ES256 signatures in the JOSE
/// form `R || S`.
public protocol ProofSigner {
    /// The x coordinate of the P-256 public key, 32 bytes big-endian.
    var x: Data { get }
    /// The y coordinate of the P-256 public key, 32 bytes big-endian.
    var y: Data { get }

    /// Signs `data` with ES256.
    ///
    /// - Parameter data: the JWS signing input.
    /// - Returns: 64 bytes, `R || S`.
    /// - Throws: when the key cannot sign.
    func sign(_ data: Data) throws -> Data
}

/// The `dpop` device key of an instance through `DeviceKeyCore`, without user confirmation
/// (docs/spec/08-identity.md 6.1; 13 5.1).
///
/// It only opens an existing key and never creates one: a key that is missing in the extension
/// means the app has not registered this device, and a new key would not match the session.
public struct DeviceKeyProofSigner: ProofSigner {
    /// See `ProofSigner.x`.
    public let x: Data
    /// See `ProofSigner.y`.
    public let y: Data
    private let manager: DeviceKeyManager
    private let scope: String

    /// Opens the `dpop` key of `scope`.
    ///
    /// - Parameters:
    ///   - manager: the device key manager over the shared access group.
    ///   - scope: the instance ID, the scope of the app's device keys.
    /// - Returns: nil when the key does not exist or cannot be read.
    public init?(manager: DeviceKeyManager, scope: String) {
        guard let info = try? manager.keyInfo(scope: scope, purpose: .dpop) else { return nil }
        self.x = info.x
        self.y = info.y
        self.manager = manager
        self.scope = scope
    }

    /// See `ProofSigner.sign(_:)`.
    public func sign(_ data: Data) throws -> Data {
        try manager.sign(scope: scope, purpose: .dpop, data: data)
    }
}

/// An error of the proof itself; the request is then not sent.
public enum DPoPProofError: Error, Equatable {
    /// The URL is not an absolute `http` or `https` URL with a host.
    case invalidURL
    /// The signer returned no 64-byte `R || S` signature.
    case invalidSignature
}

/// DPoP proofs after RFC 9449 in the profile of the Dart `DpopProofFactory` (docs/spec/
/// 08-identity.md 7.3; brief M4-09 Design note 2; brief M5-04 Design note 2).
///
/// Header members `typ` (`dpop+jwt`), `alg` (`ES256`) and `jwk` (`crv`, `kty`, `x`, `y`) in
/// this order; payload `jti`, `htm`, `htu`, `iat`, `ath` when a token is sent and `nonce` when
/// one is known, in this order; JSON without whitespace with the escaping of Dart's
/// `jsonEncode`. A contract test holds the signing input equal to the Dart output for the same
/// inputs (`Tests/ClientKitTests/Fixtures/dpop_vector.json`).
///
/// Example:
///
///     let proof = try DPoPProof.make(method: "GET", url: url, accessToken: token,
///                                    nonce: nonces.nonce(for: url), signer: signer)
public enum DPoPProof {
    /// Builds a compact JWS proof.
    ///
    /// - Parameters:
    ///   - method: the HTTP method; upper-cased into `htm`.
    ///   - url: the request URL; normalized into `htu` (`normalizeHtu`).
    ///   - accessToken: adds `ath`, the base64url SHA-256 of the token.
    ///   - nonce: the server's last `DPoP-Nonce`, or nil.
    ///   - signer: the device key.
    ///   - now: the time of `iat` (whole seconds).
    ///   - jti: the `jti`; a fresh UUIDv7 by default.
    /// - Returns: `header.payload.signature` in base64url without padding.
    /// - Throws: `DPoPProofError`, or what the signer throws.
    public static func make(
        method: String,
        url: URL,
        accessToken: String?,
        nonce: String?,
        signer: ProofSigner,
        now: Date = Date(),
        jti: String = UUIDv7.make()
    ) throws -> String {
        let input = try signingInput(
            method: method, url: url, accessToken: accessToken, nonce: nonce,
            x: signer.x, y: signer.y, now: now, jti: jti)
        let signature = try signer.sign(Data(input.utf8))
        guard signature.count == 64 else { throw DPoPProofError.invalidSignature }
        return "\(input).\(base64URL(signature))"
    }

    /// The JWS signing input `BASE64URL(header) || '.' || BASE64URL(payload)`.
    ///
    /// - Parameters: as `make`, with the public key as `x` and `y`.
    /// - Returns: the ASCII signing input.
    /// - Throws: `DPoPProofError.invalidURL`.
    static func signingInput(
        method: String, url: URL, accessToken: String?, nonce: String?,
        x: Data, y: Data, now: Date, jti: String
    ) throws -> String {
        let jwk: [(String, JSONValue)] = [
            ("crv", .string("P-256")), ("kty", .string("EC")),
            ("x", .string(base64URL(x))), ("y", .string(base64URL(y))),
        ]
        let header: [(String, JSONValue)] = [
            ("typ", .string("dpop+jwt")), ("alg", .string("ES256")), ("jwk", .object(jwk)),
        ]
        var payload: [(String, JSONValue)] = [
            ("jti", .string(jti)),
            ("htm", .string(method.uppercased())),
            ("htu", .string(try normalizeHtu(url))),
            ("iat", .integer(Int64(now.timeIntervalSince1970.rounded(.down)))),
        ]
        if let accessToken {
            payload.append(("ath", .string(accessTokenHash(accessToken))))
        }
        if let nonce {
            payload.append(("nonce", .string(nonce)))
        }
        let encodedHeader = base64URL(Data(JSONValue.object(header).encoded.utf8))
        let encodedPayload = base64URL(Data(JSONValue.object(payload).encoded.utf8))
        return "\(encodedHeader).\(encodedPayload)"
    }

    /// The `htu` form of `url` (RFC 9449 4.2; 08 7.3), as the Dart `normalizeHtu`: scheme and
    /// host in lower case, the default port removed (443 for `https`, 80 for `http`), query and
    /// fragment dropped, the path kept as sent (`/` when empty), an IPv6 host in brackets.
    ///
    /// - Parameter url: an absolute `http` or `https` URL.
    /// - Returns: the normalized URL text.
    /// - Throws: `DPoPProofError.invalidURL` for another scheme or a URL without a host.
    public static func normalizeHtu(_ url: URL) throws -> String {
        guard let parts = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let scheme = parts.scheme?.lowercased(), scheme == "https" || scheme == "http",
              let host = parts.host?.lowercased(), !host.isEmpty
        else {
            throw DPoPProofError.invalidURL
        }
        // Foundation may report an IPv6 host with or without its brackets; the form is bracketed.
        let bare = host.hasPrefix("[") && host.hasSuffix("]") ? String(host.dropFirst().dropLast()) : host
        let authority = bare.contains(":") ? "[\(bare)]" : bare
        let defaultPort = scheme == "https" ? 443 : 80
        let port = parts.port.map { $0 == defaultPort ? "" : ":\($0)" } ?? ""
        let path = parts.percentEncodedPath.isEmpty ? "/" : parts.percentEncodedPath
        return "\(scheme)://\(authority)\(port)\(path)"
    }

    /// The `ath` value of RFC 9449 4.2: base64url of the SHA-256 of the token's UTF-8 bytes.
    ///
    /// - Parameter accessToken: the token sent in `Authorization`.
    /// - Returns: the hash in base64url without padding.
    public static func accessTokenHash(_ accessToken: String) -> String {
        base64URL(Data(SHA256.hash(data: Data(accessToken.utf8))))
    }

    /// Base64url without padding (RFC 7515 2).
    ///
    /// - Parameter data: the bytes.
    /// - Returns: the encoded text.
    public static func base64URL(_ data: Data) -> String {
        data.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

/// A JSON value with ordered object members, encoded like Dart's `jsonEncode`: no whitespace,
/// `"` and `\` escaped, control characters as `\b`, `\t`, `\n`, `\f`, `\r` or `\u00xx`
/// (lower-case hex), everything else (also `/` and non-ASCII) as is.
enum JSONValue {
    case string(String)
    case integer(Int64)
    case object([(String, JSONValue)])

    /// The compact JSON text.
    var encoded: String {
        switch self {
        case let .string(text):
            return Self.quote(text)
        case let .integer(value):
            return String(value)
        case let .object(members):
            return "{" + members.map { "\(Self.quote($0.0)):\($0.1.encoded)" }.joined(separator: ",") + "}"
        }
    }

    private static func quote(_ text: String) -> String {
        var out = "\""
        for scalar in text.unicodeScalars {
            switch scalar {
            case "\"": out += "\\\""
            case "\\": out += "\\\\"
            case "\u{08}": out += "\\b"
            case "\u{09}": out += "\\t"
            case "\u{0A}": out += "\\n"
            case "\u{0C}": out += "\\f"
            case "\u{0D}": out += "\\r"
            default:
                if scalar.value < 0x20 {
                    out += String(format: "\\u%04x", scalar.value)
                } else {
                    out.unicodeScalars.append(scalar)
                }
            }
        }
        return out + "\""
    }
}

/// UUIDv7 strings (RFC 9562 5.7) for the `jti` of a proof: 48 bits of Unix milliseconds, the
/// version and variant bits, the rest random.
public enum UUIDv7 {
    /// A new UUIDv7 in the canonical lower-case form.
    ///
    /// - Parameter now: the time of the timestamp field.
    /// - Returns: for example `0192f4a0-7b1c-7848-8501-1716705c7dbe`.
    public static func make(now: Date = Date()) -> String {
        var bytes = [UInt8](repeating: 0, count: 16)
        // A failure of the system random source leaves zeros; the timestamp keeps the value
        // unique enough for a `jti`, and a proof is never retried with the same `jti` anyway.
        _ = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        let millis = UInt64(max(0, now.timeIntervalSince1970 * 1000))
        for i in 0..<6 {
            bytes[i] = UInt8((millis >> (8 * (5 - UInt64(i)))) & 0xff)
        }
        bytes[6] = (bytes[6] & 0x0f) | 0x70
        bytes[8] = (bytes[8] & 0x3f) | 0x80
        let hex = bytes.map { String(format: "%02x", $0) }.joined()
        let chars = Array(hex)
        return [0..<8, 8..<12, 12..<16, 16..<20, 20..<32].map { String(chars[$0]) }.joined(separator: "-")
    }
}

/// The last `DPoP-Nonce` per server in the App Group (08 7.3; brief M5-04 Design note 2).
///
/// A nonce is no secret and expires by itself, so it may sit in the shared container; the
/// extension adopts the newest one from every answer and starts the next fetch with it, which
/// saves the `use_dpop_nonce` round trip within the extension's short time.
public final class NonceCache {
    private let store: KeyValueStore

    /// Creates the cache over the App Group store.
    ///
    /// - Parameter store: the store of `SharedState`.
    public init(store: KeyValueStore) {
        self.store = store
    }

    /// The key of `server`: the SHA-256 of its origin, so no address sits in the container.
    static func key(for server: URL) -> String {
        let origin = (try? DPoPProof.normalizeHtu(server.deletingAllPath())) ?? server.absoluteString
        let digest = SHA256.hash(data: Data(origin.utf8)).map { String(format: "%02x", $0) }.joined()
        return "\(SharedState.prefix)nonce.\(digest)"
    }

    /// The last nonce of `server`, or nil.
    ///
    /// - Parameter server: any URL of the server.
    /// - Returns: the nonce.
    public func nonce(for server: URL) -> String? {
        store.value(forKey: Self.key(for: server)) as? String
    }

    /// Keeps `nonce` as the newest of `server`.
    ///
    /// - Parameters:
    ///   - nonce: the value of a `DPoP-Nonce` header.
    ///   - server: any URL of the server.
    public func remember(_ nonce: String, for server: URL) {
        store.set(nonce, forKey: Self.key(for: server))
    }
}

extension URL {
    /// The URL with an empty path, query and fragment: the origin.
    func deletingAllPath() -> URL {
        guard var parts = URLComponents(url: self, resolvingAgainstBaseURL: false) else { return self }
        parts.percentEncodedPath = ""
        parts.query = nil
        parts.fragment = nil
        return parts.url ?? self
    }
}

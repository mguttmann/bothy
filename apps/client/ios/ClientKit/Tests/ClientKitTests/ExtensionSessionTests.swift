// SPDX-License-Identifier: Apache-2.0
//
// The child session of the Share Extension (brief M5-06 Design note 3; docs/spec/08-identity.md
// 7.4; 15 8): the keychain item, authorized requests, the refresh of the child family only, the
// single flight and the end of the session.

import Foundation
import Testing

@testable import ClientKit

private let spacesURL = URL(string: "https://chat.example.com/api/v1/spaces")!
private let shareAccount = SharedKeychain.account(instanceID: instanceID, name: "share_session")

@Test func testShareSessionKeychainRoundTrip() throws {
    let store = MemorySecretStore()
    let keychain = ShareSessionKeychain(store: store)
    let clock = TestClock()
    #expect(try keychain.load(instanceID: instanceID) == nil)
    let tokens = childTokens(clock)
    try keychain.save(tokens, instanceID: instanceID)
    #expect(try keychain.load(instanceID: instanceID) == tokens)
    #expect(try keychain.load(instanceID: "other") == nil)

    // One item, the members of the contract, times in milliseconds.
    #expect(store.items.count == 1)
    let raw = try #require(try store.read(service: SharedKeychain.service, account: shareAccount))
    let object = try #require(try JSONSerialization.jsonObject(with: raw) as? [String: Any])
    #expect(Set(object.keys) == ["server_url", "session_id", "access_token", "access_expires_at_ms", "refresh_token", "refresh_expires_at_ms"])
    #expect(object["server_url"] as? String == "https://chat.example.com")
    #expect((object["access_expires_at_ms"] as? NSNumber)?.int64Value == Int64(clock.now().timeIntervalSince1970 * 1000) + 900_000)
    #expect(!shareAccount.contains(instanceID))

    // A new child replaces the old one; the description names no token.
    let next = childTokens(clock, access: "child_access_EXAMPLE_0009", refresh: "child_refresh_EXAMPLE_0009")
    try keychain.save(next, instanceID: instanceID)
    #expect(try keychain.load(instanceID: instanceID) == next)
    #expect(!"\(next)".contains("child_") && !String(reflecting: next).contains("child_"))
    try keychain.delete(instanceID: instanceID)
    #expect(try keychain.load(instanceID: instanceID) == nil)

    // A damaged item reads as no session.
    try store.write(Data("not json".utf8), service: SharedKeychain.service, account: shareAccount)
    #expect(try keychain.load(instanceID: instanceID) == nil)
}

@Test func testExtensionRefreshesOnlyChildFamily() async throws {
    let world = ShareWorld(accessIn: -1)
    // The app's own items sit next to the child item; the extension must never read them.
    try world.secrets.inner.write(Data("app_refresh_SECRET".utf8), service: SharedKeychain.service,
                                  account: SharedKeychain.account(instanceID: instanceID, name: "refresh_token"))
    try world.secrets.inner.write(Data("bothy_pvw_SECRET".utf8), service: SharedKeychain.service,
                                  account: SharedKeychain.account(instanceID: instanceID, name: "preview_token"))
    var persistedBeforeUse = false
    world.chat.override = { request in
        if request.url == spacesURL, request.value(forHTTPHeaderField: "Authorization") == "DPoP child_access_EXAMPLE_0002" {
            persistedBeforeUse = (try? world.keychain.load(instanceID: instanceID))?.refreshToken == "child_refresh_EXAMPLE_0002"
        }
        return nil
    }

    let (_, response) = try await world.session().send(URLRequest(url: spacesURL))
    #expect(response.statusCode == 200)

    let refresh = try #require(world.chat.refreshes.first)
    #expect(world.chat.refreshes.count == 1)
    #expect(body(refresh)["refresh_token"] as? String == childRefresh)
    #expect(refresh.value(forHTTPHeaderField: "Authorization") == nil)
    let payload = proofPayload(refresh)
    #expect(payload["ath"] == nil)
    #expect(payload["htm"] as? String == "POST")
    #expect(payload["htu"] as? String == "https://chat.example.com/api/v1/auth/token/refresh")
    // The new refresh token was in the keychain before the new access token was used.
    #expect(persistedBeforeUse)
    // Only the child item was ever read, written or deleted.
    #expect(!world.secrets.reads.isEmpty)
    #expect(world.secrets.all.allSatisfy { $0 == shareAccount })
    for request in world.chat.requests {
        let text = "\(request.allHTTPHeaderFields ?? [:])\(String(decoding: request.httpBody ?? Data(), as: UTF8.self))"
        #expect(!text.contains("SECRET"))
    }
    // The authorized request carries the new token and a proof with its `ath`.
    let used = try #require(world.chat.requests("GET", "/api/v1/spaces").last)
    #expect(proofPayload(used)["ath"] as? String == DPoPProof.accessTokenHash("child_access_EXAMPLE_0002"))
}

@Test func testSingleFlightRefresh() async throws {
    let world = ShareWorld(accessIn: -1)
    let arrived = Latch()
    let release = Latch()
    let joined = Latch()
    var refreshCount = 0
    world.chat.onRefresh = { _ in
        refreshCount += 1
        arrived.open()
        // A second refresh would be the failure under test: let the test go on and see it.
        if refreshCount > 1 { joined.open() }
        await release.wait()
    }
    let session = world.session()
    await session.observeRefreshJoins { joined.open() }

    async let first = session.send(URLRequest(url: spacesURL))
    await arrived.wait()
    async let second = session.send(URLRequest(url: spacesURL))
    await joined.wait()
    release.open()
    let (a, b) = try await (first, second)

    #expect(a.1.statusCode == 200 && b.1.statusCode == 200)
    #expect(world.chat.refreshes.count == 1)
    let authorizations = world.chat.requests("GET", "/api/v1/spaces").map { $0.value(forHTTPHeaderField: "Authorization") }
    #expect(authorizations == ["DPoP child_access_EXAMPLE_0002", "DPoP child_access_EXAMPLE_0002"])
}

@Test func testProactiveAndReactiveRefresh() async throws {
    // Shortly before the expiry the session refreshes before the request.
    let soon = ShareWorld(accessIn: 5)
    _ = try await soon.session().send(URLRequest(url: spacesURL))
    #expect(soon.chat.refreshes.count == 1)
    #expect(soon.chat.requests("GET", "/api/v1/spaces").count == 1)

    // Valid by the clock, expired on the server: `401 token_expired` refreshes once and repeats.
    let world = ShareWorld()
    world.chat.expireAccess()
    let session = world.session()
    let (_, response) = try await session.send(URLRequest(url: spacesURL))
    #expect(response.statusCode == 200)
    #expect(world.chat.refreshes.count == 1)
    #expect(world.chat.requests("GET", "/api/v1/spaces").count == 2)

    // A second `token_expired` after the refresh is returned, not refreshed again.
    world.chat.expireAccess()
    world.chat.override = { request in request.url == spacesURL ? problem(401, "token_expired") : nil }
    let (_, again) = try await session.send(URLRequest(url: spacesURL))
    #expect(again.statusCode == 401)
    #expect(world.chat.refreshes.count == 2)
}

@Test func testNonceRetryOnceAndCached() async throws {
    let world = ShareWorld()
    var asked = false
    world.chat.override = { request in
        guard request.url == spacesURL, !asked else { return nil }
        asked = true
        var reply = problem(401, "use_dpop_nonce")
        reply.headers["DPoP-Nonce"] = "fresh-nonce"
        reply.headers["WWW-Authenticate"] = "DPoP error=\"use_dpop_nonce\""
        return reply
    }
    let (_, response) = try await world.session().send(URLRequest(url: spacesURL))
    #expect(response.statusCode == 200)
    let sent = world.chat.requests("GET", "/api/v1/spaces")
    #expect(sent.count == 2)
    #expect(proofPayload(sent[0])["nonce"] == nil)
    #expect(proofPayload(sent[1])["nonce"] as? String == "fresh-nonce")
    #expect(world.nonces.nonce(for: server) == "fresh-nonce")
}

@Test func testSessionEndsOnRevocation() async throws {
    // `401 session_revoked` on any call deletes the child item.
    let revoked = ShareWorld()
    revoked.chat.override = { _ in problem(401, "session_revoked") }
    await #expect(throws: ShareError.sessionRevoked) { try await revoked.session().send(URLRequest(url: spacesURL)) }
    #expect(try revoked.keychain.load(instanceID: instanceID) == nil)

    // The app replaced the child meanwhile: the old session's end leaves the new item alone.
    let replaced = ShareWorld()
    let session = replaced.session()
    _ = try await session.send(URLRequest(url: spacesURL))
    let newer = ExtensionSessionTokens(
        serverURL: server, sessionID: "0192f4a0-7b1c-75c3-8a09-bf22e1242000", accessToken: "child_access_EXAMPLE_0100",
        accessExpiresAt: replaced.clock.now().addingTimeInterval(900), refreshToken: "child_refresh_EXAMPLE_0100",
        refreshExpiresAt: replaced.clock.now().addingTimeInterval(86_400))
    try replaced.keychain.save(newer, instanceID: instanceID)
    replaced.chat.override = { request in
        request.value(forHTTPHeaderField: "Authorization") == "DPoP \(childAccess)" ? problem(401, "session_revoked") : nil
    }
    await #expect(throws: ShareError.sessionRevoked) { try await session.send(URLRequest(url: spacesURL)) }
    #expect(try replaced.keychain.load(instanceID: instanceID) == newer)

    // A refresh answered with 401 deletes it as well.
    let rejected = ShareWorld(accessIn: -1)
    rejected.chat.override = { request in request.url?.path == "/api/v1/auth/token/refresh" ? problem(401, "unauthenticated") : nil }
    await #expect(throws: ShareError.sessionRevoked) { try await rejected.session().send(URLRequest(url: spacesURL)) }
    #expect(try rejected.keychain.load(instanceID: instanceID) == nil)

    // A refresh answered with 400 keeps the item for the app; the share goes to the inbox.
    let invalid = ShareWorld(accessIn: -1)
    invalid.chat.override = { request in request.url?.path == "/api/v1/auth/token/refresh" ? problem(400, "validation_failed") : nil }
    await #expect(throws: ShareError.noSession) { try await invalid.session().send(URLRequest(url: spacesURL)) }
    #expect(try invalid.keychain.load(instanceID: instanceID) != nil)

    // No item: no session, and nothing is sent.
    let missing = ShareWorld(session: false)
    await #expect(throws: ShareError.noSession) { try await missing.session().send(URLRequest(url: spacesURL)) }
    #expect(missing.chat.requests.isEmpty)

    // A refresh token past its expiry is not presented.
    let stale = ShareWorld(accessIn: -1)
    stale.clock.advance(86_400)
    await #expect(throws: ShareError.noSession) { try await stale.session().send(URLRequest(url: spacesURL)) }
    #expect(stale.chat.requests.isEmpty)

    // A network error or a 5xx of the refresh is transient and keeps the item.
    let offline = ShareWorld(accessIn: -1)
    offline.chat.override = { _ in throw URLError(.notConnectedToInternet) }
    await #expect(throws: ShareError.transient) { try await offline.session().send(URLRequest(url: spacesURL)) }
    #expect(try offline.keychain.load(instanceID: instanceID) != nil)
}

@Test func testTokenNeverLeavesTheServerOrigin() async throws {
    let world = ShareWorld()
    let foreign = URL(string: "https://objects.example.com/api/v1/spaces")!
    await #expect(throws: DPoPProofError.invalidURL) { try await world.session().send(URLRequest(url: foreign)) }
    #expect(world.chat.requests.isEmpty)
}

/// The extension talks `https`, and plain `http` only to a loopback address, which no network
/// can reach: the fake server of the device tests runs there (docs/decisions.d/M5-06.md).
@Test func testOnlyHttpsOrLoopback() {
    #expect(ShareAPI.isAllowed(URL(string: "https://chat.example.com/api/v1")!))
    #expect(ShareAPI.isAllowed(URL(string: "http://127.0.0.1:8080")!))
    #expect(ShareAPI.isAllowed(URL(string: "http://[::1]:9000/x")!))
    #expect(ShareAPI.isAllowed(URL(string: "http://localhost:9000")!))
    #expect(!ShareAPI.isAllowed(URL(string: "http://chat.example.com")!))
    #expect(!ShareAPI.isAllowed(URL(string: "http://127.0.0.1.example.com")!))
    #expect(!ShareAPI.isAllowed(URL(string: "ftp://127.0.0.1")!))
    #expect(!ShareAPI.isAllowed(URL(string: "https:///nohost")!))
}

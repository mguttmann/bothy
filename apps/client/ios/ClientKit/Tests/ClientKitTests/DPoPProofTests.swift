// SPDX-License-Identifier: Apache-2.0
//
// DPoP proofs in the profile of the Dart client (brief M5-04 Design note 2; 08 7.3).

import CryptoKit
import Foundation
import Testing

@testable import ClientKit

private struct Vector: Decodable {
    let method: String
    let url: String
    let access_token: String
    let nonce: String
    let iat: Int
    let x: String
    let y: String
    let jti: String
    let signing_input: String
}

private func base64URLDecode(_ text: String) -> Data? {
    var base = text.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
    base += String(repeating: "=", count: (4 - base.count % 4) % 4)
    return Data(base64Encoded: base)
}

/// The key of the vector: the P-256 generator point, private scalar 1.
private let vectorKey = try! P256.Signing.PrivateKey(rawRepresentation: Data(repeating: 0, count: 31) + Data([1]))

@Test func testProofMatchesDartProfile() throws {
    let file = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("Fixtures/dpop_vector.json")
    let vector = try JSONDecoder().decode(Vector.self, from: Data(contentsOf: file))
    let signer = TestSigner(signing: vectorKey)
    #expect(DPoPProof.base64URL(signer.x) == vector.x)
    #expect(DPoPProof.base64URL(signer.y) == vector.y)
    let proof = try DPoPProof.make(
        method: vector.method, url: URL(string: vector.url)!, accessToken: vector.access_token,
        nonce: vector.nonce, signer: signer, now: Date(timeIntervalSince1970: TimeInterval(vector.iat)), jti: vector.jti)
    let parts = proof.split(separator: ".").map(String.init)
    #expect(parts.count == 3)
    // The signing input equals the Dart one byte for byte.
    #expect("\(parts[0]).\(parts[1])" == vector.signing_input)
    // The signature is R || S and verifies with the public key.
    let signature = try P256.Signing.ECDSASignature(rawRepresentation: #require(base64URLDecode(parts[2])))
    #expect(vectorKey.publicKey.isValidSignature(signature, for: Data(vector.signing_input.utf8)))
}

@Test func testHtuNormalization() throws {
    #expect(try DPoPProof.normalizeHtu(URL(string: "HTTPS://Chat.Example.com:443/api/v1/threads?x=1#y")!) == "https://chat.example.com/api/v1/threads")
    #expect(try DPoPProof.normalizeHtu(URL(string: "http://chat.example.com:80")!) == "http://chat.example.com/")
    #expect(try DPoPProof.normalizeHtu(URL(string: "https://chat.example.com:8443/a/b")!) == "https://chat.example.com:8443/a/b")
    #expect(try DPoPProof.normalizeHtu(URL(string: "https://[::1]:8443/x")!) == "https://[::1]:8443/x")
    #expect(throws: DPoPProofError.invalidURL) { try DPoPProof.normalizeHtu(URL(string: "ftp://chat.example.com/x")!) }
    #expect(throws: DPoPProofError.invalidURL) { try DPoPProof.normalizeHtu(URL(string: "file:///x")!) }
}

@Test func testAthIsHashOfPreviewToken() throws {
    let signer = TestSigner()
    let proof = try DPoPProof.make(method: "GET", url: server, accessToken: previewToken, nonce: nil, signer: signer)
    let payload = try #require(base64URLDecode(String(proof.split(separator: ".")[1])))
    let object = try #require(try JSONSerialization.jsonObject(with: payload) as? [String: Any])
    let expected = DPoPProof.base64URL(Data(SHA256.hash(data: Data(previewToken.utf8))))
    #expect(object["ath"] as? String == expected)
    #expect(object["nonce"] == nil)
    #expect(object["htm"] as? String == "GET")
    // A fresh UUIDv7 `jti` per proof.
    let jti = try #require(object["jti"] as? String)
    #expect(jti.count == 36 && jti[jti.index(jti.startIndex, offsetBy: 14)] == "7")
}

@Test func testJsonEscapingLikeDart() {
    #expect(JSONValue.string("a\"b\\c/d\n\u{01}é").encoded == "\"a\\\"b\\\\c/d\\n\\u0001é\"")
}

@Test func testRejectsSignatureOfWrongLength() {
    final class DERSigner: ProofSigner {
        let x = Data(count: 32)
        let y = Data(count: 32)
        func sign(_ data: Data) throws -> Data { Data(count: 70) }
    }
    #expect(throws: DPoPProofError.invalidSignature) {
        try DPoPProof.make(method: "GET", url: server, accessToken: nil, nonce: nil, signer: DERSigner())
    }
}

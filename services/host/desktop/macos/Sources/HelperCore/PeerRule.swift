// SPDX-License-Identifier: Apache-2.0

import Foundation

/// What the peer rule needs to know about one file system entry (from `lstat`, so a symbolic
/// link is never followed).
public struct FileStatus: Equatable, Sendable {
    /// The kind of entry.
    public enum Kind: Equatable, Sendable {
        case directory, socket, other
    }

    /// The owner's user ID.
    public var uid: UInt32
    /// The permission bits (`st_mode & 0o7777`).
    public var mode: UInt16
    /// The kind of entry; a symbolic link is `other`.
    public var kind: Kind

    /// Creates a status.
    public init(uid: UInt32, mode: UInt16, kind: Kind) {
        self.uid = uid
        self.mode = mode
        self.kind = kind
    }
}

/// The helper's check of the host side of its channel before `hello` (D-CU-02-5, tightened by
/// the coordinator's security finding 2 on CU-02).
///
/// The rule, in order:
/// 1. The pinned host UID is the UID of the service user `bothy-host` (24 section 2), resolved
///    from the system's user database; without such an account, or when it is the helper's own
///    user, nothing is trusted.
/// 2. The peer of the socket (`getpeereid`) is the pinned UID or root, and never the helper's
///    own user.
/// 3. Every folder from `/` down to the parent of the endpoint folder is a real directory owned
///    by root and writable by neither group nor others; the endpoint folder itself (`computer/`)
///    is a real directory owned by root or the pinned UID and writable by neither group nor
///    others. So no other user can swap the socket.
/// 4. The socket itself is a socket (not a link) owned by the peer's UID.
///
/// A development build with `BOTHY_COMPUTER_HELPER_CHANNEL` skips the rule (D-CU-02-5).
public enum PeerRule {
    /// The account name of the host service's user (24 section 2).
    public static let serviceUserName = "bothy-host"

    /// Why the host side is not trusted.
    public enum Refusal: Error, Equatable {
        /// No account `bothy-host` exists.
        case noServiceUser
        /// The account `bothy-host` is the helper's own user.
        case serviceUserIsHelper
        /// The peer runs as the helper's own user.
        case peerIsHelper
        /// The peer is neither the service user nor root.
        case peerNotService
        /// A folder of the path is missing, not a directory or has a wrong owner.
        case folderOwner(String)
        /// A folder of the path is writable by group or others.
        case folderWritable(String)
        /// The endpoint is missing or not a socket.
        case notASocket
        /// The socket is not owned by the peer's user.
        case socketOwner
    }

    /// Applies the rule.
    ///
    /// - Parameters:
    ///   - socketPath: the absolute endpoint path.
    ///   - ownUID: the helper's user.
    ///   - serviceUID: the UID of `bothy-host`, nil when the account does not exist.
    ///   - peerUID: the user of the process at the other end of the socket.
    ///   - status: `lstat` of a path, nil when it does not exist.
    /// - Throws: the first `Refusal` that applies.
    public static func check(
        socketPath: String, ownUID: UInt32, serviceUID: UInt32?, peerUID: UInt32,
        status: (String) -> FileStatus?
    ) throws {
        guard let service = serviceUID else { throw Refusal.noServiceUser }
        guard service != ownUID else { throw Refusal.serviceUserIsHelper }
        guard peerUID != ownUID else { throw Refusal.peerIsHelper }
        guard peerUID == service || peerUID == 0 else { throw Refusal.peerNotService }

        let folders = folderChain(of: socketPath)
        for (index, folder) in folders.enumerated() {
            let isEndpointFolder = index == folders.count - 1
            guard let entry = status(folder), entry.kind == .directory,
                entry.uid == 0 || (isEndpointFolder && entry.uid == service)
            else { throw Refusal.folderOwner(folder) }
            guard entry.mode & 0o022 == 0 else { throw Refusal.folderWritable(folder) }
        }
        guard let socket = status(socketPath), socket.kind == .socket else {
            throw Refusal.notASocket
        }
        guard socket.uid == peerUID else { throw Refusal.socketOwner }
    }

    /// The folders from `/` down to the parent of `path`.
    static func folderChain(of path: String) -> [String] {
        let parts = path.split(separator: "/").dropLast()
        var chain = ["/"]
        var current = ""
        for part in parts {
            current += "/" + part
            chain.append(current)
        }
        return chain
    }
}

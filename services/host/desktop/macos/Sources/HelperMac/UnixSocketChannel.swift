// SPDX-License-Identifier: Apache-2.0

import Darwin
import Foundation
import HelperCore

/// The helper's side of the channel on macOS: a Unix domain socket client (27 9.4).
///
/// Before `hello` the helper checks the host side with `PeerRule` (D-CU-02-5): the peer is the
/// service user `bothy-host` or root, the folder chain of the endpoint cannot be changed by
/// another user, and the socket belongs to the peer. A development build may name another endpoint in
/// `BOTHY_COMPUTER_HELPER_CHANNEL` and then skips the check, so CI can run a fake host; a
/// verified build ignores the variable.
public final class UnixSocketChannel: HostChannel {
    /// The endpoint of the host service on macOS (27 9.4).
    public static let defaultPath = "/Library/Application Support/bothy-host/computer/computer.sock"
    /// The environment variable of the development override (D-CU-02-5).
    public static let overrideVariable = "BOTHY_COMPUTER_HELPER_CHANNEL"

    /// The socket path in use.
    public let path: String
    /// Whether the peer check of D-CU-02-5 runs.
    public let checksPeer: Bool
    private var descriptor: Int32 = -1
    private let lock = NSLock()

    /// Creates a channel.
    /// - Parameters:
    ///   - path: the socket path.
    ///   - checksPeer: whether to run the peer check.
    public init(path: String, checksPeer: Bool) {
        self.path = path
        self.checksPeer = checksPeer
    }

    /// The channel for this build: the default endpoint with the peer check, or for a
    /// development build the override endpoint without it when the variable is set.
    /// - Parameters:
    ///   - signature: the helper's signature state.
    ///   - environment: the process environment.
    public static func forBuild(signature: SignatureState, environment: [String: String])
        -> UnixSocketChannel
    {
        if signature == .development, let path = environment[overrideVariable], !path.isEmpty {
            return UnixSocketChannel(path: path, checksPeer: false)
        }
        return UnixSocketChannel(path: defaultPath, checksPeer: true)
    }

    public func connect() throws {
        close()
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw ChannelConnectError.system(errno) }
        var noSigPipe: Int32 = 1
        setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &noSigPipe, socklen_t(MemoryLayout<Int32>.size))
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8)
        guard bytes.count < MemoryLayout.size(ofValue: address.sun_path) else {
            Darwin.close(fd)
            throw ChannelConnectError.pathTooLong
        }
        withUnsafeMutableBytes(of: &address.sun_path) { buffer in
            buffer.copyBytes(from: bytes)
        }
        let result = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        guard result == 0 else {
            let code = errno
            Darwin.close(fd)
            throw ChannelConnectError.system(code)
        }
        if checksPeer {
            do {
                try Self.checkPeer(fd: fd, path: path)
            } catch {
                Darwin.close(fd)
                throw error
            }
        }
        lock.withLock { descriptor = fd }
    }

    public func send(_ data: Data) throws {
        let fd = lock.withLock { descriptor }
        guard fd >= 0 else { throw ChannelConnectError.closed }
        try data.withUnsafeBytes { raw in
            var offset = 0
            while offset < raw.count {
                let written = Darwin.send(fd, raw.baseAddress! + offset, raw.count - offset, 0)
                if written < 0 {
                    if errno == EINTR { continue }
                    throw ChannelConnectError.system(errno)
                }
                offset += written
            }
        }
    }

    public func receive() throws -> Data {
        let fd = lock.withLock { descriptor }
        guard fd >= 0 else { throw ChannelConnectError.closed }
        var buffer = [UInt8](repeating: 0, count: 64 * 1024)
        while true {
            let count = recv(fd, &buffer, buffer.count, 0)
            if count > 0 { return Data(buffer[0..<count]) }
            if count == 0 { throw ChannelConnectError.closed }
            if errno != EINTR { throw ChannelConnectError.system(errno) }
        }
    }

    public func close() {
        let fd = lock.withLock { () -> Int32 in
            let old = descriptor
            descriptor = -1
            return old
        }
        if fd >= 0 {
            // shutdown wakes a thread blocked in recv before the descriptor goes away.
            shutdown(fd, SHUT_RDWR)
            Darwin.close(fd)
        }
    }

    /// The host side check of D-CU-02-5 with the system's facts; the decision is
    /// `PeerRule.check`.
    /// - Throws: `PeerRule.Refusal` when the rule fails, `ChannelConnectError.system` when a
    ///   system call fails.
    static func checkPeer(fd: Int32, path: String) throws {
        var uid: uid_t = 0
        var gid: gid_t = 0
        guard getpeereid(fd, &uid, &gid) == 0 else { throw ChannelConnectError.system(errno) }
        try PeerRule.check(
            socketPath: path, ownUID: getuid(), serviceUID: serviceUID(), peerUID: uid,
            status: fileStatus)
    }

    /// The UID of the service user from the user database, nil without the account.
    static func serviceUID() -> UInt32? {
        guard let entry = getpwnam(PeerRule.serviceUserName) else { return nil }
        return entry.pointee.pw_uid
    }

    /// `lstat` of a path as `FileStatus`; a link is `other`, so it is never followed.
    static func fileStatus(_ path: String) -> FileStatus? {
        var info = stat()
        guard lstat(path, &info) == 0 else { return nil }
        let kind: FileStatus.Kind =
            switch info.st_mode & S_IFMT {
            case S_IFDIR: .directory
            case S_IFSOCK: .socket
            default: .other
            }
        return FileStatus(uid: info.st_uid, mode: UInt16(info.st_mode & 0o7777), kind: kind)
    }
}

/// Errors of the socket channel.
public enum ChannelConnectError: Error, Equatable {
    /// A system call failed with this `errno`.
    case system(Int32)
    /// The socket path does not fit into `sockaddr_un`.
    case pathTooLong
    /// The channel is closed.
    case closed
}

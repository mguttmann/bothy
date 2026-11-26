import Foundation

/// A JSON value of the helper channel (docs/spec/27-computer-use.md section 9.4).
///
/// The channel needs exact control over members (a `state` carries only the members that
/// changed) and over integers versus fractions (coordinates may carry two decimals, every other
/// number is an integer), so messages are built from this tree rather than from `Codable` types.
///
/// Numbers compare by value: `.int(400)` equals `.double(400)`, because an encoder may write a
/// whole fraction without a decimal point. A boolean never equals a number.
///
/// Example:
/// ```swift
/// let hello: JSONValue = ["op": "hello", "enabled": true, "generation": 3]
/// let data = try hello.serialized()
/// ```
public enum JSONValue: Equatable, Sendable {
    case null
    case bool(Bool)
    case int(Int64)
    case double(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    /// Errors of parsing and serializing.
    public enum Failure: Error, Equatable {
        /// The bytes are not valid UTF-8 JSON.
        case invalidJSON
        /// A value that JSON cannot carry, such as a NaN or infinite number.
        case unrepresentable
    }

    /// Parses UTF-8 JSON bytes.
    ///
    /// - Parameter data: the bytes of one JSON document (any top-level value).
    /// - Returns: the parsed value with integers kept apart from fractions.
    /// - Throws: `Failure.invalidJSON` when the bytes are not JSON.
    public static func parse(_ data: Data) throws -> JSONValue {
        let object: Any
        do {
            object = try JSONSerialization.jsonObject(with: data, options: [.fragmentsAllowed])
        } catch {
            throw Failure.invalidJSON
        }
        return try JSONValue(foundation: object)
    }

    /// Serializes the value as compact UTF-8 JSON with sorted members.
    ///
    /// Sorted members make the output deterministic; the channel does not depend on member order.
    /// - Returns: the JSON bytes.
    /// - Throws: `Failure.unrepresentable` for a NaN or infinite number.
    public func serialized() throws -> Data {
        do {
            return try JSONSerialization.data(
                withJSONObject: try foundationObject(),
                options: [.sortedKeys, .withoutEscapingSlashes, .fragmentsAllowed])
        } catch let failure as Failure {
            throw failure
        } catch {
            throw Failure.unrepresentable
        }
    }

    /// The member `key` of an object, or nil for a missing member or a value that is no object.
    public subscript(key: String) -> JSONValue? {
        if case .object(let members) = self { return members[key] }
        return nil
    }

    /// The string, or nil for any other kind.
    public var stringValue: String? {
        if case .string(let value) = self { return value }
        return nil
    }

    /// The integer, or nil for any other kind; a fraction with no remainder counts as integer.
    public var intValue: Int64? {
        switch self {
        case .int(let value): return value
        case .double(let value):
            guard value.isFinite, value.rounded() == value, abs(value) < 9.0e15 else { return nil }
            return Int64(value)
        default: return nil
        }
    }

    /// The number as a fraction, or nil for any other kind.
    public var doubleValue: Double? {
        switch self {
        case .int(let value): return Double(value)
        case .double(let value): return value
        default: return nil
        }
    }

    /// The boolean, or nil for any other kind.
    public var boolValue: Bool? {
        if case .bool(let value) = self { return value }
        return nil
    }

    /// The elements of an array, or nil for any other kind.
    public var arrayValue: [JSONValue]? {
        if case .array(let value) = self { return value }
        return nil
    }

    /// The members of an object, or nil for any other kind.
    public var objectValue: [String: JSONValue]? {
        if case .object(let value) = self { return value }
        return nil
    }

    /// A number from a fraction that keeps at most two decimals, as 27 9.4 allows for macOS
    /// coordinates; whole values become integers so the output carries no needless `.0`.
    ///
    /// - Parameter value: the coordinate in points.
    /// - Returns: `.int` for a whole value after rounding, else `.double`.
    public static func coordinate(_ value: Double) -> JSONValue {
        let rounded = (value * 100).rounded() / 100
        if rounded.rounded() == rounded, abs(rounded) < 9.0e15 { return .int(Int64(rounded)) }
        return .double(rounded)
    }

    public static func == (lhs: JSONValue, rhs: JSONValue) -> Bool {
        switch (lhs, rhs) {
        case (.null, .null): return true
        case (.bool(let a), .bool(let b)): return a == b
        case (.string(let a), .string(let b)): return a == b
        case (.array(let a), .array(let b)): return a == b
        case (.object(let a), .object(let b)): return a == b
        case (.int(let a), .int(let b)): return a == b
        case (.int, .double), (.double, .int), (.double, .double):
            return lhs.doubleValue == rhs.doubleValue
        default: return false
        }
    }

    private init(foundation object: Any) throws {
        switch object {
        case is NSNull:
            self = .null
        case let number as NSNumber:
            // JSONSerialization hands booleans out as a CFBoolean, so the type ID tells them
            // apart from the numbers 0 and 1.
            if CFGetTypeID(number) == CFBooleanGetTypeID() {
                self = .bool(number.boolValue)
            } else if number is NSDecimalNumber || ["d", "f"].contains(String(cString: number.objCType)) {
                self = .double(number.doubleValue)
            } else {
                self = .int(number.int64Value)
            }
        case let string as String:
            self = .string(string)
        case let array as [Any]:
            self = .array(try array.map(JSONValue.init(foundation:)))
        case let dictionary as [String: Any]:
            self = .object(try dictionary.mapValues(JSONValue.init(foundation:)))
        default:
            throw Failure.invalidJSON
        }
    }

    private func foundationObject() throws -> Any {
        switch self {
        case .null: return NSNull()
        case .bool(let value): return NSNumber(value: value)
        case .int(let value): return NSNumber(value: value)
        case .double(let value):
            guard value.isFinite else { throw Failure.unrepresentable }
            return NSNumber(value: value)
        case .string(let value): return value
        case .array(let value): return try value.map { try $0.foundationObject() }
        case .object(let value): return try value.mapValues { try $0.foundationObject() }
        }
    }
}

extension JSONValue: ExpressibleByStringLiteral {
    /// Creates a string value from a literal.
    public init(stringLiteral value: String) { self = .string(value) }
}

extension JSONValue: ExpressibleByIntegerLiteral {
    /// Creates an integer value from a literal.
    public init(integerLiteral value: Int64) { self = .int(value) }
}

extension JSONValue: ExpressibleByFloatLiteral {
    /// Creates a fraction value from a literal.
    public init(floatLiteral value: Double) { self = .double(value) }
}

extension JSONValue: ExpressibleByBooleanLiteral {
    /// Creates a boolean value from a literal.
    public init(booleanLiteral value: Bool) { self = .bool(value) }
}

extension JSONValue: ExpressibleByArrayLiteral {
    /// Creates an array value from a literal.
    public init(arrayLiteral elements: JSONValue...) { self = .array(elements) }
}

extension JSONValue: ExpressibleByDictionaryLiteral {
    /// Creates an object value from a literal; a repeated key keeps the last value.
    public init(dictionaryLiteral elements: (String, JSONValue)...) {
        self = .object(Dictionary(elements, uniquingKeysWith: { _, last in last }))
    }
}

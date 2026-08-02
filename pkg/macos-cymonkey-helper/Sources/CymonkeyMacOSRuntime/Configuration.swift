import Foundation

public struct HelperConfiguration: Codable, Sendable {
    public let allowedBundleIds: [String]
    public let appleEventCommands: [AppleEventCommand]
    public let accessibility: AccessibilityPolicy?

    public init(
        allowedBundleIds: [String],
        appleEventCommands: [AppleEventCommand] = [],
        accessibility: AccessibilityPolicy? = nil
    ) throws {
        self.allowedBundleIds = try Self.validatedBundleIDs(allowedBundleIds)
        self.appleEventCommands = appleEventCommands
        self.accessibility = accessibility
        try validate()
    }

    public func validate() throws {
        let allowed = Set(try Self.validatedBundleIDs(allowedBundleIds))
        var names = Set<String>()
        for command in appleEventCommands {
            guard allowed.contains(command.bundleId) else {
                throw RuntimeError.denied("Apple Event command target is not in allowedBundleIds")
            }
            try command.validate()
            guard names.insert(command.name).inserted else {
                throw RuntimeError.invalidRequest("Apple Event command names must be unique")
            }
        }
        if let accessibility {
            try accessibility.validate(allowedBundleIDs: allowed)
        }
    }

    public static func load(from url: URL) throws -> HelperConfiguration {
        let data = try Data(contentsOf: url, options: [.mappedIfSafe])
        let config = try JSONDecoder().decode(HelperConfiguration.self, from: data)
        try config.validate()
        return config
    }

    private static func validatedBundleIDs(_ values: [String]) throws -> [String] {
        let pattern = try NSRegularExpression(pattern: "^[A-Za-z0-9-]+(\\.[A-Za-z0-9-]+)+$")
        var seen = Set<String>()
        var result: [String] = []
        for value in values {
            let range = NSRange(value.startIndex..<value.endIndex, in: value)
            guard pattern.firstMatch(in: value, range: range)?.range == range else {
                throw RuntimeError.invalidRequest("invalid bundle identifier")
            }
            if seen.insert(value).inserted { result.append(value) }
        }
        return result.sorted()
    }
}

public struct AppleEventCommand: Codable, Equatable, Sendable {
    public let name: String
    public let bundleId: String
    public let eventClass: String
    public let eventId: String
    public let parameters: [AppleEventParameter]

    public init(
        name: String,
        bundleId: String,
        eventClass: String,
        eventId: String,
        parameters: [AppleEventParameter] = []
    ) {
        self.name = name
        self.bundleId = bundleId
        self.eventClass = eventClass
        self.eventId = eventId
        self.parameters = parameters
    }

    fileprivate func validate() throws {
        guard name.range(of: "^[a-z][a-z0-9.-]*$", options: .regularExpression) != nil,
              name.lowercased() != "do script" else {
            throw RuntimeError.invalidRequest("unsafe Apple Event command name")
        }
        guard eventClass.utf8.count == 4, eventId.utf8.count == 4,
              eventClass.utf8.allSatisfy({ $0 < 128 }), eventId.utf8.allSatisfy({ $0 < 128 }) else {
            throw RuntimeError.invalidRequest("Apple Event class and id must be four ASCII bytes")
        }
        var inputNames = Set<String>()
        var keywords = Set<String>()
        for parameter in parameters {
            try parameter.validate()
            guard inputNames.insert(parameter.input).inserted,
                  keywords.insert(parameter.keyword).inserted else {
                throw RuntimeError.invalidRequest("Apple Event parameter mappings must be unique")
            }
        }
    }
}

public struct AppleEventParameter: Codable, Equatable, Sendable {
    public enum ValueType: String, Codable, Sendable {
        case string, boolean, integer, number
    }

    public let input: String
    public let keyword: String
    public let type: ValueType
    public let required: Bool

    public init(input: String, keyword: String, type: ValueType, required: Bool = false) {
        self.input = input
        self.keyword = keyword
        self.type = type
        self.required = required
    }

    fileprivate func validate() throws {
        guard input.range(of: "^[A-Za-z][A-Za-z0-9_]*$", options: .regularExpression) != nil,
              keyword.utf8.count == 4, keyword.utf8.allSatisfy({ $0 < 128 }) else {
            throw RuntimeError.invalidRequest("invalid Apple Event parameter mapping")
        }
    }
}

public struct AccessibilityPolicy: Codable, Equatable, Sendable {
    public let allowedBundleIds: [String]
    public let allowedWritableAttributes: [String]
    public let maxDepth: Int
    public let maxResults: Int
    public let promptForAccessibilityConsent: Bool

    public init(
        allowedBundleIds: [String],
        allowedWritableAttributes: [String] = [],
        maxDepth: Int = 8,
        maxResults: Int = 50,
        promptForAccessibilityConsent: Bool = false
    ) {
        self.allowedBundleIds = allowedBundleIds
        self.allowedWritableAttributes = allowedWritableAttributes
        self.maxDepth = maxDepth
        self.maxResults = maxResults
        self.promptForAccessibilityConsent = promptForAccessibilityConsent
    }

    fileprivate func validate(allowedBundleIDs: Set<String>) throws {
        guard maxDepth > 0, maxDepth <= 32, maxResults > 0, maxResults <= 500 else {
            throw RuntimeError.invalidRequest("Accessibility bounds exceed helper limits")
        }
        guard Set(allowedBundleIds).isSubset(of: allowedBundleIDs) else {
            throw RuntimeError.denied("Accessibility target is not in allowedBundleIds")
        }
        guard allowedWritableAttributes.allSatisfy({ $0.hasPrefix("AX") && !$0.contains("\n") }) else {
            throw RuntimeError.invalidRequest("invalid Accessibility writable attribute")
        }
    }
}

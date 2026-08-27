import Foundation
import Security

/// A tiny Keychain wrapper for the session bundle (tokens + user). Tokens live in
/// the Keychain, never UserDefaults, so they are encrypted at rest and can be
/// wiped on sign-out. One item per app, keyed by the bundle id.
struct KeychainStore {
    private let service: String
    private let account = "session"

    init(service: String = Bundle.main.bundleIdentifier ?? "app.beacon") {
        self.service = service
    }

    private var baseQuery: [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
    }

    /// Stores data, replacing any existing item.
    func save(_ data: Data) {
        let attributes: [String: Any] = [
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock,
        ]
        let status = SecItemUpdate(baseQuery as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound {
            var add = baseQuery
            add.merge(attributes) { _, new in new }
            SecItemAdd(add as CFDictionary, nil)
        }
    }

    /// Loads the stored data, or nil if none.
    func load() -> Data? {
        var query = baseQuery
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: AnyObject?
        guard SecItemCopyMatching(query as CFDictionary, &result) == errSecSuccess else { return nil }
        return result as? Data
    }

    /// Removes the stored item.
    func clear() {
        SecItemDelete(baseQuery as CFDictionary)
    }
}

import Foundation
import SwiftUI

/// Typed, read-once access to the active brand's configuration.
///
/// Every brand-specific or environment value is injected from the brand xcconfig
/// into Info.plist and read here — never hardcoded in feature code. A missing
/// required value is a build-configuration mistake, so it fails loudly at launch
/// rather than silently defaulting to something wrong.
struct AppConfig {
    let apiBaseURL: URL
    let googleClientID: String?
    let brandDisplayName: String
    let accentColor: Color

    /// The resolved configuration for this build. Loaded once.
    static let current = load()

    /// Whether Google Sign-In is configured for this brand/build.
    var googleEnabled: Bool { googleClientID != nil }

    private static func load() -> AppConfig {
        let bundle = Bundle.main
        guard let raw = bundle.object(forInfoDictionaryKey: "APIBaseURL") as? String,
              let url = URL(string: raw), url.scheme != nil else {
            fatalError("AppConfig: APIBaseURL is missing or invalid — check the brand xcconfig")
        }
        // Treat the unfilled placeholder as "not configured" so email/password still
        // works before a Google client id has been added.
        let google = (bundle.object(forInfoDictionaryKey: "GoogleClientID") as? String)
            .flatMap { $0.isEmpty || $0.hasPrefix("REPLACE_WITH") ? nil : $0 }
        let name = bundle.object(forInfoDictionaryKey: "BrandDisplayName") as? String ?? "Beacon"
        let hex = bundle.object(forInfoDictionaryKey: "BrandAccentHex") as? String ?? "1D64E8"
        return AppConfig(apiBaseURL: url, googleClientID: google,
                         brandDisplayName: name, accentColor: Color(hex: hex))
    }
}

extension Color {
    /// Builds a Color from a 6-digit hex string (no leading `#`).
    init(hex: String) {
        var value: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&value)
        self.init(.sRGB,
                  red: Double((value >> 16) & 0xFF) / 255,
                  green: Double((value >> 8) & 0xFF) / 255,
                  blue: Double(value & 0xFF) / 255,
                  opacity: 1)
    }
}

import SwiftUI
#if os(iOS)
import UIKit
#else
import AppKit
#endif

/// The few things the iPhone/iPad app and the Mac app do differently. Screens call
/// these instead of UIKit or AppKit, so the same Sources/ build both.
enum Platform {
    static func copy(_ text: String) {
        #if os(iOS)
        UIPasteboard.general.string = text
        #else
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
        #endif
    }

    /// Where a person turns notifications back on for this app.
    static var notificationSettingsURL: URL? {
        #if os(iOS)
        URL(string: UIApplication.openSettingsURLString)
        #else
        URL(string: "x-apple.systempreferences:com.apple.Notifications-Settings.extension")
        #endif
    }

    @MainActor
    static func registerForRemoteNotifications() {
        #if os(iOS)
        UIApplication.shared.registerForRemoteNotifications()
        #else
        NSApplication.shared.registerForRemoteNotifications()
        #endif
    }
}

#if os(macOS)
// The forms are written once with iPhone keyboard hints, which a Mac has no use
// for. These stand-ins make those modifiers no-ops on the Mac, so the shared views
// need no #if at every text field.
enum UIKeyboardType { case `default`, URL, emailAddress, numberPad, numbersAndPunctuation }
typealias UITextContentType = NSTextContentType
struct TextInputAutocapitalization {
    static let never = Self(), words = Self(), sentences = Self(), characters = Self()
}
enum NavigationBarItem { enum TitleDisplayMode { case automatic, inline, large } }

extension View {
    func keyboardType(_: UIKeyboardType) -> some View { self }
    func textInputAutocapitalization(_: TextInputAutocapitalization?) -> some View { self }
    func navigationBarTitleDisplayMode(_: NavigationBarItem.TitleDisplayMode) -> some View { self }
}

extension ToolbarItemPlacement {
    static var topBarTrailing: Self { .primaryAction }
}
#endif

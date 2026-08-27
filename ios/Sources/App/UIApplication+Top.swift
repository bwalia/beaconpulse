import UIKit

extension UIApplication {
    /// The top-most view controller of the active scene, used to present UIKit
    /// flows (Google Sign-In) from SwiftUI.
    var topViewController: UIViewController? {
        let scene = connectedScenes
            .first { $0.activationState == .foregroundActive } as? UIWindowScene
        var top = scene?.windows.first { $0.isKeyWindow }?.rootViewController
        while let presented = top?.presentedViewController { top = presented }
        return top
    }
}

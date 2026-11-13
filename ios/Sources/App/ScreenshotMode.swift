#if DEBUG
import SwiftUI
#if os(macOS)
import AppKit
#endif

/// Debug-only driver for App Store screenshots; release builds compile none of it.
///
/// Launch with `-snapshotScreen <overview|monitors|detail|alerts|projects|settings>`
/// to open straight on that screen (iPhone/iPad: then `simctl io … screenshot`).
/// On the Mac add `-snapshotOut <file.png>`: the app sizes its window to 1440×900,
/// waits for data, saves the window at 2880×1800 and quits — no screen-recording
/// permission needed, because the app draws its own window.
enum ScreenshotMode {
    @MainActor
    static func drive(selection: Binding<MainTabView.Tab>, monitorsPath: Binding<NavigationPath>,
                      container: AppContainer) async {
        guard let screen = UserDefaults.standard.string(forKey: "snapshotScreen") else { return }
        NSLog("snapshot: screen %@", screen)
        switch screen {
        case "monitors": selection.wrappedValue = .monitors
        case "alerts": selection.wrappedValue = .alerts
        case "projects": selection.wrappedValue = .projects
        case "settings": selection.wrappedValue = .settings
        case "detail":
            selection.wrappedValue = .monitors
            // A healthy probed monitor, so the detail has a response chart.
            let page = try? await container.apiClient.send(
                .init(path: "/api/v1/monitors", query: ["limit": "50"]), as: Paginated<Monitor>.self)
            let monitors = page?.data ?? []
            let pick = monitors.first { $0.lastStatus == "up" && ["https", "http"].contains($0.type) }
                ?? monitors.first
            if let pick { monitorsPath.wrappedValue.append(pick.id) }
        default: selection.wrappedValue = .overview
        }
        #if os(macOS)
        await saveWindow()
        #endif
    }

    #if os(macOS)
    @MainActor
    private static func saveWindow() async {
        guard let out = UserDefaults.standard.string(forKey: "snapshotOut") else { return }
        var found: NSWindow?
        for _ in 0..<40 where found == nil {
            found = NSApp.windows.first { $0.isVisible && $0.contentView != nil && $0.frame.width > 300 }
            if found == nil { try? await Task.sleep(for: .milliseconds(250)) }
        }
        guard let window = found else { NSLog("snapshot: no window"); return }
        // On a Retina screen if there is one (2880×1800), frontmost so it draws active.
        let screen = NSScreen.screens.first { $0.backingScaleFactor > 1 } ?? NSScreen.main
        let origin = screen.map { NSPoint(x: $0.visibleFrame.minX + 20, y: $0.visibleFrame.minY + 10) } ?? .zero
        window.setFrame(NSRect(origin: origin, size: NSSize(width: 1440, height: 900)), display: true)
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
        try? await Task.sleep(for: .seconds(6))  // data, charts and images settle
        // The composited window as on screen (glass sidebar and toolbar included).
        // An app may always capture its own windows, so no permission prompt.
        guard let image = CGWindowListCreateImage(.null, .optionIncludingWindow, CGWindowID(window.windowNumber),
                                                  [.boundsIgnoreFraming, .bestResolution]) else {
            NSLog("snapshot: capture failed"); return
        }
        let rep = NSBitmapImageRep(cgImage: image)
        NSLog("snapshot: %dx%d", image.width, image.height)
        do {
            try rep.representation(using: .png, properties: [:])?.write(to: URL(fileURLWithPath: out))
            NSLog("snapshot: saved %@", out)
        } catch {
            NSLog("snapshot: write failed %@", "\(error)")
        }
        NSApp.terminate(nil)
    }
    #endif
}
#endif

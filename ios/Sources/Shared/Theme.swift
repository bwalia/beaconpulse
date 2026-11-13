import Charts
import SwiftUI
#if os(iOS)
import UIKit
#else
import AppKit
#endif

/// The app's visual system, matching the web dashboard: slate surfaces, white cards
/// with a hairline border and a soft shadow, the brand accent, bold tabular figures
/// and small uppercase labels. Light and dark, iPhone, iPad and Mac.
enum Theme {
    static var accent: Color { AppConfig.current.accentColor }
    static var accentDeep: Color { AppConfig.current.accentDeepColor }

    static let canvas = Color.dynamic(light: 0xF8FAFC, dark: 0x020617)  // slate-50 / slate-950
    static let card = Color.dynamic(light: 0xFFFFFF, dark: 0x0F172A)    // white / slate-900
    static let border = Color.dynamic(light: 0xE2E8F0, dark: 0x1E293B)  // slate-200 / slate-800
    static let subtle = Color.dynamic(light: 0xF1F5F9, dark: 0x1E293B)  // slate-100 / slate-800
    static let muted = Color.dynamic(light: 0x64748B, dark: 0x94A3B8)   // slate-500 / slate-400

    static let up = Color(hex: "10B981")        // emerald-500
    static let down = Color(hex: "EF4444")      // red-500
    static let degraded = Color(hex: "F59E0B")  // amber-500
    static let paused = Color(hex: "94A3B8")    // slate-400

    static func color(for status: MonitorStatus) -> Color {
        switch status {
        case .up: up
        case .down: down
        case .degraded: degraded
        case .paused, .unknown: paused
        }
    }

    static func uptimeColor(_ percent: Double) -> Color {
        switch percent {
        case 99.9...: up
        case 99...: degraded
        default: down
        }
    }
}

extension Color {
    /// A colour that follows light/dark mode, from two 0xRRGGBB values.
    static func dynamic(light: UInt32, dark: UInt32) -> Color {
        #if os(iOS)
        Color(UIColor { $0.userInterfaceStyle == .dark ? .rgb(dark) : .rgb(light) })
        #else
        Color(NSColor(name: nil) {
            $0.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? .rgb(dark) : .rgb(light)
        })
        #endif
    }
}

#if os(iOS)
private extension UIColor {
    static func rgb(_ v: UInt32) -> UIColor {
        UIColor(red: CGFloat((v >> 16) & 0xFF) / 255, green: CGFloat((v >> 8) & 0xFF) / 255,
                blue: CGFloat(v & 0xFF) / 255, alpha: 1)
    }
}
#else
private extension NSColor {
    static func rgb(_ v: UInt32) -> NSColor {
        NSColor(srgbRed: CGFloat((v >> 16) & 0xFF) / 255, green: CGFloat((v >> 8) & 0xFF) / 255,
                blue: CGFloat(v & 0xFF) / 255, alpha: 1)
    }
}
#endif

// MARK: - Surfaces

extension View {
    /// The web dashboard's card: white (slate-900 in dark), hairline border, soft shadow.
    func card(padding: CGFloat = 16) -> some View {
        self
            .padding(padding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.card, in: .rect(cornerRadius: 14))
            .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(Theme.border))
            .shadow(color: .black.opacity(0.04), radius: 3, y: 1)
    }

    /// The screen behind the cards (slate-50 / slate-950), also behind a List.
    func canvas() -> some View {
        self
            .scrollContentBackground(.hidden)
            .background(Theme.canvas.ignoresSafeArea())
    }

    /// A List row drawn as a card, with space between rows.
    func cardRow() -> some View {
        self
            .card(padding: 14)
            .listRowBackground(Color.clear)
            .listRowSeparator(.hidden)
            .listRowInsets(EdgeInsets(top: 5, leading: 16, bottom: 5, trailing: 16))
    }
}

/// A small uppercase label, as the web uses above figures and sections.
struct Eyebrow: View {
    let text: String
    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text.uppercased())
            .font(.caption2.weight(.semibold))
            .tracking(0.6)
            .foregroundStyle(Theme.muted)
    }
}

/// A headline figure in a card: label, big tabular value, optional icon and note.
struct StatTile: View {
    let label: String
    let value: String
    var tint: Color = .primary
    var icon: String?
    var note: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Eyebrow(label)
                Spacer()
                if let icon {
                    Image(systemName: icon)
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(Theme.accent)
                        .frame(width: 26, height: 26)
                        .background(Theme.accent.opacity(0.12), in: .rect(cornerRadius: 7))
                }
            }
            Text(value)
                .font(.system(.title, design: .rounded).weight(.bold))
                .monospacedDigit()
                .foregroundStyle(tint)
                .lineLimit(1)
                .minimumScaleFactor(0.6)
            if let note {
                Text(note).font(.caption).foregroundStyle(Theme.muted)
            }
        }
        .frame(minWidth: 130, maxHeight: .infinity, alignment: .topLeading)
        .card()
        .accessibilityElement(children: .combine)
    }
}

/// Cards that share one row evenly (equal widths and heights) when they fit, and
/// fall back to `narrow` (a two-column grid by default) on small screens — so three
/// tiles fill a wide Mac or iPad row instead of half of it.
struct FitRow<Content: View>: View {
    var spacing: CGFloat = 12
    var narrowColumns = 2
    @ViewBuilder var content: Content

    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .top, spacing: spacing) { content }
                .fixedSize(horizontal: false, vertical: true)
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: spacing), count: narrowColumns),
                      spacing: spacing) { content }
        }
    }
}

/// A monitor's state as a tinted pill: dot + word.
struct StatusBadge: View {
    let status: MonitorStatus

    var body: some View {
        let color = Theme.color(for: status)
        HStack(spacing: 5) {
            Circle().fill(color).frame(width: 7, height: 7)
            Text(status.rawValue.capitalized).font(.caption.weight(.semibold))
        }
        .padding(.horizontal, 9)
        .padding(.vertical, 4)
        .foregroundStyle(color)
        .background(color.opacity(0.13), in: .capsule)
        .accessibilityLabel("Status: \(status.rawValue)")
    }
}

/// A monitor type (HTTPS, DNS…) as a small monospaced tag.
struct TypeBadge: View {
    let text: String

    var body: some View {
        Text(text.uppercased())
            .font(.system(size: 10, weight: .semibold, design: .monospaced))
            .padding(.horizontal, 6)
            .padding(.vertical, 3)
            .foregroundStyle(Theme.muted)
            .background(Theme.subtle, in: .rect(cornerRadius: 5))
    }
}

// MARK: - Brand

/// The brand mark from the web (a ring with eight ticks), drawn in a 24-unit box.
struct BrandMark: Shape {
    func path(in rect: CGRect) -> Path {
        let s = min(rect.width, rect.height) / 24
        func p(_ x: CGFloat, _ y: CGFloat) -> CGPoint {
            CGPoint(x: rect.midX + (x - 12) * s, y: rect.midY + (y - 12) * s)
        }
        var path = Path()
        path.addEllipse(in: CGRect(x: rect.midX - 2.5 * s, y: rect.midY - 2.5 * s, width: 5 * s, height: 5 * s))
        let ticks: [(CGFloat, CGFloat, CGFloat, CGFloat)] = [
            (12, 4.5, 12, 6.5), (12, 17.5, 12, 19.5), (4.5, 12, 6.5, 12), (17.5, 12, 19.5, 12),
            (6.7, 6.7, 8.1, 8.1), (15.9, 15.9, 17.3, 17.3), (17.3, 6.7, 15.9, 8.1), (8.1, 15.9, 6.7, 17.3),
        ]
        for (x1, y1, x2, y2) in ticks {
            path.move(to: p(x1, y1))
            path.addLine(to: p(x2, y2))
        }
        return path
    }
}

/// The brand mark in its rounded accent tile, as in the web header.
struct BrandTile: View {
    var size: CGFloat = 36

    var body: some View {
        RoundedRectangle(cornerRadius: size * 0.26)
            .fill(LinearGradient(colors: [Theme.accent, Theme.accentDeep],
                                 startPoint: .topLeading, endPoint: .bottomTrailing))
            .frame(width: size, height: size)
            .overlay(
                BrandMark()
                    .stroke(.white, style: StrokeStyle(lineWidth: size * 1.75 / 24 * 0.62, lineCap: .round))
                    .padding(size * 0.12)
            )
            .accessibilityHidden(true)
    }
}

// MARK: - Charts

/// A trend line with a soft gradient fill and quiet axes.
struct TrendChart: View {
    let series: [MetricPoint]
    var color: Color = Theme.accent
    var height: CGFloat = 170
    var label: String

    var body: some View {
        Chart(series) { point in
            AreaMark(x: .value("Time", point.t), y: .value(label, point.v))
                .foregroundStyle(LinearGradient(colors: [color.opacity(0.28), color.opacity(0.02)],
                                                startPoint: .top, endPoint: .bottom))
                .interpolationMethod(.monotone)
            LineMark(x: .value("Time", point.t), y: .value(label, point.v))
                .foregroundStyle(color)
                .lineStyle(StrokeStyle(lineWidth: 2, lineCap: .round))
                .interpolationMethod(.monotone)
        }
        .chartYAxis {
            AxisMarks(position: .leading) { _ in
                AxisGridLine(stroke: StrokeStyle(lineWidth: 0.5, dash: [3, 3])).foregroundStyle(Theme.border)
                AxisValueLabel().font(.caption2).foregroundStyle(Theme.muted)
            }
        }
        .chartXAxis {
            AxisMarks(values: .automatic(desiredCount: 4)) { _ in
                AxisValueLabel().font(.caption2).foregroundStyle(Theme.muted)
            }
        }
        .frame(height: height)
        .accessibilityLabel("\(label) trend")
    }
}

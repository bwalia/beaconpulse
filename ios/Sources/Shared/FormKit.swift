import SwiftUI

// The web's form look, shared by every create/edit sheet: titled cards on the
// canvas, a label above each control, soft bordered inputs with an accent ring on
// focus, choice chips instead of drop-downs for short lists, and switch rows.

/// A scrolling form of cards. Gives a Mac sheet a sensible size.
struct FormScreen<Content: View>: View {
    @ViewBuilder var content: Content

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) { content }
                .padding(20)
                .frame(maxWidth: 620)
                .frame(maxWidth: .infinity)
        }
        .scrollDismissesKeyboard(.interactively)
        .canvas()
        #if os(macOS)
        .frame(minWidth: 520, idealWidth: 560, minHeight: 460, idealHeight: 640)
        #endif
    }
}

/// A titled card of fields, with an optional note under it.
struct FormCard<Content: View>: View {
    let title: String?
    let footer: String?
    let content: Content

    init(_ title: String? = nil, footer: String? = nil, @ViewBuilder content: () -> Content) {
        self.title = title
        self.footer = footer
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            if let title { Eyebrow(title).padding(.leading, 4) }
            VStack(alignment: .leading, spacing: 18) { content }
                .card(padding: 18)
            if let footer {
                Text(footer)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.horizontal, 4)
            }
        }
    }
}

/// A label above its control, with an optional hint below.
struct Field<Content: View>: View {
    let label: String
    let hint: String?
    let content: Content

    init(_ label: String, hint: String? = nil, @ViewBuilder content: () -> Content) {
        self.label = label
        self.hint = hint
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            Text(label).font(.subheadline.weight(.semibold))
            content
            if let hint {
                Text(hint)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// The web's input: soft fill, hairline border, an accent ring while focused.
private struct InputChrome: ViewModifier {
    @FocusState private var focused: Bool

    func body(content: Content) -> some View {
        content
            .textFieldStyle(.plain)
            .focused($focused)
            .focusEffectDisabled()
            .padding(.horizontal, 12)
            .padding(.vertical, 11)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.canvas, in: .rect(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10)
                .strokeBorder(focused ? Theme.accent : Theme.border, lineWidth: focused ? 1.5 : 1))
            .shadow(color: focused ? Theme.accent.opacity(0.22) : .clear, radius: 5)
            .contentShape(Rectangle())
            .onTapGesture { focused = true }
            .animation(.easeOut(duration: 0.15), value: focused)
    }
}

extension View {
    /// Styles a TextField or SecureField as the web's input.
    func inputStyle() -> some View { modifier(InputChrome()) }

    /// A drop-down for long or dynamic lists (projects), in the input look.
    @ViewBuilder
    func menuInputStyle() -> some View {
        #if os(iOS)
        self.pickerStyle(.menu)
            .labelsHidden()
            .tint(.primary)
            .padding(.horizontal, 2)
            .padding(.vertical, 4)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.canvas, in: .rect(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(Theme.border))
        #else
        self.pickerStyle(.menu)
            .labelsHidden()
            .frame(maxWidth: 320, alignment: .leading)
        #endif
    }
}

/// A value that can't be changed here, shown in the input's shape.
struct ReadOnlyValue: View {
    let text: String
    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text)
            .foregroundStyle(Theme.muted)
            .padding(.horizontal, 12)
            .padding(.vertical, 11)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.subtle, in: .rect(cornerRadius: 10))
    }
}

/// One option for `ChoiceChips`.
struct Choice<Value: Hashable>: Identifiable {
    let value: Value
    let label: String
    let icon: String?
    var id: Value { value }

    init(_ value: Value, _ label: String, icon: String? = nil) {
        self.value = value
        self.label = label
        self.icon = icon
    }
}

/// A short list of options as tappable chips; the selected one is outlined in the accent.
struct ChoiceChips<Value: Hashable>: View {
    let choices: [Choice<Value>]
    @Binding var selection: Value
    var minWidth: CGFloat = 96

    var body: some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: minWidth), spacing: 8)], spacing: 8) {
            ForEach(choices) { choice in
                let selected = choice.value == selection
                Button {
                    withAnimation(.easeOut(duration: 0.15)) { selection = choice.value }
                } label: {
                    HStack(spacing: 6) {
                        if let icon = choice.icon {
                            Image(systemName: icon).font(.footnote.weight(.semibold))
                        }
                        Text(choice.label).lineLimit(1).minimumScaleFactor(0.8)
                    }
                    .font(.subheadline.weight(selected ? .semibold : .medium))
                    .foregroundStyle(selected ? Theme.accent : Color.primary)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 10)
                    .padding(.horizontal, 8)
                    .background(selected ? Theme.accent.opacity(0.1) : Theme.canvas, in: .rect(cornerRadius: 10))
                    .overlay(RoundedRectangle(cornerRadius: 10)
                        .strokeBorder(selected ? Theme.accent : Theme.border, lineWidth: selected ? 1.5 : 1))
                    .contentShape(.rect(cornerRadius: 10))
                }
                .buttonStyle(.plain)
                .accessibilityAddTraits(selected ? .isSelected : [])
            }
        }
    }
}

/// An on/off setting: title and explanation, with a switch on the right.
struct SwitchRow: View {
    let title: String
    var subtitle: String?
    @Binding var isOn: Bool

    var body: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.body.weight(.medium))
                if let subtitle {
                    Text(subtitle)
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            Spacer(minLength: 8)
            Toggle(title, isOn: $isOn)
                .labelsHidden()
                .toggleStyle(.switch)
                .tint(Theme.accent)
        }
    }
}

/// A number set with − / +, showing the formatted value large.
struct StepperRow: View {
    let title: String
    @Binding var value: Int
    let range: ClosedRange<Int>
    var step: Int = 1
    let format: (Int) -> String

    var body: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.subheadline.weight(.semibold))
                Text(format(value))
                    .font(.system(.title3, design: .rounded).weight(.bold))
                    .monospacedDigit()
            }
            Spacer(minLength: 8)
            Stepper(title, value: $value, in: range, step: step).labelsHidden()
        }
    }
}

/// A save/delete failure, shown above the form's actions.
struct ErrorBanner: View {
    let message: String
    init(_ message: String) { self.message = message }

    var body: some View {
        Label {
            Text(message).fixedSize(horizontal: false, vertical: true)
        } icon: {
            Image(systemName: "exclamationmark.triangle.fill")
        }
        .font(.callout)
        .foregroundStyle(Theme.down)
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.down.opacity(0.08), in: .rect(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(Theme.down.opacity(0.3)))
    }
}

extension Theme {
    /// The SF Symbol for a monitor type, shared by lists and forms.
    static func symbol(forMonitorType type: String) -> String {
        switch type.lowercased() {
        case "http", "https": "globe"
        case "ssl", "tls": "lock.shield"
        case "dns": "server.rack"
        case "tcp": "network"
        case "ping", "icmp": "dot.radiowaves.left.and.right"
        case "heartbeat": "waveform.path.ecg"
        case "github_actions": "gearshape.2"
        default: "dot.radiowaves.left.and.right"
        }
    }
}

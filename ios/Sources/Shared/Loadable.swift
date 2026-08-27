import SwiftUI

/// The lifecycle of an async-loaded value. Small on purpose — the empty case is
/// modelled by the loaded value itself (an empty array), so screens decide what
/// "empty" looks like.
enum Loadable<Value> {
    case loading
    case loaded(Value)
    case failed(String)
}

/// Renders a `Loadable`: a spinner while loading, a retryable message on failure,
/// and the caller's content once loaded. Keeps every screen's loading/error UX
/// identical without each re-implementing the switch.
struct LoadableView<Value, Content: View>: View {
    let state: Loadable<Value>
    let retry: () -> Void
    @ViewBuilder let content: (Value) -> Content

    var body: some View {
        switch state {
        case .loading:
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        case let .failed(message):
            ContentUnavailableView {
                Label("Couldn’t load", systemImage: "exclamationmark.triangle")
            } description: {
                Text(message)
            } actions: {
                Button("Try Again", action: retry)
            }
        case let .loaded(value):
            content(value)
        }
    }
}

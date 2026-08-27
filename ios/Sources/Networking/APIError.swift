import Foundation

/// Every failure the API layer can produce, as one typed value so callers switch
/// exhaustively and the UI can show an honest, specific message instead of a raw
/// error or a spinner that never resolves.
enum APIError: Error, LocalizedError, Equatable {
    /// No network connectivity.
    case offline
    /// The request exceeded its timeout.
    case timedOut
    /// 401, and a token refresh could not recover it.
    case unauthorized
    /// 403.
    case forbidden
    /// 404.
    case notFound
    /// 429, with the server's Retry-After if present.
    case rateLimited(retryAfter: TimeInterval?)
    /// A 4xx/5xx carrying the standard error envelope.
    case server(status: Int, code: String?, message: String?)
    /// The response body did not match the expected shape.
    case decoding
    /// Not an HTTP response, or otherwise unusable.
    case invalidResponse

    var errorDescription: String? {
        switch self {
        case .offline:
            return "You appear to be offline. Check your connection and try again."
        case .timedOut:
            return "The request timed out. Please try again."
        case .unauthorized:
            return "Your session has expired. Please sign in again."
        case .forbidden:
            return "You don’t have permission to do that."
        case .notFound:
            return "That could not be found."
        case .rateLimited:
            return "Too many requests. Please wait a moment and try again."
        case let .server(_, _, message):
            return message ?? "Something went wrong on the server. Please try again."
        case .decoding, .invalidResponse:
            return "We couldn’t read the server’s response."
        }
    }
}

/// The API's standard error envelope: `{ "error": { "code", "message", … } }`.
struct APIErrorEnvelope: Decodable {
    struct Payload: Decodable {
        let code: String?
        let message: String?
    }
    let error: Payload
}

import Foundation

/// Supplies a bearer token and can refresh it. Implemented by the session store;
/// injected so the client stays unaware of how the session is stored.
protocol AuthProviding: AnyObject {
    /// The current access token, or nil when unauthenticated.
    func currentAccessToken() async -> String?
    /// Attempts to refresh the access token. Returns true on success.
    func refreshAccessToken() async -> Bool
    /// Called when refresh fails, so the app can sign the user out cleanly.
    func handleAuthenticationLost() async
}

/// The single HTTP entry point for the app: async/await over URLSession, bearer
/// auth via an injected provider, one transparent refresh-and-retry on 401, and
/// typed errors. No feature code builds URLs or touches URLSession directly.
final class APIClient {
    private let baseURL: URL
    private let session: URLSession
    private weak var auth: AuthProviding?

    init(baseURL: URL, auth: AuthProviding? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.auth = auth
        self.session = session
    }

    /// Late-binds the auth provider (the session store needs the client to exist
    /// first, so the authenticated client is wired up after construction).
    func setAuth(_ auth: AuthProviding) { self.auth = auth }

    /// A single API call. Paths are full, e.g. "/api/v1/monitors".
    struct Request {
        var method = "GET"
        var path: String
        var query: [String: String] = [:]
        var body: Encodable?
        /// When true (default) the request carries a bearer token and may trigger
        /// a refresh on 401. Auth endpoints set this false.
        var authenticated = true
    }

    /// Sends a request and decodes the JSON response into `T`.
    func send<T: Decodable>(_ request: Request, as type: T.Type) async throws -> T {
        let data = try await perform(request)
        guard !data.isEmpty else { throw APIError.decoding }
        do {
            return try Self.decoder().decode(T.self, from: data)
        } catch {
            throw APIError.decoding
        }
    }

    /// Sends a request that returns no body (e.g. a 204).
    func send(_ request: Request) async throws {
        _ = try await perform(request)
    }

    // MARK: - Core

    private func perform(_ request: Request, isRetry: Bool = false) async throws -> Data {
        let urlRequest = try await buildURLRequest(request)

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: urlRequest)
        } catch let urlError as URLError {
            switch urlError.code {
            case .notConnectedToInternet, .dataNotAllowed, .networkConnectionLost:
                throw APIError.offline
            case .timedOut:
                throw APIError.timedOut
            default:
                throw APIError.invalidResponse
            }
        }

        guard let http = response as? HTTPURLResponse else { throw APIError.invalidResponse }

        switch http.statusCode {
        case 200...299:
            return data
        case 401:
            // One transparent refresh + retry; then give up and hand off to sign-out.
            if request.authenticated, !isRetry, let auth, await auth.refreshAccessToken() {
                return try await perform(request, isRetry: true)
            }
            if request.authenticated { await auth?.handleAuthenticationLost() }
            throw APIError.unauthorized
        case 403:
            throw APIError.forbidden
        case 404:
            throw APIError.notFound
        case 429:
            let retryAfter = http.value(forHTTPHeaderField: "Retry-After").flatMap(TimeInterval.init)
            throw APIError.rateLimited(retryAfter: retryAfter)
        default:
            let envelope = try? Self.decoder().decode(APIErrorEnvelope.self, from: data)
            throw APIError.server(status: http.statusCode,
                                  code: envelope?.error.code,
                                  message: envelope?.error.message)
        }
    }

    private func buildURLRequest(_ request: Request) async throws -> URLRequest {
        var base = baseURL.absoluteString
        if base.hasSuffix("/") { base.removeLast() }
        let path = request.path.hasPrefix("/") ? request.path : "/" + request.path
        guard var components = URLComponents(string: base + path) else {
            throw APIError.invalidResponse
        }
        if !request.query.isEmpty {
            components.queryItems = request.query
                .sorted { $0.key < $1.key }
                .map { URLQueryItem(name: $0.key, value: $0.value) }
        }
        guard let url = components.url else { throw APIError.invalidResponse }

        var req = URLRequest(url: url)
        req.httpMethod = request.method
        req.timeoutInterval = 30
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let body = request.body {
            req.httpBody = try Self.encoder().encode(AnyEncodable(body))
        }
        if request.authenticated, let token = await auth?.currentAccessToken() {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return req
    }

    // MARK: - Coders

    /// The API uses snake_case keys and RFC3339 timestamps. A fresh decoder per
    /// call keeps the client free of shared mutable state.
    static func decoder() -> JSONDecoder {
        let d = JSONDecoder()
        d.keyDecodingStrategy = .convertFromSnakeCase
        d.dateDecodingStrategy = .custom { decoder in
            let raw = try decoder.singleValueContainer().decode(String.self)
            if let date = iso8601Fractional.date(from: raw) ?? iso8601Plain.date(from: raw) {
                return date
            }
            throw DecodingError.dataCorruptedError(
                in: try decoder.singleValueContainer(),
                debugDescription: "unrecognized date: \(raw)")
        }
        return d
    }

    static func encoder() -> JSONEncoder {
        let e = JSONEncoder()
        e.keyEncodingStrategy = .convertToSnakeCase
        e.dateEncodingStrategy = .iso8601
        return e
    }

    private static let iso8601Fractional: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()

    private static let iso8601Plain: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f
    }()
}

/// Type-erased Encodable so a request body can be any Encodable value without the
/// client being generic over it.
struct AnyEncodable: Encodable {
    private let encodeFunc: (Encoder) throws -> Void
    init(_ wrapped: Encodable) { encodeFunc = wrapped.encode }
    func encode(to encoder: Encoder) throws { try encodeFunc(encoder) }
}

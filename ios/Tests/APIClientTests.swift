import XCTest
@testable import SysOps

/// A URLProtocol that returns whatever the current handler produces, so the API
/// client is exercised end-to-end without touching the network.
final class MockURLProtocol: URLProtocol {
    static var handler: ((URLRequest) throws -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        guard let handler = MockURLProtocol.handler else {
            client?.urlProtocol(self, didFailWithError: URLError(.badServerResponse))
            return
        }
        do {
            let (response, data) = try handler(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override func stopLoading() {}
}

/// A stand-in auth provider for exercising the refresh-on-401 path.
final class FakeAuth: AuthProviding {
    var accessToken: String? = "old"
    var refreshCount = 0
    var refreshSucceeds = true
    var authLost = false

    func currentAccessToken() async -> String? { accessToken }
    func refreshAccessToken() async -> Bool {
        refreshCount += 1
        if refreshSucceeds { accessToken = "new" }
        return refreshSucceeds
    }
    func handleAuthenticationLost() async { authLost = true }
}

final class APIClientTests: XCTestCase {
    override func tearDown() {
        MockURLProtocol.handler = nil
        super.tearDown()
    }

    private func makeClient(auth: AuthProviding? = nil) -> APIClient {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [MockURLProtocol.self]
        return APIClient(baseURL: URL(string: "https://example.test")!,
                         auth: auth,
                         session: URLSession(configuration: config))
    }

    private func ok(_ req: URLRequest, _ json: String) -> (HTTPURLResponse, Data) {
        (HTTPURLResponse(url: req.url!, statusCode: 200, httpVersion: nil, headerFields: nil)!, Data(json.utf8))
    }

    func testDecodesPaginatedMonitorsWithSnakeCaseAndStatus() async throws {
        MockURLProtocol.handler = { req in
            self.ok(req, """
            {"data":[{"id":"1","name":"api","type":"https","target":"https://x",
            "enabled":true,"last_status":"up","last_checked_at":null,"project_id":null}],
            "pagination":{"total":1,"limit":100,"offset":0}}
            """)
        }
        let page = try await makeClient().send(
            .init(path: "/api/v1/monitors", authenticated: false), as: Paginated<Monitor>.self)
        XCTAssertEqual(page.data.first?.name, "api")
        XCTAssertEqual(page.data.first?.status, .up)
    }

    func testServerErrorEnvelopeSurfacesMessage() async {
        MockURLProtocol.handler = { req in
            (HTTPURLResponse(url: req.url!, statusCode: 422, httpVersion: nil, headerFields: nil)!,
             Data(#"{"error":{"code":"validation","message":"bad input"}}"#.utf8))
        }
        do {
            _ = try await makeClient().send(.init(path: "/x", authenticated: false), as: Monitor.self)
            XCTFail("expected an error")
        } catch let APIError.server(status, code, message) {
            XCTAssertEqual(status, 422)
            XCTAssertEqual(code, "validation")
            XCTAssertEqual(message, "bad input")
        } catch {
            XCTFail("wrong error: \(error)")
        }
    }

    func testRateLimitedParsesRetryAfter() async {
        MockURLProtocol.handler = { req in
            (HTTPURLResponse(url: req.url!, statusCode: 429, httpVersion: nil,
                             headerFields: ["Retry-After": "30"])!, Data())
        }
        do {
            _ = try await makeClient().send(.init(path: "/x", authenticated: false), as: Monitor.self)
            XCTFail("expected an error")
        } catch let APIError.rateLimited(retryAfter) {
            XCTAssertEqual(retryAfter, 30)
        } catch {
            XCTFail("wrong error: \(error)")
        }
    }

    func testRefreshesOnceOn401ThenRetries() async throws {
        var calls = 0
        MockURLProtocol.handler = { req in
            calls += 1
            if calls == 1 {
                return (HTTPURLResponse(url: req.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!, Data())
            }
            return self.ok(req, """
            {"id":"1","name":"api","type":"https","target":"t","enabled":true,
            "last_status":"up","last_checked_at":null,"project_id":null}
            """)
        }
        let auth = FakeAuth()
        let monitor = try await makeClient(auth: auth).send(.init(path: "/api/v1/monitors/1"), as: Monitor.self)
        XCTAssertEqual(monitor.name, "api")
        XCTAssertEqual(auth.refreshCount, 1)
    }

    func testGivesUpAndReportsAuthLostWhenRefreshFails() async {
        MockURLProtocol.handler = { req in
            (HTTPURLResponse(url: req.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!, Data())
        }
        let auth = FakeAuth()
        auth.refreshSucceeds = false
        do {
            _ = try await makeClient(auth: auth).send(.init(path: "/api/v1/monitors/1"), as: Monitor.self)
            XCTFail("expected an error")
        } catch let error as APIError {
            XCTAssertEqual(error, .unauthorized)
            XCTAssertTrue(auth.authLost)
        } catch {
            XCTFail("wrong error: \(error)")
        }
    }
}

package com.beacon.app

import kotlinx.coroutines.runBlocking
import kotlinx.serialization.encodeToString
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ApiTest {
    private val server = MockWebServer()

    @After
    fun stop() = server.shutdown()

    private fun auth(access: String, refresh: String) =
        """{"access_token":"$access","refresh_token":"$refresh","token_type":"Bearer","expires_in":900,""" +
            """"user":{"id":"u1","org_id":"o1","email":"a@example.com","name":"A","role":"owner"}}"""

    /** A signed-in session (access "old", refresh "r1") and the authenticated client, both on [server]. */
    private fun signedIn(): Pair<Session, Api> {
        server.start()
        val base = server.url("/").toString()
        val http = baseHttpClient()
        var saved: String? = null
        val session = Session(Api(base, http), { saved }, { saved = it })
        session.adopt(decode(auth("old", "r1")))
        return session to Api(base, authenticatedClient(http, session))
    }

    @Test
    fun `a 401 refreshes once and retries with the new token`() = runBlocking {
        val (session, api) = signedIn()
        server.enqueue(MockResponse().setResponseCode(401))
        server.enqueue(MockResponse().setBody(auth("new", "r2")))
        server.enqueue(MockResponse().setBody("""{"data":[{"id":"p1","name":"Web"}],"pagination":{"total":1}}"""))

        val projects = api.get<Page<Project>>("/api/v1/projects").data

        assertEquals("Web", projects.single().name)
        assertEquals("Bearer old", server.takeRequest().getHeader("Authorization"))
        val refresh = server.takeRequest()
        assertEquals("/api/v1/auth/refresh", refresh.path)
        assertEquals("""{"refresh_token":"r1"}""", refresh.body.readUtf8())
        assertEquals("Bearer new", server.takeRequest().getHeader("Authorization"))
        assertEquals("new", session.accessToken)
    }

    @Test
    fun `a refused refresh signs out`() = runBlocking {
        val (session, api) = signedIn()
        server.enqueue(MockResponse().setResponseCode(401))
        server.enqueue(MockResponse().setResponseCode(401).setBody("""{"error":{"code":"unauthorized","message":"session expired"}}"""))

        val error = runCatching { api.get<Page<Project>>("/api/v1/projects") }.exceptionOrNull() as ApiError

        assertEquals(401, error.status)
        assertNull(session.user.value)
        assertNull(session.accessToken)
    }

    @Test
    fun `server messages reach the person, 5xx details do not`() = runBlocking {
        val (_, api) = signedIn()
        server.enqueue(MockResponse().setResponseCode(422).setBody("""{"error":{"code":"validation","message":"Target must be a URL"}}"""))
        server.enqueue(MockResponse().setResponseCode(500).setBody("""{"error":{"message":"pq: relation missing"}}"""))

        assertEquals("Target must be a URL", runCatching { api.send("POST", "/api/v1/monitors") }.exceptionOrNull()?.message)
        assertEquals("Something went wrong on the server. Please try again.", runCatching { api.send("POST", "/api/v1/monitors") }.exceptionOrNull()?.message)
    }

    @Test
    fun `editing a monitor keeps settings the form doesn't show and drops cleared ones`() {
        val monitor = decode<Monitor>(
            """{"id":"m1","name":"Site","type":"https","target":"https://x","last_checked_at":"2026-11-11T10:00:00.123456Z",""" +
                """"settings":{"dns_expected_ips":["1.2.3.4"],"method":"POST","headers":{"Authorization":"********"}}}""",
        )
        val edited = monitor.settings.with(
            mapOf(
                "method" to null, // back to the default GET
                "valid_status_codes" to parseStatusCodes("200, 204, x"),
                "headers" to parseHeaders(headerLines(monitor.settings)),
            ),
        )

        assertEquals(
            """{"name":"Site","enabled":true,"interval_seconds":60,"timeout_seconds":10,""" +
                """"settings":{"dns_expected_ips":["1.2.3.4"],"headers":{"Authorization":"********"},"valid_status_codes":[200,204]}}""",
            json.encodeToString(UpdateMonitorBody("Site", null, true, 60, 10, edited)),
        )
        assertEquals("Bearer a:b", parseHeaders("Authorization: Bearer a:b\n: no name\nnot a header")?.str("Authorization"))
        assertNull(parseStatusCodes(" , x"))
    }
}

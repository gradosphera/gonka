package com.productscience.mockserver

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import io.ktor.server.testing.*
import kotlin.test.*
import org.assertj.core.api.Assertions.assertThat
import com.productscience.mockserver.service.ResponseService
import com.productscience.mockserver.ResponseServiceKey

class ApplicationTest {
    @Test
    fun testStatusEndpoint() = testApplication {
        application {
            module()
        }
        
        val response = client.get("/status")
        
        assertEquals(HttpStatusCode.OK, response.status)
        val responseText = response.bodyAsText()
        
        // Verify the response contains expected fields
        assertThat(responseText).contains("status")
        assertThat(responseText).contains("ok")
        assertThat(responseText).contains("version")
        assertThat(responseText).contains("timestamp")
    }

    @Test
    fun testChatCompletionsErrorResponse() = testApplication {
        application {
            module()
            
            // Get the ResponseService from the application and configure error response
            val responseService = attributes[ResponseServiceKey]
            responseService.setInferenceErrorResponse(
                statusCode = 500,
                errorMessage = "Internal server error occurred",
                errorType = "server_error"
            )
        }

        // Make request to chat completions endpoint
        val response = client.post("/v1/chat/completions") {
            contentType(ContentType.Application.Json)
            setBody("""
                {
                    "model": "test-model",
                    "messages": [
                        {"role": "user", "content": "Hello"}
                    ]
                }
            """.trimIndent())
        }

        // Verify error response
        assertEquals(HttpStatusCode.InternalServerError, response.status)
        val responseText = response.bodyAsText()
        
        // Verify the error response structure
        assertThat(responseText).contains("error")
        assertThat(responseText).contains("Internal server error occurred")
        assertThat(responseText).contains("server_error")
        assertThat(responseText).contains("500")
    }

    @Test
    fun testChatCompletions404ErrorResponse() = testApplication {
        application {
            module()
            
            // Configure 404 error response for chat completions
            val responseService = attributes[ResponseServiceKey]
            responseService.setInferenceErrorResponse(
                statusCode = 404,
                errorMessage = "Model not found"
            )
        }

        // Make request to chat completions endpoint
        val response = client.post("/v1/chat/completions") {
            contentType(ContentType.Application.Json)
            setBody("""
                {
                    "model": "nonexistent-model",
                    "messages": [
                        {"role": "user", "content": "Hello"}
                    ]
                }
            """.trimIndent())
        }

        // Verify error response
        assertEquals(HttpStatusCode.NotFound, response.status)
        val responseText = response.bodyAsText()
        
        // Verify the error response structure
        assertThat(responseText).contains("error")
        assertThat(responseText).contains("Model not found")
        assertThat(responseText).contains("invalid_request_error")
        assertThat(responseText).contains("404")
    }
}
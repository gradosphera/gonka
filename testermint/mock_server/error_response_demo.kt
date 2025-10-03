package com.productscience.mockserver

import com.productscience.mockserver.service.ResponseService
import com.productscience.mockserver.model.ErrorResponse

/**
 * Demo script showing how to use the new error response functionality.
 * This demonstrates the key features added to resolve the issue.
 */
fun main() {
    println("=== Mock Server Error Response Demo ===")
    println()
    
    // Create a ResponseService instance
    val responseService = ResponseService()
    
    // Example 1: Configure a 500 Internal Server Error
    println("1. Configuring 500 Internal Server Error:")
    val endpoint1 = responseService.setInferenceErrorResponse(
        statusCode = 500,
        errorMessage = "Internal server error occurred",
        errorType = "server_error",
        delay = 1000 // 1 second delay
    )
    println("   Configured error response for endpoint: $endpoint1")
    
    // Example 2: Configure a 404 Not Found Error with default message
    println("\n2. Configuring 404 Not Found Error (with default message):")
    val endpoint2 = responseService.setInferenceErrorResponse(
        statusCode = 404,
        segment = "api" // This will configure /api/v1/chat/completions
    )
    println("   Configured error response for endpoint: $endpoint2")
    
    // Example 3: Configure a 429 Too Many Requests Error
    println("\n3. Configuring 429 Too Many Requests Error:")
    val endpoint3 = responseService.setInferenceErrorResponse(
        statusCode = 429,
        errorMessage = "Rate limit exceeded. Please try again later.",
        errorType = "rate_limit_error",
        delay = 500
    )
    println("   Configured error response for endpoint: $endpoint3")
    
    // Show what the error responses look like
    println("\n=== Error Response Examples ===")
    
    println("\n1. 500 Error Response JSON:")
    val error500 = ErrorResponse(500, "Internal server error occurred", "server_error")
    println(error500.toJsonBody())
    
    println("\n2. 404 Error Response JSON (default message):")
    val error404 = ErrorResponse(404)
    println(error404.toJsonBody())
    
    println("\n3. 429 Error Response JSON:")
    val error429 = ErrorResponse(429, "Rate limit exceeded. Please try again later.", "rate_limit_error")
    println(error429.toJsonBody())
    
    println("\n=== Usage Instructions ===")
    println("To use the error response feature:")
    println("1. Get a ResponseService instance")
    println("2. Call setInferenceErrorResponse() with desired status code and optional parameters")
    println("3. Make requests to the configured chat completion endpoints")
    println("4. The mock server will return the configured error response with the specified HTTP status code")
    println()
    println("Available parameters:")
    println("- statusCode: HTTP status code (required)")
    println("- errorMessage: Custom error message (optional, defaults provided)")
    println("- errorType: Custom error type (optional, defaults provided)")
    println("- delay: Delay in milliseconds before responding (optional, default 0)")
    println("- segment: URL segment prefix (optional, default empty)")
    println()
    println("This solves the issue: 'I need the mock server to have a setting to return invalid responses for chat completions... no option for it to, say, return a 500 error.'")
}
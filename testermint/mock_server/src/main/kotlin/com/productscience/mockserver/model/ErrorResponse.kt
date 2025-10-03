package com.productscience.mockserver.model

/**
 * Data class representing an error response configuration.
 * This allows the mock server to return HTTP error responses with custom status codes and messages.
 */
data class ErrorResponse(
    val statusCode: Int,
    val errorMessage: String? = null,
    val errorType: String? = null
) {
    /**
     * Generates the error response body as a JSON string.
     */
    fun toJsonBody(): String {
        val message = errorMessage ?: getDefaultErrorMessage(statusCode)
        val type = errorType ?: getDefaultErrorType(statusCode)
        
        return """
            {
              "error": {
                "message": "$message",
                "type": "$type",
                "code": $statusCode
              }
            }
        """.trimIndent()
    }
    
    private fun getDefaultErrorMessage(code: Int): String {
        return when (code) {
            400 -> "Bad Request"
            401 -> "Unauthorized"
            403 -> "Forbidden"
            404 -> "Not Found"
            429 -> "Too Many Requests"
            500 -> "Internal Server Error"
            502 -> "Bad Gateway"
            503 -> "Service Unavailable"
            else -> "Error"
        }
    }
    
    private fun getDefaultErrorType(code: Int): String {
        return when {
            code in 400..499 -> "invalid_request_error"
            code in 500..599 -> "server_error"
            else -> "error"
        }
    }
}
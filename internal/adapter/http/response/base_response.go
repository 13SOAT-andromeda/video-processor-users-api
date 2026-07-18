package response

import "github.com/gin-gonic/gin"

type SuccessResponse[T any] struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func RespondSuccess[T any](c *gin.Context, statusCode int, data T, message string) {
	c.JSON(statusCode, SuccessResponse[T]{Status: "success", Message: message, Data: data})
}

func RespondCreated[T any](c *gin.Context, data T) {
	c.JSON(201, data)
}

func RespondError(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, ErrorResponse{Status: "error", Code: code, Message: message})
}

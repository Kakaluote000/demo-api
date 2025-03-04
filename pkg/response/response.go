package response

// Response 基础响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Response
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Response
}

// NewSuccess 创建成功响应
func NewSuccess(data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Response: Response{
			Code:    200,
			Message: "success",
			Data:    data,
		},
	}
}

// NewError 创建错误响应
func NewError(code int, message string) *ErrorResponse {
	return &ErrorResponse{
		Response: Response{
			Code:    code,
			Message: message,
		},
	}
}

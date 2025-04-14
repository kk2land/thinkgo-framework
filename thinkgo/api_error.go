package thinkgo

import "fmt"

// ApiError 接口请求错误，将返回给客户端的信息和真实的错误信息区分开来
type ApiError struct {
	apiMessage string
	message    string
	code       int
	noStack    bool
}

func NewApiError(apiMessage string) *ApiError {
	return NewApiError1(apiMessage, apiMessage)
}

func NewApiError1(apiMessage string, msg string) *ApiError {
	return &ApiError{
		apiMessage: apiMessage,
		message:    msg,
		code:       0,
	}
}

func NewApiError1f(apiMessage string, msg string, a ...interface{}) *ApiError {
	return &ApiError{
		apiMessage: apiMessage,
		message:    fmt.Sprintf(msg, a...),
		code:       0,
	}
}

func NewApiError2(apiMessage string, msg string, code int) *ApiError {
	return &ApiError{
		apiMessage: apiMessage,
		message:    msg,
		code:       code,
	}
}

func (err *ApiError) Error() string {
	return err.message
}

func (err *ApiError) ApiMessage() string {
	return err.apiMessage
}

func (err *ApiError) Code() int {
	return err.code
}

func (err *ApiError) SetNoStack(f bool) *ApiError {
	err.noStack = f
	return err
}

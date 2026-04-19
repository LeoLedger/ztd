package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const (
	CodeSuccess           = 0
	CodeSignatureInvalid  = 4001
	CodeParamInvalid      = 4002
	CodeParseFailed       = 4003
	CodeSystemError       = 5001
)

func Success(c http.ResponseWriter, data interface{}) {
	JSON(c, http.StatusOK, Response{Code: CodeSuccess, Message: "success", Data: data})
}

func Error(c http.ResponseWriter, httpStatus int, code int, message string) {
	JSON(c, httpStatus, Response{Code: code, Message: message})
}

func SuccessWithMessage(c http.ResponseWriter, message string, data interface{}) {
	JSON(c, http.StatusOK, Response{Code: CodeSuccess, Message: message, Data: data})
}

func BadRequest(c http.ResponseWriter, message string) {
	Error(c, http.StatusBadRequest, CodeParamInvalid, message)
}

func Unauthorized(c http.ResponseWriter, message string) {
	Error(c, http.StatusUnauthorized, CodeSignatureInvalid, message)
}

func InternalError(c http.ResponseWriter, message string) {
	Error(c, http.StatusInternalServerError, CodeSystemError, message)
}

func JSON(c http.ResponseWriter, status int, v interface{}) {
	c.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.WriteHeader(status)
	enc := json.NewEncoder(c)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		_ = err
	}
}

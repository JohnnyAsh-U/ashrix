package dto

import (
	"fmt"
	// "net/http"
)

type ErrorCode string

const (
	CodeNotFound        ErrorCode = "NOT_FOUND"
	CodeConflict        ErrorCode = "CONFLICT"
	CodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	CodeForbidden       ErrorCode = "FORBIDDEN"
	CodeBadRequest      ErrorCode = "BAD_REQUEST"
	CodeValidationError ErrorCode = "VALIDATION_ERROR"
	CodeInternal        ErrorCode = "INTERNAL_ERROR"
)

// AppError defines a standardized internal application error
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Status  int
	Details any `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewErrInternal(details any) *AppError {
	m := "Something went wrong on our end"
	return &AppError{Code: CodeInternal, Message: m, Details: details, Status: 500}
}

func NewBadRequestError(details any) *AppError {
	m := "The Request payload is invalid"
	return &AppError{Code: CodeBadRequest, Message: m, Details: details, Status: 400}
}

func NewUnauthorizedError(details any) *AppError {
	m := "Authentication is required to access this resource"
	return &AppError{Code: CodeUnauthorized, Message: m, Details: details, Status: 401}
}

func NewForbiddenError(details any) *AppError {
	m := "You do not have permission to perform this action"
	return &AppError{Code: CodeForbidden, Message: m, Details: details, Status: 403}
}

func NewNotFoundError(details any) *AppError {
	m := "The requested resource was not found"
	return &AppError{Code: CodeNotFound, Message: m, Details: details, Status: 404}
}

// New AppError allows creating custom errors on the fly
func NewAppError(status int, code ErrorCode, message string, details any) *AppError {
	return &AppError{
		Status:  status,
		Code:    code,
		Message: message,
		Details: details,
	}
}

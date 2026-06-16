package dto

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

var Validate = validator.New()

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) *AppError {
	const maxBytes = 1 << 20 //1 MB

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return NewBadRequestError(err.Error())
	}
	return nil
}

func ValidateStruct(dst any) []*ValidationErrorDetail {

	err := Validate.Struct(dst)

	if err == nil {
		return nil
	}

	var errors []*ValidationErrorDetail

	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ve {
			detail := &ValidationErrorDetail{
				Field:   fe.Field(),
				Tag:     fe.Tag(),
				Value:   fmt.Sprintf("%v", fe.Value()),
				Message: fmt.Sprintf("Field '%s' failed on the '%s' tag", fe.Field(), fe.Tag()),
			}
			errors = append(errors, detail)
		}
	}
	return errors
}

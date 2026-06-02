package errors

import (
	"encoding/json"
	"time"
)

// MarshalJSON implements json.Marshaler for Error
func (e *errorImpl) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Code       string    `json:"code"`
		Message    string    `json:"message"`
		HTTPStatus int       `json:"http_status,omitempty"`
		Details    []Detail  `json:"details,omitempty"`
		Timestamp  time.Time `json:"timestamp"`
		RequestID  string    `json:"request_id,omitempty"`
		Service    string    `json:"service,omitempty"`
	}

	return json.Marshal(jsonError{
		Code:       e.code.String(),
		Message:    e.message,
		HTTPStatus: e.httpStatus,
		Details:    e.details,
		Timestamp:  e.timestamp,
		RequestID:  e.requestID,
		Service:    e.service,
	})
}

// UnmarshalJSON implements json.Unmarshaler for Error
func (e *errorImpl) UnmarshalJSON(data []byte) error {
	type jsonError struct {
		Code       string        `json:"code"`
		Message    string        `json:"message"`
		HTTPStatus int           `json:"http_status,omitempty"`
		Details    []*detailImpl `json:"details,omitempty"`
		Timestamp  time.Time     `json:"timestamp"`
		RequestID  string        `json:"request_id,omitempty"`
		Service    string        `json:"service,omitempty"`
	}

	var je jsonError
	if err := json.Unmarshal(data, &je); err != nil {
		return err
	}

	e.code = Code(je.Code)
	e.message = je.Message
	e.httpStatus = je.HTTPStatus

	e.details = make([]Detail, len(je.Details))
	for i, d := range je.Details {
		e.details[i] = d
	}

	e.timestamp = je.Timestamp
	e.requestID = je.RequestID
	e.service = je.Service

	return nil
}

// MarshalJSON implements json.Marshaler for Detail
func (d *detailImpl) MarshalJSON() ([]byte, error) {
	type jsonDetail struct {
		Field   string      `json:"field,omitempty"`
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Value   interface{} `json:"value,omitempty"`
	}

	return json.Marshal(jsonDetail{
		Field:   d.field,
		Code:    d.code.String(),
		Message: d.message,
		Value:   d.value,
	})
}

// UnmarshalJSON implements json.Unmarshaler for Detail
func (d *detailImpl) UnmarshalJSON(data []byte) error {
	type jsonDetail struct {
		Field   string      `json:"field,omitempty"`
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Value   interface{} `json:"value,omitempty"`
	}

	var jd jsonDetail
	if err := json.Unmarshal(data, &jd); err != nil {
		return err
	}

	d.field = jd.Field
	d.code = Code(jd.Code)
	d.message = jd.Message
	d.value = jd.Value

	return nil
}

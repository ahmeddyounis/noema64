package appsvc

type AppError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Recoverable bool              `json:"recoverable"`
	Details     map[string]string `json:"details,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func appErr(code string, err error, recoverable bool) error {
	if err == nil {
		return nil
	}
	return &AppError{Code: code, Message: err.Error(), Recoverable: recoverable}
}

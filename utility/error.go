package utility

type AppError struct {
	message string
}

func (e *AppError) Error() string {
	return e.message
}

func Err(m string) error {
	return &AppError{m}
}

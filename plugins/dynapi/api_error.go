package dynapi

type apiError struct {
	code    string
	message string
	status  int
}

func (apiError *apiError) Error() string {
	return apiError.message
}

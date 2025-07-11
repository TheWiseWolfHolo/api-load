package types

// RetryError captures detailed information about a failed request attempt during retries.
type RetryError struct {
	StatusCode   int    `json:"status_code"`
	ErrorMessage string `json:"error_message"`
	KeyID        string `json:"key_id"`
	Attempt      int    `json:"attempt"`
}

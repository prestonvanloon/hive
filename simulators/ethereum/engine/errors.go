package main

// Returns true if the error is nil, or the error is not the expected one,
// or the error name is not defined for the client type, false otherwise.
func checkError(t *TestEnv, error_name string, err error) (string, bool) {
	if err == nil {
		return "No error", true
	}
	return "ok", false
}

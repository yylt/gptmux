package response

// BackendData is the normalized backend output before protocol conversion.
type BackendData struct {
	Err     error
	Content string
	Cookie  map[string]string
}

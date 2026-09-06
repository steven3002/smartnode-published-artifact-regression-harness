package compose

// ExtractImages returns a map of service name to the image tag it uses.
func (s *Stack) ExtractImages() map[string]string {
	images := make(map[string]string)
	for name, service := range s.Services {
		if service.Image != "" {
			images[name] = service.Image
		}
	}
	return images
}

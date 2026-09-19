package k8s

import "strings"

// parseImageRef splits a reference like "naya-api:latest" or "registry.io/app:v1".
func parseImageRef(image string) (repository, tag string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", "latest"
	}
	if _, _, ok := strings.Cut(image, "@"); ok {
		// digest reference — use full string as repository
		return image, ""
	}
	if i := strings.LastIndex(image, ":"); i > 0 {
		repo := image[:i]
		tagPart := image[i+1:]
		if tagPart != "" && !strings.Contains(tagPart, "/") {
			return repo, tagPart
		}
	}
	return image, "latest"
}

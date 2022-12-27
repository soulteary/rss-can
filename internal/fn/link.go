package fn

import (
	"net/url"
)

func LinkResolveRelative(resolveUrl string, baseUrl string) (string, error) {
	base, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}

	ref, err := url.Parse(resolveUrl)
	if err != nil {
		return "", err
	}

	url := base.ResolveReference(ref)
	return url.String(), nil
}

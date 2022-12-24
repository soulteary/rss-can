package fn

import "encoding/base64"

func Base64Encode(input string) string {
	return base64.URLEncoding.EncodeToString([]byte(input))
}

func Base64Decode(input string) string {
	decode, err := base64.URLEncoding.DecodeString(input)
	if err != nil {
		return ""
	}
	return string(decode)
}

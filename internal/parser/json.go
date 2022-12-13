package parser

import (
	"encoding/json"
)

func JSONStringify(r interface{}) (string, error) {
	out, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

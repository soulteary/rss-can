package fn

import "encoding/json"

func JSONStringify(r interface{}) string {
	out, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(out)
}

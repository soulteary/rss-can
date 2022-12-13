package define

import "time"

type RemoteBodySanitized struct {
	Code   ErrorCode `json:"code"`
	Status string    `json:"status"`
	Date   time.Time `json:"date"`
	Body   string    `json:"data,omitempty"`
}

func MixupRemoteBodySanitized(code ErrorCode, status string, date time.Time, data string) (result RemoteBodySanitized) {
	result.Code = code
	result.Status = status
	result.Date = date
	result.Body = data
	return result
}

type BodyParsed struct {
	Code   ErrorCode  `json:"code"`
	Status string     `json:"status"`
	Date   time.Time  `json:"date"`
	Body   []InfoItem `json:"data,omitempty"`
}

func MixupBodyParsed(code ErrorCode, status string, date time.Time, data []InfoItem) (result BodyParsed) {
	result.Code = code
	result.Status = status
	result.Date = date
	result.Body = data
	return result
}

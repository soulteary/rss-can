package define

const (
	ERROR_STATUS_NULL                  string = "一切正常"
	ERROR_STATUS_NETWORK               string = "网络故障"
	ERROR_STATUS_INIT_NETWORK_FAILED   string = "初始化网络组件错误"
	ERROR_STATUS_API_NOT_READY         string = "目标接口未就绪"
	ERROR_STATUS_DECODE_CAHRSET_FAILED string = "解析内容编码出错"
	ERROR_STATUS_PARSE_CONTENT_FAILED  string = "解析文档出错"
)

type ErrorCode int

const (
	ERROR_CODE_NULL                  ErrorCode = 0
	ERROR_CODE_NETWORK               ErrorCode = 1
	ERROR_CODE_INIT_NETWORK_FAILED   ErrorCode = 2
	ERROR_CODE_API_NOT_READY         ErrorCode = 3
	ERROR_CODE_DECODE_CAHRSET_FAILED ErrorCode = 4
	ERROR_CODE_PARSE_CONTENT_FAILED  ErrorCode = 5
)

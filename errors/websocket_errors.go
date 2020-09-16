package errors

const (
	WSErrorOK           = 0
	WSErrorTokenInvalid = 10001
)

var WSErrorToString = map[int]string{
	WSErrorOK:           "",
	WSErrorTokenInvalid: "token invalid",
}

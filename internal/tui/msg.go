package tui

type MsgType int

const (
	MsgLoading MsgType = iota
	MsgThought
	MsgResponse
	MsgCommit
)

var stateName = map[MsgType]string{
	MsgLoading:  "loading",
	MsgThought:  "thinking",
	MsgResponse: "response",
	MsgCommit:   "done",
}

func (ss MsgType) String() string {
	return stateName[ss]
}

type Msg struct {
	Text string
	Type MsgType
}

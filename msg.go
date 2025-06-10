package main

type MsgType int

const (
	MsgLoading MsgType = iota
	MsgDiff
	MsgThought
	MsgResponse
	MsgDone
)

var stateName = map[MsgType]string{
	MsgLoading:  "loading",
	MsgThought:  "thinking",
	MsgResponse: "response",
	MsgDone:     "done",
}

func (ss MsgType) String() string {
	return stateName[ss]
}

type Msg struct {
	text string
	kind MsgType
}

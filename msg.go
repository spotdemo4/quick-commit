package main

type MsgType int

const (
	MsgLoading MsgType = iota
	MsgDiff
	MsgThought
	MsgCommit
	MsgDone
)

var stateName = map[MsgType]string{
	MsgLoading: "loading",
	MsgDiff:    "diff",
	MsgThought: "thinking",
	MsgCommit:  "commit",
	MsgDone:    "done",
}

func (ss MsgType) String() string {
	return stateName[ss]
}

type Msg struct {
	text string
	kind MsgType
}

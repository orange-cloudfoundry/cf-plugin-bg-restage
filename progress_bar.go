package main

import (
	"fmt"
	"io"
)

type IndeterminateProgressBar struct {
	state          string
	loadingMessage string
	writer         io.Writer
}

func NewIndeterminateProgressBar(writer io.Writer, loadingMessage string) *IndeterminateProgressBar {
	ipb := &IndeterminateProgressBar{"|", loadingMessage, writer}
	ipb.write()
	return ipb
}
func (this *IndeterminateProgressBar) Next() {
	var nextState string
	switch this.state {
	case "|":
		nextState = "/"
		break
	case "/":
		nextState = "-"
		break
	case "-":
		nextState = "\\"
		break
	default:
		nextState = "|"
	}
	this.state = nextState
	this.write()
}
func (this *IndeterminateProgressBar) write() {
	fmt.Fprint(this.writer, this.state+" "+this.loadingMessage+"\r")
}

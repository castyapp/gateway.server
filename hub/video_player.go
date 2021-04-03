package hub

import "github.com/castyapp/libcasty-protocol-go/proto"

type VideoPlayer struct {
	t       *Timer
	Theater *proto.Theater
}

func NewVideoPlayer() *VideoPlayer {
	timer := &Timer{
		sendC:     make(chan float32),
		startChan: make(chan struct{}),
		closeChan: make(chan struct{}),
	}
	go timer.Listen()
	return &VideoPlayer{t: timer}
}

func (vp *VideoPlayer) InProgress() bool {
	return vp.t.inProgress
}

func (vp *VideoPlayer) SetCurrentTime(currentTime float32) {
	vp.t.ct = currentTime
}

func (vp *VideoPlayer) Timer() *Timer {
	return vp.t
}

func (vp *VideoPlayer) CurrentTime() float32 {
	return vp.t.ct
}

func (vp *VideoPlayer) CurrentTimeChan() <-chan float32 {
	return vp.t.sendC
}

func (vp *VideoPlayer) Play() {
	go vp.t.start()
}

func (vp *VideoPlayer) Pause() {
	vp.t.pause()
}

func (vp *VideoPlayer) End() {
	vp.t.end()
}

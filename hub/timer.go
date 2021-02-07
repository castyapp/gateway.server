package hub

import (
	"time"
)

type Timer struct {
	ct         float32
	sendC      chan float32
	closeChan  chan struct{}
	startChan  chan struct{}
	inProgress bool
	ticker     *time.Ticker
}

func (t *Timer) Listen() {
	if !t.inProgress {
		for {
			select {
			case <-t.startChan:
				t.inProgress = true
				t.ticker = time.NewTicker(time.Second)
				go func() {
					for {
						select {
						case <-t.ticker.C:
							newVal := t.ct + 1
							t.ct = newVal
							//log.Println(newVal)
							go func() {
								t.sendC <- newVal
							}()
						}
					}
				}()
			case <-t.closeChan:
				if t.inProgress {
					t.inProgress = false
					t.ticker.Stop()
					t.ticker = nil
				}
			}
		}
	}
}

func (t *Timer) start() {
	t.startChan <- struct{}{}
}

func (t *Timer) pause() {
	t.closeChan <- struct{}{}
}

func (t *Timer) end() {
	t.pause()
	t.ct = 0
}

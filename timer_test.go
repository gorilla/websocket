package websocket

import (
	"testing"
	"time"
)

func BenchmarkTimerNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timer := time.NewTimer(0)
		<-timer.C
	}
}

func BenchmarkTimerReset(b *testing.B) {
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for i := 0; i < b.N; i++ {
		timer.Reset(0)
		<-timer.C
	}
}

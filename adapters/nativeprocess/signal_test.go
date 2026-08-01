package nativeprocess

import (
	"os"
	"os/signal"
)

func signalNotifyInterrupt(signals chan<- os.Signal) {
	signal.Notify(signals, os.Interrupt)
}

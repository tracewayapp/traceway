package main

import (
	"github.com/tracewayapp/traceway/backend/v2/cmd"
)

func main() {
	go cmd.ShutdownOnSignal()
	cmd.RunStandalone()
}

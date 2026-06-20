package main

import (
	"math/rand/v2"
	"time"
)

func fastPath() bool {
	return rand.IntN(4) == 0
}

func slowJitter(minMs, maxMs int) {
	d := minMs
	if maxMs > minMs {
		d += rand.IntN(maxMs - minMs)
	}
	time.Sleep(time.Duration(d) * time.Millisecond)
}

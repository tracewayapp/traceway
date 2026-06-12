package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type corpus struct {
	Urls []string `json:"urls"`
}

func main() {
	bundle := flag.String("bundle", "../../testing/symbolication/node-app/dist/app.mjs", "")
	mapFile := flag.String("map", "../../testing/symbolication/node-app/dist/app.mjs.map", "")
	entries := flag.Int("entries", 1, "")
	padKB := flag.Int("pad-kb", 256, "")
	out := flag.String("out", "./corpus", "")
	flag.Parse()

	bundleBytes, err := os.ReadFile(*bundle)
	if err != nil {
		panic(err)
	}
	mapBytes, err := os.ReadFile(*mapFile)
	if err != nil {
		panic(err)
	}
	firstLine := strings.SplitN(string(bundleBytes), "\n", 2)[0]

	var pad strings.Builder
	chunk := "function __benchPad%d(a,b){var c=a*b+%d;for(var i=0;i<3;i++){c+=i*a-b}return c}\n"
	i := 0
	for pad.Len() < *padKB*1024 {
		pad.WriteString(fmt.Sprintf(chunk, i, i))
		i++
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	c := corpus{}
	for n := 0; n < *entries; n++ {
		name := fmt.Sprintf("app%d.mjs", n)
		content := firstLine + "\n" + pad.String() + "//# sourceMappingURL=" + name + ".map\n"
		if err := os.WriteFile(filepath.Join(*out, name), []byte(content), 0o644); err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(*out, name+".map"), mapBytes, 0o644); err != nil {
			panic(err)
		}
		c.Urls = append(c.Urls, name)
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "corpus.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d entries (%d KB padding each) to %s\n", *entries, *padKB, *out)
}

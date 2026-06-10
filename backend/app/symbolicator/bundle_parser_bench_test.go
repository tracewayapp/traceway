package symbolicator

import (
	"fmt"
	"strings"
	"testing"
)

var benchBundles = []struct {
	name string
	path []string
}{
	{name: "simple", path: []string{"sourcemapcache", "simple", "minified.js"}},
	{name: "inlining", path: []string{"sourcemapcache", "inlining", "module.js"}},
	{name: "webpack", path: []string{"sourcemapcache", "webpack", "bundle.js"}},
	{name: "metro", path: []string{"sourcemapcache", "hermes-metro", "react-native-metro.js"}},
	{name: "preact", path: []string{"sourcemapcache", "preact.module.js"}},
}

func syntheticBundle(targetBytes int) []byte {
	var sb strings.Builder
	sb.Grow(targetBytes + 1024)
	sb.WriteString("(function(){\"use strict\";")
	for i := 0; sb.Len() < targetBytes; i++ {
		fmt.Fprintf(&sb, "function f%d(a,b){return a+b*%d}", i, i)
		fmt.Fprintf(&sb, "var g%d=(a,b)=>{var c=f%d(a,b);return c?c:b};", i, i)
		fmt.Fprintf(&sb, "class C%d{constructor(v){this.v=v}m(x){return this.v+x}static s(y){return(()=>y*2)()}}", i)
		fmt.Fprintf(&sb, "var o%d={k%d:function(){return new C%d(%d)},a%d:async function(p){return await p}};", i, i, i, i, i)
	}
	sb.WriteString("})();")
	return []byte(sb.String())
}

func benchParser(b *testing.B, parserName string, bundle []byte) {
	parse, ok := bundleParsers[parserName]
	if !ok {
		b.Skipf("parser %s not available in this build", parserName)
	}
	if _, err := parse(bundle); err != nil {
		b.Fatalf("parse: %v", err)
	}
	b.SetBytes(int64(len(bundle)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := parse(bundle); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBundleParsers(b *testing.B) {
	for _, fixture := range benchBundles {
		bundle := readIfSet(b, fixture.path)
		for _, parserName := range AvailableParsers() {
			b.Run(fmt.Sprintf("%s/%s", fixture.name, parserName), func(b *testing.B) {
				benchParser(b, parserName, bundle)
			})
		}
	}
}

func BenchmarkBundleParsersSynthetic(b *testing.B) {
	for _, size := range []int{1 << 20, 5 << 20} {
		bundle := syntheticBundle(size)
		for _, parserName := range AvailableParsers() {
			b.Run(fmt.Sprintf("%dMB/%s", size>>20, parserName), func(b *testing.B) {
				benchParser(b, parserName, bundle)
			})
		}
	}
}

func BenchmarkNewResolver(b *testing.B) {
	mapBytes := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js.map"))
	bundle := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js"))

	for _, parserName := range AvailableParsers() {
		b.Run("preact/"+parserName, func(b *testing.B) {
			original := activeBundleParser
			defer func() { activeBundleParser = original }()
			if err := SetParser(parserName); err != nil {
				b.Skipf("parser %s not available in this build", parserName)
			}
			b.SetBytes(int64(len(mapBytes) + len(bundle)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := NewResolver(mapBytes, bundle); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOpenTW(b *testing.B) {
	mapBytes := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js.map"))
	bundle := mustRead(b, fixture(b, "sourcemapcache", "preact.module.js"))
	resolver, err := NewResolver(mapBytes, bundle)
	if err != nil {
		b.Fatal(err)
	}
	tw := resolver.MarshalTW()

	b.SetBytes(int64(len(tw)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := OpenTW(tw); err != nil {
			b.Fatal(err)
		}
	}
}

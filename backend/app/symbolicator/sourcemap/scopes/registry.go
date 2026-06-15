package scopes

import (
	"fmt"
	"slices"
	"strings"
)

type bundleParser func(bundle []byte) ([]Transition, error)

var bundleParsers = map[string]bundleParser{
	"goja": parseGoja,
}

var activeBundleParser = "goja"

func SetParser(name string) error {
	if _, ok := bundleParsers[name]; !ok {
		return fmt.Errorf("scopes: unknown bundle parser %q (available: %s)", name, strings.Join(AvailableParsers(), ", "))
	}
	activeBundleParser = name
	return nil
}

func ActiveParser() string {
	return activeBundleParser
}

func AvailableParsers() []string {
	names := make([]string, 0, len(bundleParsers))
	for name := range bundleParsers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func Parse(bundle []byte) ([]Transition, error) {
	return bundleParsers[activeBundleParser](bundle)
}

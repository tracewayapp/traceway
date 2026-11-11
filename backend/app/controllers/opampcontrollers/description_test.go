package opampcontrollers

import (
	"strconv"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
)

func TestDescriptionAttributesIdentifyingWinsAtLimit(t *testing.T) {
	attribute := func(key, value string) *protobufs.KeyValue {
		return &protobufs.KeyValue{
			Key: key,
			Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{
				StringValue: value,
			}},
		}
	}
	description := &protobufs.AgentDescription{}
	for i := range maxAttributes {
		description.NonIdentifyingAttributes = append(description.NonIdentifyingAttributes,
			attribute("attribute."+strconv.Itoa(i), "non-identifying"))
	}
	description.IdentifyingAttributes = []*protobufs.KeyValue{
		attribute("attribute.0", "identifying"),
		attribute("overflow", "ignored"),
	}

	attrs := DescriptionAttributes(description)
	if attrs["attribute.0"] != "identifying" {
		t.Fatalf("identifying attribute lost precedence at the limit: %q", attrs["attribute.0"])
	}
	if len(attrs) != maxAttributes {
		t.Fatalf("got %d attributes, want %d", len(attrs), maxAttributes)
	}
	if _, ok := attrs["overflow"]; ok {
		t.Fatal("new attributes must still respect the limit")
	}
}

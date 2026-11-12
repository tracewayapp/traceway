package opampcontrollers

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"google.golang.org/protobuf/proto"
)

func attribute(key, value string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{
			StringValue: value,
		}},
	}
}

func arrayAttribute(key string, items ...*protobufs.AnyValue) *protobufs.KeyValue {
	return &protobufs.KeyValue{Key: key, Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_ArrayValue{
		ArrayValue: &protobufs.ArrayValue{Values: items},
	}}}
}

func stringValue(s string) *protobufs.AnyValue {
	return &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: s}}
}

func TestDescriptionAttributesIdentifyingWinsAtLimit(t *testing.T) {
	description := &protobufs.AgentDescription{}
	for i := range maxAttributes {
		description.NonIdentifyingAttributes = append(description.NonIdentifyingAttributes,
			attribute("attribute."+strconv.Itoa(i), "non-identifying"))
	}
	description.IdentifyingAttributes = []*protobufs.KeyValue{
		attribute("attribute.0", "identifying"),
		attribute("service.instance.id", "device-1"),
		attribute("service.version", "2.4.0"),
	}

	attrs := DescriptionAttributes(description)
	if attrs["attribute.0"] != "identifying" {
		t.Fatalf("identifying attribute lost precedence at the limit: %q", attrs["attribute.0"])
	}
	if attrs["service.instance.id"] != "device-1" || attrs["service.version"] != "2.4.0" {
		t.Fatalf("non-identifying attributes crowded out identifying ones: service.instance.id=%q service.version=%q",
			attrs["service.instance.id"], attrs["service.version"])
	}
	if len(attrs) != maxAttributes {
		t.Fatalf("got %d attributes, want %d", len(attrs), maxAttributes)
	}
	if _, ok := attrs["attribute."+strconv.Itoa(maxAttributes-1)]; ok {
		t.Fatal("non-identifying attributes must only fill what the identifying ones leave of the limit")
	}
}

func TestDescriptionAttributesIdentifyingFillTheLimitFirst(t *testing.T) {
	description := &protobufs.AgentDescription{NonIdentifyingAttributes: []*protobufs.KeyValue{attribute("browser.name", "Chrome")}}
	for i := range maxAttributes + 5 {
		description.IdentifyingAttributes = append(description.IdentifyingAttributes,
			attribute("attribute."+strconv.Itoa(i), "identifying"))
	}
	attrs := DescriptionAttributes(description)
	if len(attrs) != maxAttributes {
		t.Fatalf("got %d attributes, want %d", len(attrs), maxAttributes)
	}
	if _, ok := attrs["browser.name"]; ok {
		t.Fatal("a non-identifying attribute got in past the limit")
	}
}

func TestDescriptionAttributesArrays(t *testing.T) {
	description := &protobufs.AgentDescription{IdentifyingAttributes: []*protobufs.KeyValue{
		arrayAttribute("browser.brands", stringValue("Chromium"), stringValue(`Google "Chrome"`),
			&protobufs.AnyValue{Value: &protobufs.AnyValue_IntValue{IntValue: 7}}),
		arrayAttribute("nested", stringValue("a"), &protobufs.AnyValue{Value: &protobufs.AnyValue_ArrayValue{
			ArrayValue: &protobufs.ArrayValue{Values: []*protobufs.AnyValue{stringValue("b")}},
		}}),
		arrayAttribute("long", stringValue(strings.Repeat("x", maxAttributeValue))),
	}}
	attrs := DescriptionAttributes(description)
	if got := attrs["browser.brands"]; got != `["Chromium","Google \"Chrome\"","7"]` {
		t.Errorf("browser.brands = %s", got)
	}
	if got := attrs["nested"]; got != `["a"]` {
		t.Errorf("nested arrays must be left out, got %s", got)
	}
	if got, ok := attrs["long"]; ok {
		t.Errorf("an array longer than %d bytes was kept (%d bytes)", maxAttributeValue, len(got))
	}
}

// A browser project token is public, so a poll that fits the body limit must not be able to make the server build
// a value whose size doubles with every level of nesting.
func TestDescriptionAttributesBoundDeeplyNestedArrays(t *testing.T) {
	deep := stringValue(`"\`)
	for range 24 {
		deep = &protobufs.AnyValue{Value: &protobufs.AnyValue_ArrayValue{
			ArrayValue: &protobufs.ArrayValue{Values: []*protobufs.AnyValue{stringValue("a"), deep}},
		}}
	}
	wide := make([]*protobufs.AnyValue, 10000)
	for i := range wide {
		wide[i] = stringValue(`"`)
	}
	body, err := proto.Marshal(&protobufs.AgentToServer{AgentDescription: &protobufs.AgentDescription{
		NonIdentifyingAttributes: []*protobufs.KeyValue{{Key: "deep", Value: deep}, arrayAttribute("wide", wide...)},
	}})
	if err != nil || len(body) > maxBodyBytes {
		t.Fatalf("test poll is %d bytes (%v), more than a poll may be", len(body), err)
	}
	var msg protobufs.AgentToServer
	if err := proto.Unmarshal(body, &msg); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	attrs := DescriptionAttributes(msg.AgentDescription)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("flattening took %s", elapsed)
	}
	if got := attrs["deep"]; got != `["a"]` {
		t.Errorf("deep = %.80s, want the nested arrays left out", got)
	}
	if got, ok := attrs["wide"]; ok {
		t.Errorf("an array whose JSON text passes the limit was kept (%d bytes)", len(got))
	}
}

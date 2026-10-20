package labels

import (
	"context"
	"testing"
)

type fakeOrders map[string]*Order

func (f fakeOrders) Get(_ context.Context, id string) (*Order, error) { return f[id], nil }

type fakeCarrier struct{ reqs []LabelRequest }

func (f *fakeCarrier) CreateLabel(_ context.Context, req LabelRequest) (*Label, error) {
	f.reqs = append(f.reqs, req)
	return &Label{TrackingNumber: "1Z999", URL: "https://labels.example/1Z999.pdf"}, nil
}

type fakeStore map[string]*Label

func (f fakeStore) SaveLabel(_ context.Context, id string, l *Label) error { f[id] = l; return nil }

func TestHandleBuysOneLabelPerPackage(t *testing.T) {
	orders := fakeOrders{"ord_1": {ID: "ord_1", ShipTo: Address{Name: "Ada", Country: "DE"},
		Packages: []Package{{WeightGrams: 900, Service: "standard"}, {WeightGrams: 300, Service: "standard"}}}}
	carrier := &fakeCarrier{}
	store := fakeStore{}
	if err := NewService(orders, carrier, store).Handle(context.Background(), OrderPaid{OrderID: "ord_1"}); err != nil {
		t.Fatal(err)
	}
	if len(carrier.reqs) != 2 || len(carrier.reqs[0].Parcels) != 1 {
		t.Fatalf("unexpected label requests %+v", carrier.reqs)
	}
	if store["ord_1"] == nil {
		t.Fatal("label not saved")
	}
}

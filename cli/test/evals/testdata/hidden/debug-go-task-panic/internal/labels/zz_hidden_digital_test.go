package labels

import (
	"context"
	"testing"
)

func TestHiddenDigitalOnlyOrderNeedsNoLabel(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Handle panics for an order without packages: %v", r)
		}
	}()
	orders := fakeOrders{"ord_gift": {ID: "ord_gift", ShipTo: Address{Name: "Bo", Country: "NL"}}}
	carrier := &fakeCarrier{}
	store := fakeStore{}
	if err := NewService(orders, carrier, store).Handle(context.Background(), OrderPaid{OrderID: "ord_gift"}); err != nil {
		t.Fatalf("an order with nothing to ship must not fail: %v", err)
	}
	if len(carrier.reqs) != 0 || len(store) != 0 {
		t.Fatalf("no label should be bought for an order without packages: %+v %+v", carrier.reqs, store)
	}
}

func TestHiddenMultiParcelLabelUnchanged(t *testing.T) {
	orders := fakeOrders{"ord_2": {ID: "ord_2", ShipTo: Address{Name: "Cy", Country: "FR"},
		Packages: []Package{{WeightGrams: 1200, Service: "express"}, {WeightGrams: 400, Service: "express"}, {WeightGrams: 80, Service: "express"}}}}
	carrier := &fakeCarrier{}
	store := fakeStore{}
	if err := NewService(orders, carrier, store).Handle(context.Background(), OrderPaid{OrderID: "ord_2"}); err != nil {
		t.Fatal(err)
	}
	if len(carrier.reqs) != 1 || len(carrier.reqs[0].Parcels) != 3 || carrier.reqs[0].Service != "express" || store["ord_2"] == nil {
		t.Fatalf("a physical order must still get one label covering every parcel: %+v", carrier.reqs)
	}
}

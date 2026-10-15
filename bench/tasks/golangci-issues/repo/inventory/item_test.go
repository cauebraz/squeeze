package inventory

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAddMerges(t *testing.T) {
	s := NewStock()
	if err := s.Add(Item{SKU: "A1", Name: "bolt", Qty: 3, Price: 0.5}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Item{SKU: "A1", Name: "bolt", Qty: 2, Price: 0.5}); err != nil {
		t.Fatal(err)
	}
	it, err := s.Get("A1")
	if err != nil || it.Qty != 5 {
		t.Fatalf("Get(A1) = %+v, %v; want Qty 5", it, err)
	}
}

func TestAddRejects(t *testing.T) {
	s := NewStock()
	if err := s.Add(Item{Name: "nut", Qty: 1}); err == nil {
		t.Fatal("Add without SKU: want error")
	}
	if err := s.Add(Item{SKU: "B1", Qty: -1}); err == nil {
		t.Fatal("Add with negative quantity: want error")
	}
}

func TestRemove(t *testing.T) {
	s := NewStock()
	_ = s.Add(Item{SKU: "C1", Name: "washer", Qty: 4})
	if err := s.Remove("C1", 5); err == nil {
		t.Fatal("Remove more than stock: want error")
	}
	if err := s.Remove("C1", 3); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("Z9", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Remove(Z9) = %v; want ErrNotFound", err)
	}
}

func TestFind(t *testing.T) {
	s := NewStock()
	_ = s.Add(Item{SKU: "AB1", Qty: 1})
	_ = s.Add(Item{SKU: "AB2", Qty: 1})
	_ = s.Add(Item{SKU: "XAB", Qty: 1})
	if got := len(s.Find("AB")); got != 2 {
		t.Fatalf("Find(AB) returned %d items; want 2", got)
	}
}

func TestSaveLoad(t *testing.T) {
	s := NewStock()
	_ = s.Add(Item{SKU: "D1", Name: "gear", Qty: 7, Price: 12.25})
	_ = s.Add(Item{SKU: "D2", Name: "shaft", Qty: 1, Price: 40})
	path := filepath.Join(t.TempDir(), "stock.csv")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	it, err := got.Get("D1")
	if err != nil || it.Qty != 7 || it.Price != 12.25 || it.Name != "gear" {
		t.Fatalf("Get(D1) = %+v, %v", it, err)
	}
}

func TestLen(t *testing.T) {
	s := NewStock()
	_ = s.Add(Item{SKU: "E1", Qty: 1})
	_ = s.Add(Item{SKU: "E2", Qty: 1})
	if s.Len() != 2 {
		t.Fatalf("Len = %d; want 2", s.Len())
	}
}

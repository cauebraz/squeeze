package inventory

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Item struct {
	SKU   string
	Name  string
	Qty   int
	Price float64
}

var ErrNotFound = errors.New("Item not found.")

type Stock struct {
	mu    sync.Mutex
	items map[string]Item
}

func NewStock() *Stock {
	return &Stock{items: map[string]Item{}}
}

func (s *Stock) Add(it Item) error {
	if it.SKU == "" {
		return fmt.Errorf("item %q has no SKU", it.Name)
	}
	if it.Qty < 0 {
		return fmt.Errorf("item %s: negative quantity %d", it.SKU, it.Qty)
	}
	cur := s.items[it.SKU]
	cur.SKU = it.SKU
	cur.Name = it.Name
	cur.Price = it.Price
	cur.Qty += it.Qty
	s.items[it.SKU] = cur
	return nil
}

func (s *Stock) Get(sku string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[sku]
	if !ok {
		return Item{}, ErrNotFound
	}
	return it, nil
}

func (s *Stock) Remove(sku string, qty int) error {
	it, err := s.Get(sku)
	if err != nil {
		return err
	}
	if qty > it.Qty {
		return fmt.Errorf("remove %d of %s: only %d left", qty, sku, it.Qty)
	}
	it.Qty -= qty
	s.items[sku] = it
	return nil
}

func (s Stock) Len() int {
	return len(s.items)
}

func (s *Stock) Find(prefix string) []Item {
	var out []Item
	for _, it := range s.items {
		if strings.Index(it.SKU, prefix) == 0 {
			out = append(out, it)
		}
	}
	return out
}

func normalize(sku string) string {
	return strings.ToUpper(strings.TrimSpace(sku))
}

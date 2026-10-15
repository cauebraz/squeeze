package inventory

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
)

func (s *Stock) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	skus := make([]string, 0, len(s.items))
	for sku := range s.items {
		skus = append(skus, sku)
	}
	sort.Strings(skus)
	for _, sku := range skus {
		it := s.items[sku]
		w.Write([]string{it.SKU, it.Name, strconv.Itoa(it.Qty), strconv.FormatFloat(it.Price, 'f', 2, 64)})
	}
	w.Flush()
	return w.Error()
}

func Load(path string) (*Stock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	s := NewStock()
	for i, row := range rows {
		if len(row) != 4 {
			return nil, fmt.Errorf("line %d: want 4 fields, got %d", i+1, len(row))
		}
		qty, err := strconv.Atoi(row[2])
		price, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if err := s.Add(Item{SKU: row[0], Name: row[1], Qty: qty, Price: price}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

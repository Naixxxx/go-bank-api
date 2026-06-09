package utils

import "testing"

func TestGenerateCardNumberLuhn(t *testing.T) {
	for i := 0; i < 25; i++ {
		n, err := GenerateCardNumber()
		if err != nil {
			t.Fatal(err)
		}
		if len(n) != 16 {
			t.Fatalf("want 16 digits, got %d", len(n))
		}
		if !ValidLuhn(n) {
			t.Fatalf("invalid luhn number: %s", n)
		}
	}
}

func TestMoneyConversion(t *testing.T) {
	if ToKopecks(12.34) != 1234 {
		t.Fatalf("bad kopecks")
	}
	if ToRub(1234) != 12.34 {
		t.Fatalf("bad rub")
	}
}

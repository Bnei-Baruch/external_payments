package validation

import "testing"

type priceRequest struct {
	Price float64 `validate:"float,min=0"`
}

func TestValidateStruct_ZeroPriceFailsByDefault(t *testing.T) {
	found, errs := ValidateStruct(priceRequest{Price: 0})
	if !found {
		t.Fatalf("expected validation error for zero price, got none")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
}

func TestValidateStruct_ZeroPricePassesWhenSkipped(t *testing.T) {
	found, errs := ValidateStruct(priceRequest{Price: 0}, "Price")
	if found {
		t.Fatalf("expected no validation error when Price is skipped, got %v", errs)
	}
}

func TestValidateStruct_PositivePricePasses(t *testing.T) {
	found, errs := ValidateStruct(priceRequest{Price: 10})
	if found {
		t.Fatalf("expected no validation error for positive price, got %v", errs)
	}
}

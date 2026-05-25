package repositories

import (
	"testing"

	"aggregator-services/internal/normalizers"
)

func TestFillTakerFlowDeltas(t *testing.T) {
	buy := 10.0
	sell := 4.0
	buyQuote := 1000.0
	sellQuote := 250.0

	got := fillTakerFlowDeltas(normalizers.NormalizedTakerFlow{
		TakerBuyVolume:       &buy,
		TakerSellVolume:      &sell,
		TakerBuyQuoteVolume:  &buyQuote,
		TakerSellQuoteVolume: &sellQuote,
	})

	if got.BuySellDelta == nil || *got.BuySellDelta != 6 {
		t.Fatalf("BuySellDelta = %v", got.BuySellDelta)
	}
	if got.BuySellDeltaQuote == nil || *got.BuySellDeltaQuote != 750 {
		t.Fatalf("BuySellDeltaQuote = %v", got.BuySellDeltaQuote)
	}
	if got.BuySellRatio == nil || *got.BuySellRatio != 2.5 {
		t.Fatalf("BuySellRatio = %v", got.BuySellRatio)
	}
}

func TestLiquidationUSDUsesFallbacks(t *testing.T) {
	price := 100.0
	quantity := 12.0

	got := liquidationUSD(normalizers.NormalizedLiquidationEvent{
		Price:    &price,
		Quantity: &quantity,
	})

	if got != 1200 {
		t.Fatalf("liquidationUSD() = %f", got)
	}
}

func TestFillBasisValues(t *testing.T) {
	futures := 105.0
	index := 100.0

	got := fillBasisValues(normalizers.NormalizedBasisPremium{
		FuturesPrice: &futures,
		IndexPrice:   &index,
	})

	if got.BasisValue == nil || *got.BasisValue != 5 {
		t.Fatalf("BasisValue = %v", got.BasisValue)
	}
	if got.BasisPercent == nil || *got.BasisPercent != 5 {
		t.Fatalf("BasisPercent = %v", got.BasisPercent)
	}
}

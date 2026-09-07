package service

var defaultDramaVideoPrices = map[string]map[string]float64{
	DramaFamilyMinimaxH3: {
		VideoBillingResolution480P:  0.08,
		VideoBillingResolution720P:  0.12,
		VideoBillingResolution1080P: 0.35,
	},
	DramaFamilySeedance20A: {
		VideoBillingResolution480P:  0.30,
		VideoBillingResolution720P:  0.50,
		VideoBillingResolution1080P: 0.90,
	},
	DramaFamilySeedance20FA: {
		VideoBillingResolution480P: 0.20,
	},
	DramaFamilySeedance20MA: {
		VideoBillingResolution480P: 0.10,
		VideoBillingResolution720P: 0.25,
	},
	DramaFamilySeedance20B: {
		VideoBillingResolution480P:  1.90,
		VideoBillingResolution720P:  3.50,
		VideoBillingResolution1080P: 9.00,
		VideoBillingResolution4K:    40.00,
	},
	DramaFamilySeedance20FB: {
		VideoBillingResolution480P: 1.80,
		VideoBillingResolution720P: 3.00,
	},
	DramaFamilySeedance20C: {
		VideoBillingResolution720P: 3.50,
	},
	DramaFamilySeedance20E: {
		VideoBillingResolution720P: 4.20,
	},
	DramaFamilySeedance20F: {
		VideoBillingResolution720P:  4.20,
		VideoBillingResolution1080P: 7.00,
	},
	DramaFamilySeedance20FF: {
		VideoBillingResolution720P: 2.80,
	},
	DramaFamilySeedance25A: {
		VideoBillingResolution480P:  0.35,
		VideoBillingResolution720P:  0.65,
		VideoBillingResolution1080P: 1.00,
	},
	DramaFamilySeedance25B: {
		VideoBillingResolution720P: 6.00,
	},
}

func DefaultDramaVideoModelPrices() map[string]map[string]float64 {
	out := make(map[string]map[string]float64, len(defaultDramaVideoPrices))
	for family, tiers := range defaultDramaVideoPrices {
		copied := make(map[string]float64, len(tiers))
		for res, price := range tiers {
			copied[res] = price
		}
		out[family] = copied
	}
	return out
}

func getDefaultDramaVideoPrice(model, resolution string) (float64, bool) {
	family := CanonicalDramaVideoPriceFamily(model)
	if family == "" {
		return 0, false
	}
	tier, ok := LookupVideoBillingResolution(resolution)
	if !ok {
		return 0, false
	}
	price, ok := defaultDramaVideoPrices[family][tier]
	return price, ok
}

func (s *BillingService) CalculateDramaVideoCost(resolved DramaVideoResolvedModel, durationSeconds int, groupConfig *VideoPriceConfig, rateMultiplier float64) *CostBreakdown {
	if resolved.Family == "" {
		return nil
	}
	unit := 0.0
	if s != nil {
		unit = s.getVideoUnitPrice(resolved.Family, resolved.Resolution, groupConfig)
	}
	if unit <= 0 {
		if price, ok := getDefaultDramaVideoPrice(resolved.Family, resolved.Resolution); ok {
			unit = price
		}
	}
	if unit <= 0 {
		return nil
	}
	total := unit
	if resolved.BillingUnit == DramaVideoBillingPerSecond {
		if durationSeconds <= 0 {
			durationSeconds = 1
		}
		total = unit * float64(durationSeconds)
	}
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	return &CostBreakdown{
		TotalCost:   total,
		ActualCost:  total * rateMultiplier,
		BillingMode: string(BillingModeVideo),
	}
}

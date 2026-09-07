package service

import "sort"

// DramaVideoCatalog is the user-facing video model directory.
type DramaVideoCatalog struct {
	Families []DramaVideoCatalogFamily `json:"families"`
}

// DramaVideoCatalogPrice is one priced resolution for a public family.
type DramaVideoCatalogPrice struct {
	Resolution string  `json:"resolution"`
	Price      float64 `json:"price"`
}

// DramaVideoCatalogFamily describes one public video family.
type DramaVideoCatalogFamily struct {
	Family                   string                   `json:"family"`
	BillingUnit              string                   `json:"billing_unit"`
	CreatePath               string                   `json:"create_path"`
	MinDuration              int                      `json:"min_duration"`
	MaxDuration              int                      `json:"max_duration"`
	DurationRequired         bool                     `json:"duration_required"`
	FixedDuration            int                      `json:"fixed_duration,omitempty"`
	DefaultDuration          int                      `json:"default_duration,omitempty"`
	DefaultResolution        string                   `json:"default_resolution"`
	DefaultAspect            string                   `json:"default_aspect"`
	AspectRatios             []string                 `json:"aspect_ratios"`
	Prices                   []DramaVideoCatalogPrice `json:"prices"`
	MaxImages                int                      `json:"max_images"`
	MaxVideos                int                      `json:"max_videos"`
	MaxAudios                int                      `json:"max_audios"`
	MaxReferences            int                      `json:"max_references"`
	MaxVideoAudio            int                      `json:"max_video_audio,omitempty"`
	AllowVideo               bool                     `json:"allow_video"`
	AllowAudio               bool                     `json:"allow_audio"`
	AllowFirstLast           bool                     `json:"allow_first_last"`
	AllowGenerateAudio       bool                     `json:"allow_generate_audio"`
	AllowSOptional           bool                     `json:"allow_s_optional"`
	FirstLastRequiresAuto    bool                     `json:"first_last_requires_auto"`
	RequireAPlaceholders     bool                     `json:"require_a_placeholders"`
	RequireImagePlaceholders bool                     `json:"require_image_placeholders"`
	Tags                     []string                 `json:"tags,omitempty"`
}

var catalogResolutionOrder = []string{
	VideoBillingResolution480P,
	VideoBillingResolution720P,
	VideoBillingResolution1080P,
	VideoBillingResolution4K,
}

// BuildDramaVideoCatalog returns the 12 public families with priced resolutions only.
func BuildDramaVideoCatalog() DramaVideoCatalog {
	defaults := DefaultDramaVideoModelPrices()
	out := DramaVideoCatalog{Families: make([]DramaVideoCatalogFamily, 0, len(DramaVideoPublicFamilies()))}
	for _, family := range DramaVideoPublicFamilies() {
		cap, ok := dramaVideoCapabilities[family]
		if !ok {
			continue
		}
		prices := make([]DramaVideoCatalogPrice, 0, len(catalogResolutionOrder))
		for _, res := range catalogResolutionOrder {
			if _, supported := cap.resolutions[res]; !supported {
				continue
			}
			price, priced := defaults[family][res]
			if !priced {
				continue
			}
			prices = append(prices, DramaVideoCatalogPrice{Resolution: res, Price: price})
		}
		item := DramaVideoCatalogFamily{
			Family:                   cap.family,
			BillingUnit:              cap.billingUnit,
			CreatePath:               cap.createPath,
			MinDuration:              cap.minDuration,
			MaxDuration:              cap.maxDuration,
			DurationRequired:         cap.durationRequired,
			FixedDuration:            cap.fixedDuration,
			DefaultDuration:          cap.defaultDuration,
			DefaultResolution:        cap.defaultResolution,
			DefaultAspect:            cap.defaultAspect,
			AspectRatios:             catalogAspectRatios(cap.aspectRatios),
			Prices:                   prices,
			MaxImages:                cap.maxImages,
			MaxVideos:                cap.maxVideos,
			MaxAudios:                cap.maxAudios,
			MaxReferences:            cap.maxReferences,
			MaxVideoAudio:            cap.maxVideoAudio,
			AllowVideo:               cap.allowVideo,
			AllowAudio:               cap.allowAudio,
			AllowFirstLast:           cap.allowFirstLast,
			AllowGenerateAudio:       cap.allowGenerateAudio || cap.allowSOptional,
			AllowSOptional:           cap.allowSOptional,
			FirstLastRequiresAuto:    cap.firstLastRequiresAuto,
			RequireAPlaceholders:     cap.requireAPlaceholders,
			RequireImagePlaceholders: cap.requireImagePlaceholders,
			Tags:                     dramaVideoCatalogTags(family),
		}
		out.Families = append(out.Families, item)
	}
	return out
}

func catalogAspectRatios(set map[string]struct{}) []string {
	preferred := []string{"adaptive", "auto", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}
	seen := make(map[string]bool, len(set))
	out := make([]string, 0, len(set))
	for _, key := range preferred {
		if _, ok := set[key]; ok {
			out = append(out, key)
			seen[key] = true
		}
	}
	extra := make([]string, 0)
	for key := range set {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

func dramaVideoCatalogTags(family string) []string {
	switch family {
	case DramaFamilyMinimaxH3:
		return []string{"skills"}
	case DramaFamilySeedance20A, DramaFamilySeedance20FA, DramaFamilySeedance20MA, DramaFamilySeedance25A:
		return []string{"不卡人脸", "不排队"}
	case DramaFamilySeedance20B:
		return []string{"933"}
	default:
		return nil
	}
}

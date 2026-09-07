package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	dramaVideoSurfaceVideos = "videos"
	dramaVideoSurfaceGens   = "generations"
)

type dramaVideoCapability struct {
	family            string
	createPath        string
	billingUnit       string
	minDuration       int
	maxDuration       int
	durationRequired  bool
	fixedDuration     int
	defaultDuration   int
	defaultResolution string
	resolutions       map[string]string // public resolution -> upstream model
	aspectRatios      map[string]struct{}
	defaultAspect     string
	maxImages         int
	maxVideos         int
	maxAudios         int
	maxReferences     int
	maxVideoAudio     int
	allowVideo        bool
	allowAudio        bool
	allowFirstLast           bool
	allowSOptional           bool
	allowGenerateAudio       bool
	firstLastRequiresAuto    bool
	requireAPlaceholders     bool
	requireImagePlaceholders bool
}

var dramaVideoCapabilities = map[string]dramaVideoCapability{
	DramaFamilyMinimaxH3: {
		family: DramaFamilyMinimaxH3, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerSecond,
		minDuration: 4, maxDuration: 15, defaultDuration: 4, defaultResolution: VideoBillingResolution480P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilyMinimaxH3, VideoBillingResolution720P: DramaFamilyMinimaxH3, VideoBillingResolution1080P: DramaFamilyMinimaxH3},
		aspectRatios: ratioSet("16:9", "9:16"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 3, maxAudios: 3, maxReferences: 12, maxVideoAudio: 3, allowVideo: true, allowAudio: true,
	},
	DramaFamilySeedance20A: {
		family: DramaFamilySeedance20A, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerSecond,
		minDuration: 4, maxDuration: 15, defaultDuration: 4, defaultResolution: VideoBillingResolution480P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilySeedance20A, VideoBillingResolution720P: DramaFamilySeedance20A, VideoBillingResolution1080P: DramaFamilySeedance20A},
		aspectRatios: ratioSet("21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "auto"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 3, maxAudios: 3, maxReferences: 12, allowVideo: true, allowAudio: true, allowFirstLast: true, firstLastRequiresAuto: true, requireAPlaceholders: true,
	},
	DramaFamilySeedance20FA: {
		family: DramaFamilySeedance20FA, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerSecond,
		minDuration: 4, maxDuration: 15, defaultDuration: 4, defaultResolution: VideoBillingResolution480P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilySeedance20FA},
		aspectRatios: ratioSet("21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "auto"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 3, maxAudios: 3, maxReferences: 12, allowVideo: true, allowAudio: true, allowFirstLast: true, firstLastRequiresAuto: true, requireAPlaceholders: true,
	},
	DramaFamilySeedance20MA: {
		family: DramaFamilySeedance20MA, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerSecond,
		minDuration: 4, maxDuration: 15, defaultDuration: 4, defaultResolution: VideoBillingResolution480P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilySeedance20MA, VideoBillingResolution720P: DramaFamilySeedance20MA},
		aspectRatios: ratioSet("21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "auto"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 3, maxAudios: 3, maxReferences: 12, allowVideo: true, allowAudio: true, allowFirstLast: true, firstLastRequiresAuto: true, requireAPlaceholders: true,
	},
	DramaFamilySeedance20B: {
		family: DramaFamilySeedance20B, createPath: DramaVideoCreatePathGens, billingUnit: DramaVideoBillingPerClip,
		minDuration: 4, maxDuration: 15, defaultDuration: 4, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilySeedance20B, VideoBillingResolution720P: DramaFamilySeedance20B, VideoBillingResolution1080P: DramaFamilySeedance20B, VideoBillingResolution4K: DramaFamilySeedance20B},
		aspectRatios: ratioSet("adaptive", "16:9", "4:3", "1:1", "3:4", "9:16", "21:9"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 0, maxAudios: 3, maxReferences: 12, allowAudio: true, allowFirstLast: true, allowSOptional: true,
	},
	DramaFamilySeedance20FB: {
		family: DramaFamilySeedance20FB, createPath: DramaVideoCreatePathGens, billingUnit: DramaVideoBillingPerClip,
		minDuration: 4, maxDuration: 15, defaultDuration: 4, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilySeedance20FB, VideoBillingResolution720P: DramaFamilySeedance20FB},
		aspectRatios: ratioSet("adaptive", "16:9", "4:3", "1:1", "3:4", "9:16", "21:9"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 0, maxAudios: 3, maxReferences: 12, allowAudio: true, allowFirstLast: true, allowSOptional: true,
	},
	DramaFamilySeedance20C: {
		family: DramaFamilySeedance20C, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerClip,
		minDuration: 5, maxDuration: 15, durationRequired: true, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution720P: DramaFamilySeedance20C},
		aspectRatios: ratioSet("16:9", "9:16"), defaultAspect: "16:9",
		maxImages: 30, maxVideos: 0, maxAudios: 10, maxReferences: 40, allowAudio: true, allowGenerateAudio: true, requireImagePlaceholders: true,
	},
	DramaFamilySeedance20E: {
		family: DramaFamilySeedance20E, createPath: DramaVideoCreatePathGens, billingUnit: DramaVideoBillingPerClip,
		minDuration: 5, maxDuration: 15, defaultDuration: 5, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution720P: "seedance2.0-E-720p"},
		aspectRatios: ratioSet("16:9", "4:3", "1:1", "3:4", "9:16", "21:9"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 3, maxAudios: 3, maxReferences: 15, allowVideo: true, allowAudio: true,
	},
	DramaFamilySeedance20F: {
		family: DramaFamilySeedance20F, createPath: DramaVideoCreatePathGens, billingUnit: DramaVideoBillingPerClip,
		minDuration: 5, maxDuration: 15, defaultDuration: 5, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution720P: "seedance2.0-F-720p", VideoBillingResolution1080P: "seedance2.0-F-1080p"},
		aspectRatios: ratioSet("16:9", "4:3", "1:1", "3:4", "9:16", "21:9"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 0, maxAudios: 3, maxReferences: 12, allowAudio: true,
	},
	DramaFamilySeedance20FF: {
		family: DramaFamilySeedance20FF, createPath: DramaVideoCreatePathGens, billingUnit: DramaVideoBillingPerClip,
		minDuration: 5, maxDuration: 15, defaultDuration: 5, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution720P: "seedance2.0-fast-F-720p"},
		aspectRatios: ratioSet("16:9", "4:3", "1:1", "3:4", "9:16", "21:9"), defaultAspect: "16:9",
		maxImages: 9, maxVideos: 0, maxAudios: 3, maxReferences: 12, allowAudio: true,
	},
	DramaFamilySeedance25A: {
		family: DramaFamilySeedance25A, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerSecond,
		minDuration: 4, maxDuration: 30, defaultDuration: 4, defaultResolution: VideoBillingResolution480P,
		resolutions: map[string]string{VideoBillingResolution480P: DramaFamilySeedance25A, VideoBillingResolution720P: DramaFamilySeedance25A, VideoBillingResolution1080P: DramaFamilySeedance25A},
		aspectRatios: ratioSet("21:9", "16:9", "4:3", "1:1", "3:4", "9:16"), defaultAspect: "16:9",
		maxImages: 30, maxVideos: 10, maxAudios: 10, maxReferences: 50, allowVideo: true, allowAudio: true, allowFirstLast: true, requireAPlaceholders: true,
	},
	DramaFamilySeedance25B: {
		family: DramaFamilySeedance25B, createPath: DramaVideoCreatePathVideos, billingUnit: DramaVideoBillingPerClip,
		minDuration: 30, maxDuration: 30, durationRequired: true, fixedDuration: 30, defaultResolution: VideoBillingResolution720P,
		resolutions: map[string]string{VideoBillingResolution720P: DramaFamilySeedance25B},
		aspectRatios: ratioSet("21:9", "16:9", "4:3", "1:1", "3:4", "9:16"), defaultAspect: "16:9",
		maxImages: 30, maxVideos: 3, maxAudios: 0, maxReferences: 33, allowVideo: true,
	},
}

type dramaVideoCreatePayload struct {
	Model           string                  `json:"model"`
	Prompt          string                  `json:"prompt"`
	Seconds         json.RawMessage         `json:"seconds"`
	Duration        json.RawMessage         `json:"duration"`
	AspectRatio     string                  `json:"aspect_ratio"`
	Ratio           string                  `json:"ratio"`
	AspectRatioAlt  string                  `json:"aspectRatio"`
	Resolution      string                  `json:"resolution"`
	TaskMode        string                  `json:"task_mode"`
	GenerateAudio   *bool                   `json:"generate_audio"`
	GenerateAudio2  *bool                   `json:"generateAudio"`
	ReturnLastFrame *bool                   `json:"return_last_frame"`
	WebSearch       *bool                   `json:"web_search"`
	Priority        *int                    `json:"priority"`
	References      []dramaVideoReference   `json:"references"`
	ReferenceImages []dramaVideoReference   `json:"referenceImages"`
	ReferenceVideos []dramaVideoReference   `json:"referenceVideos"`
	ReferenceAudios []dramaVideoReference   `json:"referenceAudios"`
	FirstFrameURL   string                  `json:"first_frame_url"`
	LastFrameURL    string                  `json:"last_frame_url"`
	FirstFrame      string                  `json:"first_frame"`
	LastFrame       string                  `json:"last_frame"`
	FirstImage      string                  `json:"first_image"`
	LastImage       string                  `json:"last_image"`
}

type dramaVideoReference struct {
	Type   string `json:"type"`
	Role   string `json:"role"`
	Source string `json:"source"`
}

var (
	dramaAImagePlaceholder  = regexp.MustCompile(`@(?:图|图片)([1-9][0-9]*)`)
	dramaAVideoPlaceholder  = regexp.MustCompile(`@视频([1-9][0-9]*)`)
	dramaAAudioPlaceholder  = regexp.MustCompile(`@音频([1-9][0-9]*)`)
	dramaCImagePlaceholder  = regexp.MustCompile(`@Image([1-9][0-9]*)`)
)

func ratioSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[strings.ToLower(v)] = struct{}{}
	}
	return out
}

// CanonicalDramaVideoPriceFamily maps public families and upstream aliases onto
// the 12 operator-facing price keys.
func CanonicalDramaVideoPriceFamily(model string) string {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		return ""
	}
	switch strings.ToLower(normalized) {
	case "sd25-30s", "seedance-2.5-b":
		return DramaFamilySeedance25B
	case "seedance2.0-e", "seedance2.0-e-720p":
		return DramaFamilySeedance20E
	case "seedance2.0-f", "seedance2.0-f-720p", "seedance2.0-f-1080p":
		return DramaFamilySeedance20F
	case "seedance2.0-fast-f", "seedance2.0-fast-f-720p":
		return DramaFamilySeedance20FF
	}
	for _, family := range DramaVideoPublicFamilies() {
		if strings.EqualFold(family, normalized) {
			return family
		}
	}
	return ""
}

func dramaFamilyFromUpstreamAlias(model string) (family, impliedResolution string, ok bool) {
	normalized := strings.TrimSpace(model)
	switch strings.ToLower(normalized) {
	case "seedance2.0-e-720p":
		return DramaFamilySeedance20E, VideoBillingResolution720P, true
	case "seedance2.0-f-720p":
		return DramaFamilySeedance20F, VideoBillingResolution720P, true
	case "seedance2.0-f-1080p":
		return DramaFamilySeedance20F, VideoBillingResolution1080P, true
	case "seedance2.0-fast-f-720p":
		return DramaFamilySeedance20FF, VideoBillingResolution720P, true
	case "sd25-30s":
		return DramaFamilySeedance25B, VideoBillingResolution720P, true
	}
	return "", "", false
}

func resolveDramaVideoModel(model, resolution string) (DramaVideoResolvedModel, error) {
	raw := strings.TrimSpace(model)
	if raw == "" {
		return DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_MODEL", "model is required")
	}
	_, impliedRes, alias := dramaFamilyFromUpstreamAlias(raw)
	family := CanonicalDramaVideoPriceFamily(raw)
	if family == "" {
		return DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_MODEL", fmt.Sprintf("model %q is not supported", raw))
	}
	cap, ok := dramaVideoCapabilities[family]
	if !ok {
		return DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_MODEL", fmt.Sprintf("model %q is not supported", raw))
	}
	res := strings.TrimSpace(resolution)
	if res == "" && impliedRes != "" {
		res = impliedRes
	}
	if res == "" {
		res = cap.defaultResolution
	}
	normalizedRes, ok := LookupVideoBillingResolution(res)
	if !ok {
		return DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_RESOLUTION", fmt.Sprintf("resolution %q is not supported", res))
	}
	if alias && impliedRes != "" && impliedRes != normalizedRes {
		return DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_RESOLUTION", "model suffix resolution conflicts with resolution field")
	}
	upstream, priced := cap.resolutions[normalizedRes]
	if !priced {
		return DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_RESOLUTION", fmt.Sprintf("model %s does not support resolution %s", family, normalizedRes))
	}
	return DramaVideoResolvedModel{
		RequestedModel: family,
		Family:         family,
		Resolution:     normalizedRes,
		UpstreamModel:  upstream,
		CreatePath:     cap.createPath,
		BillingUnit:    cap.billingUnit,
	}, nil
}

func parseDramaVideoCreatePayload(body []byte, surface string) (dramaVideoCreatePayload, DramaVideoResolvedModel, error) {
	var payload dramaVideoCreatePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, DramaVideoResolvedModel{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_JSON", "invalid video request JSON").WithCause(err)
	}
	resolved, err := resolveDramaVideoModel(payload.Model, payload.Resolution)
	if err != nil {
		return payload, resolved, err
	}
	cap := dramaVideoCapabilities[resolved.Family]
	if surface == dramaVideoSurfaceVideos && cap.createPath != DramaVideoCreatePathVideos {
		return payload, resolved, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_PATH", fmt.Sprintf("model %s must be created via POST %s", resolved.Family, cap.createPath))
	}
	if surface == dramaVideoSurfaceGens && cap.createPath != DramaVideoCreatePathGens {
		return payload, resolved, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_PATH", fmt.Sprintf("model %s must be created via POST %s", resolved.Family, cap.createPath))
	}
	payload.Model = resolved.Family
	payload.Resolution = resolved.Resolution
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	if payload.Prompt == "" {
		return payload, resolved, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_PROMPT", "prompt is required")
	}
	seconds, err := parseDramaVideoSeconds(payload, cap)
	if err != nil {
		return payload, resolved, err
	}
	payload.Seconds = json.RawMessage(strconv.Itoa(seconds))
	aspect := firstNonEmpty(payload.AspectRatio, payload.Ratio, payload.AspectRatioAlt)
	if aspect == "" {
		aspect = cap.defaultAspect
	}
	if _, ok := cap.aspectRatios[strings.ToLower(strings.TrimSpace(aspect))]; !ok {
		return payload, resolved, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_ASPECT_RATIO", fmt.Sprintf("aspect_ratio %q is not supported", aspect))
	}
	payload.AspectRatio = aspect
	if !cap.allowSOptional && (payload.ReturnLastFrame != nil || payload.WebSearch != nil || payload.Priority != nil) {
		return payload, resolved, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_FIELD", "optional S-series fields are not supported by this model")
	}
	if !cap.allowSOptional && !cap.allowGenerateAudio && (payload.GenerateAudio != nil || payload.GenerateAudio2 != nil) {
		return payload, resolved, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_FIELD", "generate_audio is not supported by this model")
	}
	if err := normalizeDramaVideoReferences(&payload, cap); err != nil {
		return payload, resolved, err
	}
	return payload, resolved, nil
}

func parseDramaVideoSeconds(payload dramaVideoCreatePayload, cap dramaVideoCapability) (int, error) {
	sec, secSet, err := decodeOptionalInt(payload.Seconds)
	if err != nil {
		return 0, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", "seconds must be an integer")
	}
	dur, durSet, err := decodeOptionalInt(payload.Duration)
	if err != nil {
		return 0, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", "duration must be an integer")
	}
	if secSet && durSet && sec != dur {
		return 0, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", "seconds and duration must match")
	}
	value := 0
	if secSet {
		value = sec
	} else if durSet {
		value = dur
	}
	if value == 0 {
		if cap.durationRequired {
			return 0, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", "seconds is required")
		}
		value = cap.defaultDuration
	}
	if cap.fixedDuration > 0 && value != cap.fixedDuration {
		return 0, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", fmt.Sprintf("seconds must be %d", cap.fixedDuration))
	}
	if value < cap.minDuration || value > cap.maxDuration {
		return 0, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", fmt.Sprintf("seconds must be between %d and %d", cap.minDuration, cap.maxDuration))
	}
	return value, nil
}

func decodeOptionalInt(raw json.RawMessage) (int, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false, nil
	}
	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt, true, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, false, nil
		}
		n, err := strconv.Atoi(asString)
		if err != nil {
			return 0, true, err
		}
		return n, true, nil
	}
	return 0, true, fmt.Errorf("not an integer")
}

func normalizeDramaVideoReferences(payload *dramaVideoCreatePayload, cap dramaVideoCapability) error {
	refs := append([]dramaVideoReference{}, payload.References...)
	for _, item := range payload.ReferenceImages {
		item.Type = "image"
		refs = append(refs, item)
	}
	for _, item := range payload.ReferenceVideos {
		item.Type = "video"
		refs = append(refs, item)
	}
	for _, item := range payload.ReferenceAudios {
		item.Type = "audio"
		refs = append(refs, item)
	}
	first := firstNonEmpty(payload.FirstFrameURL, payload.FirstFrame, payload.FirstImage)
	last := firstNonEmpty(payload.LastFrameURL, payload.LastFrame, payload.LastImage)
	if first != "" || last != "" {
		if !cap.allowFirstLast {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "first/last frame is not supported by this model")
		}
		if first == "" || last == "" {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "first_frame and last_frame must be provided together")
		}
		refs = append(refs,
			dramaVideoReference{Type: "image", Role: "first_frame", Source: first},
			dramaVideoReference{Type: "image", Role: "last_frame", Source: last},
		)
	}
	seen := map[string]struct{}{}
	images, videos, audios := 0, 0, 0
	firstCount, lastCount := 0, 0
	for i := range refs {
		refs[i].Type = strings.ToLower(strings.TrimSpace(refs[i].Type))
		refs[i].Role = strings.ToLower(strings.TrimSpace(refs[i].Role))
		refs[i].Source = strings.TrimSpace(refs[i].Source)
		if refs[i].Type == "" {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "reference type is required")
		}
		if refs[i].Role == "" || refs[i].Role == "reference_image" || refs[i].Role == "reference_video" || refs[i].Role == "reference_audio" {
			refs[i].Role = "reference"
		}
		if refs[i].Source == "" || !isPublicDramaMediaSource(refs[i].Source) {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "reference source must be a public HTTPS URL or data URI")
		}
		if _, dup := seen[refs[i].Source]; dup {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "each reference source must be unique")
		}
		seen[refs[i].Source] = struct{}{}
		switch refs[i].Type {
		case "image":
			images++
		case "video":
			if !cap.allowVideo {
				return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "video references are not supported by this model")
			}
			if refs[i].Role != "reference" {
				return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "video references must use role reference")
			}
			videos++
		case "audio":
			if !cap.allowAudio {
				return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "audio references are not supported by this model")
			}
			if refs[i].Role != "reference" {
				return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "audio references must use role reference")
			}
			audios++
		default:
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "reference type must be image, video, or audio")
		}
		switch refs[i].Role {
		case "first_frame":
			if refs[i].Type != "image" || !cap.allowFirstLast {
				return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "first_frame must be an image and is not supported by this model")
			}
			firstCount++
		case "last_frame":
			if refs[i].Type != "image" || !cap.allowFirstLast {
				return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "last_frame must be an image and is not supported by this model")
			}
			lastCount++
		case "reference":
		default:
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "reference role is not supported")
		}
	}
	if firstCount != lastCount {
		return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "first_frame and last_frame must be provided together")
	}
	if firstCount > 0 {
		if cap.firstLastRequiresAuto && strings.ToLower(strings.TrimSpace(payload.AspectRatio)) != "auto" {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_ASPECT_RATIO", "first/last frame mode only supports Auto aspect ratio")
		}
		if cap.firstLastRequiresAuto && (images != 2 || videos > 0 || audios > 0) {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "first/last frame mode cannot be mixed with ordinary references")
		}
	}
	if cap.maxReferences > 0 && len(refs) > cap.maxReferences {
		return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", fmt.Sprintf("references may contain at most %d items", cap.maxReferences))
	}
	if images > cap.maxImages || videos > cap.maxVideos || audios > cap.maxAudios {
		return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "reference count exceeds the model limit")
	}
	if cap.maxVideoAudio > 0 && videos+audios > cap.maxVideoAudio {
		return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "video and audio references together exceed the model limit")
	}
	if cap.requireAPlaceholders && (images+videos+audios > 0) {
		if err := validateDramaAPlaceholders(payload.Prompt, images, videos, audios); err != nil {
			return err
		}
	}
	if cap.requireImagePlaceholders && images > 0 {
		if err := validateDramaCPlaceholders(payload.Prompt, images); err != nil {
			return err
		}
	}
	payload.References = refs
	return nil
}

func validateDramaAPlaceholders(prompt string, images, videos, audios int) error {
	if err := validatePlaceholderSet(prompt, dramaAImagePlaceholder, images, "image"); err != nil {
		return err
	}
	if err := validatePlaceholderSet(prompt, dramaAVideoPlaceholder, videos, "video"); err != nil {
		return err
	}
	return validatePlaceholderSet(prompt, dramaAAudioPlaceholder, audios, "audio")
}

func validateDramaCPlaceholders(prompt string, images int) error {
	return validatePlaceholderSet(prompt, dramaCImagePlaceholder, images, "image")
}

func validatePlaceholderSet(prompt string, re *regexp.Regexp, count int, kind string) error {
	if count == 0 {
		return nil
	}
	seen := map[int]struct{}{}
	for _, match := range re.FindAllStringSubmatch(prompt, -1) {
		n, _ := strconv.Atoi(match[1])
		if n < 1 || n > count {
			return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", fmt.Sprintf("%s placeholder number is out of range", kind))
		}
		seen[n] = struct{}{}
	}
	if len(seen) != count {
		return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", fmt.Sprintf("every %s reference must be cited in the prompt", kind))
	}
	return nil
}

func isPublicDramaMediaSource(source string) bool {
	if strings.HasPrefix(strings.ToLower(source), "data:") {
		return strings.Contains(source, ",")
	}
	u, err := url.Parse(source)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return false
	}
	return true
}

func hasDramaFirstLast(refs []dramaVideoReference) bool {
	first, last := false, false
	for _, ref := range refs {
		switch ref.Role {
		case "first_frame":
			first = true
		case "last_frame":
			last = true
		}
	}
	return first && last
}

func hasDramaImageRefs(refs []dramaVideoReference) bool {
	for _, ref := range refs {
		if ref.Type == "image" && ref.Role == "reference" {
			return true
		}
	}
	return false
}

func dramaVideoSecondsValue(payload dramaVideoCreatePayload) int {
	n, _, _ := decodeOptionalInt(payload.Seconds)
	return n
}

func marshalDramaVideoUpstreamBody(payload dramaVideoCreatePayload, upstreamModel string) ([]byte, error) {
	seconds := dramaVideoSecondsValue(payload)
	body := map[string]any{
		"model":        upstreamModel,
		"prompt":       payload.Prompt,
		"seconds":      seconds,
		"resolution":   payload.Resolution,
		"aspect_ratio": payload.AspectRatio,
	}
	if len(payload.References) > 0 {
		refs := make([]map[string]string, 0, len(payload.References))
		for _, ref := range payload.References {
			refs = append(refs, map[string]string{"type": ref.Type, "role": ref.Role, "source": ref.Source})
		}
		body["references"] = refs
	}
	if payload.GenerateAudio != nil {
		body["generate_audio"] = *payload.GenerateAudio
	} else if payload.GenerateAudio2 != nil {
		body["generate_audio"] = *payload.GenerateAudio2
	}
	if payload.TaskMode != "" {
		body["task_mode"] = payload.TaskMode
	} else if cap, ok := dramaVideoCapabilities[CanonicalDramaVideoPriceFamily(payload.Model)]; ok && cap.allowSOptional {
		if hasDramaFirstLast(payload.References) {
			body["task_mode"] = "first_last_frame"
		} else if hasDramaImageRefs(payload.References) {
			body["task_mode"] = "image_reference"
		}
	}
	if cap, ok := dramaVideoCapabilities[CanonicalDramaVideoPriceFamily(payload.Model)]; ok && cap.requireImagePlaceholders && payload.GenerateAudio == nil && payload.GenerateAudio2 == nil {
		body["generate_audio"] = false
	}
	if payload.ReturnLastFrame != nil {
		body["return_last_frame"] = *payload.ReturnLastFrame
	}
	if payload.WebSearch != nil {
		body["web_search"] = *payload.WebSearch
	}
	if payload.Priority != nil {
		body["priority"] = *payload.Priority
	}
	return json.Marshal(body)
}

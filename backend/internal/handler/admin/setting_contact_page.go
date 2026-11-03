package admin

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	contactPageImageMaxBytes = 800 * 1024 // 600KB image after base64 encoding (~4/3) plus data URL prefix
	contactPageTextMaxRunes  = 2000
)

func boolOrPrevious(v *bool, previous bool) bool {
	if v != nil {
		return *v
	}
	return previous
}

// validateContactPageSettings checks the admin-configured contact page content.
// Empty values are allowed and hide the corresponding block on the page.
func validateContactPageSettings(image, link, text string) error {
	image = strings.TrimSpace(image)
	if image != "" {
		if len(image) > contactPageImageMaxBytes {
			return errors.New("Contact page image is too large (max 600KB)")
		}
		if !strings.HasPrefix(image, "data:image/") {
			if err := config.ValidateAbsoluteHTTPURL(image); err != nil {
				return errors.New("Contact page image must be an uploaded image or an http(s) URL")
			}
		}
	}
	if link = strings.TrimSpace(link); link != "" {
		if err := config.ValidateAbsoluteHTTPURL(link); err != nil {
			return errors.New("Contact page link must be an absolute http(s) URL")
		}
	}
	if utf8.RuneCountInString(text) > contactPageTextMaxRunes {
		return errors.New("Contact page text is too long (max 2000 characters)")
	}
	return nil
}

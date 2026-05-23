package service

import (
	"context"
	"errors"
	"strings"
)

const (
	RegistrationCodeTypeAffiliate = "affiliate"
	RegistrationCodeTypeRedeem    = "redeem"
)

type RegistrationCodeValidationResult struct {
	Valid          bool
	CodeType       string
	AvailableSeats *int
	ErrorCode      string
}

type registrationInvitation struct {
	Code            string
	CodeType        string
	AffiliateInvite *AffiliateRegistrationInvite
	RedeemCode      *RedeemCode
}

func effectiveRegistrationCode(invitationCode, affiliateCode string) string {
	code := strings.TrimSpace(invitationCode)
	if code != "" {
		return code
	}
	return strings.TrimSpace(affiliateCode)
}

func (s *AuthService) ValidateRegistrationCode(ctx context.Context, rawCode string) RegistrationCodeValidationResult {
	if s == nil || s.settingService == nil || !s.settingService.IsInvitationCodeEnabled(ctx) {
		return RegistrationCodeValidationResult{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_DISABLED",
		}
	}

	code := strings.TrimSpace(rawCode)
	if code == "" {
		return RegistrationCodeValidationResult{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_REQUIRED",
		}
	}

	if invite, err := s.lookupRegistrationAffiliateInvite(ctx, code); err == nil {
		available := invite.RegistrationSeatAvailable
		if s.affiliateRegistrationSeatsFree(ctx) {
			available = -1
		}
		return RegistrationCodeValidationResult{
			Valid:          true,
			CodeType:       RegistrationCodeTypeAffiliate,
			AvailableSeats: &available,
		}
	} else if errors.Is(err, ErrRegistrationInviteSeatsEmpty) {
		available := 0
		return RegistrationCodeValidationResult{
			Valid:          false,
			CodeType:       RegistrationCodeTypeAffiliate,
			AvailableSeats: &available,
			ErrorCode:      "REGISTRATION_INVITE_SEATS_EMPTY",
		}
	}

	redeemCode, err := s.lookupRegistrationRedeemCode(ctx, code)
	if err != nil {
		return RegistrationCodeValidationResult{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_NOT_FOUND",
		}
	}
	if redeemCode.Type != RedeemTypeInvitation {
		return RegistrationCodeValidationResult{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_INVALID",
		}
	}
	if redeemCode.Status != StatusUnused {
		return RegistrationCodeValidationResult{
			Valid:     false,
			CodeType:  RegistrationCodeTypeRedeem,
			ErrorCode: "INVITATION_CODE_USED",
		}
	}

	return RegistrationCodeValidationResult{
		Valid:    true,
		CodeType: RegistrationCodeTypeRedeem,
	}
}

func (s *AuthService) resolveRegistrationInvitation(ctx context.Context, invitationCode, affiliateCode string, missingErr error) (*registrationInvitation, error) {
	if s == nil || s.settingService == nil || !s.settingService.IsInvitationCodeEnabled(ctx) {
		return nil, nil
	}

	code := effectiveRegistrationCode(invitationCode, affiliateCode)
	if strings.TrimSpace(code) == "" {
		if missingErr != nil {
			return nil, missingErr
		}
		return nil, ErrInvitationCodeRequired
	}

	if invite, err := s.lookupRegistrationAffiliateInvite(ctx, code); err == nil {
		return &registrationInvitation{
			Code:            strings.ToUpper(strings.TrimSpace(code)),
			CodeType:        RegistrationCodeTypeAffiliate,
			AffiliateInvite: invite,
		}, nil
	} else if errors.Is(err, ErrRegistrationInviteSeatsEmpty) {
		return nil, ErrRegistrationInviteSeatsEmpty
	}

	redeemCode, err := s.lookupRegistrationRedeemCode(ctx, code)
	if err != nil {
		return nil, ErrInvitationCodeInvalid
	}
	if redeemCode.Type != RedeemTypeInvitation || redeemCode.Status != StatusUnused {
		return nil, ErrInvitationCodeInvalid
	}
	return &registrationInvitation{
		Code:       strings.TrimSpace(code),
		CodeType:   RegistrationCodeTypeRedeem,
		RedeemCode: redeemCode,
	}, nil
}

func (s *AuthService) lookupRegistrationAffiliateInvite(ctx context.Context, rawCode string) (*AffiliateRegistrationInvite, error) {
	if s == nil || s.affiliateService == nil || s.affiliateService.repo == nil {
		return nil, ErrAffiliateProfileNotFound
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" || !isValidAffiliateCodeFormat(code) {
		return nil, ErrAffiliateProfileNotFound
	}
	invite, err := s.affiliateService.repo.GetRegistrationInviteByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if s.affiliateRegistrationSeatsFree(ctx) {
		return invite, nil
	}
	if invite.RegistrationSeatAvailable <= 0 {
		return invite, ErrRegistrationInviteSeatsEmpty
	}
	return invite, nil
}

func (s *AuthService) affiliateRegistrationSeatsFree(ctx context.Context) bool {
	return s != nil && s.affiliateService != nil && s.affiliateService.registrationSeatCost(ctx) <= 0
}

func (s *AuthService) lookupRegistrationRedeemCode(ctx context.Context, rawCode string) (*RedeemCode, error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return nil, ErrRedeemCodeNotFound
	}
	if s == nil || (s.redeemRepo == nil && s.oauthEmailFlowClient(ctx) == nil) {
		return nil, ErrServiceUnavailable
	}
	return s.loadOAuthRegistrationInvitation(ctx, code)
}

func (s *AuthService) applyRegistrationInvitation(ctx context.Context, invite *registrationInvitation, userID int64) error {
	if invite == nil {
		return nil
	}
	switch invite.CodeType {
	case RegistrationCodeTypeAffiliate:
		if s == nil || s.affiliateService == nil {
			return ErrServiceUnavailable
		}
		if _, err := s.affiliateService.ConsumeRegistrationInviteSeat(ctx, invite.Code, userID); err != nil {
			return err
		}
		return s.affiliateService.BindRegistrationInviterByCode(ctx, userID, invite.Code)
	case RegistrationCodeTypeRedeem:
		if s == nil || s.redeemRepo == nil || invite.RedeemCode == nil {
			return ErrServiceUnavailable
		}
		if err := s.redeemRepo.Use(ctx, invite.RedeemCode.ID, userID); err != nil {
			return ErrInvitationCodeInvalid
		}
		return nil
	default:
		return nil
	}
}

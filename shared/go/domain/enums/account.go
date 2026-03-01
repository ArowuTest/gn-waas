package enums

// AccountCategory represents the billing classification of a water account
// aligned with PURC 2026 tariff categories
type AccountCategory string

const (
	AccountCategoryResidential AccountCategory = "RESIDENTIAL"
	AccountCategoryPublicGovt  AccountCategory = "PUBLIC_GOVT"
	AccountCategoryCommercial  AccountCategory = "COMMERCIAL"
	AccountCategoryIndustrial  AccountCategory = "INDUSTRIAL"
	AccountCategoryBottledWater AccountCategory = "BOTTLED_WATER"
	AccountCategoryUnknown     AccountCategory = "UNKNOWN"
)

func (a AccountCategory) IsValid() bool {
	switch a {
	case AccountCategoryResidential, AccountCategoryPublicGovt,
		AccountCategoryCommercial, AccountCategoryIndustrial,
		AccountCategoryBottledWater, AccountCategoryUnknown:
		return true
	}
	return false
}

func (a AccountCategory) DisplayName() string {
	switch a {
	case AccountCategoryResidential:
		return "Residential"
	case AccountCategoryPublicGovt:
		return "Public / Government"
	case AccountCategoryCommercial:
		return "Commercial"
	case AccountCategoryIndustrial:
		return "Industrial"
	case AccountCategoryBottledWater:
		return "Bottled Water Producer"
	default:
		return "Unknown"
	}
}

// AccountStatus represents the current status of a water account
type AccountStatus string

const (
	AccountStatusActive      AccountStatus = "ACTIVE"
	AccountStatusInactive    AccountStatus = "INACTIVE"
	AccountStatusSuspended   AccountStatus = "SUSPENDED"
	AccountStatusFlagged     AccountStatus = "FLAGGED"
	AccountStatusUnderAudit  AccountStatus = "UNDER_AUDIT"
	AccountStatusGhost       AccountStatus = "GHOST" // Phantom/unverified account
)

func (a AccountStatus) IsValid() bool {
	switch a {
	case AccountStatusActive, AccountStatusInactive, AccountStatusSuspended,
		AccountStatusFlagged, AccountStatusUnderAudit, AccountStatusGhost:
		return true
	}
	return false
}

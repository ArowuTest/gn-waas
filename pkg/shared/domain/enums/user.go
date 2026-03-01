package enums

// UserRole defines RBAC roles within GN-WAAS
// Each role has a specific scope of access
type UserRole string

const (
	// System-level roles
	UserRoleSuperAdmin      UserRole = "SUPER_ADMIN"       // Full system access (your company)
	UserRoleSystemAdmin     UserRole = "SYSTEM_ADMIN"       // System configuration

	// Government roles
	UserRoleMinisterView    UserRole = "MINISTER_VIEW"      // Ministry of Finance - read-only national view
	UserRoleGRAOfficer      UserRole = "GRA_OFFICER"        // GRA - compliance monitoring
	UserRoleMOFAuditor      UserRole = "MOF_AUDITOR"        // Ministry of Finance auditor

	// GWL operational roles
	UserRoleGWLExecutive    UserRole = "GWL_EXECUTIVE"      // GWL management - district dashboards
	UserRoleGWLManager      UserRole = "GWL_MANAGER"        // GWL district manager
	UserRoleGWLAnalyst      UserRole = "GWL_ANALYST"        // GWL data analyst

	// Field roles
	UserRoleFieldSupervisor UserRole = "FIELD_SUPERVISOR"   // Manages field officers
	UserRoleFieldOfficer    UserRole = "FIELD_OFFICER"      // Field audit officer (mobile app)

	// MDA roles
	UserRoleMDAUser         UserRole = "MDA_USER"           // Ministry/Dept/Agency - own bills only
)

func (u UserRole) IsValid() bool {
	switch u {
	case UserRoleSuperAdmin, UserRoleSystemAdmin, UserRoleMinisterView,
		UserRoleGRAOfficer, UserRoleMOFAuditor, UserRoleGWLExecutive,
		UserRoleGWLManager, UserRoleGWLAnalyst, UserRoleFieldSupervisor,
		UserRoleFieldOfficer, UserRoleMDAUser:
		return true
	}
	return false
}

func (u UserRole) CanAccessNationalView() bool {
	switch u {
	case UserRoleSuperAdmin, UserRoleSystemAdmin, UserRoleMinisterView,
		UserRoleMOFAuditor, UserRoleGRAOfficer:
		return true
	}
	return false
}

func (u UserRole) CanManageThresholds() bool {
	switch u {
	case UserRoleSuperAdmin, UserRoleSystemAdmin, UserRoleGWLExecutive:
		return true
	}
	return false
}

func (u UserRole) CanDispatchFieldOfficers() bool {
	switch u {
	case UserRoleSuperAdmin, UserRoleSystemAdmin, UserRoleGWLManager,
		UserRoleFieldSupervisor:
		return true
	}
	return false
}

func (u UserRole) IsFieldRole() bool {
	return u == UserRoleFieldOfficer || u == UserRoleFieldSupervisor
}

// UserStatus represents the account status of a system user
type UserStatus string

const (
	UserStatusActive    UserStatus = "ACTIVE"
	UserStatusInactive  UserStatus = "INACTIVE"
	UserStatusSuspended UserStatus = "SUSPENDED"
	UserStatusPending   UserStatus = "PENDING"
)

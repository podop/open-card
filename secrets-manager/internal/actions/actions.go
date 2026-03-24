package actions

const (
	ReadKeys              = "ReadKeys"
	WriteKeys             = "WriteKeys"
	ManageKeyAccess       = "ManageKeyAccess"
	ReadRotationPolicies  = "ReadRotationPolicies"
	WriteRotationPolicies = "WriteRotationPolicies"
	ReadProviders         = "ReadProviders"
	WriteProviders        = "WriteProviders"
	ReadKeyUsageLog       = "ReadKeyUsageLog"
	UseKey                = "UseKey"
	ReadHSMEndpoints      = "ReadHSMEndpoints"
	WriteHSMEndpoints     = "WriteHSMEndpoints"
	UseHSM                = "UseHSM"
	ReadHSMAudit          = "ReadHSMAudit"
	ReadHSMMetrics        = "ReadHSMMetrics"
	AdminHSM              = "AdminHSM"
	AdminKeys             = "AdminKeys"
)

var AllActions = []string{
	ReadKeys,
	WriteKeys,
	ManageKeyAccess,
	ReadRotationPolicies,
	WriteRotationPolicies,
	ReadProviders,
	WriteProviders,
	ReadKeyUsageLog,
	UseKey,
	ReadHSMEndpoints,
	WriteHSMEndpoints,
	UseHSM,
	ReadHSMAudit,
	ReadHSMMetrics,
	AdminHSM,
	AdminKeys,
}

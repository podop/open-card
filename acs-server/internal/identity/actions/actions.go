package actions

// Action constants — single source of truth for all protected API operations.
const (
	ReadDashboard         = "ReadDashboard"
	ReadTransactions      = "ReadTransactions"
	ReadAuditLogs         = "ReadAuditLogs"
	ReadConfig            = "ReadConfig"
	WriteConfig           = "WriteConfig"
	ReadNetworkSettings   = "ReadNetworkSettings"
	WriteNetworkSettings  = "WriteNetworkSettings"
	ReadCertificates      = "ReadCertificates"
	WriteCertificates     = "WriteCertificates"
	ReadBINSettings       = "ReadBINSettings"
	WriteBINSettings      = "WriteBINSettings"
	ReadCardRanges        = "ReadCardRanges"
	WriteCardRanges       = "WriteCardRanges"
	ReadMFAProviders      = "ReadMFAProviders"
	WriteMFAProviders     = "WriteMFAProviders"
	ReadNetworkRules      = "ReadNetworkRules"
	WriteNetworkRules     = "WriteNetworkRules"
	ReadTestScenario      = "ReadTestScenario"
	WriteTestScenario     = "WriteTestScenario"
	RunTestScenario       = "RunTestScenario"
	ReadChannelResolvers  = "ReadChannelResolvers"
	WriteChannelResolvers = "WriteChannelResolvers"
	ReadIdentitySettings  = "ReadIdentitySettings"
	WriteIdentitySettings = "WriteIdentitySettings"
	ReadOReqHistory       = "ReadOReqHistory"
	ManageUsers           = "ManageUsers"
	ManageGroups          = "ManageGroups"

	// Secrets Manager actions
	ReadKeys              = "ReadKeys"
	WriteKeys             = "WriteKeys"
	ManageKeyAccess       = "ManageKeyAccess"
	ReadRotationPolicies  = "ReadRotationPolicies"
	WriteRotationPolicies = "WriteRotationPolicies"
	ReadProviders         = "ReadProviders"
	WriteProviders        = "WriteProviders"
	ReadKeyUsageLog       = "ReadKeyUsageLog"
	UseKey                = "UseKey"

	// HSM Gateway actions
	ReadHSMEndpoints      = "ReadHSMEndpoints"
	WriteHSMEndpoints     = "WriteHSMEndpoints"
	UseHSM                = "UseHSM"
	ReadHSMAudit          = "ReadHSMAudit"
	ReadHSMMetrics        = "ReadHSMMetrics"
	AdminHSM              = "AdminHSM"
)

// AllActions contains every defined action. Used to seed the admin group.
var AllActions = []string{
	ReadDashboard,
	ReadTransactions,
	ReadAuditLogs,
	ReadConfig,
	WriteConfig,
	ReadNetworkSettings,
	WriteNetworkSettings,
	ReadCertificates,
	WriteCertificates,
	ReadBINSettings,
	WriteBINSettings,
	ReadCardRanges,
	WriteCardRanges,
	ReadMFAProviders,
	WriteMFAProviders,
	ReadNetworkRules,
	WriteNetworkRules,
	ReadTestScenario,
	WriteTestScenario,
	RunTestScenario,
	ReadChannelResolvers,
	WriteChannelResolvers,
	ReadIdentitySettings,
	WriteIdentitySettings,
	ReadOReqHistory,
	ManageUsers,
	ManageGroups,
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
}

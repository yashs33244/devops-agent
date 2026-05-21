package scheduling

// Storage Record collection (scheduling.md §7).
const schedulesCollection = "schedules"

// Record attribute keys on schedules.
const (
	attrUserID  = "user_id"
	attrEnabled = "enabled"
	attrCron    = "cron"
)

// recordContentType matches the convention used by chunks 11/15/16
// (`application/x-protobuf` for proto-marshaled Record bodies).
const recordContentType = "application/x-protobuf"

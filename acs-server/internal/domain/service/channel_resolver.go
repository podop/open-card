package service

import (
	"context"

	"github.com/your-org/acs-server/internal/domain/entity"
)

// ChannelAvailability describes whether a challenge channel (OTP or OOB) is
// reachable for a given cardholder.
type ChannelAvailability struct {
	Available bool
	Reason    string
}

// OTPChannelResolver checks whether the OTP channel (SMS/email) is reachable
// for the cardholder identified by PAN and network.
type OTPChannelResolver interface {
	CheckOTPAvailable(ctx context.Context, pan string, network entity.Network) (*ChannelAvailability, error)
}

// OOBChannelResolver checks whether the OOB channel (banking app push) is
// reachable for the cardholder identified by PAN and network.
type OOBChannelResolver interface {
	CheckOOBAvailable(ctx context.Context, pan string, network entity.Network) (*ChannelAvailability, error)
}

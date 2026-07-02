package authserver

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
)

var (
	ErrAuthorizationPending = errors.New("authorization_pending")
	ErrSlowDown             = errors.New("slow_down")
	ErrAccessDenied         = errors.New("access_denied")
	ErrExpiredToken         = errors.New("expired_token")
	ErrDeviceNotFound       = errors.New("device_not_found")
	ErrDeviceExpired        = errors.New("device_code_expired")
	ErrUnknownClient        = errors.New("invalid_client")
)

// Only first-party clients may start the device flow: the approval screen
// renders the client's display name as a trusted party, so an arbitrary
// attacker-chosen client_id would make phishing prompts look first-party.
var knownDeviceClients = map[string]bool{
	cliClientID: true,
	mcpClientID: true,
}

func CreateDeviceAuth(clientId, baseURL string) (*models.DeviceAuthorizeResponse, error) {
	if clientId == "" {
		clientId = cliClientID
	}
	if !knownDeviceClients[clientId] {
		return nil, ErrUnknownClient
	}

	deviceCode := newOpaqueToken("")
	userCode := newUserCode()
	interval := int(devicePollInterval.Seconds())
	ttl := deviceCodeTTL

	// Opportunistically drop expired rows so this unauthenticated endpoint can
	// never accumulate more than one device-code TTL's worth of rows between
	// the daily prune-worker runs.
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if _, err := repositories.DeviceAuthorizationRepository.PruneExpired(tx, time.Now()); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, repositories.DeviceAuthorizationRepository.Create(tx, deviceCode, userCode, clientId, interval, time.Now().Add(ttl))
	})
	if err != nil {
		return nil, err
	}

	verificationURI := strings.TrimRight(baseURL, "/") + "/device"
	return &models.DeviceAuthorizeResponse{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + userCode,
		ExpiresIn:               int(ttl.Seconds()),
		Interval:                interval,
	}, nil
}

// PollDeviceToken owns its own transactions: the OAuth token endpoint returns
// 400 for normal flow control (pending/slow_down), so it cannot rely on the
// request-scoped Transactional middleware (which only commits on 2xx).
func PollDeviceToken(deviceCode string) (*models.TokenSetResponse, error) {
	da, err := repositories.DeviceAuthorizationRepository.FindByDeviceCode(db.DB, deviceCode)
	if err != nil {
		return nil, err
	}
	if da == nil || time.Now().After(da.ExpiresAt) {
		return nil, ErrExpiredToken
	}

	switch da.Status {
	case "denied":
		return nil, ErrAccessDenied
	case "approved":
		if da.UserId == nil {
			return nil, ErrAccessDenied
		}
		return db.ExecuteTransaction(func(tx *sql.Tx) (*models.TokenSetResponse, error) {
			// Consume the device code first: a device code is single-use, so if
			// the row is already gone another concurrent poll redeemed it and we
			// must not mint a second token set for the same code.
			deleted, err := repositories.DeviceAuthorizationRepository.Delete(tx, deviceCode)
			if err != nil {
				return nil, err
			}
			if deleted == 0 {
				return nil, ErrExpiredToken
			}
			user, err := repositories.UserRepository.FindById(tx, *da.UserId)
			if err != nil {
				return nil, err
			}
			if user == nil {
				return nil, ErrAccessDenied
			}
			return IssueTokenSet(tx, *da.UserId, user.Email, da.ClientId)
		})
	default:
		now := time.Now()
		interval := da.IntervalSeconds
		slow := da.LastPolledAt != nil && now.Before(da.LastPolledAt.Add(time.Duration(da.IntervalSeconds)*time.Second))
		if slow {
			interval = da.IntervalSeconds + 5
		}
		_, _ = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
			return struct{}{}, repositories.DeviceAuthorizationRepository.MarkPolled(tx, deviceCode, now, interval)
		})
		if slow {
			return nil, ErrSlowDown
		}
		return nil, ErrAuthorizationPending
	}
}

func Approve(tx *sql.Tx, userCode string, userId int) error {
	return resolveDevice(tx, userCode, func(canonicalCode string) (int64, error) {
		return repositories.DeviceAuthorizationRepository.Approve(tx, canonicalCode, userId)
	})
}

func Deny(tx *sql.Tx, userCode string) error {
	return resolveDevice(tx, userCode, func(canonicalCode string) (int64, error) {
		return repositories.DeviceAuthorizationRepository.Deny(tx, canonicalCode)
	})
}

// resolveDevice loads the authorization first so an expired-but-still-pending
// code is rejected here (the poll endpoint would refuse it anyway, so letting
// the approval "succeed" would only mislead the user), and so the status
// update runs against the stored canonical user code.
func resolveDevice(tx *sql.Tx, userCode string, update func(canonicalCode string) (int64, error)) error {
	da, err := repositories.DeviceAuthorizationRepository.FindByUserCode(tx, userCode)
	if err != nil {
		return err
	}
	if da == nil {
		return ErrDeviceNotFound
	}
	if time.Now().After(da.ExpiresAt) {
		return ErrDeviceExpired
	}
	rows, err := update(da.UserCode)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDeviceNotFound
	}
	return nil
}

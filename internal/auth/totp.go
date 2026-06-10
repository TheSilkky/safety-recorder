package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	TOTPIssuer               = "Proofline"
	TOTPDefaultPeriodSeconds = 30
	TOTPDefaultDigits        = 6
	TOTPDefaultSkewSteps     = 1
	TOTPDefaultSecretBytes   = 20
	TOTPAlgorithmSHA1        = "SHA1"
)

var ErrInvalidTOTPPolicy = errors.New("invalid TOTP policy")

type TOTPEnrollment struct {
	Secret        string
	URL           string
	PeriodSeconds int
	Digits        int
	Algorithm     string
}

func GenerateTOTPEnrollment(accountName string) (TOTPEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: strings.TrimSpace(accountName),
		Period:      uint(TOTPDefaultPeriodSeconds),
		SecretSize:  TOTPDefaultSecretBytes,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return TOTPEnrollment{}, err
	}
	return TOTPEnrollment{
		Secret:        key.Secret(),
		URL:           key.URL(),
		PeriodSeconds: TOTPDefaultPeriodSeconds,
		Digits:        TOTPDefaultDigits,
		Algorithm:     TOTPAlgorithmSHA1,
	}, nil
}

func MatchTOTPCode(secret, code string, now time.Time, periodSeconds, digits int, algorithm string) (int64, bool, error) {
	secret = strings.TrimSpace(secret)
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return 0, false, nil
	}
	opts, err := totpValidateOptions(periodSeconds, digits, algorithm)
	if err != nil {
		return 0, false, err
	}
	if len(code) != opts.Digits.Length() {
		return 0, false, nil
	}
	now = now.UTC()
	period := int64(periodSeconds)
	currentStep := now.Unix() / period
	for offset := -int64(TOTPDefaultSkewSteps); offset <= int64(TOTPDefaultSkewSteps); offset++ {
		step := currentStep + offset
		if step < 0 {
			continue
		}
		stepTime := time.Unix(step*period, 0).UTC()
		want, err := totp.GenerateCodeCustom(secret, stepTime, opts)
		if err != nil {
			return 0, false, err
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step, true, nil
		}
	}
	return 0, false, nil
}

func GenerateTOTPCodeForTest(secret string, at time.Time) (string, error) {
	opts, err := totpValidateOptions(TOTPDefaultPeriodSeconds, TOTPDefaultDigits, TOTPAlgorithmSHA1)
	if err != nil {
		return "", err
	}
	return totp.GenerateCodeCustom(secret, at.UTC(), opts)
}

func totpValidateOptions(periodSeconds, digits int, algorithm string) (totp.ValidateOpts, error) {
	if periodSeconds <= 0 {
		return totp.ValidateOpts{}, fmt.Errorf("%w: period must be positive", ErrInvalidTOTPPolicy)
	}
	if digits != TOTPDefaultDigits {
		return totp.ValidateOpts{}, fmt.Errorf("%w: digits must be %d", ErrInvalidTOTPPolicy, TOTPDefaultDigits)
	}
	if algorithm != TOTPAlgorithmSHA1 {
		return totp.ValidateOpts{}, fmt.Errorf("%w: algorithm must be %s", ErrInvalidTOTPPolicy, TOTPAlgorithmSHA1)
	}
	return totp.ValidateOpts{
		Period:    uint(periodSeconds),
		Skew:      0,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}, nil
}

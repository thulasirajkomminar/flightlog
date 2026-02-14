package domain

import (
	"errors"
	"strings"
)

// Sentinel validation errors.
var (
	ErrIATALength    = errors.New("IATA airline code must be exactly 2 characters")
	ErrIATAChars     = errors.New("IATA airline code must contain only A-Z and 0-9")
	ErrIATATwoDigits = errors.New("IATA airline code cannot be two digits")

	ErrFlightNumEmpty     = errors.New("flight number is empty")
	ErrFlightNumTooLong   = errors.New("flight number too long, max 4 digits + 1 optional letter suffix")
	ErrFlightNumNoDigit   = errors.New("flight number must start with a digit")
	ErrFlightNumMaxDigits = errors.New("flight number allows max 4 digits")
	ErrFlightNumLeadZero  = errors.New("flight number must not start with 0")
	ErrFlightNumSuffix    = errors.New("flight number suffix must be a single letter A-Z")

	ErrFlightTooShort   = errors.New("flight designator too short")
	ErrFlightBadAirline = errors.New("could not identify a valid IATA airline prefix")
)

const (
	iataLen      = 2
	maxFlightNum = 5
	maxDigits    = 4
)

// ValidateIATA checks if code is a valid IATA airline designator.
func ValidateIATA(code string) error {
	code = strings.TrimSpace(strings.ToUpper(code))
	if len(code) != iataLen {
		return ErrIATALength
	}

	if !isAlphanumIATA(code) {
		return ErrIATAChars
	}

	if isDigit(code[0]) && isDigit(code[1]) {
		return ErrIATATwoDigits
	}

	return nil
}

// ValidateFlightNumber checks the numeric part (without airline prefix).
func ValidateFlightNumber(num string) error {
	num = strings.TrimSpace(strings.ToUpper(num))

	err := checkFlightNumLength(num)
	if err != nil {
		return err
	}

	digitCount := countLeadingDigits(num)

	err = checkDigits(num, digitCount)
	if err != nil {
		return err
	}

	return checkSuffix(num[digitCount:])
}

// ValidateFlight validates a full IATA flight designator like "U27898" or "AA123".
func ValidateFlight(flight string) error {
	flight = strings.TrimSpace(strings.ToUpper(flight))
	if len(flight) < iataLen+1 {
		return ErrFlightTooShort
	}

	err := ValidateIATA(flight[:iataLen])
	if err != nil {
		return ErrFlightBadAirline
	}

	return ValidateFlightNumber(flight[iataLen:])
}

// NormalizeFlightNumber strips spaces and uppercases a flight number for storage.
func NormalizeFlightNumber(flightNumber string) string {
	return strings.ToUpper(strings.ReplaceAll(flightNumber, " ", ""))
}

// FormatFlightNumber inserts a space after the 2-character airline prefix for display.
func FormatFlightNumber(flightNumber string) string {
	n := NormalizeFlightNumber(flightNumber)
	if len(n) <= iataLen {
		return n
	}

	return n[:iataLen] + " " + n[iataLen:]
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isAlphanumIATA(code string) bool {
	for i := range len(code) {
		b := code[i]
		if (b < 'A' || b > 'Z') && !isDigit(b) {
			return false
		}
	}

	return true
}

func countLeadingDigits(s string) int {
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}

	return n
}

func checkFlightNumLength(num string) error {
	if len(num) == 0 {
		return ErrFlightNumEmpty
	}

	if len(num) > maxFlightNum {
		return ErrFlightNumTooLong
	}

	return nil
}

func checkDigits(num string, digitCount int) error {
	if digitCount == 0 {
		return ErrFlightNumNoDigit
	}

	if digitCount > maxDigits {
		return ErrFlightNumMaxDigits
	}

	if num[0] == '0' {
		return ErrFlightNumLeadZero
	}

	return nil
}

func checkSuffix(suffix string) error {
	if len(suffix) == 0 {
		return nil
	}

	if len(suffix) > 1 || suffix[0] < 'A' || suffix[0] > 'Z' {
		return ErrFlightNumSuffix
	}

	return nil
}

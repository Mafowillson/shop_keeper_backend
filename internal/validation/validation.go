package validation

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9]{7,15}$`)
	uuidPattern  = regexp.MustCompile(`^[0-9a-fA-F\-]{36}$`)
)

func ValidateString(value string, fieldName string, min, max int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New(fieldName + " is required")
	}
	if len(trimmed) < min {
		return errors.New(fieldName + " must be at least " + strconv.Itoa(min) + " characters")
	}
	if len(trimmed) > max {
		return errors.New(fieldName + " must be at most " + strconv.Itoa(max) + " characters")
	}
	return nil
}

func ValidateOptionalString(value string, fieldName string, max int) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if len(value) > max {
		return errors.New(fieldName + " must be at most " + strconv.Itoa(max) + " characters")
	}
	return nil
}

func ValidateEmail(value string) error {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return errors.New("email is required")
	}
	if !emailPattern.MatchString(value) {
		return errors.New("email must be a valid address")
	}
	return nil
}

func ValidatePhone(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("phone number is required")
	}
	if !phonePattern.MatchString(value) {
		return errors.New("phone number must be between 7 and 15 digits, optionally starting with +")
	}
	return nil
}

func ValidateUUID(value string, fieldName string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New(fieldName + " is required")
	}
	if !uuidPattern.MatchString(value) {
		return errors.New(fieldName + " must be a valid uuid")
	}
	return nil
}

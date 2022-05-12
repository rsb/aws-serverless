package domain

import (
	"fmt"
	"strings"

	"github.com/rsb/failure"
)

const (
	DefaultKeySeparator = ":"
)

type Key struct {
	cat    string
	domain string
	org    string
	sep    string
}

func NewKey(org, cat, domain string) (Key, error) {
	var key Key
	var err error

	sep := DefaultKeySeparator
	org, err = validateKeyProperty("org", org, sep)
	if err != nil {
		return key, err
	}

	cat, err = validateKeyProperty("cat", cat, sep)
	if err != nil {
		return key, err
	}

	cat, err = validateKeyProperty("domain", domain, sep)
	if err != nil {
		return key, err
	}

	key = Key{
		org:    org,
		sep:    sep,
		domain: domain,
		cat:    cat,
	}

	return key, nil
}

func (k Key) String() string {
	return fmt.Sprintf("%s%s%s%s%s", k.org, k.sep, k.cat, k.sep, k.domain)
}

func (k Key) Category() string {
	return k.cat
}

func (k Key) Org() string {
	return k.org
}

func (k Key) Domain() string {
	return k.domain
}

func (k Key) Separator() string {
	return k.sep
}

func validateKeyProperty(name, value, sep string) (string, error) {
	var safe string
	if value == "" {
		return safe, failure.InvalidParam("[%s] is empty", name)
	}

	if strings.Contains(value, sep) {
		return safe, failure.InvalidParam("[%s] contains key separator (%s) value (%s)", name, sep, value)
	}

	return strings.ToLower(value), nil
}

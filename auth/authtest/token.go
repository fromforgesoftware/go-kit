package authtest

import "time"

type tokenClaims struct {
	subject string
}

func NewTokenClaims(subject string) *tokenClaims {
	return &tokenClaims{subject: subject}
}

func (c *tokenClaims) Subject() string {
	return c.subject
}

func (c *tokenClaims) Expiry() time.Time {
	return time.Now().Add(time.Hour)
}

func (c *tokenClaims) Get(key string) any {
	return nil
}

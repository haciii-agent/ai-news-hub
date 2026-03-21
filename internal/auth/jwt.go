package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Claims represents the JWT payload.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
}

// TokenPair holds an access token and its expiry in seconds.
type TokenPair struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(secret []byte, data string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// GenerateToken creates a JWT with format: header.payload.signature
func GenerateToken(secret []byte, expiry time.Duration, userID int64, username, role string) (*TokenPair, error) {
	if len(secret) == 0 {
		return nil, errors.New("jwt secret is empty")
	}

	now := time.Now()
	exp := now.Add(expiry)

	// Header: {"alg":"HS256","typ":"JWT"}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Payload
	payload := map[string]interface{}{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"iat":      now.Unix(),
		"exp":      exp.Unix(),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Signature: HMAC-SHA256 over "header.payload"
	signingInput := headerB64 + "." + payloadB64
	sig := hmacSHA256(secret, signingInput)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := signingInput + "." + sigB64

	return &TokenPair{
		AccessToken: token,
		ExpiresIn:   int64(expiry.Seconds()),
	}, nil
}

// ValidateToken verifies a JWT and returns its Claims.
func ValidateToken(secret []byte, tokenStr string) (*Claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig := hmacSHA256(secret, signingInput)
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expectedSig, actualSig) {
		return nil, errors.New("invalid token signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload")
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, errors.New("invalid token claims")
	}

	// Check expiration
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

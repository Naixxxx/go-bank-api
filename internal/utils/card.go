package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func ComputeHMAC(data string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(data))

	return hex.EncodeToString(h.Sum(nil))
}

func GenerateCVV() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%03d", n.Int64()), nil
}

func GenerateCardNumber() (string, error) {
	prefix := "2200"
	body := prefix

	for len(body) < 15 {
		n, err := rand.Int(rand.Reader, big.NewInt(10))

		if err != nil {
			return "", err
		}

		body += fmt.Sprint(n.Int64())
	}

	return body + fmt.Sprint(luhnCheckDigit(body)), nil
}

func luhnCheckDigit(num string) int {
	sum := 0
	double := true

	for i := len(num) - 1; i >= 0; i-- {
		d := int(num[i] - '0')

		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		double = !double
	}

	return (10 - sum%10) % 10
}

func ValidLuhn(num string) bool {
	num = strings.ReplaceAll(num, " ", "")
	sum := 0
	double := false

	for i := len(num) - 1; i >= 0; i-- {
		if num[i] < '0' || num[i] > '9' {
			return false
		}

		d := int(num[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		double = !double
	}

	return sum%10 == 0
}

func Expiry() string { return time.Now().AddDate(3, 0, 0).Format("01/06") }

func MaskCard(number string) string {
	if len(number) < 4 {
		return "****"
	}

	return "**** **** **** " + number[len(number)-4:]
}

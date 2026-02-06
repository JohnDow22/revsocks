package common

import (
	"crypto/rand"
	"math/big"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandString генерирует случайную строку заданной длины
func RandString(n int) string {
	r := make([]byte, n)
	_, err := rand.Read(r)
	if err != nil {
		return ""
	}

	b := make([]byte, n)
	l := len(letters)
	for i := range b {
		b[i] = letters[int(r[i])%l]
	}
	return string(b)
}

// RandBytes генерирует случайный массив байтов заданной длины
func RandBytes(n int) []byte {
	r := make([]byte, n)
	_, err := rand.Read(r)
	if err != nil {
		return nil
	}
	return r
}

// RandBigInt генерирует случайное большое целое число до max
func RandBigInt(max *big.Int) *big.Int {
	r, _ := rand.Int(rand.Reader, max)
	return r
}

package utils

import (
	"hash/fnv"
	"math/rand"
	"time"
)

var defaultSrc = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func hash(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	nr := h.Sum64()
	return int64(nr)
}

func RandString(length int) string {
	return RandStringSeed(length, defaultSrc)
}

func RandStringStrSeed(length int, src string) string {
	seed := rand.NewSource(hash(src))
	return RandStringSeed(length, seed)
}

func RandStringSeed(length int, src rand.Source) string {
	b := make([]byte, length)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := length-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

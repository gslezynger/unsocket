package internal

import (

	"time"
	"math/rand"
)


func Randomstring(strlen int) string {


	var randomSeed *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[randomSeed.Intn(len(chars))]
	}
	return string(result)
}

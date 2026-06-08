// Package hashpath generates an S3 object key from the SHA-512 hash of the content according to the following scheme
package hashpath

import (
	"crypto/sha512"
	"encoding/hex"
)

func SumHex(data []byte) string {
	sum := sha512.Sum512(data)
	return hex.EncodeToString(sum[:])
}

// hex[0:4]/hex[4:8]/hexi
func KeyFromHex(hexsum string) string {
	if len(hexsum) < 8 {
		panic("hashpath: hex sum too short")
	}
	return hexsum[0:4] + "/" + hexsum[4:8] + "/" + hexsum
}

func Key(data []byte) string {
	return KeyFromHex(SumHex(data))
}

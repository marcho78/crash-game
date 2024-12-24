package crypto

import (
    "crypto/rand"
    "encoding/binary"
)

func GenerateSecureRandomInt(max int64) (int64, error) {
    var b [8]byte
    _, err := rand.Read(b[:])
    if err != nil {
        return 0, err
    }

    n := binary.BigEndian.Uint64(b[:])
    return int64(n) % max, nil
}

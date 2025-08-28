package stamp

import "time"

// NTP_EPOCH_OFFSET eh a diferenca em segundos entre a epoca Unix e a epoca NTP.
const ntpEpochOffset = 2208988800

// ToNTP converts a time.Time to NTP 64-bit timestamp format.
func ToNTP(t time.Time) uint64 {
    secs := uint64(t.Unix()) + ntpEpochOffset
    nanos := uint64(t.Nanosecond())
    // Converte nanossegundos para a fração de 2^32
    fraction := (nanos << 32) / 1e9
    return (secs << 32) | fraction
}

// FromNTP converts an NTP timestamp to a time.Time.
func FromNTP(ntp uint64) time.Time {
    secs := (ntp >> 32) - ntpEpochOffset
    fraction := ntp & 0xFFFFFFFF
    nanos := (fraction * 1e9) >> 32
    return time.Unix(int64(secs), int64(nanos))
}

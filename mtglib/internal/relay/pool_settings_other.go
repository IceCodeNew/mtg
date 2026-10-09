//go:build !mips && !mipsle

package relay

import "github.com/IceCodeNew/mtg/mtglib/internal/tls"

const (
	bufPoolSize = tls.MaxRecordPayloadSize
)

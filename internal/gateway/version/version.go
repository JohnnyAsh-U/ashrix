package version

import (
	"strconv"
	"strings"
	"time"
)

var (
	Version = "v1.0.0"
	BuildDate = time.Now().String()
)

func GetGatewayProtocolVersion() uint32 {

	if Version == "dev" || Version == ""{
		return 0
	}

	v := strings.TrimPrefix(Version, "v")

	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")

	var major, minor, patch uint32 = 0, 0, 0

	if len(parts) > 0 {
		if val, err := strconv.ParseUint(parts[0], 10, 8); err == nil {
			major = uint32(val)
		}
	}

	if len(parts) > 1 {
		if val, err := strconv.ParseUint(parts[1], 10, 8); err == nil {
			minor = uint32(val)
		}
	}

	if len(parts) > 2 {
		if val, err := strconv.ParseUint(parts[2], 10, 8); err == nil {
			patch = uint32(val)
		}
	}


	return (major << 24) | (minor << 16) | patch
}

func GetGatewayVersion() string {
	return Version
}

func GetGatewayBuildDate() string {
	return BuildDate
}
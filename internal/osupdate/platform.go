package osupdate

import "fmt"

type platformContract struct {
	goArchitecture, ociArchitecture, artifactArchitecture, ociPlatform string
}

func platformFor(architecture string) (platformContract, error) {
	switch architecture {
	case "arm64":
		return platformContract{"arm64", "arm64", "aarch64", "linux/arm64"}, nil
	case "amd64":
		return platformContract{"amd64", "amd64", "x86_64", "linux/amd64"}, nil
	default:
		return platformContract{}, fmt.Errorf("unsupported Soda runtime architecture %q", architecture)
	}
}

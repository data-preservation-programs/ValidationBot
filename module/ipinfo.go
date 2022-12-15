package module

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"validation-bot/resources"

	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

type IPInfoResolver struct {
	Continents map[string]string
}

func NewIPInfoResolver() (IPInfoResolver, error) {
	payload := make(map[string]string)

	if err := json.Unmarshal(resources.CountryToContinentJSON, &payload); err != nil {
		return IPInfoResolver{}, errors.Wrap(err, "ipinfo: failed to unmarshal continents")
	}

	return IPInfoResolver{
		Continents: payload,
	}, nil
}

func (i IPInfoResolver) ResolveIP(ctx context.Context, ip net.IP) (string, error) {
	url := fmt.Sprintf("https://ipinfo.io/%s?token=%s", ip, os.Getenv("IPINFO_TOKEN"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return "", errors.Wrap(err, "failed to create http request")
	}

	req.Header.Set("Accept", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return "", errors.Wrap(err, "failed to resolve IP")
	}

	defer resp.Body.Close()

	payload := make(map[string]interface{})

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", errors.Wrap(err, "ipinfo: failed to unmarshal response")
	}

	if countryCode, ok := payload["country"]; ok {
		if countryCodeStr, ok := countryCode.(string); ok {
			return countryCodeStr, nil
		}
	}

	return "", nil
}

func (i IPInfoResolver) ResolveIPStr(ctx context.Context, ip string) (string, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", errors.Errorf("failed to parse IP address %s", ip)
	}

	countryCode, err := i.ResolveIP(ctx, parsed)

	if err != nil {
		return "", errors.Wrap(err, "failed to resolve IP")
	}

	return countryCode, nil
}

func (i IPInfoResolver) ResolveMultiAddr(ctx context.Context, addr multiaddr.Multiaddr) (string, error) {
	host, isHostName, _, err := ResolveHostAndIP(addr)
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve host and port")
	}

	if isHostName {
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", errors.Wrapf(err, "failed to lookup host %s", host)
		}

		host = ips[0].String()
	}

	return i.ResolveIPStr(ctx, host)
}

package main

import (
	"io"
	"net"
	"net/http"

	"github.com/anaganisk/digitalocean-dynamic-dns-ip/config"
	"github.com/anaganisk/digitalocean-dynamic-dns-ip/logger"
)

// CheckLocalIPs : get current IP of server. checks both IPv4 and Ipv6 to support dual stack environments
func CheckLocalIPs() (ipv4, ipv6 net.IP) {
	var ipv4String, ipv6String string
	ipv4CheckURL := "https://api.ipify.org/?format=text"
	ipv6CheckURL := "https://api64.ipify.org/?format=text"
	conf := config.Get()
	if len(conf.IPv4CheckURL) > 0 {
		ipv4CheckURL = conf.IPv4CheckURL
	}
	if len(conf.IPv6CheckURL) > 0 {
		ipv6CheckURL = conf.IPv6CheckURL
	}

	if conf.UseIPv4 == nil || *(conf.UseIPv4) {
		logger.Debug("Checking IPv4 with URL: %s", ipv4CheckURL)
		ipv4String, _ = getURLBody(ipv4CheckURL)
		if ipv4String == "" {
			logger.Warning("No IPv4 address found. Consider disabling IPv4 checks in the config `\"useIPv4\": false`")
		} else {
			ipv4 = net.ParseIP(ipv4String)
			if ipv4 != nil {
				// make sure we got back an actual ipv4 address
				ipv4 = ipv4.To4()
				logger.Debug("Discovered IPv4 address `%s`", ipv4.String())
			}
			if ipv4 == nil {
				logger.Warningf("Unable to parse `%s` as an IPv4 address", ipv4String)
			}
		}
	}

	if conf.UseIPv6 == nil || *(conf.UseIPv6) {
		logger.Debug("Checking IPv6 with URL: %s", ipv6CheckURL)
		ipv6String, _ = getURLBody(ipv6CheckURL)
		if ipv6String == "" {
			logger.Warning("No IPv6 address found. Consider disabling IPv6 checks in the config `\"useIPv6\": false`")
		} else {
			ipv6 = net.ParseIP(ipv6String)
			if ipv6 == nil {
				logger.Warningf("Unable to parse `%s` as an IPv6 address", ipv6String)
			} else {
				logger.Debug("Discovered IPv6 address `%s`", ipv6.String())
			}
		}
	}
	return ipv4, ipv6
}

// getURLBody fetches the body of the given URL as a string
func getURLBody(url string) (string, error) {
	client := &http.Client{Timeout: config.Get().GetHTTPTimeout()}
	request, err := client.Get(url)
	logger.CheckError(err)
	defer request.Body.Close()
	body, err := io.ReadAll(request.Body)
	logger.CheckError(err)
	return string(body), nil
}

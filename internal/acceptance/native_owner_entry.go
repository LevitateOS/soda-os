package acceptance

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Probe the tempting first-login paths before the operator registers the owner.
// Credentials travel on the existing stdin boundary, never in command arguments.
func verifyOwnerEntry(ctx context.Context, person personFixture, evidence string) error {
	config := fmt.Sprintf("silent\nshow-error\nfail\nmax-time = 15\nurl = %s\n", curlConfigQuote(forgejoLoopbackEndpoint+"/"))
	home, err := person.Remote.CaptureOutput(ctx, evidence+"-home", []byte(config), "curl", "--config", "-")
	if err != nil {
		return err
	}
	if !strings.Contains(string(home), "Create your Forgejo administrator account") {
		return fmt.Errorf("first-owner guidance is missing from the unclaimed Forgejo homepage")
	}
	password := string(bytes.TrimRight(person.LinuxPassword, "\r\n"))
	config = fmt.Sprintf("silent\nshow-error\nmax-time = 15\noutput = \"/dev/null\"\nwrite-out = \"%%{http_code}\"\nuser = %s\nurl = %s\n", curlConfigQuote(person.Remote.Username+":"+password), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user"))
	status, err := person.Remote.CaptureOutput(ctx, evidence+"-early-api", []byte(config), "curl", "--config", "-")
	if err != nil {
		return err
	}
	if string(status) != "401" {
		return fmt.Errorf("pre-owner PAM API login must return 401, got %q", status)
	}
	form := url.Values{"user_name": {person.Remote.Username}, "password": {password}}.Encode()
	config = fmt.Sprintf("silent\nshow-error\nmax-time = 15\noutput = \"/dev/null\"\nwrite-out = \"%%{http_code} %%{redirect_url}\"\nheader = %s\ndata = %s\nurl = %s\n", curlConfigQuote("Origin: "+forgejoLoopbackEndpoint), curlConfigQuote(form), curlConfigQuote(forgejoLoopbackEndpoint+"/user/login"))
	status, err = person.Remote.CaptureOutput(ctx, evidence+"-early-web", []byte(config), "curl", "--config", "-")
	if err != nil {
		return err
	}
	if string(status) != "303 "+forgejoLoopbackEndpoint+"/user/sign_up" {
		return fmt.Errorf("pre-owner web login must lead to native registration, got %q", status)
	}
	return nil
}

package caddy_edgeone_ip

import (
	"context"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/pkg/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	teo "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/teo/v20220901"
	"go.uber.org/zap"
)

type EdgeOneIPRange struct {
	Interval    caddy.Duration `json:"interval,omitempty"`
	Timeout     caddy.Duration `json:"timeout,omitempty"`
	APIEndpoint string         `json:"api_endpoint,omitempty"`
	SecretID    string         `json:"secret_id,omitempty"`
	SecretKey   string         `json:"secret_key,omitempty"`
	ZoneID      string         `json:"zone_id,omitempty"`

	logger *zap.Logger
	ranges []netip.Prefix

	ctx  caddy.Context
	lock *sync.RWMutex
}

func init() {
	caddy.RegisterModule(EdgeOneIPRange{})
}

func (EdgeOneIPRange) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.ip_sources.edgeone",
		New: func() caddy.Module { return new(EdgeOneIPRange) },
	}
}

func (s *EdgeOneIPRange) getContext() (context.Context, context.CancelFunc) {
	if s.Timeout > 0 {
		return context.WithTimeout(s.ctx, time.Duration(s.Timeout))
	}
	return context.WithCancel(s.ctx)
}

func (s *EdgeOneIPRange) fetch() ([]netip.Prefix, error) {
	ctx, cancel := s.getContext()
	defer cancel()

	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = s.APIEndpoint
	if s.APIEndpoint == "" {
		s.APIEndpoint = "teo.tencentcloudapi.com"
	}
	if s.SecretID == "" || s.SecretKey == "" {
		return nil, errors.New("secret_id and secret_key are required")
	}
	credential := common.NewCredential(s.SecretID, s.SecretKey)
	client, err := teo.NewClient(credential, "", cpf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create teo client")
	}

	request := teo.NewDescribeOriginACLRequest()
	request.ZoneId = common.StringPtr(s.ZoneID)
	request.SetContext(ctx)
	response, err := client.DescribeOriginACL(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe origin acl")
	}

	prefixes := make([]netip.Prefix, 0)
	if current := response.Response.OriginACLInfo.CurrentOriginACL; current != nil {
		currentPrefixes, err := s.addressesToPrefix(current.EntireAddresses)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert current origin acl to prefixes")
		}
		prefixes = append(prefixes, currentPrefixes...)
	}
	if next := response.Response.OriginACLInfo.NextOriginACL; next != nil {
		nextPrefixes, err := s.addressesToPrefix(next.EntireAddresses)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert preview origin acl to prefixes")
		}
		prefixes = append(prefixes, nextPrefixes...)
	}
	if len(prefixes) == 0 {
		return nil, errors.New("no prefixes found")
	}
	return prefixes, nil
}

func (s *EdgeOneIPRange) addressesToPrefix(addresses *teo.Addresses) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0)
	for _, address := range addresses.IPv4 {
		prefix, err := caddyhttp.CIDRExpressionToPrefix(*address)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse ipv4 prefix %s", *address)
		}
		prefixes = append(prefixes, prefix)
	}
	for _, address := range addresses.IPv6 {
		prefix, err := caddyhttp.CIDRExpressionToPrefix(*address)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse ipv6 prefix %s", *address)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func (s *EdgeOneIPRange) getPrefixes() *[]netip.Prefix {
	s.lock.Lock()
	defer s.lock.Unlock()
	ranges, err := s.fetch()
	if err != nil {
		s.logger.Error("failed to get prefixes", zap.Error(err))
	}
	return &ranges
}

func (s *EdgeOneIPRange) refreshLoop() {
	if s.Interval == 0 {
		s.Interval = caddy.Duration(time.Hour)
	}

	ticker := time.NewTicker(time.Duration(s.Interval))
	if ranges := s.getPrefixes(); ranges != nil {
		s.ranges = *ranges
	}
	for {
		select {
		case <-ticker.C:
			if ranges := s.getPrefixes(); ranges != nil {
				s.ranges = *ranges
			}
		case <-s.ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (s *EdgeOneIPRange) Provision(ctx caddy.Context) error {
	s.ctx = ctx
	s.logger = ctx.Logger()
	s.lock = new(sync.RWMutex)

	go s.refreshLoop()
	return nil
}

func (s *EdgeOneIPRange) GetIPRanges(_ *http.Request) []netip.Prefix {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.ranges
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
//
//	edgeone <zone_id> <secret_id> <secret_key> {
//	   api_endpoint val
//	   interval val
//	   timeout val
//	}
func (s *EdgeOneIPRange) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // Skip module name.
	if !d.NextArg() {
		return d.ArgErr()
	}
	s.ZoneID = d.Val()
	if !d.NextArg() {
		return d.ArgErr()
	}
	s.SecretID = d.Val()
	if !d.NextArg() {
		return d.ArgErr()
	}
	s.SecretKey = d.Val()

	for nesting := d.Nesting(); d.NextBlock(nesting); {
		switch d.Val() {
		case "interval":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return err
			}
			s.Interval = caddy.Duration(val)
		case "timeout":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return err
			}
			s.Timeout = caddy.Duration(val)
		case "api_endpoint":
			if !d.NextArg() {
				return d.ArgErr()
			}
			s.APIEndpoint = d.Val()
		default:
			return d.ArgErr()
		}
	}

	return nil
}

var (
	_ caddy.Module            = (*EdgeOneIPRange)(nil)
	_ caddy.Provisioner       = (*EdgeOneIPRange)(nil)
	_ caddyfile.Unmarshaler   = (*EdgeOneIPRange)(nil)
	_ caddyhttp.IPRangeSource = (*EdgeOneIPRange)(nil)
)

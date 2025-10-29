package caddy_edgeone_ip

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/pkg/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	teo "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/teo/v20220901"
	"go.uber.org/zap"
)

type EdgeOneIPRange struct {
	CacheSize   int            `json:"cache_size,omitempty"`
	CacheTTL    caddy.Duration `json:"cache_ttl,omitempty"`
	Timeout     caddy.Duration `json:"timeout,omitempty"`
	APIEndpoint string         `json:"api_endpoint,omitempty"`
	SecretID    string         `json:"secret_id,omitempty"`
	SecretKey   string         `json:"secret_key,omitempty"`

	logger *zap.Logger
	cache  *lru.LRU[string, netip.Prefix]
	client *teo.Client
	ctx    caddy.Context
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

func (s *EdgeOneIPRange) Provision(ctx caddy.Context) error {
	s.ctx = ctx
	s.logger = ctx.Logger()

	cacheSize := s.CacheSize
	if cacheSize == 0 {
		cacheSize = 1000
	}
	cacheTTL := time.Duration(s.CacheTTL)
	if cacheTTL == 0 {
		cacheTTL = time.Hour
	}
	s.cache = lru.NewLRU[string, netip.Prefix](cacheSize, nil, cacheTTL)

	cpf := profile.NewClientProfile()
	credential := common.NewCredential(s.SecretID, s.SecretKey)
	apiEndpoint := s.APIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = "teo.tencentcloudapi.com"
	}
	cpf.HttpProfile.Endpoint = apiEndpoint
	timeout := int(s.Timeout)
	if timeout == 0 {
		timeout = 5
	}
	cpf.HttpProfile.ReqTimeout = timeout
	client, err := teo.NewClient(credential, "", cpf)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *EdgeOneIPRange) validateIP(request *http.Request) (*netip.Prefix, error) {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return nil, errors.Wrap(err, "split host port failed")
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return nil, errors.Wrap(err, "parse addr failed")
	}
	ipStr := ip.String()
	if cached, ok := s.cache.Get(ipStr); ok {
		return &cached, nil
	}
	describeRequest := teo.NewDescribeIPRegionRequest()
	describeRequest.IPs = []*string{&ipStr}
	describeResponse, err := s.client.DescribeIPRegion(describeRequest)
	if err != nil {
		return nil, errors.Wrap(err, "describe IP region failed")
	}
	valid := slices.ContainsFunc(describeResponse.Response.IPRegionInfo, func(info *teo.IPRegionInfo) bool {
		if info.IsEdgeOneIP == nil || *info.IsEdgeOneIP != "yes" {
			return false
		}
		parsed, err := netip.ParseAddr(*info.IP)
		if err != nil {
			return false
		}
		return parsed.Compare(ip) == 0
	})
	if !valid {
		return nil, nil
	}
	prefix := netip.PrefixFrom(ip, ip.BitLen())
	s.cache.Add(ipStr, prefix)
	return &prefix, nil
}

func (s *EdgeOneIPRange) GetIPRanges(request *http.Request) []netip.Prefix {
	validated, err := s.validateIP(request)
	if err != nil {
		s.logger.Error("validate IP failed", zap.Error(err), zap.String("ip", request.RemoteAddr))
		return []netip.Prefix{}
	}
	if validated == nil {
		s.logger.Debug("IP is not valid", zap.String("ip", request.RemoteAddr))
		return []netip.Prefix{}
	}
	return []netip.Prefix{*validated}
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
//
//	edgeone <secret_id> <secret_key> {
//	   api_endpoint val
//	   cache_size val
//	   cache_ttl val
//	   timeout val
//	}
func (s *EdgeOneIPRange) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // Skip module name.
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
		case "cache_size":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return err
			}
			s.CacheSize = val
		case "cache_ttl":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return err
			}
			s.CacheTTL = caddy.Duration(val)
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

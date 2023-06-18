package ipset

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/wafv2"
	"github.com/aws/aws-sdk-go/service/wafv2/wafv2iface"
	"github.com/stretchr/testify/assert"
)

const ipSetName = "test-ip-set"

type MockWAFV2API struct {
	wafv2iface.WAFV2API
	MockUpdateIPSetWithContext func(aws.Context, *wafv2.UpdateIPSetInput, ...request.Option) (*wafv2.UpdateIPSetOutput, error)
}

func (m *MockWAFV2API) UpdateIPSetWithContext(ctx aws.Context, in *wafv2.UpdateIPSetInput, opts ...request.Option) (*wafv2.UpdateIPSetOutput, error) {
	if m.MockUpdateIPSetWithContext != nil {
		return m.MockUpdateIPSetWithContext(ctx, in, opts...)
	}
	return m.WAFV2API.UpdateIPSetWithContext(ctx, in, opts...)
}

func TestAppendToIPSet(t *testing.T) {
	ctx := context.Background()
	cidr := "192.0.2.44/32"
	t.Run("cidr not exists", func(t *testing.T) {
		ipSet := setupIPSet(t)
		assert.NoError(t, AppendToIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.True(t, existsCIDR(t, ipSet, cidr))
	})
	t.Run("cidr already exists", func(t *testing.T) {
		ipSet := setupIPSet(t)
		assert.NoError(t, AppendToIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.True(t, existsCIDR(t, ipSet, cidr))
		assert.NoError(t, AppendToIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.True(t, existsCIDR(t, ipSet, cidr))
	})
	t.Run("handle optimistic lock error", func(t *testing.T) {
		ipSet := setupIPSet(t)
		// mock to UpdateIPSetWithContext
		bk := newWAFv2
		t.Cleanup(func() {
			newWAFv2 = bk
		})
		var lockErrTriggered bool
		newWAFv2 = func() wafv2iface.WAFV2API {
			api := bk()
			mockAPI := MockWAFV2API{WAFV2API: api}
			mockAPI.MockUpdateIPSetWithContext = func(ctx aws.Context, in *wafv2.UpdateIPSetInput, opts ...request.Option) (*wafv2.UpdateIPSetOutput, error) {
				if !lockErrTriggered {
					_, err := api.UpdateIPSetWithContext(ctx, in, opts...)
					assert.NoError(t, err)
				}
				out, err := api.UpdateIPSetWithContext(ctx, in, opts...)
				var lockErr *wafv2.WAFOptimisticLockException
				if errors.As(err, &lockErr) {
					lockErrTriggered = true
				}
				return out, err
			}
			return &mockAPI
		}
		assert.NoError(t, AppendToIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.True(t, existsCIDR(t, ipSet, cidr))
		assert.True(t, lockErrTriggered)
	})
}

func TestRemoveFromIPSet(t *testing.T) {
	ctx := context.Background()
	cidr := "192.0.2.44/32"
	t.Run("cidr not exists", func(t *testing.T) {
		ipSet := setupIPSet(t)
		assert.NoError(t, RemoveFromIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.False(t, existsCIDR(t, ipSet, cidr))
	})
	t.Run("cidr already exists", func(t *testing.T) {
		ipSet := setupIPSet(t)
		assert.NoError(t, AppendToIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.True(t, existsCIDR(t, ipSet, cidr))
		assert.NoError(t, RemoveFromIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.False(t, existsCIDR(t, ipSet, cidr))
	})
	t.Run("handle optimistic lock error", func(t *testing.T) {
		ipSet := setupIPSet(t)
		assert.NoError(t, AppendToIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.True(t, existsCIDR(t, ipSet, cidr))
		// mock to UpdateIPSetWithContext
		bk := newWAFv2
		t.Cleanup(func() {
			newWAFv2 = bk
		})
		var lockErrTriggered bool
		newWAFv2 = func() wafv2iface.WAFV2API {
			api := bk()
			mockAPI := MockWAFV2API{WAFV2API: api}
			mockAPI.MockUpdateIPSetWithContext = func(ctx aws.Context, in *wafv2.UpdateIPSetInput, opts ...request.Option) (*wafv2.UpdateIPSetOutput, error) {
				if !lockErrTriggered {
					_, err := api.UpdateIPSetWithContext(ctx, in, opts...)
					assert.NoError(t, err)
				}
				out, err := api.UpdateIPSetWithContext(ctx, in, opts...)
				var lockErr *wafv2.WAFOptimisticLockException
				if errors.As(err, &lockErr) {
					lockErrTriggered = true
				}
				return out, err
			}
			return &mockAPI
		}
		assert.NoError(t, RemoveFromIPSet(ctx, aws.StringValue(ipSet.Id), ipSetName, cidr))
		assert.False(t, existsCIDR(t, ipSet, cidr))
		assert.True(t, lockErrTriggered)
	})
}

func existsCIDR(t *testing.T, ipSet *wafv2.IPSetSummary, cidr string) bool {
	t.Helper()
	api := newWAFv2()
	out, err := api.GetIPSet(&wafv2.GetIPSetInput{
		Id:    ipSet.Id,
		Name:  ipSet.Name,
		Scope: aws.String("REGIONAL"),
	})
	if err != nil {
		t.Error(err)
		return false
	}
	for _, a := range out.IPSet.Addresses {
		if aws.StringValue(a) == cidr {
			return true
		}
	}
	return false
}

func setupIPSet(t *testing.T) *wafv2.IPSetSummary {
	t.Helper()
	for _, is := range listAllIPSets(t) {
		if aws.StringValue(is.Name) == ipSetName {
			// ip set already exists
			setCleanupIPSet(t)
			return is
		}
	}
	api := newWAFv2()
	out, err := api.CreateIPSet(&wafv2.CreateIPSetInput{
		Addresses:        []*string{},
		IPAddressVersion: aws.String("IPV4"),
		Name:             aws.String(ipSetName),
		Scope:            aws.String("REGIONAL"),
	})
	if err != nil {
		t.Fatal(err)
	}
	setCleanupIPSet(t)
	return out.Summary
}

func setCleanupIPSet(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, is := range listAllIPSets(t) {
			if aws.StringValue(is.Name) == ipSetName {
				api := newWAFv2()
				if _, err := api.DeleteIPSet(&wafv2.DeleteIPSetInput{
					Id:        is.Id,
					LockToken: is.LockToken,
					Name:      is.Name,
					Scope:     aws.String("REGIONAL"),
				}); err != nil {
					t.Error(err)
				}
				return
			}
		}
	})
}

func listAllIPSets(t *testing.T) []*wafv2.IPSetSummary {
	t.Helper()
	api := newWAFv2()
	var nextMarker *string = nil
	ipSets := make([]*wafv2.IPSetSummary, 0)
	for {
		out, err := api.ListIPSets(&wafv2.ListIPSetsInput{
			Limit:      aws.Int64(100),
			NextMarker: nextMarker,
			Scope:      aws.String("REGIONAL"),
		})
		if err != nil {
			t.Fatal(err)
		}
		ipSets = append(ipSets, out.IPSets...)
		nextMarker = out.NextMarker
		if nextMarker == nil {
			return ipSets
		}
	}
}

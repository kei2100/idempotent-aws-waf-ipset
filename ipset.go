package ipset

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/wafv2"
	"github.com/aws/aws-sdk-go/service/wafv2/wafv2iface"
)

var newWAFv2 = func() wafv2iface.WAFV2API {
	return wafv2.New(Session)
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

type updateIPSetFunc func(ctx context.Context, ipSetID, ipSetName, cidr string) error

// AppendToIPSet appends cidr to the WAF IP set
func AppendToIPSet(ctx context.Context, ipSetID, ipSetName, cidr string) error {
	return retryOptimisticLockErr(ctx, appendToIPSet, ipSetID, ipSetName, cidr)
}

// RemoveFromIPSet removes cidr from the WAF IP set
func RemoveFromIPSet(ctx context.Context, ipSetID, ipSetName, cidr string) error {
	return retryOptimisticLockErr(ctx, removeFromIPSet, ipSetID, ipSetName, cidr)
}

func retryOptimisticLockErr(ctx context.Context, fn updateIPSetFunc, ipSetID, ipSetName, cidr string) error {
	var err error
	var attempts int
	for {
		if attempts > 3 {
			return err
		}
		err = fn(ctx, ipSetID, ipSetName, cidr)
		if err != nil {
			var lockErr *wafv2.WAFOptimisticLockException
			if !errors.As(err, &lockErr) {
				return err
			}
			attempts++
			time.Sleep(time.Duration(100+random.Int63n(101)) * time.Millisecond)
			continue
		}
		return nil
	}
}

var appendToIPSet updateIPSetFunc = func(ctx context.Context, ipSetID, ipSetName, cidr string) error {
	api := newWAFv2()
	// append cidr to ip set if not exists
	current, err := api.GetIPSetWithContext(ctx, &wafv2.GetIPSetInput{
		Id:    aws.String(ipSetID),
		Name:  aws.String(ipSetName),
		Scope: aws.String("REGIONAL"),
	})
	if err != nil {
		return fmt.Errorf("ipset: get ip set: %w", err)
	}
	var alreadyExists bool
	for _, a := range current.IPSet.Addresses {
		if aws.StringValue(a) == cidr {
			alreadyExists = true
		}
	}
	if !alreadyExists {
		current.IPSet.Addresses = append(current.IPSet.Addresses, aws.String(cidr))
	}
	// update ip set
	_, err = api.UpdateIPSetWithContext(ctx, &wafv2.UpdateIPSetInput{
		Id:        aws.String(ipSetID),
		Name:      aws.String(ipSetName),
		Scope:     aws.String("REGIONAL"),
		LockToken: current.LockToken,
		Addresses: current.IPSet.Addresses,
	})
	if err != nil {
		return fmt.Errorf("ipset: update ip set: %w", err)
	}
	return nil
}

var removeFromIPSet updateIPSetFunc = func(ctx context.Context, ipSetID, ipSetName, cidr string) error {
	api := newWAFv2()
	// remove cidr from IP set if exists
	current, err := api.GetIPSetWithContext(ctx, &wafv2.GetIPSetInput{
		Id:    aws.String(ipSetID),
		Name:  aws.String(ipSetName),
		Scope: aws.String("REGIONAL"),
	})
	if err != nil {
		return fmt.Errorf("ipset: get ip set: %w", err)
	}
	for i, a := range current.IPSet.Addresses {
		if aws.StringValue(a) == cidr {
			n := copy(current.IPSet.Addresses[i:], current.IPSet.Addresses[i+1:])
			current.IPSet.Addresses = current.IPSet.Addresses[:i+n]
			break
		}
	}
	// update ip set
	_, err = api.UpdateIPSetWithContext(ctx, &wafv2.UpdateIPSetInput{
		Id:        aws.String(ipSetID),
		Name:      aws.String(ipSetName),
		Scope:     aws.String("REGIONAL"),
		LockToken: current.LockToken,
		Addresses: current.IPSet.Addresses,
	})
	if err != nil {
		return fmt.Errorf("ipset: update ip set: %w", err)
	}
	return nil
}

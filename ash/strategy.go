// Package ash implements a highly configurable and callback based ACL that can
// be used to authorize controller operations in a declarative way.
package ash

import "github.com/256dpi/fire"

// L is a short-hand type to create a list of authorizers.
type L []*Authorizer

// M is a short-hand type to create a map of authorizers.
type M map[string][]*Authorizer

// C is a short-hand to define a strategy and return its callback.
func C(s *Strategy) *fire.Callback {
	return s.Callback()
}

// Strategy contains lists of authorizers that are used to authorize operations.
type Strategy struct {
	// Single operations.
	List   []*Authorizer
	Find   []*Authorizer
	Create []*Authorizer
	Update []*Authorizer
	Delete []*Authorizer

	// Single action operations.
	CollectionAction map[string][]*Authorizer
	ResourceAction   map[string][]*Authorizer

	// All action operations.
	CollectionActions []*Authorizer
	ResourceActions   []*Authorizer

	// All List and Find operations.
	Read []*Authorizer

	// All Create, Update and Delete operations.
	Write []*Authorizer

	// All CollectionAction and ResourceAction operations.
	Actions []*Authorizer

	// All operations.
	All []*Authorizer
}

// Callback will return a callback that authorizes operations using the strategy.
func (s *Strategy) Callback() *fire.Callback {
	return fire.C("ash/Strategy.Callback", nil, func(ctx *fire.Context) error {
		switch ctx.Operation {
		case fire.List:
			return s.call(ctx, s.List, s.Read, s.All)
		case fire.Find:
			return s.call(ctx, s.Find, s.Read, s.All)
		case fire.Create:
			return s.call(ctx, s.Create, s.Write, s.All)
		case fire.Update:
			return s.call(ctx, s.Update, s.Write, s.All)
		case fire.Delete:
			return s.call(ctx, s.Delete, s.Write, s.All)
		case fire.CollectionAction:
			return s.call(ctx, s.CollectionAction[ctx.JSONAPIRequest.CollectionAction], s.CollectionActions, s.Actions, s.All)
		case fire.ResourceAction:
			return s.call(ctx, s.ResourceAction[ctx.JSONAPIRequest.ResourceAction], s.ResourceActions, s.Actions, s.All)
		}

		// panic on unknown operation
		panic("ash: unknown operation")
	})
}

func (s *Strategy) call(ctx *fire.Context, lists ...[]*Authorizer) error {
	// loop through all lists
	for _, list := range lists {
		// loop through all callbacks
		for _, authorizer := range list {
			// check if authenticator can be run
			if !authorizer.Matcher(ctx) {
				continue
			}

			// run callback and return on error
			enforcers, err := authorizer.Handler(ctx)
			if err != nil {
				return err
			}

			// run enforcers on success
			if enforcers != nil {
				for _, enforcer := range enforcers {
					// check if enforcer can be run
					if !enforcer.Matcher(ctx) {
						// an authorizer should not return an enforcer that cannot
						// enforce the authentication
						panic("ash: not supported")
					}

					// run callback and return error
					err = enforcer.Handler(ctx)
					if err != nil {
						return err
					}
				}

				// return nil if all enforcers ran successfully
				return nil
			}
		}
	}

	return fire.ErrAccessDenied
}
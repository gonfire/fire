package ash

import "github.com/256dpi/fire"

// A is a short-hand function to construct an authorizer. It will also add tracing
// code around the execution of the authorizer.
func A(name string, m fire.Matcher, h Handler) *Authorizer {
	// default to all
	if m == nil {
		m = fire.All()
	}

	return &Authorizer{
		Matcher: m,
		Handler: func(ctx *fire.Context) (*Enforcer, error) {
			// begin trace
			ctx.Tracer.Push(name)

			// call handler
			enforcer, err := h(ctx)
			if err != nil {
				return nil, err
			}

			// finish trace
			ctx.Tracer.Pop()

			return enforcer, nil
		},
	}
}

// Handler is a function that inspects an operation context and eventually
// returns an enforcer or an error.
type Handler func(*fire.Context) (*Enforcer, error)

// An Authorizer should inspect the specified context and assesses if it is able
// to enforce authorization with the data that is available. If yes, the
// authorizer should return an Enforcer that will enforce the authorization.
type Authorizer struct {
	Matcher fire.Matcher
	Handler Handler
}

// And will run both authorizers and return immediately if one does not return an
// enforcer. The two successfully returned enforcers are wrapped in one that will
// execute both.
//
// Note: The authorizer is only run if both authorizers match the context.
func And(a, b *Authorizer) *Authorizer {
	return A("ash/And", func(ctx *fire.Context)bool{
		return a.Matcher(ctx) && b.Matcher(ctx)
	}, func(ctx *fire.Context) (*Enforcer, error) {
		// check if callback a can be run
		if !a.Matcher(ctx) {
			return nil, nil
		}

		// run first callback
		enforcer1, err := a.Handler(ctx)
		if err != nil {
			return nil, err
		} else if enforcer1 == nil {
			return nil, nil
		}

		// check if callback b can be run
		if !b.Matcher(ctx) {
			return nil, nil
		}

		// run second callback
		enforcer2, err := b.Handler(ctx)
		if err != nil {
			return nil, err
		} else if enforcer2 == nil {
			return nil, nil
		}

		// return an enforcer that calls both enforcers
		return E("ash/And", func(ctx *fire.Context) bool {
			return enforcer1.Matcher(ctx) && enforcer2.Matcher(ctx)
		}, func(ctx *fire.Context) error{
			// call first enforcer
			err := enforcer1.Handler(ctx)
			if err != nil {
				return err
			}

			// call second enforcer
			err  = enforcer2.Handler(ctx)
			if err != nil {
				return err
			}

			return nil
		}), nil
	})
}

// And will run And() with the current and specified authorizer.
func (a *Authorizer) And(b *Authorizer) *Authorizer {
	return And(a, b)
}

// Or will run the first authorizer and return its enforcer on success. If no
// enforcer is returned it will run the second authorizer and return its result.
func Or(a, b *Authorizer) *Authorizer {
	return A("ash/Or", nil, func(ctx *fire.Context) (*Enforcer, error) {
		// run first callback
		enforcer1, err := a.Handler(ctx)
		if err != nil {
			return nil, err
		}

		// return on success
		if enforcer1 != nil {
			return enforcer1, nil
		}

		// run second callback
		enforcer2, err := b.Handler(ctx)
		if err != nil {
			return nil, err
		}

		return enforcer2, nil
	})
}

// Or will run Or() with the current and specified authorizer.
func (a *Authorizer) Or(b *Authorizer) *Authorizer {
	return Or(a, b)
}

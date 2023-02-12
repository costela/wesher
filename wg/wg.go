package wg

import "context"

type Adapter interface {
	Run(context.Context) error
}

type runner struct {
}

func New() Adapter {
	return &runner{}
}

func (r *runner) Run(context.Context) error {

	<-make(chan struct{})
	return nil
}

package guard

type Guard interface {
	Guard() error
}

// Mux runs more than one guard in parallel, if an error is received from
// any guards then it is returned immediately.
type Mux []Guard

// Guard runs all the guards in parallel, the error from the first guard to
// return will be returend and should be considered fatal. No other guards will
// be stopped.
func (m Mux) Guard() error {
	errChan := make(chan error)
	for _, g := range m {
		go func(g Guard) {
			errChan <- g.Guard()
		}(g)
	}
	return <-errChan
}

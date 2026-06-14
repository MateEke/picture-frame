package startup

// MakeRestartFunc returns the restart callback for the HTTP layer and updater: it enqueues an
// in-place re-exec (same PID, so systemd sees no crash), dropping the request if one is already
// queued. One kind suffices, the kiosk reloads itself on the next SSE version change.
func MakeRestartFunc(ch chan<- struct{}) func() error {
	return func() error {
		select {
		case ch <- struct{}{}:
		default:
		}
		return nil
	}
}

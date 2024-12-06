package contractor

import "go.thebigfile.com/bigd/modules"

// Alerts implements the modules.Alerter interface for the contractor. It returns
// all alerts of the contractor.
func (c *Contractor) Alerts() (crit, err, warn, info []modules.Alert) {
	return c.staticAlerter.Alerts()
}

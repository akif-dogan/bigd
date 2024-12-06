package miner

import "go.thebigfile.com/bigd/modules"

// Alerts implements the modules.Alerter interface for the miner.
func (m *Miner) Alerts() (crit, err, warn, info []modules.Alert) {
	return
}

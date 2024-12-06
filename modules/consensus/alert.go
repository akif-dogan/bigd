package consensus

import (
	"go.thebigfile.com/bigd/modules"
)

// Alerts implements the Alerter interface for the consensusset.
func (c *ConsensusSet) Alerts() (crit, err, warn, info []modules.Alert) {
	return
}

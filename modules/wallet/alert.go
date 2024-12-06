package wallet

import (
	"go.thebigfile.com/bigd/modules"
)

// Alerts implements the Alerter interface for the wallet.
func (w *Wallet) Alerts() (crit, err, warn, info []modules.Alert) {
	return
}

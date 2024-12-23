package siatest

import "go.thebigfile.com/bigd/types"

// BlockHeight returns the node's consensus modules's synced block height.
func (tn *TestNode) BlockHeight() (types.BlockHeight, error) {
	cg, err := tn.ConsensusGet()
	if err != nil {
		return 0, err
	}
	return cg.Height, nil
}

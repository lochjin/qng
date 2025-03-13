package blockchain

import "fmt"

// BehaviorFlags is a bitmask defining tweaks to the normal behavior when
// performing chain processing and consensus rules checks.
type BehaviorFlags uint32

const (
	// BFNone is a convenience value to specifically indicate no flags.
	BFNone BehaviorFlags = 0

	// BFFastAdd may be set to indicate that several checks can be avoided
	// for the block since it is already known to fit into the chain due to
	// already proving it correct links into the chain up to a known
	// checkpoint.  This is primarily used for headers-first mode.
	BFFastAdd BehaviorFlags = 1 << iota

	// BFNoProofCheck may be set to indicate the proof of work check which
	// ensures a block hashes to a value less than the required target will
	// not be performed.
	BFNoProofCheck

	BFP2PAdd

	// Add block from RPC interface
	BFRPCAdd

	// Add block from broadcast interface
	BFBroadcast
)

var bfStrings = map[BehaviorFlags]string{
	BFNone:         "BFNone",
	BFFastAdd:      "BFFastAdd",
	BFNoProofCheck: "BFNoProofCheck",
	BFP2PAdd:       "BFP2PAdd",
	BFRPCAdd:       "BFRPCAdd",
	BFBroadcast:    "BFBroadcast",
}

func (bf BehaviorFlags) String() string {
	if s, ok := bfStrings[bf]; ok {
		return s
	}
	return fmt.Sprintf("Unknown BehaviorFlags (%d)", uint32(bf))
}

func (bf BehaviorFlags) Has(value BehaviorFlags) bool {
	return (bf & value) == value
}

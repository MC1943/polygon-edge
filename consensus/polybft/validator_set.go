package polybft

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
)

const (

	// MaxTotalVotingPower - the maximum allowed total voting power.
	// It needs to be sufficiently small to, in all cases:
	// 1. prevent clipping in incrementProposerPriority()
	// 2. let (diff+diffMax-1) not overflow in IncrementProposerPriority()
	// (Proof of 1 is tricky, left to the reader).
	// It could be higher, but this is sufficiently large for our purposes,
	// and leaves room for defensive purposes.
	MaxTotalVotingPower = int64(math.MaxInt64) / 8

	// PriorityWindowSizeFactor - is a constant that when multiplied with the
	// total voting power gives the maximum allowed distance between validator
	// priorities.
	PriorityWindowSizeFactor = 2
)

type ValidatorAccount struct {
	Metadata         *ValidatorMetadata
	ProposerPriority int64
}

func (a ValidatorAccount) Copy() *ValidatorAccount {
	return NewValidator(a.Metadata.Copy())
}

func (v *ValidatorAccount) CompareProposerPriority(other *ValidatorAccount) *ValidatorAccount {
	if v == nil {
		return other
	}
	switch {
	case v.ProposerPriority > other.ProposerPriority:
		return v
	case v.ProposerPriority < other.ProposerPriority:
		return other
	default:
		result := bytes.Compare(v.Metadata.Address.Bytes(), other.Metadata.Address.Bytes())
		switch {
		case result < 0:
			return v
		case result > 0:
			return other
		default:
			panic("Cannot compare identical validators")
		}
	}
}

// ValidatorSet is a wrapper interface around pbft.ValidatorSet and it holds current validator set
type ValidatorSet interface {
	CalcProposer(round uint64) string
	Includes(id string) bool
	Len() int

	// IncrementProposerPriority increments ProposerPriority of each validator and updates the proposer
	IncrementProposerPriority(times uint64) error

	Accounts() AccountSet
}

func (v *validatorSet) IncrementProposerPriority(times uint64) error {
	if v.IsNilOrEmpty() {
		return fmt.Errorf("validator set cannot be empty")
	}
	if times <= 0 {
		return fmt.Errorf("cannot call IncrementProposerPriority with non-positive times")
	}

	// Cap the difference between priorities to be proportional to 2*totalPower by
	// re-normalizing priorities, i.e., rescale all priorities by multiplying with:
	//  2*totalVotingPower/(maxPriority - minPriority)
	diffMax := PriorityWindowSizeFactor * v.TotalVotingPower()
	v.rescalePriorities(diffMax)
	v.shiftByAvgProposerPriority()

	var proposer *ValidatorAccount
	// Call IncrementProposerPriority(1) times times.
	for i := uint64(0); i < times; i++ {
		proposer = v.incrementProposerPriority()
	}

	v.proposer = proposer

	return nil
}

func (v *validatorSet) incrementProposerPriority() *ValidatorAccount {
	for _, val := range v.validators {
		// Check for overflow for sum.
		newPrio := safeAddClip(val.ProposerPriority, val.Metadata.VotingPower)
		val.ProposerPriority = newPrio
	}
	// Decrement the validator with most ProposerPriority.
	mostest := v.getValWithMostPriority()
	// Mind the underflow.
	mostest.ProposerPriority = safeSubClip(mostest.ProposerPriority, v.TotalVotingPower())

	return mostest
}

func (v *validatorSet) getValWithMostPriority() *ValidatorAccount {
	var res *ValidatorAccount
	for _, val := range v.validators {
		res = res.CompareProposerPriority(val)
	}
	return res
}

func (v *validatorSet) shiftByAvgProposerPriority() {
	if v.IsNilOrEmpty() {
		panic("empty validator set")
	}
	avgProposerPriority := v.computeAvgProposerPriority()
	for _, val := range v.validators {
		val.ProposerPriority = safeSubClip(val.ProposerPriority, avgProposerPriority)
	}
}

// Should not be called on an empty validator set.
func (v *validatorSet) computeAvgProposerPriority() int64 {
	n := int64(len(v.validators))
	sum := big.NewInt(0)
	for _, val := range v.validators {
		sum.Add(sum, big.NewInt(val.ProposerPriority))
	}
	avg := sum.Div(sum, big.NewInt(n))
	if avg.IsInt64() {
		return avg.Int64()
	}

	// This should never happen: each val.ProposerPriority is in bounds of int64.
	panic(fmt.Sprintf("Cannot represent avg ProposerPriority as an int64 %v", avg))
}

// rescalePriorities rescales the priorities such that the distance between the
// maximum and minimum is smaller than `diffMax`. Panics if validator set is
// empty.
func (v *validatorSet) rescalePriorities(diffMax int64) {
	if v.IsNilOrEmpty() {
		panic("empty validator set")
	}
	// NOTE: This check is merely a sanity check which could be
	// removed if all tests would init. voting power appropriately;
	// i.e. diffMax should always be > 0
	if diffMax <= 0 {
		return
	}

	// Calculating ceil(diff/diffMax):
	// Re-normalization is performed by dividing by an integer for simplicity.
	// NOTE: This may make debugging priority issues easier as well.
	diff := computeMaxMinPriorityDiff(v)
	ratio := (diff + diffMax - 1) / diffMax
	if diff > diffMax {
		for _, val := range v.validators {
			val.ProposerPriority /= ratio
		}
	}
}

// Compute the difference between the max and min ProposerPriority of that set.
func computeMaxMinPriorityDiff(vals *validatorSet) int64 {
	if vals.IsNilOrEmpty() {
		panic("empty validator set")
	}
	max := int64(math.MinInt64)
	min := int64(math.MaxInt64)
	for _, v := range vals.validators {
		if v.ProposerPriority < min {
			min = v.ProposerPriority
		}
		if v.ProposerPriority > max {
			max = v.ProposerPriority
		}
	}
	diff := max - min
	if diff < 0 {
		return -1 * diff
	}
	return diff
}

// IsNilOrEmpty returns true if validator set is nil or empty.
func (v *validatorSet) IsNilOrEmpty() bool {
	return v == nil || len(v.validators) == 0
}

// TotalVotingPower returns the sum of the voting powers of all validators.
// It recomputes the total voting power if required.
func (v *validatorSet) TotalVotingPower() int64 {
	if v.totalVotingPower == 0 {
		v.updateTotalVotingPower()
	}
	return v.totalVotingPower
}

// Forces recalculation of the set's total voting power.
// Panics if total voting power is bigger than MaxTotalVotingPower.
func (v *validatorSet) updateTotalVotingPower() {
	sum := int64(0)
	for _, val := range v.validators {
		// mind overflow
		sum = safeAddClip(sum, val.Metadata.VotingPower)
		if sum > MaxTotalVotingPower {
			panic(fmt.Sprintf(
				"Total voting power should be guarded to not exceed %v; got: %v",
				MaxTotalVotingPower,
				sum))
		}
	}

	v.totalVotingPower = sum
}

func safeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return -1, true
	} else if b < 0 && a < math.MinInt64-b {
		return -1, true
	}
	return a + b, false
}

func safeSub(a, b int64) (int64, bool) {
	if b > 0 && a < math.MinInt64+b {
		return -1, true
	} else if b < 0 && a > math.MaxInt64+b {
		return -1, true
	}
	return a - b, false
}

func safeSubClip(a, b int64) int64 {
	c, overflow := safeSub(a, b)
	if overflow {
		if b > 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

func safeAddClip(a, b int64) int64 {
	c, overflow := safeAdd(a, b)
	if overflow {
		if b < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return c
}

//  ======================================================================================================================
type validatorSet struct {
	// current list of validators (slice of (Address, BlsPublicKey) pairs)
	validators []*ValidatorAccount

	// proposer of a block
	proposer *ValidatorAccount

	// totalVotingPower denotes voting power of entire validator set
	totalVotingPower int64
}

// NewValidator returns a new validator with the given pubkey and voting power.
func NewValidator(metadata *ValidatorMetadata) *ValidatorAccount {
	return &ValidatorAccount{
		Metadata:         metadata,
		ProposerPriority: 0,
	}
}

func NewValidatorSet(valz AccountSet) *validatorSet {
	var validators []*ValidatorAccount
	for _, v := range valz {
		validators = append(validators, NewValidator(v))
	}

	validatorSet := &validatorSet{
		validators: validators,
		// votingPowerMap: make(map[pbft.NodeID]uint64, len(validators)),
	}

	validatorSet.updateWithChangeSet()
	// _, quorum, err := pbft.CalculateQuorum(validatorSet.VotingPower())
	// if err != nil {
	// 	panic(fmt.Sprintf("cannot calculate quorum for validator set: %v", err))
	// }

	// validatorSet.quorumSize = quorum
	if len(valz) > 0 {
		validatorSet.IncrementProposerPriority(1)
	}
	return validatorSet
}

// updateWithChangeSet function used by UpdateWithChangeSet() and NewValidatorSet().
func (v *validatorSet) updateWithChangeSet() {
	v.updateTotalVotingPower() // will panic if total voting power > MaxTotalVotingPower

	// Scale and center.
	v.rescalePriorities(PriorityWindowSizeFactor * v.TotalVotingPower())
	v.shiftByAvgProposerPriority()
}

func (v validatorSet) Accounts() AccountSet {
	var accountSet []*ValidatorMetadata
	for _, validator := range v.validators {
		accountSet = append(accountSet, validator.Metadata)
	}
	return accountSet
}

func (v *validatorSet) Copy() *validatorSet {
	return &validatorSet{
		validators:       validatorListCopy(v.validators),
		proposer:         v.proposer,
		totalVotingPower: v.totalVotingPower,
	}
}

// Makes a copy of the validator list.
func validatorListCopy(valsList []*ValidatorAccount) []*ValidatorAccount {
	valsCopy := make([]*ValidatorAccount, len(valsList))
	for i, val := range valsList {
		valsCopy[i] = val.Copy()
	}
	return valsCopy
}

func (v validatorSet) CalcProposer(round uint64) string {
	vc := v.Copy()
	_ = vc.IncrementProposerPriority(round + 1) // if round = 0 then we need one iteration
	return vc.GetProposer().Metadata.GetNodeID()
}

func (v *validatorSet) GetProposer() (proposer *ValidatorAccount) {
	if len(v.validators) == 0 {
		return nil
	}
	if v.proposer == nil {
		v.proposer = v.findProposer()
	}

	return v.proposer.Copy()
}

func (v *validatorSet) findProposer() *ValidatorAccount {
	var proposer *ValidatorAccount
	for _, val := range v.validators {
		if proposer == nil || !bytes.Equal(val.Metadata.Address.Bytes(), proposer.Metadata.Address.Bytes()) {
			proposer = proposer.CompareProposerPriority(val)
		}
	}
	return proposer
}

func (v validatorSet) Includes(id string) bool {
	for _, validator := range v.validators {
		if validator.Metadata.Address.String() == id {
			return true
		}
	}
	return false
}

func (v validatorSet) Len() int {
	return len(v.validators)
}

package polybft

import (
	"bytes"
	"math"
	"testing"
	"testing/quick"

	"github.com/0xPolygon/pbft-consensus"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
)

func TestValSetIndex(t *testing.T) {
	addresses := []types.Address{{0x10}, {0x52}, {0x33}, {0x74}, {0x60}}

	vs := NewValidatorSet([]*ValidatorMetadata{
		{Address: addresses[0], VotingPower: 10},
		{Address: addresses[1], VotingPower: 100},
		{Address: addresses[2], VotingPower: 1},
		{Address: addresses[3], VotingPower: 50},
		{Address: addresses[4], VotingPower: 30},
	})

	// validate no changes to validator set positions
	for i, v := range vs.Accounts() {
		assert.Equal(t, addresses[i], v.Address)
	}

	address := vs.GetProposer().Metadata.Address
	assert.Equal(t, address, addresses[1])

	// validate no changes to validator set positions
	for i, v := range vs.Accounts() {
		assert.Equal(t, addresses[i], v.Address)
	}
}
func TestCalculateProposer(t *testing.T) {
	vs := NewValidatorSet([]*ValidatorMetadata{
		{
			Address:     types.Address{0x1},
			VotingPower: 1,
		},
		{
			Address:     types.Address{0x2},
			VotingPower: 2,
		},
		{
			Address:     types.Address{0x3},
			VotingPower: 3},
		{
			Address:     types.Address{0x4},
			VotingPower: 4,
		},
		{
			Address:     types.Address{0x5},
			VotingPower: 5,
		},
	})

	assert.Equal(t, int64(15), vs.totalVotingPower)

	curr := vs.GetProposer()
	assert.Equal(t, types.Address{0x5}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)
	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x4}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)
	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x3}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)
	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x2}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)
	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x5}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)
	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x4}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)
	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x1}, curr.Metadata.Address)
}

func TestCalcProposer(t *testing.T) {
	vs := NewValidatorSet([]*ValidatorMetadata{
		{Address: types.Address{0x1}, VotingPower: 1},
		{Address: types.Address{0x2}, VotingPower: 2},
		{Address: types.Address{0x3}, VotingPower: 3},
	})

	proposer := vs.CalcProposer(0)
	assert.Equal(t, pbft.NodeID("0x0300000000000000000000000000000000000000"), proposer)
	proposer = vs.CalcProposer(1)
	assert.Equal(t, pbft.NodeID("0x0200000000000000000000000000000000000000"), proposer)
	proposer = vs.CalcProposer(2)
	assert.Equal(t, pbft.NodeID("0x0100000000000000000000000000000000000000"), proposer)

}

func TestProposerSelection1(t *testing.T) {
	vset := NewValidatorSet([]*ValidatorMetadata{
		{
			Address:     types.Address{0x1},
			VotingPower: 1000,
		},
		{
			Address:     types.Address{0x2},
			VotingPower: 300,
		},
		{
			Address:     types.Address{0x3},
			VotingPower: 330,
		},
	})

	var proposers []types.Address
	for i := 0; i < 99; i++ {
		val := vset.GetProposer()
		proposers = append(proposers, val.Metadata.Address)
		vset.IncrementProposerPriority(1)
	}

	expected := []types.Address{
		{0x1}, {0x3}, {0x1}, {0x2}, {0x1}, {0x1}, {0x3}, {0x1}, {0x2}, {0x1}, {0x1}, {0x3}, {0x1}, {0x1}, {0x2}, {0x1},
		{0x3}, {0x1}, {0x1}, {0x2}, {0x1}, {0x1}, {0x3}, {0x1}, {0x2}, {0x1}, {0x1}, {0x3}, {0x1}, {0x2}, {0x1}, {0x1},
		{0x3}, {0x1}, {0x1}, {0x2}, {0x1}, {0x3}, {0x1}, {0x1}, {0x2}, {0x1}, {0x3}, {0x1}, {0x1}, {0x2}, {0x1}, {0x3},
		{0x1}, {0x1}, {0x2}, {0x1}, {0x3}, {0x1}, {0x1}, {0x1}, {0x3}, {0x2}, {0x1}, {0x1}, {0x1}, {0x3}, {0x1}, {0x2},
		{0x1}, {0x1}, {0x3}, {0x1}, {0x2}, {0x1}, {0x1}, {0x3}, {0x1}, {0x2}, {0x1}, {0x1}, {0x3}, {0x1}, {0x2}, {0x1},
		{0x1}, {0x3}, {0x1}, {0x1}, {0x2}, {0x1}, {0x3}, {0x1}, {0x1}, {0x2}, {0x1}, {0x3}, {0x1}, {0x1}, {0x2}, {0x1},
		{0x3}, {0x1}, {0x1},
	}

	for i, p := range proposers {
		assert.True(t, bytes.Equal(expected[i].Bytes(), p.Bytes()))
	}
}

// Test that IncrementProposerPriority requires positive times.
func TestIncrementProposerPriorityPositiveTimes(t *testing.T) {
	vset := NewValidatorSet([]*ValidatorMetadata{
		{
			Address:     types.Address{0x1},
			VotingPower: 1000,
		},
		{
			Address:     types.Address{0x2},
			VotingPower: 300,
		},
		{
			Address:     types.Address{0x3},
			VotingPower: 330,
		},
	})

	assert.Panics(t, func() { vset.IncrementProposerPriority(0) })
	vset.IncrementProposerPriority(1)
}

func TestIncrementProposerPrioritySameVotingPower(t *testing.T) {
	vs := NewValidatorSet([]*ValidatorMetadata{
		{
			Address:     types.Address{0x1},
			VotingPower: 1,
		},
		{
			Address:     types.Address{0x2},
			VotingPower: 1,
		},
		{
			Address:     types.Address{0x3},
			VotingPower: 1,
		},
	})

	assert.Equal(t, int64(3), vs.totalVotingPower)

	// when voting power is the same order is by address
	curr := vs.GetProposer()
	assert.Equal(t, types.Address{0x1}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)

	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x2}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)

	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x3}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)

	curr = vs.GetProposer()
	assert.Equal(t, types.Address{0x1}, curr.Metadata.Address)
	vs.IncrementProposerPriority(1)

}

func TestAveragingInIncrementProposerPriorityWithVotingPower(t *testing.T) {
	// Tests how each ProposerPriority changes in relation to the validator's voting power respectively.
	// average is zero in each round:
	vp0 := int64(10)
	vp1 := int64(1)
	vp2 := int64(1)
	total := vp0 + vp1 + vp2
	avg := (vp0 + vp1 + vp2 - total) / 3 // avg is used to center priorities around zero

	// in every iteration expected proposer is the one with the highest priority based on the voting power
	// priority is calculated: priority = iterationNO * voting power, once node is selected total voting power
	// is subtracted from priority of selected node
	valz := []*ValidatorMetadata{
		{
			Address:     types.Address{0x1},
			VotingPower: vp0,
		},
		{
			Address:     types.Address{0x2},
			VotingPower: vp1,
		},
		{
			Address:     types.Address{0x3},
			VotingPower: vp2,
		},
	}

	vals := NewValidatorSet(valz)

	tcs := []struct {
		vals                  *validatorSet
		wantProposerPrioritys []int64
		times                 uint64
		wantProposer          *ValidatorMetadata
	}{

		0: {
			vals.Copy(),
			[]int64{
				// Acumm+VotingPower-Avg:
				0 + vp0 - total - avg, // mostest will be subtracted by total voting power (12)
				0 + vp1,
				0 + vp2},
			1,
			vals.validators[0].Metadata},
		1: {
			vals.Copy(),
			[]int64{
				0 + 2*(vp0-total) - avg, // this will be mostest on 2nd iter, too
				(0 + vp1) + vp1,
				(0 + vp2) + vp2},
			2,
			vals.validators[0].Metadata}, // increment twice -> expect average to be subtracted twice
		2: {
			vals.Copy(),
			[]int64{
				0 + 3*(vp0-total) - avg, // 3rd iteration, still mostest
				0 + 3*vp1,
				0 + 3*vp2},
			3,
			vals.validators[0].Metadata},
		3: {
			vals.Copy(),
			[]int64{
				0 + 4*(vp0-total), // 4th iteration still mostest
				0 + 4*vp1,
				0 + 4*vp2},
			4,
			vals.validators[0].Metadata},
		4: {
			vals.Copy(),
			[]int64{
				0 + 4*(vp0-total) + vp0, // 4 iteration was mostest
				0 + 5*vp1 - total,       // 5th iteration this val is mostest for the 1st time (hence -12==totalVotingPower)
				0 + 5*vp2},
			5,
			vals.validators[1].Metadata},
		5: {
			vals.Copy(),
			[]int64{
				0 + 6*vp0 - 5*total, // 6th iteration mostest again
				0 + 6*vp1 - total,   // mostest once up to here
				0 + 6*vp2},
			6,
			vals.validators[0].Metadata},
		6: {
			vals.Copy(),
			[]int64{
				0 + 7*vp0 - 6*total, // in 7 iteration this val is mostest 6 times
				0 + 7*vp1 - total,   // in 7 iteration this val is mostest 1 time
				0 + 7*vp2},
			7,
			vals.validators[0].Metadata},
		7: {
			vals.Copy(),
			[]int64{
				0 + 8*vp0 - 7*total, // 8th iteration mostest again is picked
				0 + 8*vp1 - total,
				0 + 8*vp2},
			8,
			vals.validators[0].Metadata},
		8: {
			vals.Copy(),
			[]int64{
				0 + 9*vp0 - 7*total,
				0 + 9*vp1 - total,
				0 + 9*vp2 - total}, // 9th iteration and now first time mostest is picked
			9,
			vals.validators[2].Metadata},
		9: {
			vals.Copy(),
			[]int64{
				0 + 10*vp0 - 8*total, // after 10 iterations this is mostest again
				0 + 10*vp1 - total,   // after 6 iterations this val is "mostest" once and not in between
				0 + 10*vp2 - total},  // in between 10 iterations this val is "mostest" once
			10,
			vals.validators[0].Metadata},
		10: {
			vals.Copy(),
			[]int64{
				0 + 11*vp0 - 9*total, // 11th iteration again is picked
				0 + 11*vp1 - total,   // after 6 iterations this val is "mostest" once and not in between
				0 + 11*vp2 - total},  // after 10 iterations this val is "mostest" once
			11,
			vals.validators[0].Metadata},
	}
	for i, tc := range tcs {
		tc.vals.IncrementProposerPriority(tc.times)

		assert.Equal(t, tc.wantProposer.Address, tc.vals.GetProposer().Metadata.Address,
			"test case: %v",
			i)

		for valIdx, val := range tc.vals.validators {
			assert.Equal(t,
				tc.wantProposerPrioritys[valIdx],
				val.ProposerPriority,
				"test case: %v, validator: %v",
				i,
				valIdx)
		}
	}
}

func TestValidatorSetTotalVotingPowerPanicsOnOverflow(t *testing.T) {
	// NewValidatorSet calls IncrementProposerPriority which calls TotalVotingPower()
	// which should panic on overflows:
	shouldPanic := func() {
		vs := NewValidatorSet([]*ValidatorMetadata{
			{Address: types.Address{0x1}, VotingPower: math.MaxInt64},
			{Address: types.Address{0x2}, VotingPower: math.MaxInt64},
			{Address: types.Address{0x3}, VotingPower: math.MaxInt64},
		})
		vs.IncrementProposerPriority(1)
	}

	assert.Panics(t, shouldPanic)
}

func TestSafeAdd(t *testing.T) {
	f := func(a, b int64) bool {
		c, overflow := safeAdd(a, b)
		return overflow || (!overflow && c == a+b)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSafeAddClip(t *testing.T) {
	assert.EqualValues(t, math.MaxInt64, safeAddClip(math.MaxInt64, 10))
	assert.EqualValues(t, math.MaxInt64, safeAddClip(math.MaxInt64, math.MaxInt64))
	assert.EqualValues(t, math.MinInt64, safeAddClip(math.MinInt64, -10))
}

func TestSafeSubClip(t *testing.T) {
	assert.EqualValues(t, math.MinInt64, safeSubClip(math.MinInt64, 10))
	assert.EqualValues(t, 0, safeSubClip(math.MinInt64, math.MinInt64))
	assert.EqualValues(t, math.MinInt64, safeSubClip(math.MinInt64, math.MaxInt64))
	assert.EqualValues(t, math.MaxInt64, safeSubClip(math.MaxInt64, -10))
}

func TestUpdatesForNewValidatorSet(t *testing.T) {
	v1 := &ValidatorMetadata{Address: types.Address{0x1}, VotingPower: 100}
	v2 := &ValidatorMetadata{Address: types.Address{0x2}, VotingPower: 100}
	accountSet := []*ValidatorMetadata{v1, v2}
	valSet := NewValidatorSet(accountSet)
	valSet.IncrementProposerPriority(1)
	verifyValidatorSet(t, valSet)
}

func verifyValidatorSet(t *testing.T, valSet *validatorSet) {
	// verify that the capacity and length of validators is the same
	assert.Equal(t, len(valSet.Accounts()), cap(valSet.validators))
	// verify that the set's total voting power has been updated
	tvp := valSet.totalVotingPower
	valSet.updateTotalVotingPower()
	expectedTvp := valSet.TotalVotingPower()
	assert.Equal(t, expectedTvp, tvp,
		"expected TVP %d. Got %d, valSet=%s", expectedTvp, tvp, valSet)
	// verify that validator priorities are centered
	valsCount := int64(len(valSet.validators))
	tpp := valSetTotalProposerPriority(valSet)
	assert.True(t, tpp < valsCount && tpp > -valsCount,
		"expected total priority in (-%d, %d). Got %d", valsCount, valsCount, tpp)
	// verify that priorities are scaled
	dist := computeMaxMinPriorityDiff(valSet)
	assert.True(t, dist <= PriorityWindowSizeFactor*tvp,
		"expected priority distance < %d. Got %d", PriorityWindowSizeFactor*tvp, dist)
}

func valSetTotalProposerPriority(valSet *validatorSet) int64 {
	sum := int64(0)
	for _, val := range valSet.validators {
		// mind overflow
		sum = safeAddClip(sum, val.ProposerPriority)
	}
	return sum
}

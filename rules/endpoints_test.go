// Copyright (c) 2017-2018 Tigera, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rules_test

import (
	"strings"

	. "github.com/kubeovn/felix/rules"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubeovn/felix/ipsets"
	. "github.com/kubeovn/felix/iptables"
)

var _ = Describe("Endpoints", func() {
	const (
		ProtoUDP  = 17
		ProtoIPIP = 4
	)

	for _, trueOrFalse := range []bool{true, false} {
		kubeIPVSEnabled := trueOrFalse
		var rrConfigNormalMangleReturn = Config{
			IPIPEnabled:                 true,
			IPIPTunnelAddress:           nil,
			IPSetConfigV4:               ipsets.NewIPVersionConfig(ipsets.IPFamilyV4, "cali", nil, nil),
			IPSetConfigV6:               ipsets.NewIPVersionConfig(ipsets.IPFamilyV6, "cali", nil, nil),
			IptablesMarkAccept:          0x8,
			IptablesMarkPass:            0x10,
			IptablesMarkScratch0:        0x20,
			IptablesMarkScratch1:        0x40,
			IptablesMarkEndpoint:        0xff00,
			IptablesMarkNonCaliEndpoint: 0x0100,
			KubeIPVSSupportEnabled:      kubeIPVSEnabled,
			IptablesMangleAllowAction:   "RETURN",
		}

		var rrConfigConntrackDisabledReturnAction = Config{
			IPIPEnabled:                 true,
			IPIPTunnelAddress:           nil,
			IPSetConfigV4:               ipsets.NewIPVersionConfig(ipsets.IPFamilyV4, "cali", nil, nil),
			IPSetConfigV6:               ipsets.NewIPVersionConfig(ipsets.IPFamilyV6, "cali", nil, nil),
			IptablesMarkAccept:          0x8,
			IptablesMarkPass:            0x10,
			IptablesMarkScratch0:        0x20,
			IptablesMarkScratch1:        0x40,
			IptablesMarkEndpoint:        0xff00,
			IptablesMarkNonCaliEndpoint: 0x0100,
			KubeIPVSSupportEnabled:      kubeIPVSEnabled,
			DisableConntrackInvalid:     true,
			IptablesFilterAllowAction:   "RETURN",
		}

		var renderer RuleRenderer
		var epMarkMapper EndpointMarkMapper

		dropIPIPRule := Rule{
			Match:   Match().ProtocolNum(ProtoIPIP),
			Action:  DropAction{},
			Comment: "Drop IPinIP encapped packets originating in pods",
		}

		Context("with normal config", func() {
			BeforeEach(func() {
				renderer = NewRenderer(rrConfigNormalMangleReturn)
				epMarkMapper = NewEndpointMarkMapper(rrConfigNormalMangleReturn.IptablesMarkEndpoint,
					rrConfigNormalMangleReturn.IptablesMarkNonCaliEndpoint)
			})

			It("should render a minimal workload endpoint", func() {
				Expect(renderer.WorkloadEndpointToIptablesChains(
					"cali1234", epMarkMapper,
					true,
					nil,
					nil,
					nil)).To(Equal(trimSMChain(kubeIPVSEnabled, []*Chain{
					{
						Name: "cali-tw-cali1234",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},
							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-fw-cali1234",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},
							dropIPIPRule,
							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-sm-cali1234",
						Rules: []Rule{
							{Action: SetMaskedMarkAction{Mark: 0xd400, Mask: 0xff00}},
						},
					},
				})))
			})

			It("should render a disabled workload endpoint", func() {
				Expect(renderer.WorkloadEndpointToIptablesChains(
					"cali1234", epMarkMapper,
					false,
					nil,
					nil,
					nil,
				)).To(Equal(trimSMChain(kubeIPVSEnabled, []*Chain{
					{
						Name: "cali-tw-cali1234",
						Rules: []Rule{
							{Action: DropAction{},
								Comment: "Endpoint admin disabled"},
						},
					},
					{
						Name: "cali-fw-cali1234",
						Rules: []Rule{
							{Action: DropAction{},
								Comment: "Endpoint admin disabled"},
						},
					},
					{
						Name: "cali-sm-cali1234",
						Rules: []Rule{
							{Action: SetMaskedMarkAction{Mark: 0xd400, Mask: 0xff00}},
						},
					},
				})))
			})

			It("should render a fully-loaded workload endpoint", func() {
				Expect(renderer.WorkloadEndpointToIptablesChains(
					"cali1234",
					epMarkMapper,
					true,
					[]string{"ai", "bi"},
					[]string{"ae", "be"},
					[]string{"prof1", "prof2"},
				)).To(Equal(trimSMChain(kubeIPVSEnabled, []*Chain{
					{
						Name: "cali-tw-cali1234",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-ai"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-bi"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action:  DropAction{},
								Comment: "Drop if no policies passed packet"},

							{Action: JumpAction{Target: "cali-pri-prof1"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},
							{Action: JumpAction{Target: "cali-pri-prof2"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},

							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-fw-cali1234",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},
							dropIPIPRule,

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-ae"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-be"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action:  DropAction{},
								Comment: "Drop if no policies passed packet"},

							{Action: JumpAction{Target: "cali-pro-prof1"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},
							{Action: JumpAction{Target: "cali-pro-prof2"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},

							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-sm-cali1234",
						Rules: []Rule{
							{Action: SetMaskedMarkAction{Mark: 0xd400, Mask: 0xff00}},
						},
					},
				})))
			})

			It("should render a host endpoint", func() {
				Expect(renderer.HostEndpointToFilterChains("eth0",
					epMarkMapper,
					[]string{"ai", "bi"}, []string{"ae", "be"},
					[]string{"afi", "bfi"}, []string{"afe", "bfe"},
					[]string{"prof1", "prof2"})).To(Equal(trimSMChain(kubeIPVSEnabled, []*Chain{
					{
						Name: "cali-th-eth0",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							// Host endpoints get extra failsafe rules.
							{Action: JumpAction{Target: "cali-failsafe-out"}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-ae"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-be"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action:  DropAction{},
								Comment: "Drop if no policies passed packet"},

							{Action: JumpAction{Target: "cali-pro-prof1"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},
							{Action: JumpAction{Target: "cali-pro-prof2"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},

							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-fh-eth0",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							// Host endpoints get extra failsafe rules.
							{Action: JumpAction{Target: "cali-failsafe-in"}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-ai"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-bi"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action:  DropAction{},
								Comment: "Drop if no policies passed packet"},

							{Action: JumpAction{Target: "cali-pri-prof1"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},
							{Action: JumpAction{Target: "cali-pri-prof2"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if profile accepted"},

							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-thfw-eth0",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-afe"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-bfe"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action:  DropAction{},
								Comment: "Drop if no policies passed packet"},
						},
					},
					{
						Name: "cali-fhfw-eth0",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-afi"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-bfi"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},
							{Match: Match().MarkClear(0x10),
								Action:  DropAction{},
								Comment: "Drop if no policies passed packet"},
						},
					},
					{
						Name: "cali-sm-eth0",
						Rules: []Rule{
							{Action: SetMaskedMarkAction{Mark: 0xa200, Mask: 0xff00}},
						},
					},
				})))
			})

			It("should render host endpoint raw chains with untracked policies", func() {
				Expect(renderer.HostEndpointToRawChains("eth0", []string{"c"}, []string{"c"})).To(Equal([]*Chain{
					{
						Name: "cali-th-eth0",
						Rules: []Rule{
							// Host endpoints get extra failsafe rules.
							{Action: JumpAction{Target: "cali-failsafe-out"}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-po-c"}},
							// Extra NOTRACK action before returning in raw table.
							{Match: Match().MarkSingleBitSet(0x8),
								Action: NoTrackAction{}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},

							// No drop actions or profiles in raw table.
						},
					},
					{
						Name: "cali-fh-eth0",
						Rules: []Rule{
							// Host endpoints get extra failsafe rules.
							{Action: JumpAction{Target: "cali-failsafe-in"}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-c"}},
							// Extra NOTRACK action before returning in raw table.
							{Match: Match().MarkSingleBitSet(0x8),
								Action: NoTrackAction{}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},

							// No drop actions or profiles in raw table.
						},
					},
				}))
			})

			It("should render host endpoint mangle chains with pre-DNAT policies", func() {
				Expect(renderer.HostEndpointToMangleChains(
					"eth0",
					[]string{"c"},
				)).To(Equal([]*Chain{
					{
						Name: "cali-fh-eth0",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: SetMarkAction{Mark: 0x8}},
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: ReturnAction{}},
							{Match: Match().ConntrackState("INVALID"),
								Action: DropAction{}},

							// Host endpoints get extra failsafe rules.
							{Action: JumpAction{Target: "cali-failsafe-in"}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-c"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},

							// No drop actions or profiles in raw table.
						},
					},
				}))
			})
		})

		Describe("with ctstate=INVALID disabled", func() {
			BeforeEach(func() {
				renderer = NewRenderer(rrConfigConntrackDisabledReturnAction)
				epMarkMapper = NewEndpointMarkMapper(rrConfigConntrackDisabledReturnAction.IptablesMarkEndpoint,
					rrConfigConntrackDisabledReturnAction.IptablesMarkNonCaliEndpoint)
			})

			It("should render a minimal workload endpoint", func() {
				Expect(renderer.WorkloadEndpointToIptablesChains(
					"cali1234",
					epMarkMapper,
					true,
					nil,
					nil,
					nil,
				)).To(Equal(trimSMChain(kubeIPVSEnabled, []*Chain{
					{
						Name: "cali-tw-cali1234",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: SetMarkAction{Mark: 0x8}},
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: ReturnAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-fw-cali1234",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: SetMarkAction{Mark: 0x8}},
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: ReturnAction{}},

							{Action: ClearMarkAction{Mark: 0x8}},
							dropIPIPRule,

							{Action: DropAction{},
								Comment: "Drop if no profiles matched"},
						},
					},
					{
						Name: "cali-sm-cali1234",
						Rules: []Rule{
							{Action: SetMaskedMarkAction{Mark: 0xd400, Mask: 0xff00}},
						},
					},
				})))
			})

			It("should render host endpoint mangle chains with pre-DNAT policies", func() {
				Expect(renderer.HostEndpointToMangleChains(
					"eth0",
					[]string{"c"},
				)).To(Equal([]*Chain{
					{
						Name: "cali-fh-eth0",
						Rules: []Rule{
							// conntrack rules.
							{Match: Match().ConntrackState("RELATED,ESTABLISHED"),
								Action: AcceptAction{}},

							// Host endpoints get extra failsafe rules.
							{Action: JumpAction{Target: "cali-failsafe-in"}},

							{Action: ClearMarkAction{Mark: 0x8}},

							{Comment: "Start of policies",
								Action: ClearMarkAction{Mark: 0x10}},
							{Match: Match().MarkClear(0x10),
								Action: JumpAction{Target: "cali-pi-c"}},
							{Match: Match().MarkSingleBitSet(0x8),
								Action:  ReturnAction{},
								Comment: "Return if policy accepted"},

							// No drop actions or profiles in raw table.
						},
					},
				}))
			})
		})
	}
})

func trimSMChain(ipvsEnable bool, chains []*Chain) []*Chain {
	result := []*Chain{}
	for _, chain := range chains {
		if !ipvsEnable && strings.HasPrefix(chain.Name, "cali-sm") {
			continue
		}
		result = append(result, chain)
	}

	return result
}

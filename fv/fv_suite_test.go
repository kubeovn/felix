// +build fvtests

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

package fv_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"github.com/kubeovn/felix/fv/infrastructure"
	"github.com/kubeovn/felix/fv/workload"

	"github.com/projectcalico/libcalico-go/lib/testutils"
)

func init() {
	testutils.HookLogrusForGinkgo()
}

func TestFv(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("../report/fv_suite.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "FV Suite", []Reporter{junitReporter})
}

var _ = AfterEach(func() {
	defer workload.UnactivatedConnectivityCheckers.Clear()
	if CurrentGinkgoTestDescription().Failed {
		// If the test has already failed, ignore any connectivity checker leak.
		return
	}
	Expect(workload.UnactivatedConnectivityCheckers.Len()).To(BeZero(),
		"Test bug: ConnectivityChecker was created but not activated.")
})

var _ = AfterSuite(func() {
	if infrastructure.K8sInfra != nil {
		infrastructure.TearDownK8sInfra(infrastructure.K8sInfra)
		infrastructure.K8sInfra = nil
	}
	infrastructure.RemoveTLSCredentials()
})

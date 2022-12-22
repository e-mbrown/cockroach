// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package fmtsafe_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/build/bazel"
	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/fmtsafe"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"golang.org/x/tools/go/analysis/analysistest"
)

func init() {
	if bazel.BuiltWithBazel() {
		bazel.SetGoEnv()
	}
}

func Test(t *testing.T) {
	skip.UnderStress(t)
	fmtsafe.Tip = ""
	testdata := datapathutils.TestDataPath(t)
	analysistest.TestData = func() string { return testdata }
	results := analysistest.Run(t, testdata, fmtsafe.Analyzer, "a")
	for _, r := range results {
		for _, d := range r.Diagnostics {
			t.Logf("%s: %v: %s", r.Pass.Analyzer.Name, d.Pos, d.Message)
		}
	}
}

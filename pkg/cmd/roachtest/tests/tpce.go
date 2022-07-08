// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/cluster"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/option"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/registry"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/spec"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/test"
	"github.com/cockroachdb/cockroach/pkg/roachprod/install"
	"github.com/cockroachdb/errors"
)

func registerTPCE(r registry.Registry) {
	type tpceOptions struct {
		customers int
		nodes     int
		cpus      int
		ssds      int

		tags    []string
		timeout time.Duration
	}

	runTPCE := func(ctx context.Context, t test.Test, c cluster.Cluster, opts tpceOptions) {
		roachNodes := c.Range(1, opts.nodes)
		loadNode := c.Node(opts.nodes + 1)
		racks := opts.nodes

		t.Status("installing cockroach")
		c.Put(ctx, t.Cockroach(), "./cockroach", roachNodes)

		startOpts := option.DefaultStartOpts()
		startOpts.RoachprodOpts.StoreCount = opts.ssds
		settings := install.MakeClusterSettings(install.NumRacksOption(racks))
		c.Start(ctx, t.L(), startOpts, settings, roachNodes)

		t.Status("installing docker")
		if err := c.Install(ctx, t.L(), loadNode, "docker"); err != nil {
			t.Fatal(err)
		}

		// Configure to increase the speed of the import.
		func() {
			db := c.Conn(ctx, t.L(), 1)
			defer db.Close()
			if _, err := db.ExecContext(
				ctx, "SET CLUSTER SETTING kv.bulk_io_write.concurrent_addsstable_requests = $1", 4*opts.ssds,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(
				ctx, "SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false",
			); err != nil {
				t.Fatal(err)
			}
		}()

		m := c.NewMonitor(ctx, roachNodes)
		m.Go(func(ctx context.Context) error {
			const dockerRun = `sudo docker run cockroachdb/tpc-e:latest`

			roachNodeIPs, err := c.InternalIP(ctx, t.L(), roachNodes)
			if err != nil {
				return err
			}
			roachNodeIPFlags := make([]string, len(roachNodeIPs))
			for i, ip := range roachNodeIPs {
				roachNodeIPFlags[i] = fmt.Sprintf("--hosts=%s", ip)
			}

			t.Status("preparing workload")
			c.Run(ctx, loadNode, fmt.Sprintf("%s --customers=%d --racks=%d --init %s",
				dockerRun, opts.customers, racks, roachNodeIPFlags[0]))

			t.Status("running workload")
			duration := 2 * time.Hour
			threads := opts.nodes * opts.cpus
			result, err := c.RunWithDetailsSingleNode(ctx, t.L(), loadNode,
				fmt.Sprintf("%s --customers=%d --racks=%d --duration=%s --threads=%d %s",
					dockerRun, opts.customers, racks, duration, threads, strings.Join(roachNodeIPFlags, " ")))
			if err != nil {
				t.Fatal(err.Error())
			}
			t.L().Printf("workload output:\n%s\n", result.Stdout)
			if strings.Contains(result.Stdout, "Reported tpsE :    --   (not between 80% and 100%)") {
				return errors.New("invalid tpsE fraction")
			}
			return nil
		})
		m.Wait()
	}

	for _, opts := range []tpceOptions{
		// Nightly, small scale configurations.
		{customers: 5_000, nodes: 3, cpus: 4, ssds: 1},
		// Weekly, large scale configurations.
		{customers: 100_000, nodes: 5, cpus: 32, ssds: 2, tags: []string{"weekly"}, timeout: 36 * time.Hour},
	} {
		opts := opts

		r.Add(registry.TestSpec{
			Name:    fmt.Sprintf("tpce/c=%d/nodes=%d", opts.customers, opts.nodes),
			Owner:   registry.OwnerKV,
			Tags:    opts.tags,
			Timeout: opts.timeout,
			Cluster: r.MakeClusterSpec(opts.nodes+1, spec.CPU(opts.cpus), spec.SSD(opts.ssds)),
			Run: func(ctx context.Context, t test.Test, c cluster.Cluster) {
				runTPCE(ctx, t, c, opts)
			},
		})
	}
}

func registerMultiTPCE(r registry.Registry) {
	type tpceOptions struct {
		customers int
		nodes     int
		cpus      int
		ssds      int

		tags    []string
		timeout time.Duration
	}

	runMultiTPCE := func(ctx context.Context, t test.Test, c1 cluster.Cluster, c2 cluster.Cluster, opts tpceOptions) {
		roachNodes1 := c1.Range(1, opts.nodes)
		loadNode1 := c1.Node(opts.nodes + 1)
		roachNodes2 := c1.Range(1, opts.nodes)
		loadNode2 := c2.Node(opts.nodes + 1)
		racks := opts.nodes

		t.Status("installing cockroach cluster 1")
		c1.Put(ctx, t.Cockroach(), "./cockroach", roachNodes1)

		t.Status("installing cockroach cluster 2")
		c2.Put(ctx, t.Cockroach(), "./cockroach", roachNodes2)

		startOpts := option.DefaultStartOpts()
		startOpts.RoachprodOpts.StoreCount = opts.ssds
		settings := install.MakeClusterSettings(install.NumRacksOption(racks))
		c1.Start(ctx, t.L(), startOpts, settings, roachNodes1)
		c2.Start(ctx, t.L(), startOpts, settings, roachNodes1)

		t.Status("installing docker cluster 1")
		if err := c1.Install(ctx, t.L(), loadNode1, "docker"); err != nil {
			t.Fatal(err)
		}

		t.Status("installing docker cluster 2")
		if err := c2.Install(ctx, t.L(), loadNode2, "docker"); err != nil {
			t.Fatal(err)

			// Configure to increase the speed of the import.
			func() {
				db := c1.Conn(ctx, t.L(), 1)
				defer db.Close()
				if _, err := db.ExecContext(
					ctx, "SET CLUSTER SETTING kv.bulk_io_write.concurrent_addsstable_requests = $1", 4*opts.ssds,
				); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(
					ctx, "SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false",
				); err != nil {
					t.Fatal(err)
				}
			}()

			// Configure to increase the speed of the import.
			func() {
				db := c2.Conn(ctx, t.L(), 1)
				defer db.Close()
				if _, err := db.ExecContext(
					ctx, "SET CLUSTER SETTING kv.bulk_io_write.concurrent_addsstable_requests = $1", 4*opts.ssds,
				); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(
					ctx, "SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false",
				); err != nil {
					t.Fatal(err)
				}
			}()
		}

		m1 := c1.NewMonitor(ctx, roachNodes1)
		m2 := c2.NewMonitor(ctx, roachNodes2)

		m1.Go(func(ctx context.Context) error {
			const dockerRun = `sudo docker run cockroachdb/tpc-e:latest`

			roachNodeIPs, err := c1.InternalIP(ctx, t.L(), roachNodes1)
			if err != nil {
				return err
			}
			roachNodeIPFlags := make([]string, len(roachNodeIPs))
			for i, ip := range roachNodeIPs {
				roachNodeIPFlags[i] = fmt.Sprintf("--hosts=%s", ip)
			}

			t.Status("preparing workload")
			c1.Run(ctx, loadNode1, fmt.Sprintf("%s --customers=%d --racks=%d --init %s",
				dockerRun, opts.customers, racks, roachNodeIPFlags[0]))

			t.Status("running workload")
			duration := 2 * time.Hour
			threads := opts.nodes * opts.cpus
			result, err := c1.RunWithDetailsSingleNode(ctx, t.L(), loadNode1,
				fmt.Sprintf("%s --customers=%d --racks=%d --duration=%s --threads=%d %s",
					dockerRun, opts.customers, racks, duration, threads, strings.Join(roachNodeIPFlags, " ")))
			if err != nil {
				t.Fatal(err.Error())
			}
			t.L().Printf("workload output:\n%s\n", result.Stdout)
			if strings.Contains(result.Stdout, "Reported tpsE :    --   (not between 80% and 100%)") {
				return errors.New("invalid tpsE fraction")
			}
			return nil
		})
		m1.Wait()

		m2.Go(func(ctx context.Context) error {
			const dockerRun = `sudo docker run cockroachdb/tpc-e:latest`

			roachNodeIPs, err := c2.InternalIP(ctx, t.L(), roachNodes2)
			if err != nil {
				return err
			}
			roachNodeIPFlags := make([]string, len(roachNodeIPs))
			for i, ip := range roachNodeIPs {
				roachNodeIPFlags[i] = fmt.Sprintf("--hosts=%s", ip)
			}

			t.Status("preparing workload")
			c2.Run(ctx, loadNode2, fmt.Sprintf("%s --customers=%d --racks=%d --init %s",
				dockerRun, opts.customers, racks, roachNodeIPFlags[0]))

			t.Status("running workload")
			duration := 2 * time.Hour
			threads := opts.nodes * opts.cpus
			result, err := c2.RunWithDetailsSingleNode(ctx, t.L(), loadNode2,
				fmt.Sprintf("%s --customers=%d --racks=%d --duration=%s --threads=%d %s",
					dockerRun, opts.customers, racks, duration, threads, strings.Join(roachNodeIPFlags, " ")))
			if err != nil {
				t.Fatal(err.Error())
			}
			t.L().Printf("workload output:\n%s\n", result.Stdout)
			if strings.Contains(result.Stdout, "Reported tpsE :    --   (not between 80% and 100%)") {
				return errors.New("invalid tpsE fraction")
			}
			return nil
		})
		m2.Wait()
	}

	for _, opts := range []tpceOptions{
		// Nightly, small scale configurations.
		{customers: 5_000, nodes: 3, cpus: 4, ssds: 1},
		// Weekly, large scale configurations.
		//{customers: 100_000, nodes: 5, cpus: 32, ssds: 2, tags: []string{"weekly"}, timeout: 36 * time.Hour},
	} {
		opts := opts
		clusters := []spec.ClusterSpec{
			r.MakeClusterSpec(opts.nodes+1, spec.CPU(opts.cpus), spec.SSD(opts.ssds), spec.ReuseNone()),
			r.MakeClusterSpec(opts.nodes+1, spec.CPU(opts.cpus), spec.SSD(opts.ssds), spec.ReuseNone()),
		}
		r.Add(registry.TestSpec{
			Name:         fmt.Sprintf("tpce/c=%d/nodes=%d/multicluster", opts.customers, opts.nodes),
			Owner:        registry.OwnerTestEng,
			Tags:         opts.tags,
			Timeout:      opts.timeout,
			MultiCluster: clusters,
			RunMulti: func(ctx context.Context, t test.Test, c cluster.Cluster, c2 cluster.Cluster) {
				runMultiTPCE(ctx, t, c, c2, opts)
			},
		})
	}
}

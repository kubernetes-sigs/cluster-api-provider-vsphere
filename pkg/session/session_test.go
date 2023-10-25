/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

func TestGetSession(t *testing.T) {
	g := NewWithT(t)
	ctrl.SetLogger(klog.Background())

	model := simulator.VPX()
	model.Cluster = 2

	simr, err := vcsim.NewBuilder().
		WithModel(model).Build()
	if err != nil {
		t.Fatalf("failed to create VC simulator")
	}
	defer simr.Destroy()

	params := NewParams().
		WithServer(simr.ServerURL().Host).
		WithUserInfo(simr.Username(), simr.Password()).WithDatacenter("*")

	// Get first session
	ctx := context.Background()
	s, err := GetOrCreate(ctx, params)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(s).ToNot(BeNil())
	assertSessionCountEqualTo(g, simr, 1)

	// Get session key
	sessionInfo, err := s.SessionManager.UserSession(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sessionInfo).ToNot(BeNil())
	firstSession := sessionInfo.Key

	// remove session expect no session
	g.Expect(s.TagManager.Logout(ctx)).To(Succeed())
	g.Expect(simr.Run(fmt.Sprintf("session.rm %s", firstSession))).To(Succeed())
	assertSessionCountEqualTo(g, simr, 0)

	// request session again should be a new and different session
	s, err = GetOrCreate(ctx, params)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(s).ToNot(BeNil())

	// Get session info, session key should be different from
	// last session
	sessionInfo, err = s.SessionManager.UserSession(ctx)
	g.Expect(sessionInfo).ToNot(BeNil())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sessionInfo.Key).ToNot(BeEquivalentTo(firstSession))
	assertSessionCountEqualTo(g, simr, 1)
}

func sessionCount(stdout io.Reader) (int, error) {
	buf := make([]byte, 1024)
	count := 0
	lineSep := []byte(infrav1.GroupVersion.String())

	for {
		c, err := stdout.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func assertSessionCountEqualTo(g *WithT, simr *vcsim.Simulator, count int) {
	g.Eventually(func() bool {
		stdout := gbytes.NewBuffer()
		g.Expect(simr.Run("session.ls", stdout)).To(Succeed())
		sessions, err := sessionCount(stdout)
		g.Expect(err).ToNot(HaveOccurred())
		return sessions == count
	}, 30*time.Second).Should(BeTrue())
}

func TestGetSessionWithKeepAlive(t *testing.T) {
	g := NewWithT(t)
	ctrl.SetLogger(klog.Background())

	model := simulator.VPX()
	model.Cluster = 2

	simr, err := vcsim.NewBuilder().
		WithModel(model).Build()
	if err != nil {
		t.Fatalf("failed to create VC simulator")
	}
	defer simr.Destroy()

	params := NewParams().
		WithServer(simr.ServerURL().Host).
		WithUserInfo(simr.Username(), simr.Password()).
		WithDatacenter("*")

	// Get first Session
	ctx := context.Background()
	s, err := GetOrCreate(ctx, params)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(s).ToNot(BeNil())
	assertSessionCountEqualTo(g, simr, 1)

	// Get session key
	sessionInfo, err := s.SessionManager.UserSession(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sessionInfo).ToNot(BeNil())
	firstSession := sessionInfo.Key

	// Get the session again
	// as keep alive is enabled and session is
	// not expired we must get the same cached session
	s, err = GetOrCreate(ctx, params)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(s).ToNot(BeNil())
	sessionInfo, err = s.SessionManager.UserSession(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sessionInfo).ToNot(BeNil())
	g.Expect(sessionInfo.Key).To(BeEquivalentTo(firstSession))
	assertSessionCountEqualTo(g, simr, 1)

	// Try to remove vim session
	g.Expect(s.Logout(ctx)).To(Succeed())

	// after logging out old session must be deleted,
	// we must get a new different session
	// total session count must remain 1
	s, err = GetOrCreate(ctx, params)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(s).ToNot(BeNil())
	sessionInfo, err = s.SessionManager.UserSession(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sessionInfo).ToNot(BeNil())
	g.Expect(sessionInfo.Key).ToNot(BeEquivalentTo(firstSession))
	assertSessionCountEqualTo(g, simr, 1)
}
